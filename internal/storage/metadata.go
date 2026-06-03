package storage

import "time"

type DumpMetadata struct {
	ID         string        `json:"id"`
	CreatedAt  time.Time     `json:"created_at"`
	SourceDB   string        `json:"source_db"`
	Tables     []TableMeta   `json:"tables"`
	SchemaFile string        `json:"schema_file"`
	Duration   time.Duration `json:"duration_ns"`
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
}

func (t TableMeta) FullName() string {
	if t.Schema == "" || t.Schema == "public" {
		return t.Name
	}
	return t.Schema + "." + t.Name
}
