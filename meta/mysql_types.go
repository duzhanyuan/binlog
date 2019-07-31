package meta

import (
	"strings"

	"cmd/libra/binlog/event"
)

//it means the sql statement type
type StatementType int

const (
	StatementUnknown = iota
	StatementBegin
	StatementCommit
	StatementRollback
	StatementInsert
	StatementUpdate
	StatementDelete
	StatementCreate
	StatementAlter
	StatementDrop
	StatementTruncate
	StatementRename
	StatementSet
)

var (
	statementPrefixes = map[string]StatementType{
		"begin":    StatementBegin,
		"commit":   StatementCommit,
		"rollback": StatementRollback,
		"insert":   StatementInsert,
		"update":   StatementUpdate,
		"delete":   StatementDelete,
		"create":   StatementCreate,
		"alter":    StatementAlter,
		"drop":     StatementDrop,
		"truncate": StatementTruncate,
		"rename":   StatementRename,
		"set":      StatementSet,
	}

	statementStrings = map[StatementType]string{
		StatementBegin:    "begin",
		StatementCommit:   "commit",
		StatementRollback: "rollback",
		StatementInsert:   "insert",
		StatementUpdate:   "update",
		StatementDelete:   "delete",
		StatementCreate:   "create",
		StatementAlter:    "alter",
		StatementDrop:     "drop",
		StatementTruncate: "truncate",
		StatementRename:   "rename",
		StatementSet:      "set",
	}
)

func (s StatementType) String() string {
	if s, ok := statementStrings[s]; ok {
		return s
	}
	return "unknown"
}

func (s StatementType) IsDDL() bool {
	switch s {
	case StatementAlter, StatementDrop, StatementCreate, StatementTruncate, StatementRename:
		return true
	default:
		return false
	}
}

//we can get statement type from a SQL
func GetStatementCategory(sql string) StatementType {
	if i := strings.IndexByte(sql, byte(' ')); i >= 0 {
		sql = sql[:i]
	}
	if s, ok := statementPrefixes[strings.ToLower(sql)]; ok {
		return s
	}
	return StatementUnknown
}

const (
	ColumnTypeDecimal    = event.TypeDecimal
	ColumnTypeTiny       = event.TypeTiny
	ColumnTypeShort      = event.TypeShort
	ColumnTypeLong       = event.TypeLong
	ColumnTypeFloat      = event.TypeFloat
	ColumnTypeDouble     = event.TypeDouble
	ColumnTypeNull       = event.TypeNull
	ColumnTypeTimestamp  = event.TypeTimestamp
	ColumnTypeLongLong   = event.TypeLongLong
	ColumnTypeInt24      = event.TypeInt24
	ColumnTypeDate       = event.TypeDate
	ColumnTypeTime       = event.TypeTime
	ColumnTypeDateTime   = event.TypeDateTime
	ColumnTypeYear       = event.TypeYear
	ColumnTypeNewDate    = event.TypeNewDate
	ColumnTypeVarchar    = event.TypeVarchar
	ColumnTypeBit        = event.TypeBit
	ColumnTypeTimestamp2 = event.TypeTimestamp2
	ColumnTypeDateTime2  = event.TypeDateTime2
	ColumnTypeTime2      = event.TypeTime2
	ColumnTypeJSON       = event.TypeJSON
	ColumnTypeNewDecimal = event.TypeNewDecimal
	ColumnTypeEnum       = event.TypeEnum
	ColumnTypeSet        = event.TypeSet
	ColumnTypeTinyBlob   = event.TypeTinyBlob
	ColumnTypeMediumBlob = event.TypeMediumBlob
	ColumnTypeLongBlob   = event.TypeLongBlob
	ColumnTypeBlob       = event.TypeBlob
	ColumnTypeVarString  = event.TypeVarString
	ColumnTypeString     = event.TypeString
	ColumnTypeGeometry   = event.TypeGeometry
)

//从binlog中获取的列类型
type ColumnType int

var (
	columnTypeStrings = map[ColumnType]string{
		ColumnTypeDecimal:    "Decimal",
		ColumnTypeTiny:       "Tiny",
		ColumnTypeShort:      "Short",
		ColumnTypeLong:       "Long",
		ColumnTypeFloat:      "Float",
		ColumnTypeDouble:     "Double",
		ColumnTypeNull:       "Null",
		ColumnTypeTimestamp:  "Timestamp",
		ColumnTypeLongLong:   "LongLong",
		ColumnTypeInt24:      "Int24",
		ColumnTypeDate:       "Date",
		ColumnTypeTime:       "Time",
		ColumnTypeDateTime:   "DateTime",
		ColumnTypeYear:       "Year",
		ColumnTypeNewDate:    "NewDate",
		ColumnTypeVarchar:    "Varchar",
		ColumnTypeBit:        "Bit",
		ColumnTypeTimestamp2: "Timestamp2",
		ColumnTypeDateTime2:  "DateTime2",
		ColumnTypeTime2:      "Time2",
		ColumnTypeJSON:       "JSON",
		ColumnTypeNewDecimal: "NewDecimal",
		ColumnTypeEnum:       "Enum",
		ColumnTypeSet:        "Set",
		ColumnTypeTinyBlob:   "TinyBlob",
		ColumnTypeMediumBlob: "MediumBlob",
		ColumnTypeLongBlob:   "LongBlob",
		ColumnTypeBlob:       "Blob",
		ColumnTypeVarString:  "VarString",
		ColumnTypeString:     "String",
		ColumnTypeGeometry:   "Geometry",
	}
)

func (c ColumnType) String() string {
	if s, ok := columnTypeStrings[c]; ok {
		return s
	}
	return "unknown"
}

func (c ColumnType) IsInteger() bool {
	switch c {
	case ColumnTypeTiny, ColumnTypeShort, ColumnTypeInt24, ColumnTypeLong,
		ColumnTypeLongLong:
		return true
	default:
		return false
	}
}

func (c ColumnType) IsFloat() bool {
	switch c {
	case ColumnTypeFloat, ColumnTypeDouble:
		return true
	default:
		return false
	}
}

func (c ColumnType) IsDecimal() bool {
	switch c {
	case ColumnTypeDecimal, ColumnTypeNewDecimal:
		return true
	default:
		return false
	}
}

func (c ColumnType) IsNumeric() bool {
	return c.IsFloat() || c.IsInteger() || c.IsDecimal()
}

func (c ColumnType) IsTimestamp() bool {
	switch c {
	case ColumnTypeTimestamp, ColumnTypeTimestamp2:
		return true
	default:
		return false
	}
}
func (c ColumnType) IsTime() bool {
	switch c {
	case ColumnTypeTime, ColumnTypeTime2:
		return true
	default:
		return false
	}
}

func (c ColumnType) IsDate() bool {
	switch c {
	case ColumnTypeDate, ColumnTypeNewDate:
		return true
	default:
		return false
	}
}

func (c ColumnType) IsDateTime() bool {
	switch c {
	case ColumnTypeDateTime, ColumnTypeDateTime2:
		return true
	default:
		return false
	}
}

func (c ColumnType) IsBlob() bool {
	switch c {
	case ColumnTypeTinyBlob, ColumnTypeMediumBlob, ColumnTypeLongBlob, ColumnTypeBlob:
		return true
	default:
		return false
	}
}

func (c ColumnType) IsBit() bool {
	switch c {
	case ColumnTypeBit:
		return true
	default:
		return false
	}
}

func (c ColumnType) IsString() bool {
	switch c {
	case ColumnTypeVarchar, ColumnTypeVarString, ColumnTypeString:
		return true
	default:
		return false
	}
}

func (c ColumnType) IsGeometry() bool {
	switch c {
	case ColumnTypeGeometry:
		return true
	default:
		return false
	}
}

//binlog 格式
type BinlogFormatType string

const (
	BinlogFormatTypeRow       = BinlogFormatType("ROW")
	BinlogFormatTypeMixed     = BinlogFormatType("MIXED")
	BinlogFormatTypeStatement = BinlogFormatType("STATEMENT")
)

//show BinlogFormat is Row
func (f BinlogFormatType) IsRow() bool {
	return f == BinlogFormatTypeRow
}
