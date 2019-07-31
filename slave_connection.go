package binlog

import (
	"context"
	"fmt"

	"github.com/onlyac0611/binlog/dump"
	"github.com/onlyac0611/binlog/event"
	"github.com/onlyac0611/binlog/meta"
)

// slaveConn modify based on github.com/youtube/vitess/go/vt/mysqlctl/slave_connection.go
// slaveConn通过StartDumpFromBinlogPosition和mysql库进行binlog dump，将自己伪装成slave，
// 先执行SET @master_binlog_checksum=@@global.binlog_checksum，然后发送 binlog dump包，
// 最后获取binlog日志，通过chan将binlog日志通过binlog event的格式传出。
type slaveConn struct {
	mc     *dump.MysqlConn
	cancel context.CancelFunc
}

func newSlaveConn(dsn string) (*slaveConn, error) {
	m, err := dump.NewMysqlConn(dsn)
	if err != nil {
		return nil, err
	}

	s := &slaveConn{
		mc: m,
	}

	if err := s.prepareForReplication(); err != nil {
		s.mc.Close()
		return nil, err
	}

	return s, nil
}

func (s *slaveConn) close() {
	if s.mc != nil {
		s.mc.Close()
		logger.Infof("Close closing slave socket to unblock reads")
		s.mc = nil
	}
}

func (s *slaveConn) prepareForReplication() error {
	if err := s.mc.Exec("SET @master_binlog_checksum=@@global.binlog_checksum"); err != nil {
		return fmt.Errorf("prepareForReplication failed to set @master_binlog_checksum=@@global.binlog_checksum: %v", err)
	}
	return nil
}

func (s *slaveConn) startDumpFromBinlogPosition(ctx context.Context, serverID uint32, pos meta.BinlogPosition) (<-chan event.BinlogEvent, error) {
	ctx, s.cancel = context.WithCancel(ctx)

	logger.Infof("startDumpFromBinlogPosition sending binlog dump command: startPos: %v slaveID: %v", pos, serverID)
	if err := s.mc.NoticeDump(serverID, uint32(pos.Offset), pos.FileName, 0); err != nil {
		logger.Errorf("startDumpFromBinlogPosition couldn't send binlog dump command err: %v", err)
		return nil, err
	}

	buf, err := s.mc.ReadPacket()
	if err != nil {
		logger.Errorf("startDumpFromBinlogPosition couldn't start binlog dump: %v", err)
		return nil, err
	}

	// FIXME(xd.fang) I think we can use a buffered channel for better performance.
	eventChan := make(chan event.BinlogEvent)

	go func() {
		defer func() {
			close(eventChan)
			s.close()
			logger.Infof("startDumpFromBinlogPosition close slave dump thread to end")
		}()

		for {
			switch buf[0] {
			case dump.PacketEOF:
				logger.Infof("startDumpFromBinlogPosition received EOF packet in binlog dump: %#v", buf)
				return
			case dump.PacketERR:
				err := s.mc.HandleErrorPacket(buf)
				logger.Infof("startDumpFromBinlogPosition received error packet in binlog dump: %v", err)
				return
			}

			select {
			case eventChan <- event.NewMysql56BinlogEvent(buf[1:]):
			case <-ctx.Done():
				return
			}

			buf, err = s.mc.ReadPacket()
			if err != nil {
				logger.Errorf("startDumpFromBinlogPosition couldn't start binlog dump: %v", err)
				return
			}
		}
	}()

	return eventChan, nil
}
