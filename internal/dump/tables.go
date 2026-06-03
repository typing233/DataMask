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

	// Single query that discovers tables AND their columns together,
	// avoiding the need for per-table queries on the same connection.
	query := `
		SELECT
			n.nspname,
			c.relname,
			a.attname,
			format_type(a.atttypid, a.atttypmod)
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_attribute a ON a.attrelid = c.oid
		WHERE c.relkind = 'r'
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
		  AND a.attnum > 0
		  AND NOT a.attisdropped
		ORDER BY n.nspname, c.relname, a.attnum`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying tables and columns: %w", err)
	}
	defer rows.Close()

	type tableKey struct{ schema, name string }
	tableOrder := []tableKey{}
	tableColumns := make(map[tableKey]*TableInfo)

	for rows.Next() {
		var schema, tableName, colName, colType string
		if err := rows.Scan(&schema, &tableName, &colName, &colType); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}

		key := tableKey{schema, tableName}
		if _, exists := tableColumns[key]; !exists {
			tableColumns[key] = &TableInfo{
				Schema: schema,
				Name:   tableName,
			}
			tableOrder = append(tableOrder, key)
		}
		tableColumns[key].Columns = append(tableColumns[key].Columns, colName)
		tableColumns[key].ColumnTypes = append(tableColumns[key].ColumnTypes, colType)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %w", err)
	}

	var tables []TableInfo
	for _, key := range tableOrder {
		tbl := tableColumns[key]
		if !d.cfg.ShouldIncludeTable(tbl.Schema, tbl.Name) {
			continue
		}
		tables = append(tables, *tbl)
	}

	return tables, nil
}
