package meta

const (
	MysqlPrimaryKeyDescription    = "PRI"            //主键
	MysqlUniqueIndexDescription   = "UNI"            //唯一索引
	MysqlAutoIncrementDescription = "auto_increment" //自增
)

//mysql的表信息
type MysqlTableInfo struct {
	Name    MysqlTableName
	Columns []MysqlColumnAttribute
}

//mysql的表名
type MysqlTableName struct {
	DbName    string //数据库名
	TableName string //表名
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

//列属性
type MysqlColumnAttribute struct {
	Field   string //列名
	Type    string //列类型
	Null    string //是否为空
	Key     string //PRI代表主键，UNI代表唯一索引
	Default []byte //默认值
	Extra   string //其他备注信息
}
