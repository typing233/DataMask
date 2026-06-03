package transformer

import (
	"fmt"
	"sort"
)

var registry = make(map[string]Transformer)

func Register(t Transformer) {
	registry[t.Name()] = t
}

func Get(name string) (Transformer, error) {
	t, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown transformer: %q (available: %v)", name, List())
	}
	return t, nil
}

func List() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
