package meta

//Transaction 代表一组有事务的binlog evnet
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

//单个列的信息
type ColumnData struct {
	Filed   string     //表信息
	Type    ColumnType //binlog中的列类型
	IsKey   bool       // is primary key
	IsEmpty bool       // data is empty,即该列没有变化
	Data    []byte     // the data
}

func NewColumnData(filed string, typ ColumnType, isEmpty bool) *ColumnData {
	return &ColumnData{
		Filed:   filed,
		Type:    typ,
		IsEmpty: isEmpty,
	}
}

type RowData struct {
	Columns []*ColumnData
}

func NewRowData(cnt int) *RowData {
	return &RowData{
		Columns: make([]*ColumnData, 0, cnt),
	}
}
