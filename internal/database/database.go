package database

import (
	"context"
	"io"
)

type TableInfo struct {
	Schema      string
	Name        string
	Columns     []ColumnInfo
}

func (t TableInfo) FullName() string {
	if t.Schema == "" || t.Schema == "public" {
		return t.Name
	}
	return t.Schema + "." + t.Name
}

type ColumnInfo struct {
	Name     string
	DataType string
	Position int
	Nullable bool
}

type ForeignKey struct {
	ConstraintName string
	FromSchema     string
	FromTable      string
	FromColumns    []string
	ToSchema       string
	ToTable        string
	ToColumns      []string
}

func (fk ForeignKey) FromFullName() string {
	if fk.FromSchema == "" || fk.FromSchema == "public" {
		return fk.FromTable
	}
	return fk.FromSchema + "." + fk.FromTable
}

func (fk ForeignKey) ToFullName() string {
	if fk.ToSchema == "" || fk.ToSchema == "public" {
		return fk.ToTable
	}
	return fk.ToSchema + "." + fk.ToTable
}

type Database interface {
	Connect(ctx context.Context, dsn string) error
	Close(ctx context.Context) error

	DiscoverTables(ctx context.Context) ([]TableInfo, error)
	DiscoverForeignKeys(ctx context.Context) ([]ForeignKey, error)

	ExportSchema(ctx context.Context, outputPath string) error
	RestoreSchema(ctx context.Context, schemaPath, targetDSN string) error

	CopyTo(ctx context.Context, schema, table string, columns []string, w io.Writer) (int64, error)
	CopyFrom(ctx context.Context, schema, table string, r io.Reader) (int64, error)

	QueryRows(ctx context.Context, schema, table string, columns []string, whereClause string, w io.Writer) (int64, error)

	ServerVersion(ctx context.Context) string
}
