package transformer

type redact struct{}

func init() { Register(&redact{}) }

func (r *redact) Name() string { return "redact" }

func (r *redact) Transform(value string, col ColumnInfo) (string, error) {
	return "***REDACTED***", nil
}

func (r *redact) Description() string {
	return "Replaces all values with ***REDACTED*** regardless of input"
}

func (r *redact) DetailedHelp() string {
	return `Unconditionally replaces the value with the fixed string "***REDACTED***".
Non-deterministic in the sense that all distinct inputs produce the same output,
destroying uniqueness. Use when the data must be completely hidden.`
}

func (r *redact) SupportedTypes() []string {
	return []string{"text", "varchar", "character varying", "integer", "bigint", "numeric", "timestamp", "date", "boolean", "json", "jsonb"}
}

func (r *redact) Examples() []Example {
	return []Example{
		{Input: "sensitive data", Output: "***REDACTED***", DataType: "text"},
		{Input: "12345", Output: "***REDACTED***", DataType: "integer"},
	}
}
