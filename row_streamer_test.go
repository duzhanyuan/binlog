package binlog

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/onlyac0611/binlog/event"
	"github.com/onlyac0611/binlog/meta"
)

var (
	testBinlogPosParseEvents = meta.BinlogPosition{
		FileName: "binlog.000005",
		Offset:   0,
	}
	tesInfo = meta.MysqlTableInfo{

		Name: meta.MysqlTableName{
			DbName:    "vt_test_keyspace",
			TableName: "vt_a",
		},
		Columns: []meta.MysqlColumnAttribute{
			meta.MysqlColumnAttribute{
				Field: "id",
				Type:  "int(11)",
				Key:   meta.MysqlPrimaryKeyDescription,
				Null:  "",
				Extra: meta.MysqlAutoIncrementDescription,
			},
			meta.MysqlColumnAttribute{
				Field: "message",
				Type:  "varchar(256)",
				Key:   "",
				Null:  "",
				Extra: "",
			},
		},
	}
)

const (
	testDSN      = "test:123456@tcp(192.168.88.128:3306)/mysql"
	testServerID = 1234
)

type mockMapper struct {
}

func newMockMapper() *mockMapper {
	return &mockMapper{}
}

func (m *mockMapper) GetTableInfo(name meta.MysqlTableName) (meta.MysqlTableInfo, error) {
	return tesInfo, nil
}

func getInputData() []event.BinlogEvent {
	// Create a tableMap event on the table.

	f := event.NewMySQL56BinlogFormat()
	s := event.NewFakeBinlogStream()
	s.ServerID = 62344

	tableID := uint64(0x102030405060)
	tm := &event.TableMap{
		Flags:    0x8090,
		Database: "vt_test_keyspace",
		Name:     "vt_a",
		Types: []byte{
			event.TypeLong,
			event.TypeVarchar,
		},
		CanBeNull: event.NewServerBitmap(2),
		Metadata: []uint16{
			0,
			384, // A VARCHAR(128) in utf8 would result in 384.
		},
	}
	tm.CanBeNull.Set(1, true)

	// Do an insert packet with all fields set.
	insertRows := event.Rows{
		Flags:       0x1234,
		DataColumns: event.NewServerBitmap(2),
		Rows: []event.Row{
			{
				NullColumns: event.NewServerBitmap(2),
				Data: []byte{
					0x10, 0x20, 0x30, 0x40, // long
					0x04, 0x00, // len('abcd')
					'a', 'b', 'c', 'd', // 'abcd'
				},
			},
		},
	}
	insertRows.DataColumns.Set(0, true)
	insertRows.DataColumns.Set(1, true)

	// Do an update packet with all fields set.
	updateRows := event.Rows{
		Flags:           0x1234,
		IdentifyColumns: event.NewServerBitmap(2),
		DataColumns:     event.NewServerBitmap(2),
		Rows: []event.Row{
			{
				NullIdentifyColumns: event.NewServerBitmap(2),
				NullColumns:         event.NewServerBitmap(2),
				Identify: []byte{
					0x10, 0x20, 0x30, 0x40, // long
					0x03, 0x00, // len('abc')
					'a', 'b', 'c', // 'abc'
				},
				Data: []byte{
					0x10, 0x20, 0x30, 0x40, // long
					0x04, 0x00, // len('abcd')
					'a', 'b', 'c', 'd', // 'abcd'
				},
			},
		},
	}
	updateRows.IdentifyColumns.Set(0, true)
	updateRows.IdentifyColumns.Set(1, true)
	updateRows.DataColumns.Set(0, true)
	updateRows.DataColumns.Set(1, true)

	// Do a delete packet with all fields set.
	deleteRows := event.Rows{
		Flags:           0x1234,
		IdentifyColumns: event.NewServerBitmap(2),
		Rows: []event.Row{
			{
				NullIdentifyColumns: event.NewServerBitmap(2),
				Identify: []byte{
					0x10, 0x20, 0x30, 0x40, // long
					0x03, 0x00, // len('abc')
					'a', 'b', 'c', // 'abc'
				},
			},
		},
	}
	deleteRows.IdentifyColumns.Set(0, true)
	deleteRows.IdentifyColumns.Set(1, true)

	return []event.BinlogEvent{
		event.NewRotateEvent(f, s, uint64(testBinlogPosParseEvents.Offset), testBinlogPosParseEvents.FileName),
		event.NewFormatDescriptionEvent(f, s),
		event.NewTableMapEvent(f, s, tableID, tm),
		event.NewQueryEvent(f, s, event.Query{
			Database: "vt_test_keyspace",
			SQL:      "BEGIN"}),
		event.NewWriteRowsEvent(f, s, tableID, insertRows),
		event.NewUpdateRowsEvent(f, s, tableID, updateRows),
		event.NewDeleteRowsEvent(f, s, tableID, deleteRows),
		event.NewXIDEvent(f, s),
	}
}

func checkTransactionEqual(t *meta.Transaction, right *meta.Transaction) error {
	if t.NowPosition != right.NowPosition {
		return fmt.Errorf("NowPosition is not equal. left: %v, right: %v", t.NowPosition, right.NowPosition)
	}
	if t.NextPosition != right.NextPosition {
		return fmt.Errorf("NextPosition is not equal. left: %v, right: %v", t.NextPosition, right.NextPosition)
	}

	if len(t.Events) != len(right.Events) {
		return fmt.Errorf("len of Events is not match.left: %v right: %v", len(t.Events), len(right.Events))
	}

	for i := range t.Events {
		if err := checkStreamEventEqual(t.Events[i], right.Events[i]); err != nil {
			return fmt.Errorf("%d RowValues is not match for %v", i, err)
		}
	}
	return nil
}

func checkStreamEventEqual(s *meta.StreamEvent, right *meta.StreamEvent) error {
	if s.Type != right.Type {
		return fmt.Errorf("type is not equal. left: %v, right: %v", s.Type, right.Type)
	}
	if s.Table != right.Table {
		return fmt.Errorf("TableName is not equal. left: %v, right: %v", s.Table, right.Table)
	}
	if s.Timestamp != right.Timestamp {
		return fmt.Errorf("timestamp is not equal. left: %v, right: %v", s.Timestamp, right.Timestamp)
	}
	if s.SQL != right.SQL {
		return fmt.Errorf("sql is not equal. left: %v, right: %v", s.SQL, right.SQL)
	}

	if len(s.RowValues) != len(right.RowValues) {
		return fmt.Errorf("len of RowValues is not match.left: %v right: %v",
			len(s.RowValues), len(right.RowValues))
	}

	if len(s.RowIdentifies) != len(right.RowIdentifies) {
		return fmt.Errorf("len of RowIdentifies is not match.left: %v right: %v",
			len(s.RowIdentifies), len(right.RowIdentifies))
	}

	for i := range s.RowValues {
		if err := checkRowDataEqual(s.RowValues[i], right.RowValues[i]); err != nil {
			return fmt.Errorf("%d RowValues is not match for %v", i, err)
		}
	}

	for i := range s.RowIdentifies {
		if err := checkRowDataEqual(s.RowIdentifies[i], right.RowIdentifies[i]); err != nil {
			return fmt.Errorf("%d RowIdentifies is not match for %v", i, err)
		}
	}
	return nil
}

func checkRowDataEqual(r *meta.RowData, right *meta.RowData) error {
	if len(r.Columns) != len(right.Columns) {
		return fmt.Errorf("len of Columns is not match.left: %v right: %v", len(r.Columns), len(right.Columns))
	}
	for i := range r.Columns {
		if err := checkColumnDataEqual(r.Columns[i], right.Columns[i]); err != nil {
			return fmt.Errorf("%d Column is not match for %v", i, err)
		}
	}
	return nil
}

func checkColumnDataEqual(c *meta.ColumnData, right *meta.ColumnData) error {
	if c.Filed != right.Filed {
		return fmt.Errorf("filed is not equal. left: %v, right: %v", c.Filed, right.Filed)
	}

	if c.Type != right.Type {
		return fmt.Errorf("type is not equal. left: %v, right: %v", c.Type, right.Type)
	}

	if c.IsEmpty != right.IsEmpty {
		return fmt.Errorf("isEmpty is not equal. left: %v, right: %v", c.IsEmpty, right.IsEmpty)
	}

	if bytes.Compare(c.Data, right.Data) != 0 {
		return fmt.Errorf("data is not equal. left: %v, right: %v", string(c.Data), string(right.Data))
	}
	return nil
}

func TestRowStreamer_parseEvents(t *testing.T) {

	input := getInputData()

	want := &meta.Transaction{
		NowPosition: testBinlogPosParseEvents,
		NextPosition: meta.BinlogPosition{
			FileName: testBinlogPosParseEvents.FileName,
			Offset:   4,
		},
		Events: []*meta.StreamEvent{
			&meta.StreamEvent{
				Type:      meta.StatementInsert,
				Timestamp: 1407805592,
				Table:     tesInfo.Name,
				SQL:       "",
				RowValues: []*meta.RowData{
					&meta.RowData{
						Columns: []*meta.ColumnData{
							&meta.ColumnData{
								Filed: "id",
								IsKey: true,
								Data:  []byte("1076895760"),
								Type:  meta.ColumnTypeLong,
							},
							&meta.ColumnData{
								Filed: "message",
								IsKey: false,
								Data:  []byte("abcd"),
								Type:  meta.ColumnTypeVarchar,
							},
						},
					},
				},
			},
			&meta.StreamEvent{
				Type:      meta.StatementUpdate,
				Table:     tesInfo.Name,
				Timestamp: 1407805592,
				RowIdentifies: []*meta.RowData{
					&meta.RowData{
						Columns: []*meta.ColumnData{
							&meta.ColumnData{
								Filed: "id",
								IsKey: true,
								Data:  []byte("1076895760"),
								Type:  meta.ColumnTypeLong,
							},
							&meta.ColumnData{
								Filed: "message",
								IsKey: false,
								Data:  []byte("abc"),
								Type:  meta.ColumnTypeVarchar,
							},
						},
					},
				},
				RowValues: []*meta.RowData{
					&meta.RowData{
						Columns: []*meta.ColumnData{
							&meta.ColumnData{
								Filed: "id",
								IsKey: true,
								Data:  []byte("1076895760"),
								Type:  meta.ColumnTypeLong,
							},
							&meta.ColumnData{
								Filed: "message",
								IsKey: false,
								Data:  []byte("abcd"),
								Type:  meta.ColumnTypeVarchar,
							},
						},
					},
				},
			},
			&meta.StreamEvent{
				Type:      meta.StatementDelete,
				Timestamp: 1407805592,
				Table:     tesInfo.Name,
				RowIdentifies: []*meta.RowData{
					&meta.RowData{
						Columns: []*meta.ColumnData{
							&meta.ColumnData{
								Filed: "id",
								IsKey: true,
								Data:  []byte("1076895760"),
								Type:  meta.ColumnTypeLong,
							},
							&meta.ColumnData{
								Filed: "message",
								IsKey: false,
								Data:  []byte("abc"),
								Type:  meta.ColumnTypeVarchar,
							},
						},
					},
				},
			},
		},
	}

	m := newMockMapper()

	r, err := NewRowStreamer(testDSN, testServerID, m)
	if err != nil {
		t.Fatalf("NewRowStreamer err: %#v", err)
		return
	}
	r.SetStartBinlogPosition(testBinlogPosParseEvents)

	var out *meta.Transaction
	r.sendTransaction = func(tran *meta.Transaction) error {
		out = tran
		return nil
	}

	events := make(chan event.BinlogEvent)
	go func() {
		for i := range input {
			events <- input[i]
		}
		close(events)
	}()

	ctx := context.Background()

	_, err = r.parseEvents(ctx, events)

	if err != ErrStreamEOF {
		t.Fatalf("parseEvents err != %v, err: %v", ErrStreamEOF, err)
	}

	if err := checkTransactionEqual(out, want); err != nil {
		t.Fatalf("NowPosition want != out, err: %v", err)
	}
}

func TestRowStreamer_SetStartBinlogPosition(t *testing.T) {
	m := newMockMapper()
	r, err := NewRowStreamer(testDSN, testServerID, m)
	if err != nil {
		t.Fatalf("NewRowStreamer err: %v", err)
		return
	}
	r.SetStartBinlogPosition(testBinlogPosParseEvents)
	if r.startPos != testBinlogPosParseEvents {
		t.Fatalf("want != out, input:%+v want:%+v out %+v", testBinlogPosParseEvents, testBinlogPosParseEvents, r.startPos)
	}
}
