package database

import (
	"fmt"
	"sync"
)

type Factory func() Database

var (
	mu       sync.RWMutex
	drivers  = make(map[string]Factory)
)

func RegisterDriver(name string, factory Factory) {
	mu.Lock()
	defer mu.Unlock()
	drivers[name] = factory
}

func GetDriver(name string) (Database, error) {
	mu.RLock()
	defer mu.RUnlock()
	factory, ok := drivers[name]
	if !ok {
		return nil, fmt.Errorf("unknown database driver: %q (available: %v)", name, ListDrivers())
	}
	return factory(), nil
}

func ListDrivers() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(drivers))
	for name := range drivers {
		names = append(names, name)
	}
	return names
}
