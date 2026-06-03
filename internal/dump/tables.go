package dump

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type TableInfo struct {
	Schema      string
	Name        string
	Columns     []string
	ColumnTypes []string
}

func (t TableInfo) FullName() string {
	if t.Schema == "" || t.Schema == "public" {
		return t.Name
	}
	return t.Schema + "." + t.Name
}

func (d *Dumper) discoverTables(ctx context.Context) ([]TableInfo, error) {
	conn, err := pgx.Connect(ctx, d.dsn)
	if err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}
	defer conn.Close(ctx)

	query := `
		SELECT
			n.nspname AS schema_name,
			c.relname AS table_name
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind = 'r'
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
		ORDER BY n.nspname, c.relname`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying tables: %w", err)
	}
	defer rows.Close()

	var tables []TableInfo
	for rows.Next() {
		var schema, name string
		if err := rows.Scan(&schema, &name); err != nil {
			return nil, fmt.Errorf("scanning table row: %w", err)
		}

		if !d.cfg.ShouldIncludeTable(schema, name) {
			continue
		}

		cols, colTypes, err := d.getColumns(ctx, conn, schema, name)
		if err != nil {
			return nil, fmt.Errorf("getting columns for %s.%s: %w", schema, name, err)
		}

		tables = append(tables, TableInfo{
			Schema:      schema,
			Name:        name,
			Columns:     cols,
			ColumnTypes: colTypes,
		})
	}

	return tables, rows.Err()
}

func (d *Dumper) getColumns(ctx context.Context, conn *pgx.Conn, schema, table string) ([]string, []string, error) {
	query := `
		SELECT
			a.attname,
			format_type(a.atttypid, a.atttypmod)
		FROM pg_attribute a
		JOIN pg_class c ON c.oid = a.attrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1
		  AND c.relname = $2
		  AND a.attnum > 0
		  AND NOT a.attisdropped
		ORDER BY a.attnum`

	rows, err := conn.Query(ctx, query, schema, table)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var columns, columnTypes []string
	for rows.Next() {
		var col, colType string
		if err := rows.Scan(&col, &colType); err != nil {
			return nil, nil, err
		}
		columns = append(columns, col)
		columnTypes = append(columnTypes, colType)
	}

	return columns, columnTypes, rows.Err()
}
