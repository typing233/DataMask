package depgraph

import "fmt"

type Graph struct {
	nodes map[string]bool
	edges map[string]map[string]bool // edges[from][to] = from depends on to
}

func New() *Graph {
	return &Graph{
		nodes: make(map[string]bool),
		edges: make(map[string]map[string]bool),
	}
}

func (g *Graph) AddNode(name string) {
	g.nodes[name] = true
	if _, ok := g.edges[name]; !ok {
		g.edges[name] = make(map[string]bool)
	}
}

func (g *Graph) AddEdge(from, to string) {
	g.AddNode(from)
	g.AddNode(to)
	g.edges[from][to] = true
}

func (g *Graph) TopologicalSort() ([][]string, error) {
	inDegree := make(map[string]int)
	for node := range g.nodes {
		inDegree[node] = 0
	}
	for from, deps := range g.edges {
		_ = from
		for to := range deps {
			inDegree[to] += 0
			_ = to
		}
	}

	reverse := make(map[string]map[string]bool)
	for node := range g.nodes {
		reverse[node] = make(map[string]bool)
	}
	for from, deps := range g.edges {
		for to := range deps {
			reverse[to][from] = true
		}
	}

	for node := range g.nodes {
		inDegree[node] = len(g.edges[node])
	}

	var queue []string
	for node, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, node)
		}
	}

	var layers [][]string
	visited := make(map[string]bool)

	for len(queue) > 0 {
		layer := queue
		queue = nil
		layers = append(layers, layer)

		for _, node := range layer {
			visited[node] = true
			for dependent := range reverse[node] {
				if visited[dependent] {
					continue
				}
				inDegree[dependent]--
				if inDegree[dependent] == 0 {
					queue = append(queue, dependent)
				}
			}
		}
	}

	if len(visited) != len(g.nodes) {
		return layers, fmt.Errorf("cycle detected: %d nodes could not be sorted", len(g.nodes)-len(visited))
	}

	return layers, nil
}

func (g *Graph) FindCycles() [][]string {
	index := 0
	stack := []string{}
	onStack := make(map[string]bool)
	indices := make(map[string]int)
	lowlinks := make(map[string]int)
	var sccs [][]string

	var strongconnect func(v string)
	strongconnect = func(v string) {
		indices[v] = index
		lowlinks[v] = index
		index++
		stack = append(stack, v)
		onStack[v] = true

		for w := range g.edges[v] {
			if _, visited := indices[w]; !visited {
				strongconnect(w)
				if lowlinks[w] < lowlinks[v] {
					lowlinks[v] = lowlinks[w]
				}
			} else if onStack[w] {
				if indices[w] < lowlinks[v] {
					lowlinks[v] = indices[w]
				}
			}
		}

		if lowlinks[v] == indices[v] {
			var scc []string
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[w] = false
				scc = append(scc, w)
				if w == v {
					break
				}
			}
			if len(scc) > 1 {
				sccs = append(sccs, scc)
			}
		}
	}

	for node := range g.nodes {
		if _, visited := indices[node]; !visited {
			strongconnect(node)
		}
	}

	return sccs
}

func (g *Graph) Nodes() []string {
	result := make([]string, 0, len(g.nodes))
	for node := range g.nodes {
		result = append(result, node)
	}
	return result
}
