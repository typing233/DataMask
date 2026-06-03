package depgraph

import (
	"sort"
	"testing"
)

func TestTopologicalSort(t *testing.T) {
	g := New()
	g.AddNode("orders")
	g.AddNode("users")
	g.AddNode("products")
	g.AddEdge("orders", "users")
	g.AddEdge("orders", "products")

	layers, err := g.TopologicalSort()
	if err != nil {
		t.Fatal(err)
	}

	if len(layers) < 2 {
		t.Fatalf("expected at least 2 layers, got %d", len(layers))
	}

	firstLayer := layers[0]
	sort.Strings(firstLayer)
	if len(firstLayer) != 2 {
		t.Fatalf("expected 2 nodes in first layer, got %d: %v", len(firstLayer), firstLayer)
	}
	if firstLayer[0] != "products" || firstLayer[1] != "users" {
		t.Errorf("expected [products users] in first layer, got %v", firstLayer)
	}

	if layers[1][0] != "orders" {
		t.Errorf("expected orders in second layer, got %v", layers[1])
	}
}

func TestTopologicalSortCycle(t *testing.T) {
	g := New()
	g.AddEdge("a", "b")
	g.AddEdge("b", "c")
	g.AddEdge("c", "a")

	_, err := g.TopologicalSort()
	if err == nil {
		t.Fatal("expected error for cycle")
	}
}

func TestFindCycles(t *testing.T) {
	g := New()
	g.AddEdge("a", "b")
	g.AddEdge("b", "c")
	g.AddEdge("c", "a")
	g.AddNode("d")
	g.AddEdge("d", "a")

	cycles := g.FindCycles()
	if len(cycles) != 1 {
		t.Fatalf("expected 1 cycle, got %d", len(cycles))
	}
	if len(cycles[0]) != 3 {
		t.Errorf("expected cycle of length 3, got %d: %v", len(cycles[0]), cycles[0])
	}
}

func TestNoCycles(t *testing.T) {
	g := New()
	g.AddEdge("a", "b")
	g.AddEdge("b", "c")

	cycles := g.FindCycles()
	if len(cycles) != 0 {
		t.Errorf("expected no cycles, got %d", len(cycles))
	}
}

func TestSingleNode(t *testing.T) {
	g := New()
	g.AddNode("solo")

	layers, err := g.TopologicalSort()
	if err != nil {
		t.Fatal(err)
	}
	if len(layers) != 1 || layers[0][0] != "solo" {
		t.Errorf("expected [[solo]], got %v", layers)
	}
}
