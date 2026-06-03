package codec

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type IntCodec struct{}

func (c *IntCodec) CanHandle(pgType string) bool {
	switch normalizeType(pgType) {
	case "integer", "bigint", "smallint", "int4", "int8", "int2", "serial", "bigserial", "smallserial":
		return true
	}
	return false
}

func (c *IntCodec) Decode(raw string, pgType string) (interface{}, error) {
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (c *IntCodec) Encode(value interface{}, pgType string) (string, error) {
	switch v := value.(type) {
	case int64:
		return strconv.FormatInt(v, 10), nil
	case int:
		return strconv.Itoa(v), nil
	case float64:
		return strconv.FormatInt(int64(v), 10), nil
	case string:
		if _, err := strconv.ParseInt(v, 10, 64); err != nil {
			return "", fmt.Errorf("invalid integer value: %q", v)
		}
		return v, nil
	default:
		return fmt.Sprintf("%v", v), nil
	}
}

type FloatCodec struct{}

func (c *FloatCodec) CanHandle(pgType string) bool {
	switch normalizeType(pgType) {
	case "real", "double precision", "float4", "float8", "numeric", "decimal":
		return true
	}
	return false
}

func (c *FloatCodec) Decode(raw string, pgType string) (interface{}, error) {
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (c *FloatCodec) Encode(value interface{}, pgType string) (string, error) {
	switch v := value.(type) {
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case string:
		if _, err := strconv.ParseFloat(v, 64); err != nil {
			return "", fmt.Errorf("invalid numeric value: %q", v)
		}
		return v, nil
	default:
		return fmt.Sprintf("%v", v), nil
	}
}

type BoolCodec struct{}

func (c *BoolCodec) CanHandle(pgType string) bool {
	return normalizeType(pgType) == "boolean" || normalizeType(pgType) == "bool"
}

func (c *BoolCodec) Decode(raw string, pgType string) (interface{}, error) {
	switch strings.ToLower(raw) {
	case "t", "true", "1", "yes", "on":
		return true, nil
	case "f", "false", "0", "no", "off":
		return false, nil
	default:
		return nil, fmt.Errorf("invalid boolean: %q", raw)
	}
}

func (c *BoolCodec) Encode(value interface{}, pgType string) (string, error) {
	switch v := value.(type) {
	case bool:
		if v {
			return "t", nil
		}
		return "f", nil
	case string:
		switch strings.ToLower(v) {
		case "t", "true", "1", "yes", "on":
			return "t", nil
		case "f", "false", "0", "no", "off":
			return "f", nil
		default:
			return "", fmt.Errorf("invalid boolean value: %q", v)
		}
	default:
		return fmt.Sprintf("%v", v), nil
	}
}

type TimestampCodec struct{}

func (c *TimestampCodec) CanHandle(pgType string) bool {
	t := normalizeType(pgType)
	return t == "timestamp without time zone" || t == "timestamp with time zone" ||
		t == "timestamp" || t == "timestamptz"
}

func (c *TimestampCodec) Decode(raw string, pgType string) (interface{}, error) {
	formats := []string{
		"2006-01-02 15:04:05.999999-07",
		"2006-01-02 15:04:05.999999+07",
		"2006-01-02 15:04:05.999999",
		"2006-01-02 15:04:05-07",
		"2006-01-02 15:04:05+07",
		"2006-01-02 15:04:05",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, raw); err == nil {
			return t, nil
		}
	}
	return raw, nil
}

func (c *TimestampCodec) Encode(value interface{}, pgType string) (string, error) {
	switch v := value.(type) {
	case time.Time:
		if strings.Contains(pgType, "with time zone") || normalizeType(pgType) == "timestamptz" {
			return v.Format("2006-01-02 15:04:05.999999-07"), nil
		}
		return v.Format("2006-01-02 15:04:05.999999"), nil
	case string:
		return v, nil
	default:
		return fmt.Sprintf("%v", v), nil
	}
}

type DateCodec struct{}

func (c *DateCodec) CanHandle(pgType string) bool {
	return normalizeType(pgType) == "date"
}

func (c *DateCodec) Decode(raw string, pgType string) (interface{}, error) {
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return raw, nil
	}
	return t, nil
}

func (c *DateCodec) Encode(value interface{}, pgType string) (string, error) {
	switch v := value.(type) {
	case time.Time:
		return v.Format("2006-01-02"), nil
	case string:
		return v, nil
	default:
		return fmt.Sprintf("%v", v), nil
	}
}

type ByteaCodec struct{}

func (c *ByteaCodec) CanHandle(pgType string) bool {
	return normalizeType(pgType) == "bytea"
}

func (c *ByteaCodec) Decode(raw string, pgType string) (interface{}, error) {
	if strings.HasPrefix(raw, "\\x") {
		b, err := hex.DecodeString(raw[2:])
		if err != nil {
			return nil, err
		}
		return b, nil
	}
	return []byte(raw), nil
}

func (c *ByteaCodec) Encode(value interface{}, pgType string) (string, error) {
	switch v := value.(type) {
	case []byte:
		return "\\x" + hex.EncodeToString(v), nil
	case string:
		return v, nil
	default:
		return fmt.Sprintf("%v", v), nil
	}
}

type JSONCodec struct{}

func (c *JSONCodec) CanHandle(pgType string) bool {
	t := normalizeType(pgType)
	return t == "json" || t == "jsonb"
}

func (c *JSONCodec) Decode(raw string, pgType string) (interface{}, error) {
	var v json.RawMessage
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return raw, nil
	}
	return v, nil
}

func (c *JSONCodec) Encode(value interface{}, pgType string) (string, error) {
	switch v := value.(type) {
	case json.RawMessage:
		return string(v), nil
	case string:
		return v, nil
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
}

type TextCodec struct{}

func (c *TextCodec) CanHandle(pgType string) bool {
	return true
}

func (c *TextCodec) Decode(raw string, pgType string) (interface{}, error) {
	return raw, nil
}

func (c *TextCodec) Encode(value interface{}, pgType string) (string, error) {
	if s, ok := value.(string); ok {
		return s, nil
	}
	return fmt.Sprintf("%v", value), nil
}

func normalizeType(pgType string) string {
	t := strings.ToLower(strings.TrimSpace(pgType))
	if idx := strings.Index(t, "("); idx > 0 {
		base := t[:idx]
		suffix := ""
		if closeParen := strings.Index(t, ")"); closeParen > 0 && closeParen < len(t)-1 {
			suffix = strings.TrimSpace(t[closeParen+1:])
		}
		if suffix != "" {
			return base + " " + suffix
		}
		return base
	}
	return t
}
