package transformer

type redact struct{}

func init() { Register(&redact{}) }

func (r *redact) Name() string { return "redact" }

func (r *redact) Transform(value string, col ColumnInfo) (string, error) {
	return "***REDACTED***", nil
}
