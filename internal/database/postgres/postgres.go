package postgres

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/bojin/datamask/internal/database"
	"github.com/jackc/pgx/v5"
)

func init() {
	database.RegisterDriver("postgres", func() database.Database {
		return &PostgresDB{}
	})
}

type PostgresDB struct {
	conn *pgx.Conn
	dsn  string
}

func (p *PostgresDB) Connect(ctx context.Context, dsn string) error {
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connecting to PostgreSQL: %w", err)
	}
	p.conn = conn
	p.dsn = dsn
	return nil
}

func (p *PostgresDB) Close(ctx context.Context) error {
	if p.conn != nil {
		return p.conn.Close(ctx)
	}
	return nil
}

func (p *PostgresDB) DiscoverTables(ctx context.Context) ([]database.TableInfo, error) {
	query := `
		SELECT
			n.nspname,
			c.relname,
			a.attname,
			format_type(a.atttypid, a.atttypmod),
			a.attnum,
			NOT a.attnotnull
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_attribute a ON a.attrelid = c.oid
		WHERE c.relkind = 'r'
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
		  AND a.attnum > 0
		  AND NOT a.attisdropped
		ORDER BY n.nspname, c.relname, a.attnum`

	rows, err := p.conn.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying tables: %w", err)
	}
	defer rows.Close()

	type tableKey struct{ schema, name string }
	tableOrder := []tableKey{}
	tableMap := make(map[tableKey]*database.TableInfo)

	for rows.Next() {
		var schema, tableName, colName, colType string
		var position int16
		var nullable bool
		if err := rows.Scan(&schema, &tableName, &colName, &colType, &position, &nullable); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}

		key := tableKey{schema, tableName}
		if _, exists := tableMap[key]; !exists {
			tableMap[key] = &database.TableInfo{
				Schema: schema,
				Name:   tableName,
			}
			tableOrder = append(tableOrder, key)
		}
		tableMap[key].Columns = append(tableMap[key].Columns, database.ColumnInfo{
			Name:     colName,
			DataType: colType,
			Position: int(position),
			Nullable: nullable,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %w", err)
	}

	tables := make([]database.TableInfo, 0, len(tableOrder))
	for _, key := range tableOrder {
		tables = append(tables, *tableMap[key])
	}
	return tables, nil
}

func (p *PostgresDB) DiscoverForeignKeys(ctx context.Context) ([]database.ForeignKey, error) {
	query := `
		SELECT
			con.conname,
			nsp.nspname AS from_schema,
			cls.relname AS from_table,
			att.attname AS from_column,
			ref_nsp.nspname AS to_schema,
			ref_cls.relname AS to_table,
			ref_att.attname AS to_column,
			u.pos
		FROM pg_constraint con
		JOIN pg_class cls ON cls.oid = con.conrelid
		JOIN pg_namespace nsp ON nsp.oid = cls.relnamespace
		JOIN pg_class ref_cls ON ref_cls.oid = con.confrelid
		JOIN pg_namespace ref_nsp ON ref_nsp.oid = ref_cls.relnamespace
		CROSS JOIN LATERAL unnest(con.conkey, con.confkey) WITH ORDINALITY AS u(from_attnum, to_attnum, pos)
		JOIN pg_attribute att ON att.attrelid = con.conrelid AND att.attnum = u.from_attnum
		JOIN pg_attribute ref_att ON ref_att.attrelid = con.confrelid AND ref_att.attnum = u.to_attnum
		WHERE con.contype = 'f'
		  AND nsp.nspname NOT IN ('pg_catalog', 'information_schema')
		ORDER BY con.conname, u.pos`

	rows, err := p.conn.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying foreign keys: %w", err)
	}
	defer rows.Close()

	type fkKey struct{ name string }
	fkOrder := []string{}
	fkMap := make(map[string]*database.ForeignKey)

	for rows.Next() {
		var conName, fromSchema, fromTable, fromCol, toSchema, toTable, toCol string
		var pos int64
		if err := rows.Scan(&conName, &fromSchema, &fromTable, &fromCol, &toSchema, &toTable, &toCol, &pos); err != nil {
			return nil, fmt.Errorf("scanning FK row: %w", err)
		}

		if _, exists := fkMap[conName]; !exists {
			fkMap[conName] = &database.ForeignKey{
				ConstraintName: conName,
				FromSchema:     fromSchema,
				FromTable:      fromTable,
				ToSchema:       toSchema,
				ToTable:        toTable,
			}
			fkOrder = append(fkOrder, conName)
		}
		fk := fkMap[conName]
		fk.FromColumns = append(fk.FromColumns, fromCol)
		fk.ToColumns = append(fk.ToColumns, toCol)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating FK rows: %w", err)
	}

	result := make([]database.ForeignKey, 0, len(fkOrder))
	for _, name := range fkOrder {
		result = append(result, *fkMap[name])
	}
	return result, nil
}

func (p *PostgresDB) ExportSchema(ctx context.Context, outputPath string) error {
	if _, err := exec.LookPath("pg_dump"); err != nil {
		return fmt.Errorf("pg_dump not found in PATH: %w", err)
	}

	cmd := exec.CommandContext(ctx, "pg_dump",
		"--format=custom",
		"--schema-only",
		"--no-owner",
		"--no-privileges",
		"--file", outputPath,
		"--dbname", p.dsn,
	)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (p *PostgresDB) RestoreSchema(ctx context.Context, schemaPath, targetDSN string) error {
	if _, err := exec.LookPath("pg_restore"); err != nil {
		return fmt.Errorf("pg_restore not found in PATH: %w", err)
	}

	cmd := exec.CommandContext(ctx, "pg_restore",
		"--schema-only",
		"--no-owner",
		"--no-privileges",
		"--dbname", targetDSN,
		schemaPath,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (p *PostgresDB) CopyTo(ctx context.Context, schema, table string, columns []string, w io.Writer) (int64, error) {
	conn, err := pgx.Connect(ctx, p.dsn)
	if err != nil {
		return 0, fmt.Errorf("connecting for COPY TO: %w", err)
	}
	defer conn.Close(ctx)

	qualifiedTable := fmt.Sprintf("%q.%q", schema, table)
	copySQL := fmt.Sprintf("COPY %s TO STDOUT", qualifiedTable)

	rawConn := conn.PgConn()
	result, err := rawConn.CopyTo(ctx, w, copySQL)
	if err != nil {
		return 0, fmt.Errorf("COPY TO: %w", err)
	}
	return result.RowsAffected(), nil
}

func (p *PostgresDB) CopyFrom(ctx context.Context, schema, table string, r io.Reader) (int64, error) {
	conn, err := pgx.Connect(ctx, p.dsn)
	if err != nil {
		return 0, fmt.Errorf("connecting for COPY FROM: %w", err)
	}
	defer conn.Close(ctx)

	qualifiedTable := fmt.Sprintf("%q.%q", schema, table)
	copySQL := fmt.Sprintf("COPY %s FROM STDIN", qualifiedTable)

	rawConn := conn.PgConn()
	result, err := rawConn.CopyFrom(ctx, r, copySQL)
	if err != nil {
		return 0, fmt.Errorf("COPY FROM: %w", err)
	}
	return result.RowsAffected(), nil
}

func (p *PostgresDB) QueryRows(ctx context.Context, schema, table string, columns []string, whereClause string, w io.Writer) (int64, error) {
	conn, err := pgx.Connect(ctx, p.dsn)
	if err != nil {
		return 0, fmt.Errorf("connecting for query: %w", err)
	}
	defer conn.Close(ctx)

	qualifiedTable := fmt.Sprintf("%q.%q", schema, table)
	copySQL := fmt.Sprintf("COPY (SELECT * FROM %s WHERE %s) TO STDOUT", qualifiedTable, whereClause)

	rawConn := conn.PgConn()
	result, err := rawConn.CopyTo(ctx, w, copySQL)
	if err != nil {
		return 0, fmt.Errorf("COPY (query) TO: %w", err)
	}
	return result.RowsAffected(), nil
}

func (p *PostgresDB) ServerVersion(ctx context.Context) string {
	if p.conn == nil {
		return ""
	}
	var version string
	if err := p.conn.QueryRow(ctx, "SHOW server_version").Scan(&version); err != nil {
		return ""
	}
	return version
}
