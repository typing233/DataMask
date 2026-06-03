package subset

import (
	"context"
	"fmt"

	"github.com/bojin/datamask/internal/database"
	"github.com/bojin/datamask/internal/depgraph"
)

type SubsetConfig struct {
	Tables          map[string]TableSubset
	ResolveParents  bool
	ResolveChildren bool
	MaxDepth        int
}

type TableSubset struct {
	Where string
}

type SubsetResult struct {
	Tables []TableResult
}

type TableResult struct {
	Schema   string
	Table    string
	RowCount int64
}

type Extractor struct {
	db     database.Database
	config SubsetConfig
	fks    []database.ForeignKey
	graph  *depgraph.Graph
}

func NewExtractor(db database.Database, config SubsetConfig) *Extractor {
	return &Extractor{
		db:     db,
		config: config,
		graph:  depgraph.New(),
	}
}

func (e *Extractor) Plan(ctx context.Context) (*SubsetPlan, error) {
	fks, err := e.db.DiscoverForeignKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("discovering foreign keys: %w", err)
	}
	e.fks = fks

	tables, err := e.db.DiscoverTables(ctx)
	if err != nil {
		return nil, fmt.Errorf("discovering tables: %w", err)
	}

	tableColumns := make(map[string][]database.ColumnInfo)
	for _, tbl := range tables {
		tableColumns[tbl.FullName()] = tbl.Columns
	}

	for _, fk := range fks {
		from := fk.FromFullName()
		to := fk.ToFullName()
		e.graph.AddNode(from)
		e.graph.AddNode(to)
		e.graph.AddEdge(from, to)
	}

	plan := &SubsetPlan{
		SeedTables:   make(map[string]string),
		Dependencies: make(map[string][]Dependency),
		TableColumns: tableColumns,
	}

	for tableName, ts := range e.config.Tables {
		plan.SeedTables[tableName] = ts.Where
	}

	if e.config.ResolveParents {
		e.resolveDependencies(plan)
	}

	return plan, nil
}

func (e *Extractor) resolveDependencies(plan *SubsetPlan) {
	visited := make(map[string]bool)
	queue := make([]string, 0, len(plan.SeedTables))

	for tableName := range plan.SeedTables {
		queue = append(queue, tableName)
		visited[tableName] = true
	}

	depth := 0
	for len(queue) > 0 && depth < e.config.MaxDepth {
		nextQueue := []string{}
		for _, tableName := range queue {
			for _, fk := range e.fks {
				fromName := fk.FromFullName()
				toName := fk.ToFullName()

				if fromName == tableName && !visited[toName] {
					visited[toName] = true
					nextQueue = append(nextQueue, toName)
					plan.Dependencies[tableName] = append(plan.Dependencies[tableName], Dependency{
						FromTable:   fromName,
						FromColumns: fk.FromColumns,
						ToTable:     toName,
						ToColumns:   fk.ToColumns,
					})
				}
			}
		}
		queue = nextQueue
		depth++
	}
}

type SubsetPlan struct {
	SeedTables   map[string]string
	Dependencies map[string][]Dependency
	TableColumns map[string][]database.ColumnInfo
}

type Dependency struct {
	FromTable   string
	FromColumns []string
	ToTable     string
	ToColumns   []string
}
