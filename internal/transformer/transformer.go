package transformer

type ColumnInfo struct {
	TableName  string
	ColumnName string
	DataType   string
	Position   int
}

type Transformer interface {
	Name() string
	Transform(value string, col ColumnInfo) (string, error)
}
