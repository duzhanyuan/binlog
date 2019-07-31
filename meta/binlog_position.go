//package meta 用于定义一些基本数据：
// binlog位置,以文件名和位移量作定义，
// 表信息，主要是表名和列信息，
// 事务信息，主要是一个完整的binlog events(以begin开始， 以commit结束)。
package meta

// BinlogPosition指定binlog的位置，以文件名和位移
type BinlogPosition struct {
	FileName string //binlog文件名
	Offset   int64  //在binlog文件中的位移
}

// IsZero means BinlogPosition is existed
func (b BinlogPosition) IsZero() bool {
	return b.FileName == "" || b.Offset == 0
}
