package codec

import (
	"testing"
	"time"
)

func TestIntCodec(t *testing.T) {
	c := &IntCodec{}

	if !c.CanHandle("integer") {
		t.Error("should handle integer")
	}
	if !c.CanHandle("bigint") {
		t.Error("should handle bigint")
	}
	if c.CanHandle("text") {
		t.Error("should not handle text")
	}

	val, err := c.Decode("42", "integer")
	if err != nil {
		t.Fatal(err)
	}
	if val.(int64) != 42 {
		t.Errorf("expected 42, got %v", val)
	}

	encoded, err := c.Encode(int64(42), "integer")
	if err != nil {
		t.Fatal(err)
	}
	if encoded != "42" {
		t.Errorf("expected '42', got %q", encoded)
	}
}

func TestBoolCodec(t *testing.T) {
	c := &BoolCodec{}

	val, err := c.Decode("t", "boolean")
	if err != nil {
		t.Fatal(err)
	}
	if val.(bool) != true {
		t.Error("expected true")
	}

	val, err = c.Decode("f", "boolean")
	if err != nil {
		t.Fatal(err)
	}
	if val.(bool) != false {
		t.Error("expected false")
	}

	encoded, err := c.Encode(true, "boolean")
	if err != nil {
		t.Fatal(err)
	}
	if encoded != "t" {
		t.Errorf("expected 't', got %q", encoded)
	}
}

func TestTimestampCodec(t *testing.T) {
	c := &TimestampCodec{}

	if !c.CanHandle("timestamp without time zone") {
		t.Error("should handle timestamp without time zone")
	}
	if !c.CanHandle("timestamptz") {
		t.Error("should handle timestamptz")
	}

	val, err := c.Decode("2024-01-15 10:30:00", "timestamp without time zone")
	if err != nil {
		t.Fatal(err)
	}
	ts, ok := val.(time.Time)
	if !ok {
		t.Fatal("expected time.Time")
	}
	if ts.Year() != 2024 || ts.Month() != 1 || ts.Day() != 15 {
		t.Errorf("unexpected time: %v", ts)
	}

	encoded, err := c.Encode(ts, "timestamp without time zone")
	if err != nil {
		t.Fatal(err)
	}
	if encoded != "2024-01-15 10:30:00" {
		t.Errorf("expected '2024-01-15 10:30:00', got %q", encoded)
	}
}

func TestRegistryNull(t *testing.T) {
	r := NewPostgresRegistry()
	val, err := r.Decode("\\N", "integer")
	if err != nil {
		t.Fatal(err)
	}
	if !val.IsNull {
		t.Error("expected null")
	}

	encoded, err := r.Encode(val, "integer")
	if err != nil {
		t.Fatal(err)
	}
	if encoded != "\\N" {
		t.Errorf("expected '\\N', got %q", encoded)
	}
}

func TestFloatCodec(t *testing.T) {
	c := &FloatCodec{}

	val, err := c.Decode("3.14", "numeric")
	if err != nil {
		t.Fatal(err)
	}
	if val.(float64) != 3.14 {
		t.Errorf("expected 3.14, got %v", val)
	}

	encoded, err := c.Encode(3.14, "numeric")
	if err != nil {
		t.Fatal(err)
	}
	if encoded != "3.14" {
		t.Errorf("expected '3.14', got %q", encoded)
	}
}
