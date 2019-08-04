package binlog

//Transaction 代表一组有事务的binlog evnet
type Transaction struct {
	NowPosition  Position       //在binlog中的当前位置
	NextPosition Position       //在binlog中的下一个位置
	Timestamp    int64          //执行时间
	Events       []*StreamEvent //一组有事务的binlog evnet
}

//NewTransaction 创建Transaction
func NewTransaction(now, next Position, timestamp int64,
	events []*StreamEvent) *Transaction {
	return &Transaction{
		NowPosition:  now,
		NextPosition: next,
		Timestamp:    timestamp,
		Events:       events,
	}
}

//StreamEvent means a SQL or a rows in binlog
type StreamEvent struct {
	Type          StatementType  //语句类型
	Table         MysqlTableName //表名
	SQL           string         //sql
	Timestamp     int64          //执行时间
	RowValues     []*RowData     //which data come to used for StatementInsert and  StatementUpdate
	RowIdentifies []*RowData     //which data come from used for  StatementUpdate and StatementDelete
}

//NewStreamEvent 创建StreamEvent
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

//ColumnData 单个列的信息
type ColumnData struct {
	Filed   string     // 字段信息
	Type    ColumnType // binlog中的列类型
	IsEmpty bool       // data is empty,即该列没有变化
	Data    []byte     // the data
}

//NewColumnData 创建ColumnData
func NewColumnData(filed string, typ ColumnType, isEmpty bool) *ColumnData {
	return &ColumnData{
		Filed:   filed,
		Type:    typ,
		IsEmpty: isEmpty,
	}
}

//RowData 行数据
type RowData struct {
	Columns []*ColumnData
}

//NewRowData 创建RowData
func NewRowData(cnt int) *RowData {
	return &RowData{
		Columns: make([]*ColumnData, 0, cnt),
	}
}
