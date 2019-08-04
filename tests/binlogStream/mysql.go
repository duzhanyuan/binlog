package main

import (
	"database/sql"
	"fmt"
	"io"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/onlyac0611/binlog"
)

const (
	mysqlUnsigned = "unsigned" //无符号
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
	return strings.Contains(strings.ToLower(m.typ), mysqlUnsigned)
}

type mysqlTableInfo struct {
	name    binlog.MysqlTableName
	columns []binlog.MysqlColumn
}

func (m *mysqlTableInfo) Name() binlog.MysqlTableName {
	return m.name
}

func (m *mysqlTableInfo) Columns() []binlog.MysqlColumn {
	return m.columns
}

type exampleMysqlTableMapper struct {
	db *sql.DB
}

func (e *exampleMysqlTableMapper) GetBinlogFormat() (format binlog.FormatType, err error) {
	query := "SHOW VARIABLES LIKE 'binlog_format'"
	var name, str string
	err = e.db.QueryRow(query).Scan(&name, &str)
	if err != nil {
		err = fmt.Errorf("QueryRow fail. query: %s, error: %v", query, err)
		return
	}
	format = binlog.FormatType(str)
	return
}

func (e *exampleMysqlTableMapper) GetBinlogPosition() (pos binlog.Position, err error) {
	query := "SHOW MASTER STATUS"
	var metaDoDb, metaIgnoreDb, executedGTidSet string
	err = e.db.QueryRow(query).Scan(&pos.Filename, &pos.Offset, &metaDoDb, &metaIgnoreDb, &executedGTidSet)
	if err != nil {
		err = fmt.Errorf("query fail. query: %s, error: %v", query, err)
		return
	}
	return
}

func (e *exampleMysqlTableMapper) MysqlTable(name binlog.MysqlTableName) (binlog.MysqlTable, error) {
	info := &mysqlTableInfo{
		name:    name,
		columns: make([]binlog.MysqlColumn, 0, 10),
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
		//log.Printf("field: %v type: %v", column.Field(), column.IsUnSignedInt())
	}
	return info, nil
}

func showTransaction(t *binlog.Transaction, w io.Writer) {
	/*	for _, vi := range t.Events {
		fmt.Fprintln(w, "nextPos:", t.NextPosition)
		for _, vj := range vi.RowIdentifies {
			fmt.Fprintln(w, "type:", vi.Type.String(), "table:", vi.Table.String())
			for _, vk := range vj.Columns {
				fmt.Fprintln(w, "field:", vk.Filed, "data:", string(vk.Data))
			}
		}
		for _, vj := range vi.RowValues {
			fmt.Fprintln(w, "type:", vi.Type.String(), "table:", vi.Table.String())
			for _, vk := range vj.Columns {
				fmt.Fprintln(w, "field:", vk.Filed, "data:", string(vk.Data))
			}
		}
	}*/
	b, err := t.MarshalJSON()
	if err != nil {
		return
	}
	fmt.Fprintln(w, string(b))
}
