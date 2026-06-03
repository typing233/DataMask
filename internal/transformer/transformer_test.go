package transformer

import (
	"strings"
	"testing"
)

func TestMaskEmail(t *testing.T) {
	tx, err := Get("mask-email")
	if err != nil {
		t.Fatal(err)
	}

	col := ColumnInfo{TableName: "users", ColumnName: "email"}

	result, err := tx.Transform("alice@example.com", col)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(result, "@example.com") {
		t.Errorf("expected domain preserved, got %q", result)
	}
	if strings.HasPrefix(result, "alice@") {
		t.Errorf("expected local part masked, got %q", result)
	}

	// Deterministic
	result2, _ := tx.Transform("alice@example.com", col)
	if result != result2 {
		t.Errorf("expected deterministic output, got %q and %q", result, result2)
	}
}

func TestMaskName(t *testing.T) {
	tx, err := Get("mask-name")
	if err != nil {
		t.Fatal(err)
	}

	col := ColumnInfo{TableName: "users", ColumnName: "name"}

	result, err := tx.Transform("Alice Wonderland", col)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, " ") {
		t.Errorf("expected first+last name, got %q", result)
	}

	// Deterministic
	result2, _ := tx.Transform("Alice Wonderland", col)
	if result != result2 {
		t.Errorf("expected deterministic, got %q and %q", result, result2)
	}
}

func TestRedact(t *testing.T) {
	tx, err := Get("redact")
	if err != nil {
		t.Fatal(err)
	}

	result, err := tx.Transform("sensitive data", ColumnInfo{})
	if err != nil {
		t.Fatal(err)
	}
	if result != "***REDACTED***" {
		t.Errorf("expected ***REDACTED***, got %q", result)
	}
}

func TestHashSHA256(t *testing.T) {
	tx, err := Get("hash-sha256")
	if err != nil {
		t.Fatal(err)
	}

	result, err := tx.Transform("hello", ColumnInfo{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 64 {
		t.Errorf("expected 64 hex chars, got %d: %q", len(result), result)
	}

	// Deterministic
	result2, _ := tx.Transform("hello", ColumnInfo{})
	if result != result2 {
		t.Errorf("not deterministic")
	}
}

func TestRandomInt(t *testing.T) {
	tx, err := Get("random-int")
	if err != nil {
		t.Fatal(err)
	}

	result, err := tx.Transform("42", ColumnInfo{})
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestPreserve(t *testing.T) {
	tx, err := Get("preserve")
	if err != nil {
		t.Fatal(err)
	}

	result, err := tx.Transform("keep me", ColumnInfo{})
	if err != nil {
		t.Fatal(err)
	}
	if result != "keep me" {
		t.Errorf("expected 'keep me', got %q", result)
	}
}

func TestRegistryList(t *testing.T) {
	names := List()
	expected := []string{"hash-sha256", "mask-email", "mask-name", "preserve", "random-int", "redact"}
	if len(names) != len(expected) {
		t.Fatalf("expected %d transformers, got %d: %v", len(expected), len(names), names)
	}
	for i, name := range expected {
		if names[i] != name {
			t.Errorf("position %d: expected %q, got %q", i, name, names[i])
		}
	}
}

func TestGetUnknown(t *testing.T) {
	_, err := Get("nonexistent")
	if err == nil {
		t.Error("expected error for unknown transformer")
	}
}
