package subset

import (
	"github.com/bojin/datamask/internal/database"
	"github.com/bojin/datamask/internal/depgraph"
)

type FKGraph struct {
	graph *depgraph.Graph
	fks   []database.ForeignKey
}

func BuildFKGraph(fks []database.ForeignKey) *FKGraph {
	g := depgraph.New()
	for _, fk := range fks {
		from := fk.FromFullName()
		to := fk.ToFullName()
		g.AddNode(from)
		g.AddNode(to)
		g.AddEdge(from, to)
	}
	return &FKGraph{graph: g, fks: fks}
}

func (fg *FKGraph) ParentsOf(table string) []database.ForeignKey {
	var result []database.ForeignKey
	for _, fk := range fg.fks {
		if fk.FromFullName() == table {
			result = append(result, fk)
		}
	}
	return result
}

func (fg *FKGraph) ChildrenOf(table string) []database.ForeignKey {
	var result []database.ForeignKey
	for _, fk := range fg.fks {
		if fk.ToFullName() == table {
			result = append(result, fk)
		}
	}
	return result
}

func (fg *FKGraph) TopologicalOrder() ([][]string, error) {
	return fg.graph.TopologicalSort()
}

func (fg *FKGraph) Cycles() [][]string {
	return fg.graph.FindCycles()
}
