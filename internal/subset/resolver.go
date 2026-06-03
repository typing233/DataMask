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
	db      database.Database
	plan    *SubsetPlan
	config  SubsetConfig
	visited map[string]map[string]bool // table -> set of composite key values already fetched
	results map[string]*bytes.Buffer   // table -> collected row data
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
	// Phase 1: Extract seed tables
	for tableName, whereClause := range r.plan.SeedTables {
		if err := r.extractTable(ctx, tableName, whereClause); err != nil {
			return nil, fmt.Errorf("extracting seed table %s: %w", tableName, err)
		}
	}

	// Phase 2: Unified BFS resolving both parents and children to fixed point
	if r.config.ResolveParents || r.config.ResolveChildren {
		if err := r.resolveToFixedPoint(ctx); err != nil {
			return nil, fmt.Errorf("resolving dependencies: %w", err)
		}
	}

	return r.results, nil
}

type resolveDir int

const (
	dirParent resolveDir = iota
	dirChild
)

func (r *Resolver) resolveToFixedPoint(ctx context.Context) error {
	iteration := 0
	for iteration < r.config.MaxDepth {
		newDataFound := false

		// Snapshot current tables with data to iterate over
		tablesToProcess := make([]string, 0)
		for tableName, buf := range r.results {
			if buf.Len() > 0 {
				tablesToProcess = append(tablesToProcess, tableName)
			}
		}

		for _, tableName := range tablesToProcess {
			// Resolve parent dependencies
			if r.config.ResolveParents {
				for _, dep := range r.plan.ParentDeps[tableName] {
					targetTable := dep.ToTable
					where := r.buildDependencyWhere(tableName, dep, dirParent)
					if where == "" {
						continue
					}

					prevLen := r.bufferLen(targetTable)
					if err := r.extractTable(ctx, targetTable, where); err != nil {
						return fmt.Errorf("resolving parent %s for %s: %w", targetTable, tableName, err)
					}
					if r.bufferLen(targetTable) > prevLen {
						newDataFound = true
					}
				}
			}

			// Resolve child dependencies
			if r.config.ResolveChildren {
				for _, dep := range r.plan.ChildDeps[tableName] {
					targetTable := dep.FromTable
					where := r.buildDependencyWhere(tableName, dep, dirChild)
					if where == "" {
						continue
					}

					prevLen := r.bufferLen(targetTable)
					if err := r.extractTable(ctx, targetTable, where); err != nil {
						return fmt.Errorf("resolving child %s for %s: %w", targetTable, tableName, err)
					}
					if r.bufferLen(targetTable) > prevLen {
						newDataFound = true
					}
				}
			}
		}

		iteration++
		if !newDataFound {
			break
		}
	}
	return nil
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
	if len(cols) == 0 {
		return fmt.Errorf("no column info for table %s", tableName)
	}
	colNames := make([]string, len(cols))
	for i, c := range cols {
		colNames[i] = c.Name
	}

	var tmpBuf bytes.Buffer
	_, err := r.db.QueryRows(ctx, schema, table, colNames, whereClause, &tmpBuf)
	if err != nil {
		return err
	}

	// Deduplicate: only add rows we haven't seen before
	buf := r.getBuffer(tableName)
	visitedSet := r.getVisitedSet(tableName)

	lines := strings.Split(tmpBuf.String(), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		rowKey := line // use full row as dedup key
		if !visitedSet[rowKey] {
			visitedSet[rowKey] = true
			buf.WriteString(line)
			buf.WriteByte('\n')
		}
	}
	return nil
}

// buildDependencyWhere builds a WHERE clause to fetch related rows via FK.
// Supports compound (multi-column) foreign keys.
func (r *Resolver) buildDependencyWhere(sourceTable string, dep Dependency, dir resolveDir) string {
	buf, exists := r.results[sourceTable]
	if !exists || buf.Len() == 0 {
		return ""
	}

	sourceCols := r.plan.TableColumns[sourceTable]

	// Determine which columns to extract from source and which to match in target
	var sourceColNames, targetColNames []string
	if dir == dirParent {
		// Source table has FK columns, target table has referenced columns
		sourceColNames = dep.FromColumns
		targetColNames = dep.ToColumns
	} else {
		// Source table has the referenced columns, target table has the FK columns
		sourceColNames = dep.ToColumns
		targetColNames = dep.FromColumns
	}

	// Find column indices in source table
	colIndices := make([]int, len(sourceColNames))
	for i, colName := range sourceColNames {
		colIndices[i] = -1
		for j, c := range sourceCols {
			if c.Name == colName {
				colIndices[i] = j
				break
			}
		}
		if colIndices[i] < 0 {
			return ""
		}
	}

	// Extract distinct value tuples from source data
	lines := strings.Split(buf.String(), "\n")

	if len(sourceColNames) == 1 {
		// Single-column FK: use simple IN clause
		values := make(map[string]bool)
		idx := colIndices[0]
		for _, line := range lines {
			if line == "" {
				continue
			}
			fields := strings.Split(line, "\t")
			if idx < len(fields) && fields[idx] != "\\N" {
				values[fields[idx]] = true
			}
		}
		if len(values) == 0 {
			return ""
		}

		quoted := make([]string, 0, len(values))
		for v := range values {
			quoted = append(quoted, "'"+strings.ReplaceAll(v, "'", "''")+"'")
		}
		return fmt.Sprintf("%s IN (%s)", quoteIdent(targetColNames[0]), strings.Join(quoted, ","))
	}

	// Compound FK: use (col1, col2, ...) IN ((v1, v2), ...) or OR conditions
	type tuple []string
	tupleSet := make(map[string]bool)
	var tuples []tuple

	for _, line := range lines {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		t := make(tuple, len(colIndices))
		allValid := true
		for i, idx := range colIndices {
			if idx >= len(fields) || fields[idx] == "\\N" {
				allValid = false
				break
			}
			t[i] = fields[idx]
		}
		if !allValid {
			continue
		}
		key := strings.Join(t, "\x00")
		if !tupleSet[key] {
			tupleSet[key] = true
			tuples = append(tuples, t)
		}
	}

	if len(tuples) == 0 {
		return ""
	}

	// Build: (col1, col2) IN ((v1a, v2a), (v1b, v2b), ...)
	colList := make([]string, len(targetColNames))
	for i, c := range targetColNames {
		colList[i] = quoteIdent(c)
	}

	valueRows := make([]string, 0, len(tuples))
	for _, t := range tuples {
		vals := make([]string, len(t))
		for i, v := range t {
			vals[i] = "'" + strings.ReplaceAll(v, "'", "''") + "'"
		}
		valueRows = append(valueRows, "("+strings.Join(vals, ",")+")")
	}

	return fmt.Sprintf("(%s) IN (%s)", strings.Join(colList, ","), strings.Join(valueRows, ","))
}

func (r *Resolver) getBuffer(tableName string) *bytes.Buffer {
	if buf, ok := r.results[tableName]; ok {
		return buf
	}
	buf := &bytes.Buffer{}
	r.results[tableName] = buf
	return buf
}

func (r *Resolver) getVisitedSet(tableName string) map[string]bool {
	if s, ok := r.visited[tableName]; ok {
		return s
	}
	s := make(map[string]bool)
	r.visited[tableName] = s
	return s
}

func (r *Resolver) hasData(tableName string) bool {
	buf, ok := r.results[tableName]
	return ok && buf.Len() > 0
}

func (r *Resolver) bufferLen(tableName string) int {
	buf, ok := r.results[tableName]
	if !ok {
		return 0
	}
	return buf.Len()
}

func (r *Resolver) GetTableData(tableName string) io.Reader {
	if buf, ok := r.results[tableName]; ok {
		return bytes.NewReader(buf.Bytes())
	}
	return strings.NewReader("")
}

func quoteIdent(name string) string {
	return "\"" + strings.ReplaceAll(name, "\"", "\"\"") + "\""
}
