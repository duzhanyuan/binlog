package binlog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"

	//_ "github.com/go-sql-driver/mysql" you need it in you own project
	"github.com/onlyac0611/binlog/meta"
)

var ErrResultEmptyRow = errors.New("query results has no rows")

const (
	mysqlPrimaryKeyDescription    = "PRI"            //主键
	mysqlAutoIncrementDescription = "auto_increment" //自增
	mysqlUnsigned                 = "unsigned"
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

type ExampleTableInfoMapper struct {
	db *sql.DB
}

func (e *ExampleTableInfoMapper) MysqlTable(name meta.MysqlTableName) (meta.MysqlTable, error) {
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

func (e *ExampleTableInfoMapper) GetBinlogPosition() (pos meta.BinlogPosition, err error) {
	var rows *sql.Rows
	query := "SHOW MASTER STATUS"
	rows, err = e.db.Query(query)
	if err != nil {
		err = fmt.Errorf("query fail. query: %s, error: %v", query, err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var metaDoDb, metaIgnoreDb, executedGtidSet string
		err = rows.Scan(&pos.FileName, &pos.Offset, &metaDoDb, &metaIgnoreDb, &executedGtidSet)
		return
	}
	return
}

func (e *ExampleTableInfoMapper) GetBinlogFormat() (format meta.BinlogFormatType, err error) {
	var rows *sql.Rows
	query := "SHOW VARIABLES LIKE 'binlog_format'"
	rows, err = e.db.Query(query)
	if err != nil {
		err = fmt.Errorf("query fail. query: %s, error: %v", query, err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		var str string
		err = rows.Scan(&name, &str)
		if err != nil {
			err = fmt.Errorf("scan fail. query: %s, error: %v", query, err)
			return
		}
		format = meta.BinlogFormatType(str)
		return format, err
	}
	return format, ErrResultEmptyRow
}

func showTransaction(t *meta.Transaction) {
	for _, vi := range t.Events {
		logger.Print("nextPos:", t.NextPosition)
		for _, vj := range vi.RowIdentifies {
			logger.Print("type:", vi.Type.String(), "table:", vi.Table.String())
			for _, vk := range vj.Columns {
				logger.Print("data：", string(vk.Data))
			}
		}
		for _, vj := range vi.RowValues {
			logger.Print("type:", vi.Type.String(), "table:", vi.Table.String())
			for _, vk := range vj.Columns {
				logger.Print("data：", string(vk.Data))
			}
		}
	}
}

func ExampleRowStreamer_Stream() {
	SetLogger(NewDefaultLogger(os.Stdout, DebugLevel))
	dsn := "user:password@tcp(ip:3306)/mysql?charset=utf8mb4"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		logger.Errorf("open fail. err: %v", err)
		return
	}
	defer db.Close()

	db.SetMaxIdleConns(2)
	db.SetMaxOpenConns(4)

	e := &ExampleTableInfoMapper{db: db}
	format, err := e.GetBinlogFormat()
	if err != nil {
		logger.Errorf("getBinlogFormat fail. err: %v", err)
		return
	}

	if !format.IsRow() {
		logger.Errorf("format is not row. format: %v", format)
		return
	}

	pos, err := e.GetBinlogPosition()
	if err != nil {
		logger.Errorf("getBinlogPosition fail. err: %v", err)
		return
	}

	r, err := NewRowStreamer(dsn, 1234, e)
	if err != nil {
		logger.Errorf("NewRowStreamer fail. err: %v", err)
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
