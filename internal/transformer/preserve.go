package transformer

type preserve struct{}

func init() { Register(&preserve{}) }

func (p *preserve) Name() string { return "preserve" }

func (p *preserve) Transform(value string, col ColumnInfo) (string, error) {
	return value, nil
}
