package binlog

import (
	"context"
	"fmt"
	"sync"

	"github.com/onlyac0611/binlog/dump"
	"github.com/onlyac0611/binlog/event"
	"github.com/onlyac0611/binlog/meta"
)

type dumpConn interface {
	Close() error
	Exec(string) error
	NoticeDump(uint32, uint32, string, uint16) error
	ReadPacket() ([]byte, error)
	HandleErrorPacket([]byte) error
}

// slaveConn modify based on github.com/youtube/vitess/go/vt/mysqlctl/slave_connection.go
// slaveConn通过StartDumpFromBinlogPosition和mysql库进行binlog dump，将自己伪装成slave，
// 先执行SET @master_binlog_checksum=@@global.binlog_checksum，然后发送 binlog dump包，
// 最后获取binlog日志，通过chan将binlog日志通过binlog event的格式传出。
type slaveConn struct {
	dc          dumpConn
	cancel      context.CancelFunc
	destruction sync.Once
}

func newSlaveConn(conn func() (dumpConn, error)) (*slaveConn, error) {
	m, err := conn()
	if err != nil {
		return nil, err
	}

	s := &slaveConn{
		dc: m,
	}

	if err := s.prepareForReplication(); err != nil {
		s.close()
		return nil, err
	}

	return s, nil
}

func (s *slaveConn) close() {
	s.destruction.Do(
		func() {
			if s.dc != nil {
				s.dc.Close()
				lw.logger().Infof("Close closing slave socket to unblock reads")
			}
		})
}

func (s *slaveConn) prepareForReplication() error {
	if err := s.dc.Exec("SET @master_binlog_checksum=@@global.binlog_checksum"); err != nil {
		return fmt.Errorf("prepareForReplication failed to set @master_binlog_checksum=@@global.binlog_checksum: %v",
			err)
	}
	return nil
}

func (s *slaveConn) startDumpFromBinlogPosition(ctx context.Context, serverID uint32,
	pos meta.BinlogPosition) (<-chan event.BinlogEvent, error) {
	ctx, s.cancel = context.WithCancel(ctx)

	lw.logger().Infof("startDumpFromBinlogPosition sending binlog dump command: startPos: %+v slaveID: %v", pos, serverID)
	if err := s.dc.NoticeDump(serverID, uint32(pos.Offset), pos.FileName, 0); err != nil {
		return nil, fmt.Errorf("noticeDump fail. err: %v", err)
	}

	buf, err := s.dc.ReadPacket()
	if err != nil {
		return nil, fmt.Errorf("readPacket fail. err: %v", err)
	}

	// FIXME(xd.fang) I think we can use a buffered channel for better performance.
	eventChan := make(chan event.BinlogEvent)

	go func() {
		defer close(eventChan)

		for {
			switch buf[0] {
			case dump.PacketEOF:
				lw.logger().Infof("startDumpFromBinlogPosition received EOF packet in binlog dump: %+v", buf)
				return
			case dump.PacketERR:
				err := s.dc.HandleErrorPacket(buf)
				lw.logger().Errorf("startDumpFromBinlogPosition received error packet in binlog dump. error: %v", err)
				return
			}

			select {
			case eventChan <- event.NewMysql56BinlogEvent(buf[1:]):
			case <-ctx.Done():
				lw.logger().Infof("startDumpFromBinlogPosition stop by ctx. reason: %v", ctx.Err())
				return
			}

			buf, err = s.dc.ReadPacket()
			if err != nil {
				lw.logger().Errorf("startDumpFromBinlogPosition ReadPacket fail. error: %v", err)
				return
			}
		}
	}()

	return eventChan, nil
}
