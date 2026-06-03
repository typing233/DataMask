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

// TypedTransformer operates on decoded native Go values (int64, time.Time, etc.)
// instead of raw COPY-format strings. When a transformer implements this interface,
// the pipeline will decode the value first, pass the native value in, and encode
// the returned native value back to COPY format.
type TypedTransformer interface {
	Transformer
	TransformTyped(value interface{}, col ColumnInfo) (interface{}, error)
}

type Example struct {
	Input    string
	Output   string
	DataType string
}

type Described interface {
	Description() string
	DetailedHelp() string
	SupportedTypes() []string
	Examples() []Example
}
