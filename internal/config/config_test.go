package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	content := `
storage_dir: /tmp/test_dumps
parallelism: 8

connection:
  host: localhost
  port: 5432
  user: testuser
  password: testpass
  dbname: testdb
  sslmode: disable

exclude_tables:
  - migrations

tables:
  users:
    columns:
      email: mask-email
      name: mask-name
  logs:
    exclude: true
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.StorageDir != "/tmp/test_dumps" {
		t.Errorf("storage_dir: got %q", cfg.StorageDir)
	}
	if cfg.Parallelism != 8 {
		t.Errorf("parallelism: got %d", cfg.Parallelism)
	}
	if cfg.Connection.DBName != "testdb" {
		t.Errorf("dbname: got %q", cfg.Connection.DBName)
	}

	dsn := cfg.Connection.DSN()
	if dsn != "host=localhost port=5432 user=testuser password=testpass dbname=testdb sslmode=disable" {
		t.Errorf("DSN: got %q", dsn)
	}
}

func TestShouldIncludeTable(t *testing.T) {
	cfg := &Config{
		ExcludeTables: []string{"migrations", "sessions"},
		Tables: map[string]TableConfig{
			"logs": {Exclude: true},
		},
	}

	tests := []struct {
		schema, table string
		want          bool
	}{
		{"public", "users", true},
		{"public", "migrations", false},
		{"public", "sessions", false},
		{"public", "logs", false},
		{"public", "orders", true},
	}

	for _, tt := range tests {
		got := cfg.ShouldIncludeTable(tt.schema, tt.table)
		if got != tt.want {
			t.Errorf("ShouldIncludeTable(%q, %q) = %v, want %v", tt.schema, tt.table, got, tt.want)
		}
	}
}

func TestShouldIncludeTableWithIncludeList(t *testing.T) {
	cfg := &Config{
		IncludeTables: []string{"users", "orders"},
	}

	if !cfg.ShouldIncludeTable("public", "users") {
		t.Error("users should be included")
	}
	if cfg.ShouldIncludeTable("public", "logs") {
		t.Error("logs should not be included")
	}
}

func TestDefaults(t *testing.T) {
	content := `
connection:
  host: localhost
  dbname: mydb
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Parallelism != 4 {
		t.Errorf("default parallelism: got %d, want 4", cfg.Parallelism)
	}
	if cfg.StorageDir != "./dumps" {
		t.Errorf("default storage_dir: got %q, want ./dumps", cfg.StorageDir)
	}
}
