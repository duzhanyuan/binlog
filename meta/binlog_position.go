package meta

type BinlogPosition struct {
	FileName string
	Offset   int64
}

// IsZero means BinlogPosition is existed
func (b BinlogPosition) IsZero() bool {
	return b.FileName == "" || b.Offset == 0
}
