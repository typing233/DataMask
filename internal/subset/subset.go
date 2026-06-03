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

	cycles := e.graph.FindCycles()

	plan := &SubsetPlan{
		SeedTables:     make(map[string]string),
		ParentDeps:     make(map[string][]Dependency),
		ChildDeps:      make(map[string][]Dependency),
		TableColumns:   tableColumns,
		ForeignKeys:    fks,
		CyclicTables:   make(map[string]bool),
		CycleGroups:    cycles,
	}

	for _, cycle := range cycles {
		for _, t := range cycle {
			plan.CyclicTables[t] = true
		}
	}

	for tableName, ts := range e.config.Tables {
		plan.SeedTables[tableName] = ts.Where
	}

	e.buildDependencyMap(plan)

	return plan, nil
}

func (e *Extractor) buildDependencyMap(plan *SubsetPlan) {
	for _, fk := range e.fks {
		from := fk.FromFullName()
		to := fk.ToFullName()

		// Parent dependency: from table references to table
		plan.ParentDeps[from] = append(plan.ParentDeps[from], Dependency{
			FromTable:   from,
			FromColumns: fk.FromColumns,
			ToTable:     to,
			ToColumns:   fk.ToColumns,
		})

		// Child dependency: to table is referenced by from table
		plan.ChildDeps[to] = append(plan.ChildDeps[to], Dependency{
			FromTable:   from,
			FromColumns: fk.FromColumns,
			ToTable:     to,
			ToColumns:   fk.ToColumns,
		})
	}
}

type SubsetPlan struct {
	SeedTables   map[string]string
	ParentDeps   map[string][]Dependency // table -> FKs pointing from this table to parents
	ChildDeps    map[string][]Dependency // table -> FKs pointing to this table from children
	TableColumns map[string][]database.ColumnInfo
	ForeignKeys  []database.ForeignKey
	CyclicTables map[string]bool
	CycleGroups  [][]string
}

type Dependency struct {
	FromTable   string
	FromColumns []string // columns in the referencing (child) table
	ToTable     string
	ToColumns   []string // columns in the referenced (parent) table
}
