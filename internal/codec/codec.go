package codec

import "fmt"

type Value struct {
	IsNull bool
	Raw    string
	Native interface{}
}

type TypeCodec interface {
	CanHandle(pgType string) bool
	Decode(raw string, pgType string) (interface{}, error)
	Encode(value interface{}, pgType string) (string, error)
}

type Registry struct {
	codecs []TypeCodec
}

func NewPostgresRegistry() *Registry {
	r := &Registry{}
	r.Register(&IntCodec{})
	r.Register(&FloatCodec{})
	r.Register(&BoolCodec{})
	r.Register(&TimestampCodec{})
	r.Register(&DateCodec{})
	r.Register(&ByteaCodec{})
	r.Register(&JSONCodec{})
	r.Register(&TextCodec{})
	return r
}

func (r *Registry) Register(c TypeCodec) {
	r.codecs = append(r.codecs, c)
}

func (r *Registry) Decode(raw string, pgType string) (*Value, error) {
	if raw == "\\N" {
		return &Value{IsNull: true, Raw: raw}, nil
	}

	for _, c := range r.codecs {
		if c.CanHandle(pgType) {
			native, err := c.Decode(raw, pgType)
			if err != nil {
				return nil, fmt.Errorf("decoding %q as %s: %w", raw, pgType, err)
			}
			return &Value{Raw: raw, Native: native}, nil
		}
	}

	return &Value{Raw: raw, Native: raw}, nil
}

func (r *Registry) Encode(val *Value, pgType string) (string, error) {
	if val.IsNull {
		return "\\N", nil
	}

	for _, c := range r.codecs {
		if c.CanHandle(pgType) {
			encoded, err := c.Encode(val.Native, pgType)
			if err != nil {
				return "", fmt.Errorf("encoding as %s: %w", pgType, err)
			}
			return encoded, nil
		}
	}

	if s, ok := val.Native.(string); ok {
		return s, nil
	}
	return fmt.Sprintf("%v", val.Native), nil
}
