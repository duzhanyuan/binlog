package meta

import (
	"fmt"
	"reflect"
)

// one StreamEvent means a SQL or a rows in binlog
type StreamEvent struct {
	Type      StatementType
	Table     MysqlTableName
	SQL       string
	Timestamp int64
	//which data come to used for StatementInsert and  StatementUpdate
	RowValues []*RowData
	//which data come from used for  StatementUpdate and StatementDelete
	RowIdentifies []*RowData
}

func NewStreamEvent(tranType StatementType,
	timestamp int64, table MysqlTableName) *StreamEvent {
	return &StreamEvent{
		Type:          tranType,
		Table:         table,
		Timestamp:     timestamp,
		SQL:           "",
		RowValues:     make([]*RowData, 0, 10),
		RowIdentifies: make([]*RowData, 0, 10),
	}
}

func (s *StreamEvent) CheckEqual(right *StreamEvent) error {
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

	for i := range s.RowValues {
		if err := s.RowValues[i].CheckEqual(right.RowValues[i]); err != nil {
			return fmt.Errorf("%d RowValues is not match for %v", i, err)
		}
	}

	for i := range s.RowIdentifies {
		if err := s.RowIdentifies[i].CheckEqual(right.RowIdentifies[i]); err != nil {
			return fmt.Errorf("%d RowIdentifies is not match for %v", i, err)
		}
	}
	return nil
}

type Transaction struct {
	NowPosition  BinlogPosition
	NextPosition BinlogPosition
	Timestamp    int64
	Events       []*StreamEvent
}

func NewTransaction(now, next BinlogPosition, timestamp int64,
	events []*StreamEvent) *Transaction {
	return &Transaction{
		NowPosition:  now,
		NextPosition: next,
		Timestamp:    timestamp,
		Events:       events,
	}
}

func (t *Transaction) CheckEqual(right *Transaction) error {
	if t.NowPosition != right.NowPosition {
		return fmt.Errorf("NowPosition is not equal. left: %v, right: %v", t.NowPosition, right.NowPosition)
	}
	if t.NextPosition != right.NextPosition {
		return fmt.Errorf("NextPosition is not equal. left: %v, right: %v", t.NextPosition, right.NextPosition)
	}
	return nil
}

type ColumnData struct {
	Filed string
	Type  ColumnType
	// is primary key
	IsKey bool
	// data is empty
	IsEmpty bool
	// the data
	Data []byte
}

func NewColumnData(filed string, typ ColumnType, isEmpty bool) *ColumnData {
	return &ColumnData{
		Filed:   filed,
		Type:    typ,
		IsEmpty: isEmpty,
	}
}

func (c *ColumnData) CheckEqual(right *ColumnData) error {
	if c.Filed != right.Filed {
		return fmt.Errorf("filed is not equal. left: %v, right: %v", c.Filed, right.Filed)
	}
	if c.Type != right.Type {
		return fmt.Errorf("type is not equal. left: %v, right: %v", c.Type, right.Type)
	}
	if c.IsKey != right.IsKey {
		return fmt.Errorf("isKey is not equal. left: %v, right: %v", c.IsKey, right.IsKey)
	}

	if c.IsEmpty != right.IsEmpty {
		return fmt.Errorf("isEmpty is not equal. left: %v, right: %v", c.IsEmpty, right.IsEmpty)
	}

	if reflect.DeepEqual(c.Data, right.Data) {
		return fmt.Errorf("data is not equal. left: %v, right: %v", string(c.Data), string(right.Data))
	}
	return nil
}

type RowData struct {
	Columns []*ColumnData
}

func NewRowData(cnt int) *RowData {
	return &RowData{
		Columns: make([]*ColumnData, 0, cnt),
	}
}

func (r *RowData) CheckEqual(right *RowData) error {
	if len(r.Columns) != len(right.Columns) {
		return fmt.Errorf("len of Columns is not match.left: %v right: %v", len(r.Columns), len(right.Columns))
	}
	for i := range r.Columns {
		if err := r.Columns[i].CheckEqual(right.Columns[i]); err != nil {
			return fmt.Errorf("%d Column is not match for %v", i, err)
		}
	}
	return nil
}
