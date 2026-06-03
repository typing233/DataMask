package storage

import "time"

type DumpMetadata struct {
	ID         string        `json:"id"`
	CreatedAt  time.Time     `json:"created_at"`
	SourceDB   string        `json:"source_db"`
	Tables     []TableMeta   `json:"tables"`
	SchemaFile string        `json:"schema_file"`
	Duration   time.Duration `json:"duration_ns"`

	Description    string   `json:"description,omitempty"`
	Version        string   `json:"version,omitempty"`
	PGVersion      string   `json:"pg_version,omitempty"`
	TotalRows      int64    `json:"total_rows,omitempty"`
	TotalSize      int64    `json:"total_size_bytes,omitempty"`
	Tags           []string `json:"tags,omitempty"`
	ConfigSnapshot string   `json:"config_snapshot,omitempty"`
}

type TableMeta struct {
	Schema         string            `json:"schema"`
	Name           string            `json:"name"`
	RowCount       int64             `json:"row_count"`
	Columns        []string          `json:"columns"`
	ColumnTypes    []string          `json:"column_types"`
	Transformers   map[string]string `json:"transformers"`
	DataFile       string            `json:"data_file"`
	CompressedSize int64             `json:"compressed_size_bytes"`
	DumpDuration   time.Duration     `json:"dump_duration_ns,omitempty"`
	OriginalSize   int64             `json:"original_size_bytes,omitempty"`
}

func (t TableMeta) FullName() string {
	if t.Schema == "" || t.Schema == "public" {
		return t.Name
	}
	return t.Schema + "." + t.Name
}
