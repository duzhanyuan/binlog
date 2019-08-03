//Package binlog 将自己伪装成slave获取mysql主从复杂流来获取mysql数据库的数据变更，提供轻量级，快速的dump协议交互以及binlog的row模式下的格式解析
package binlog

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/onlyac0611/binlog/dump"
	"github.com/onlyac0611/binlog/event"
	"github.com/onlyac0611/binlog/meta"
)

//信息流到达EOF错误信息用于标识binlog流结束
var (
	ErrStreamEOF = errors.New("stream reached EOF") //信息流到达EOF
)

//MysqlTableMapper 用于获取表信息的接口
type MysqlTableMapper interface {
	MysqlTable(name meta.MysqlTableName) (meta.MysqlTable, error)
}

//RowStreamer modify based on github.com/youtube/vitess/go/vt/binlog/binlog_streamer.go
//专门用来RowStreamer解析row模式的binlog event，将其变为对应的事务
type RowStreamer struct {
	dsn             string
	serverID        uint32
	startPos        atomic.Value
	tableMapper     MysqlTableMapper
	sendTransaction SendTransactionFunc
}

//SendTransactionFunc 处理事务信息函数，你可以将一个chan注册到这个函数中如
//   func getTransaction(tran *meta.Transaction) error{
//	     Transactions <- tran
//	     return nil
//   }
type SendTransactionFunc func(*meta.Transaction) error

type tableCache struct {
	tableMap *event.TableMap
	table    meta.MysqlTable
}

//NewRowStreamer dsn是mysql数据库的信息，serverID是标识该数据库的信息
func NewRowStreamer(dsn string, serverID uint32,
	tableMapper MysqlTableMapper) (*RowStreamer, error) {
	return &RowStreamer{
		dsn:         dsn,
		serverID:    serverID,
		tableMapper: tableMapper,
	}, nil
}

//SetStartBinlogPosition 设置开始的binlog位置
func (s *RowStreamer) SetStartBinlogPosition(startPos meta.BinlogPosition) {
	s.startPos.Store(startPos)
}

func (s *RowStreamer) startBinlogPosition() meta.BinlogPosition {
	return s.startPos.Load().(meta.BinlogPosition)
}

//Stream 注册一个处理事务信息函数到Stream中
func (s *RowStreamer) Stream(ctx context.Context, sendTransaction SendTransactionFunc) error {
	conn, err := newSlaveConn(func() (conn dumpConn, e error) {
		return dump.NewMysqlConn(s.dsn)
	})
	if err != nil {
		return fmt.Errorf("newMysqlConn fail. err: %v", err)
	}
	defer conn.close()
	s.sendTransaction = sendTransaction
	var events <-chan event.BinlogEvent
	var pos meta.BinlogPosition
	events, err = conn.startDumpFromBinlogPosition(ctx, s.serverID, s.startBinlogPosition())
	pos, err = s.parseEvents(ctx, events)
	if err != nil {
		return fmt.Errorf("parseEvents fail in pos: %+v error: %v", s.startPos, err)
	}
	s.SetStartBinlogPosition(pos)
	return nil
}

func (s *RowStreamer) parseEvents(ctx context.Context, events <-chan event.BinlogEvent) (meta.BinlogPosition, error) {
	var tranEvents []*meta.StreamEvent
	var format event.BinlogFormat
	var err error
	pos := s.startBinlogPosition()
	tablesMaps := make(map[uint64]*tableCache)
	autocommit := true

	begin := func() {
		if tranEvents != nil {
			// If this happened, it would be a legitimate error.
			logger.Errorf("parseEvents BEGIN in binlog stream while still in another transaction; dropping %d transactionEvents: %+v", len(tranEvents), tranEvents)
		}
		tranEvents = make([]*meta.StreamEvent, 0, 10)
		autocommit = false
	}

	commit := func(ev event.BinlogEvent) error {
		now := pos
		pos.Offset = ev.NextPosition()
		next := pos
		tran := meta.NewTransaction(now, next, int64(ev.Timestamp()), tranEvents)
		if err = s.sendTransaction(tran); err != nil {
			return fmt.Errorf("parseEvents sendTransaction error: %v", err)
		}
		tranEvents = nil
		autocommit = true
		return nil
	}

	for {
		var ev event.BinlogEvent
		var ok bool
		select {
		case ev, ok = <-events:
			if !ok {
				logger.Infof("parseEvents reached end of binlog event stream")
				return pos, ErrStreamEOF
			}
		case <-ctx.Done():
			logger.Infof("parseEvents stopping early due to binlog Streamer service shutdown or client disconnect")
			return pos, ctx.Err()
		}

		// Validate the buffer before reading fields from it.
		if !ev.IsValid() {
			return pos, fmt.Errorf("parseEvents can't parse binlog event, invalid data: %+v", ev)
		}

		// We need to keep checking for FORMAT_DESCRIPTION_EVENT even after we've
		// seen one, because another one might come along (e.g. on logger rotate due to
		// binlog settings change) that changes the format.
		if ev.IsFormatDescription() {
			format, err = ev.Format()
			if err != nil {
				return pos, fmt.Errorf("parseEvents can't parse FORMAT_DESCRIPTION_EVENT: %v, event data: %+v", err, ev)
			}
			logger.Debugf("parseEvents pos: %+v binlog event is a format description event:%+v", ev.NextPosition(), format)
			continue
		}

		// We can't parse anything until we get a FORMAT_DESCRIPTION_EVENT that
		// tells us the size of the event header.
		if format.IsZero() {
			// The only thing that should come before the FORMAT_DESCRIPTION_EVENT
			// is a fake ROTATE_EVENT, which the master sends to tell us the name
			// of the current logger file.
			if ev.IsRotate() {
				continue
			}
			return pos, fmt.Errorf("parseEvents got a real event before FORMAT_DESCRIPTION_EVENT: %+v", ev)
		}

		// Strip the checksum, if any. We don't actually verify the checksum, so discard it.
		ev, _, err = ev.StripChecksum(format)
		if err != nil {
			return pos, fmt.Errorf("parseEvents can't strip checksum from binlog event: %v, event data: %+v", err, ev)
		}

		switch {
		case ev.IsXID(): // XID_EVENT (equivalent to COMMIT)
			logger.Debugf("parseEvents pos: %+v binlog event is a xid event:", pos, ev)
			if err = commit(ev); err != nil {
				return pos, err
			}

		case ev.IsRotate():
			logger.Debugf("parseEvents pos: %+v binlog event is a xid event %+v:", pos, ev)
			var filename string
			var offset int64
			if filename, offset, err = ev.Rotate(format); err != nil {
				return pos, err
			}
			pos.FileName = filename
			pos.Offset = offset
		case ev.IsQuery():
			q, err := ev.Query(format)
			if err != nil {
				return pos, fmt.Errorf("parseEvents can't get query from binlog event: %v, event data: %+v", err, ev)
			}
			typ := meta.GetStatementCategory(q.SQL)

			logger.Debugf("parseEvents pos: %+v binlog event is a query event: %+v query: %v", pos, ev, q.SQL)

			switch typ {
			case meta.StatementBegin:
				begin()
			case meta.StatementRollback:
				tranEvents = nil
				fallthrough
			case meta.StatementCommit:
				if err = commit(ev); err != nil {
					return pos, err
				}
			default:
				logger.Errorf("parseEvents we have a sql in binlog position: %+v error: %v", pos,
					fmt.Errorf("parseEvents SQL query %s  statement in row binlog SQL: %s", typ.String(), q.SQL))
				//return pos, fmt.Errorf("parseEvents SQL query %s  statement in row binlog SQL: %s", typ.String(), q.SQL)
			}

		case ev.IsTableMap():
			tableID := ev.TableID(format)
			tm, err := ev.TableMap(format)

			if err != nil {
				return pos, err
			}
			logger.Debugf("parseEvents pos: %+v binlog event is a table map event, tableID: %v table map: %+v",
				pos, tableID, *tm)

			if _, ok = tablesMaps[tableID]; ok {
				tablesMaps[tableID].tableMap = tm
				continue
			}

			tc := &tableCache{
				tableMap: tm,
			}

			name := meta.NewMysqlTableName(tm.Database, tm.Name)

			var info meta.MysqlTable
			if info, err = s.tableMapper.MysqlTable(name); err != nil {
				return pos, fmt.Errorf("parseEvents MysqlTable fail. table: %v, err： %v", name.String(), err)
			}

			if len(info.Columns()) != tm.CanBeNull.Count() {
				return meta.BinlogPosition{},
					fmt.Errorf("parseEvents the length of column in tableMap(%d) "+
						"did not equal to the length of column in table info(%d)", tm.CanBeNull.Count(),
						len(info.Columns()))
			}
			tc.table = info
			tablesMaps[tableID] = tc

		case ev.IsWriteRows():
			tableID := ev.TableID(format)
			tc, ok := tablesMaps[tableID]
			if !ok {
				return pos, fmt.Errorf("parseEvents unknown tableID %v in WriteRows event", tableID)
			}
			logger.Debugf("parseEvents pos: %+v binlog event is a write rows event, tableID: %v tc.tableMap: %+v",
				pos, tableID, tc.tableMap)
			rows, err := ev.Rows(format, tc.tableMap)
			if err != nil {
				return pos, err
			}
			logger.Debugf("parseEvents pos: %+v binlog event is a write rows event, tableID: %v rows: %+v",
				pos, tableID, rows)

			tranEvent, err := appendInsertEventFromRows(tc, &rows, int64(ev.Timestamp()))
			if err != nil {
				return pos, err
			}

			tranEvents = append(tranEvents, tranEvent)
			if autocommit {
				if err = commit(ev); err != nil {
					return pos, err
				}
			}

		case ev.IsUpdateRows():
			tableID := ev.TableID(format)
			tc, ok := tablesMaps[tableID]
			if !ok {
				return pos, fmt.Errorf("parseEvents unknown tableID %v in UpdateRows event", tableID)
			}
			logger.Debugf("parseEvents pos: %+v binlog event is a update rows event, tableID: %v tc.tableMap: %+v",
				pos, tableID, tc.tableMap)
			rows, err := ev.Rows(format, tc.tableMap)
			if err != nil {
				return pos, err
			}

			logger.Debugf("parseEvents pos: %+v binlog event is a update rows event, tableID: %v rows: %+v",
				pos, tableID, rows)

			tranEvent, err := appendUpdateEventFromRows(tc, &rows, int64(ev.Timestamp()))
			if err != nil {
				return pos, err
			}
			tranEvents = append(tranEvents, tranEvent)
			if autocommit {
				if err = commit(ev); err != nil {
					return pos, err
				}
			}
		case ev.IsDeleteRows():
			tableID := ev.TableID(format)
			tc, ok := tablesMaps[tableID]
			if !ok {
				return pos, fmt.Errorf("parseEvents unknown tableID %v in DeleteRows event", tableID)
			}

			logger.Debugf("parseEvents pos: %+v binlog event is a delete rows event, tableID: %v tc.tableMap: %+v",
				pos, tableID, tc.tableMap)

			rows, err := ev.Rows(format, tc.tableMap)
			if err != nil {
				return pos, err
			}

			logger.Debugf("parseEvents pos: %+v", "binlog event is a delete rows event, tableID: %v rows: %+v",
				pos, tableID, rows)
			tranEvent, err := appendDeleteEventFromRows(tc, &rows, int64(ev.Timestamp()))
			if err != nil {
				return pos, err
			}

			tranEvents = append(tranEvents, tranEvent)
			if autocommit {
				if err = commit(ev); err != nil {
					return pos, err
				}
			}
		case ev.IsPreviousGTIDs():
			logger.Debugf("parseEvents pos: %+v binlog event is a PreviousGTIDs event: %+v", pos, ev)
		case ev.IsGTID():
			logger.Debugf("parseEvents pos: %+v binlog event is a GTID event: %+v", pos, ev)

		case ev.IsRand():
			//todo deal with the Rand error
			return pos, fmt.Errorf("binlog event is a Rand event: %+v", ev)
		case ev.IsIntVar():
			//todo deal with the IntVar error
			return pos, fmt.Errorf("binlog event is a IntVar event: %+v", ev)
		case ev.IsRowsQuery():
			//todo deal with the RowsQuery error
			return pos, fmt.Errorf("binlog event is a RowsQuery event: %+v", ev)
		}

	}
}

func appendUpdateEventFromRows(tc *tableCache, rows *event.Rows, timestamp int64) (*meta.StreamEvent, error) {
	ev := meta.NewStreamEvent(meta.StatementUpdate, timestamp, tc.table.Name())
	for i := range rows.Rows {
		identifies, err := getIdentifiesFromRow(tc, rows, i)
		if err != nil {
			return ev, err
		}
		ev.RowIdentifies = append(ev.RowIdentifies, identifies)

		values, err := getValuesFromRow(tc, rows, i)
		if err != nil {
			return ev, err
		}
		ev.RowValues = append(ev.RowValues, values)
	}

	return ev, nil
}

func appendInsertEventFromRows(tc *tableCache, rows *event.Rows, timestamp int64) (*meta.StreamEvent, error) {
	ev := meta.NewStreamEvent(meta.StatementInsert, timestamp, tc.table.Name())
	for i := range rows.Rows {
		values, err := getValuesFromRow(tc, rows, i)
		if err != nil {
			return ev, err
		}
		ev.RowValues = append(ev.RowValues, values)
	}
	return ev, nil
}

func appendDeleteEventFromRows(tc *tableCache, rows *event.Rows, timestamp int64) (*meta.StreamEvent, error) {
	ev := meta.NewStreamEvent(meta.StatementDelete, timestamp, tc.table.Name())
	for i := range rows.Rows {
		identifies, err := getIdentifiesFromRow(tc, rows, i)
		if err != nil {
			return ev, err
		}
		ev.RowIdentifies = append(ev.RowIdentifies, identifies)
	}
	return ev, nil
}

func getValuesFromRow(tc *tableCache, rs *event.Rows, rowIndex int) (*meta.RowData, error) {
	data := rs.Rows[rowIndex].Data
	valueIndex := 0
	pos := 0

	if rs.DataColumns.Count() != len(tc.table.Columns()) {
		return nil, fmt.Errorf("getValuesFromRow the length of column(%d) in rows did not equal to "+
			"the length of column in table metadata(%d)", rs.DataColumns.Count(), len(tc.table.Columns()))
	}
	values := meta.NewRowData(rs.IdentifyColumns.Count())

	for c := 0; c < rs.DataColumns.Count(); c++ {
		column := meta.NewColumnData(tc.table.Columns()[c].Field(), meta.ColumnType(tc.tableMap.Types[c]),
			false)

		if !rs.DataColumns.Bit(c) {
			column.IsEmpty = true
			values.Columns = append(values.Columns, column)
			continue
		}

		if rs.Rows[rowIndex].NullColumns.Bit(valueIndex) {
			column.Data = nil
			values.Columns = append(values.Columns, column)
			valueIndex++
			continue
		}

		var l int
		var err error

		column.Data, l, err = event.CellBytes(data, pos, tc.tableMap.Types[c], tc.tableMap.Metadata[c],
			tc.table.Columns()[c].IsUnSignedInt())

		if err != nil {
			return nil, err
		}

		values.Columns = append(values.Columns, column)

		pos += l
		valueIndex++
	}

	return values, nil
}

func getIdentifiesFromRow(tc *tableCache, rs *event.Rows, rowIndex int) (*meta.RowData, error) {
	data := rs.Rows[rowIndex].Identify
	identifyIndex := 0
	pos := 0
	if rs.IdentifyColumns.Count() != len(tc.table.Columns()) {
		return nil, fmt.Errorf("getIdentifiesFromRow the length of IdentifyColumns(%d) in rows did not equal to "+
			"the length of column in table metadata(%d)", rs.IdentifyColumns.Count(), len(tc.table.Columns()))
	}
	identifies := meta.NewRowData(rs.IdentifyColumns.Count())
	for c := 0; c < rs.IdentifyColumns.Count(); c++ {

		column := meta.NewColumnData(tc.table.Columns()[c].Field(), meta.ColumnType(tc.tableMap.Types[c]),
			false)
		if !rs.IdentifyColumns.Bit(c) {
			column.IsEmpty = true
			identifies.Columns = append(identifies.Columns, column)
			continue
		}

		if rs.Rows[rowIndex].NullIdentifyColumns.Bit(identifyIndex) {
			column.Data = nil
			identifies.Columns = append(identifies.Columns, column)
			identifyIndex++
			continue
		}

		var l int
		var err error

		column.Data, l, err = event.CellBytes(data, pos, tc.tableMap.Types[c], tc.tableMap.Metadata[c],
			tc.table.Columns()[c].IsUnSignedInt())
		if err != nil {
			return nil, err
		}

		identifies.Columns = append(identifies.Columns, column)

		pos += l
		identifyIndex++
	}

	return identifies, nil
}
