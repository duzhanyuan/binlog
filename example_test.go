package binlog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/signal"

	_ "github.com/go-sql-driver/mysql"
	"github.com/onlyac0611/binlog/meta"
)

var ErrResultEmptyRow = errors.New("query results has no rows")

type ExampleTableInfoMapper struct {
	db *sql.DB
}

func (e *ExampleTableInfoMapper) GetTableInfo(name meta.MysqlTableName) (meta.MysqlTableInfo, error) {
	sql := "desc " + name.String()
	rows, err := e.db.Query(sql)
	if err != nil {
		return meta.MysqlTableInfo{}, fmt.Errorf("query failed sql: %s, error: %v", sql, err)
	}
	defer rows.Close()

	info := meta.MysqlTableInfo{
		Name:    name,
		Columns: make([]meta.MysqlColumnAttribute, 0, 10),
	}
	for i := 0; rows.Next(); i++ {
		column := meta.MysqlColumnAttribute{}
		err = rows.Scan(&column.Field, &column.Type, &column.Null, &column.Key, &column.Default, &column.Extra)
		if err != nil {
			return meta.MysqlTableInfo{}, err
		}
		info.Columns = append(info.Columns, column)
	}
	return info, nil
}

func (e *ExampleTableInfoMapper) GetBinlogPosition() (pos meta.BinlogPosition, err error) {
	var rows *sql.Rows
	sql := "SHOW MASTER STATUS"
	rows, err = e.db.Query(sql)
	if err != nil {
		err = fmt.Errorf("query fail. sql: %s, error: %v", sql, err)
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
	sql := "SHOW VARIABLES LIKE 'binlog_format'"
	rows, err = e.db.Query(sql)
	if err != nil {
		err = fmt.Errorf("query fail. sql: %s, error: %v", sql, err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		var str string
		err = rows.Scan(&name, &str)
		if err != nil {
			err = fmt.Errorf("scan fail. sql: %s, error: %v", sql, err)
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
