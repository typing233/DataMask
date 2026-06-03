package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type Store struct {
	baseDir string
}

func New(baseDir string) (*Store, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("creating storage dir: %w", err)
	}
	return &Store{baseDir: baseDir}, nil
}

func (s *Store) CreateDump(dbName, suffix string) (string, error) {
	name := time.Now().Format("20060102_150405") + "_" + dbName
	if suffix != "" {
		name += "_" + suffix
	}
	dir := filepath.Join(s.baseDir, name)
	if err := os.MkdirAll(filepath.Join(dir, "data"), 0755); err != nil {
		return "", fmt.Errorf("creating dump directory: %w", err)
	}
	return dir, nil
}

func (s *Store) ListDumps() ([]DumpMetadata, error) {
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return nil, fmt.Errorf("reading storage dir: %w", err)
	}

	var dumps []DumpMetadata
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		metaPath := filepath.Join(s.baseDir, entry.Name(), "metadata.json")
		meta, err := loadMetadataFile(metaPath)
		if err != nil {
			continue
		}
		dumps = append(dumps, *meta)
	}

	sort.Slice(dumps, func(i, j int) bool {
		return dumps[i].CreatedAt.After(dumps[j].CreatedAt)
	})
	return dumps, nil
}

func (s *Store) DeleteDump(id string) error {
	dir := filepath.Join(s.baseDir, id)
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("dump %q not found: %w", id, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("dump %q is not a directory", id)
	}
	return os.RemoveAll(dir)
}

func (s *Store) WriteMetadata(dumpDir string, meta *DumpMetadata) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}
	return os.WriteFile(filepath.Join(dumpDir, "metadata.json"), data, 0644)
}

func (s *Store) LoadMetadata(dumpID string) (*DumpMetadata, error) {
	metaPath := filepath.Join(s.baseDir, dumpID, "metadata.json")
	return loadMetadataFile(metaPath)
}

func (s *Store) SchemaPath(dumpID string) string {
	metaPath := filepath.Join(s.baseDir, dumpID, "metadata.json")
	meta, err := loadMetadataFile(metaPath)
	if err != nil || meta.SchemaFile == "" {
		return filepath.Join(s.baseDir, dumpID, "schema.dump")
	}
	return filepath.Join(s.baseDir, dumpID, meta.SchemaFile)
}

func (s *Store) DataFilePath(dumpID, dataFile string) string {
	return filepath.Join(s.baseDir, dumpID, "data", dataFile)
}

func (s *Store) DumpDir(dumpID string) string {
	return filepath.Join(s.baseDir, dumpID)
}

func loadMetadataFile(path string) (*DumpMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var meta DumpMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}
