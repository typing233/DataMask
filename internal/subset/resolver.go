package subset

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/bojin/datamask/internal/database"
)

type Resolver struct {
	db       database.Database
	plan     *SubsetPlan
	config   SubsetConfig
	visited  map[string]map[string]bool // table -> set of PK values already included
	results  map[string]*bytes.Buffer   // table -> collected row data
}

func NewResolver(db database.Database, plan *SubsetPlan, config SubsetConfig) *Resolver {
	return &Resolver{
		db:      db,
		plan:    plan,
		config:  config,
		visited: make(map[string]map[string]bool),
		results: make(map[string]*bytes.Buffer),
	}
}

func (r *Resolver) Resolve(ctx context.Context) (map[string]*bytes.Buffer, error) {
	for tableName, whereClause := range r.plan.SeedTables {
		if err := r.extractTable(ctx, tableName, whereClause); err != nil {
			return nil, fmt.Errorf("extracting seed table %s: %w", tableName, err)
		}
	}

	if r.config.ResolveParents {
		if err := r.resolveParents(ctx); err != nil {
			return nil, fmt.Errorf("resolving parent dependencies: %w", err)
		}
	}

	return r.results, nil
}

func (r *Resolver) extractTable(ctx context.Context, tableName, whereClause string) error {
	parts := strings.SplitN(tableName, ".", 2)
	schema := "public"
	table := tableName
	if len(parts) == 2 {
		schema = parts[0]
		table = parts[1]
	}

	cols := r.plan.TableColumns[tableName]
	colNames := make([]string, len(cols))
	for i, c := range cols {
		colNames[i] = c.Name
	}

	buf := r.getBuffer(tableName)
	_, err := r.db.QueryRows(ctx, schema, table, colNames, whereClause, buf)
	if err != nil {
		return err
	}
	return nil
}

func (r *Resolver) resolveParents(ctx context.Context) error {
	depth := 0
	pendingTables := make([]string, 0, len(r.plan.SeedTables))
	for t := range r.plan.SeedTables {
		pendingTables = append(pendingTables, t)
	}

	for len(pendingTables) > 0 && depth < r.config.MaxDepth {
		var nextPending []string
		for _, tableName := range pendingTables {
			deps := r.plan.Dependencies[tableName]
			for _, dep := range deps {
				if _, exists := r.results[dep.ToTable]; exists {
					continue
				}
				where := r.buildParentWhere(tableName, dep)
				if where == "" {
					continue
				}
				if err := r.extractTable(ctx, dep.ToTable, where); err != nil {
					continue
				}
				nextPending = append(nextPending, dep.ToTable)
			}
		}
		pendingTables = nextPending
		depth++
	}
	return nil
}

func (r *Resolver) buildParentWhere(childTable string, dep Dependency) string {
	buf, exists := r.results[childTable]
	if !exists || buf.Len() == 0 {
		return ""
	}

	cols := r.plan.TableColumns[childTable]
	colIndex := -1
	for i, c := range cols {
		if c.Name == dep.FromColumns[0] {
			colIndex = i
			break
		}
	}
	if colIndex < 0 {
		return ""
	}

	values := make(map[string]bool)
	lines := strings.Split(buf.String(), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if colIndex < len(fields) && fields[colIndex] != "\\N" {
			values[fields[colIndex]] = true
		}
	}

	if len(values) == 0 {
		return ""
	}

	quoted := make([]string, 0, len(values))
	for v := range values {
		quoted = append(quoted, fmt.Sprintf("'%s'", strings.ReplaceAll(v, "'", "''")))
	}

	return fmt.Sprintf("%s IN (%s)", dep.ToColumns[0], strings.Join(quoted, ","))
}

func (r *Resolver) getBuffer(tableName string) *bytes.Buffer {
	if buf, ok := r.results[tableName]; ok {
		return buf
	}
	buf := &bytes.Buffer{}
	r.results[tableName] = buf
	return buf
}

func (r *Resolver) GetTableData(tableName string) io.Reader {
	if buf, ok := r.results[tableName]; ok {
		return bytes.NewReader(buf.Bytes())
	}
	return strings.NewReader("")
}
