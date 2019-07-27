package meta

const (
	MysqlPrimaryKeyDescription    = "PRI"
	MysqlUniqueIndexDescription   = "UNI"
	MysqlAutoIncrementDescription = "auto_increment"
)

type MysqlTableInfo struct {
	Name    MysqlTableName
	Columns []MysqlColumnAttribute
}

type MysqlTableName struct {
	DbName    string
	TableName string
}

func (m *MysqlTableName) String() string {
	return "`" + m.DbName + "`.`" + m.TableName + "`"
}

func NewMysqlTableName(database, table string) MysqlTableName {
	return MysqlTableName{
		DbName:    database,
		TableName: table,
	}
}

type MysqlColumnAttribute struct {
	Field   string
	Type    string
	Null    string
	Key     string
	Default []byte
	Extra   string
}
