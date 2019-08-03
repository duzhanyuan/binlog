package binlog

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"strings"

	//_ "github.com/go-sql-driver/mysql" you need it in you own project
	"github.com/onlyac0611/binlog/meta"
)

const (
	mysqlPrimaryKeyDescription    = "PRI"            //主键
	mysqlAutoIncrementDescription = "auto_increment" //自增
	mysqlUnsigned                 = "unsigned"       //无符号
)

//列属性
type mysqlColumnAttribute struct {
	field         string //列名
	typ           string //列类型
	null          string //是否为空
	key           string //PRI代表主键，UNI代表唯一索引
	columnDefault []byte //默认值
	extra         string //其他备注信息
}

func (m *mysqlColumnAttribute) Field() string {
	return m.field
}

func (m *mysqlColumnAttribute) IsUnSignedInt() bool {
	return strings.Contains(m.typ, mysqlUnsigned)
}

type mysqlTableInfo struct {
	name    meta.MysqlTableName
	columns []meta.MysqlColumn
}

func (m *mysqlTableInfo) Name() meta.MysqlTableName {
	return m.name
}

func (m *mysqlTableInfo) Columns() []meta.MysqlColumn {
	return m.columns
}

type exampleMysqlTableMapper struct {
	db *sql.DB
}

func (e *exampleMysqlTableMapper) GetBinlogFormat() (format meta.BinlogFormatType, err error) {
	query := "SHOW VARIABLES LIKE 'binlog_format'"
	var name, str string
	err = e.db.QueryRow(query).Scan(&name, &str)
	if err != nil {
		err = fmt.Errorf("QueryRow fail. query: %s, error: %v", query, err)
		return
	}
	format = meta.BinlogFormatType(str)
	return
}

func (e *exampleMysqlTableMapper) GetBinlogPosition() (pos meta.BinlogPosition, err error) {
	query := "SHOW MASTER STATUS"
	var metaDoDb, metaIgnoreDb, executedGTidSet string
	err = e.db.QueryRow(query).Scan(&pos.FileName, &pos.Offset, &metaDoDb, &metaIgnoreDb, &executedGTidSet)
	if err != nil {
		err = fmt.Errorf("query fail. query: %s, error: %v", query, err)
		return
	}
	return
}

func (e *exampleMysqlTableMapper) MysqlTable(name meta.MysqlTableName) (meta.MysqlTable, error) {
	info := &mysqlTableInfo{
		name:    name,
		columns: make([]meta.MysqlColumn, 0, 10),
	}

	query := "desc " + name.String()
	rows, err := e.db.Query(query)
	if err != nil {
		return info, fmt.Errorf("query failed query: %s, error: %v", query, err)
	}
	defer rows.Close()

	for i := 0; rows.Next(); i++ {
		column := &mysqlColumnAttribute{}
		err = rows.Scan(&column.field, &column.typ, &column.null, &column.key, &column.columnDefault, &column.extra)
		if err != nil {
			return info, err
		}
		info.columns = append(info.columns, column)
	}
	return info, nil
}

func showTransaction(t *meta.Transaction) {
	for _, vi := range t.Events {
		lw.logger().Print("nextPos:", t.NextPosition)
		for _, vj := range vi.RowIdentifies {
			lw.logger().Print("type:", vi.Type.String(), "table:", vi.Table.String())
			for _, vk := range vj.Columns {
				lw.logger().Print("data：", string(vk.Data))
			}
		}
		for _, vj := range vi.RowValues {
			lw.logger().Print("type:", vi.Type.String(), "table:", vi.Table.String())
			for _, vk := range vj.Columns {
				lw.logger().Print("data：", string(vk.Data))
			}
		}
	}
}

func ExampleRowStreamer_Stream() {
	SetLogger(NewDefaultLogger(os.Stdout, DebugLevel))
	dsn := "example:example@tcp(localhost:3306)/mysql?charset=utf8mb4"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		lw.logger().Errorf("open fail. err: %v", err)
		return
	}
	defer db.Close()

	db.SetMaxIdleConns(2)
	db.SetMaxOpenConns(4)

	e := &exampleMysqlTableMapper{db: db}
	format, err := e.GetBinlogFormat()
	if err != nil {
		lw.logger().Errorf("getBinlogFormat fail. err: %v", err)
		return
	}

	if !format.IsRow() {
		lw.logger().Errorf("binlog format is not row. format: %v", format)
		return
	}

	pos, err := e.GetBinlogPosition()
	if err != nil {
		lw.logger().Errorf("GetBinlogPosition fail. err: %v", err)
		return
	}

	r, err := NewRowStreamer(dsn, 1234, e)
	if err != nil {
		lw.logger().Errorf("NewRowStreamer fail. err: %v", err)
		return
	}
	r.SetStartBinlogPosition(pos)

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	processWait := make(chan os.Signal, 1)
	signal.Notify(processWait, os.Kill, os.Interrupt)

	go func() {
		select {
		case <-processWait:
			cancel()
		}
	}()
	err = r.Stream(ctx, func(t *meta.Transaction) error {
		showTransaction(t)
		return nil
	})
}
