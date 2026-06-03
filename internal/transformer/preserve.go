package transformer

type preserve struct{}

func init() { Register(&preserve{}) }

func (p *preserve) Name() string { return "preserve" }

func (p *preserve) Transform(value string, col ColumnInfo) (string, error) {
	return value, nil
}

func (p *preserve) Description() string {
	return "Identity transformer that passes values through unchanged"
}

func (p *preserve) DetailedHelp() string {
	return `Returns the input value without any modification. Use to explicitly mark
columns that should not be masked, overriding a default_transformer setting.`
}

func (p *preserve) SupportedTypes() []string {
	return []string{"text", "varchar", "character varying", "integer", "bigint", "smallint", "numeric", "real", "double precision", "boolean", "timestamp", "timestamptz", "date", "json", "jsonb", "uuid", "bytea"}
}

func (p *preserve) Examples() []Example {
	return []Example{
		{Input: "any value", Output: "any value", DataType: "text"},
		{Input: "42", Output: "42", DataType: "integer"},
	}
}
