package pipeline

import (
	"strings"
	"testing"

	_ "github.com/bojin/datamask/internal/transformer"
)

func TestTransformRow(t *testing.T) {
	columns := []string{"id", "email", "name", "age"}
	columnTypes := []string{"integer", "text", "text", "integer"}

	transforms := map[string]string{
		"email": "mask-email",
		"name":  "redact",
	}

	transformers, err := BuildTransformers(columns, transforms)
	if err != nil {
		t.Fatal(err)
	}

	line := "1\talice@example.com\tAlice\t30"
	result, err := TransformRow(line, columns, columnTypes, "users", transformers)
	if err != nil {
		t.Fatal(err)
	}

	fields := strings.Split(result, "\t")
	if fields[0] != "1" {
		t.Errorf("id should be preserved, got %q", fields[0])
	}
	if fields[1] == "alice@example.com" {
		t.Error("email should be masked")
	}
	if !strings.HasSuffix(fields[1], "@example.com") {
		t.Errorf("email domain should be preserved, got %q", fields[1])
	}
	if fields[2] != "***REDACTED***" {
		t.Errorf("name should be redacted, got %q", fields[2])
	}
	if fields[3] != "30" {
		t.Errorf("age should be preserved, got %q", fields[3])
	}
}

func TestTransformRowNullHandling(t *testing.T) {
	columns := []string{"id", "email"}
	columnTypes := []string{"integer", "text"}

	transforms := map[string]string{"email": "mask-email"}
	transformers, err := BuildTransformers(columns, transforms)
	if err != nil {
		t.Fatal(err)
	}

	line := "1\t\\N"
	result, err := TransformRow(line, columns, columnTypes, "users", transformers)
	if err != nil {
		t.Fatal(err)
	}

	fields := strings.Split(result, "\t")
	if fields[1] != "\\N" {
		t.Errorf("NULL should be preserved, got %q", fields[1])
	}
}

func TestBuildTransformersUnknown(t *testing.T) {
	columns := []string{"id", "email"}
	transforms := map[string]string{"email": "nonexistent-transformer"}

	_, err := BuildTransformers(columns, transforms)
	if err == nil {
		t.Error("expected error for unknown transformer")
	}
}
