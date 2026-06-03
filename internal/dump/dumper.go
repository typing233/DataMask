package dump

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/bojin/datamask/internal/config"
	"github.com/bojin/datamask/internal/pipeline"
	"github.com/bojin/datamask/internal/storage"
)

type Dumper struct {
	cfg     *config.Config
	store   *storage.Store
	dsn     string
	suffix  string
}

func New(cfg *config.Config, store *storage.Store, dsn, suffix string) *Dumper {
	return &Dumper{
		cfg:    cfg,
		store:  store,
		dsn:    dsn,
		suffix: suffix,
	}
}

func (d *Dumper) Run(ctx context.Context) error {
	start := time.Now()

	dbName := d.cfg.Connection.DBName
	dumpDir, err := d.store.CreateDump(dbName, d.suffix)
	if err != nil {
		return fmt.Errorf("creating dump directory: %w", err)
	}

	fmt.Printf("Dump directory: %s\n", dumpDir)

	fmt.Print("Exporting schema via pg_dump... ")
	if err := d.exportSchema(ctx, dumpDir); err != nil {
		return fmt.Errorf("exporting schema: %w", err)
	}
	fmt.Println("done")

	fmt.Print("Discovering tables... ")
	tables, err := d.discoverTables(ctx)
	if err != nil {
		return fmt.Errorf("discovering tables: %w", err)
	}
	fmt.Printf("found %d tables\n", len(tables))

	dataDir := filepath.Join(dumpDir, "data")
	sem := make(chan struct{}, d.cfg.Parallelism)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var tableMetas []storage.TableMeta
	var errors []error

	for _, tbl := range tables {
		sem <- struct{}{}
		wg.Add(1)
		go func(t TableInfo) {
			defer wg.Done()
			defer func() { <-sem }()

			meta, err := d.dumpTable(ctx, dataDir, t)
			mu.Lock()
			if err != nil {
				errors = append(errors, fmt.Errorf("table %s: %w", t.FullName(), err))
				fmt.Fprintf(os.Stderr, "  ERROR %s: %v\n", t.FullName(), err)
			} else {
				tableMetas = append(tableMetas, *meta)
				fmt.Printf("  ✓ %s (%d rows)\n", t.FullName(), meta.RowCount)
			}
			mu.Unlock()
		}(tbl)
	}
	wg.Wait()

	dumpMeta := &storage.DumpMetadata{
		ID:         filepath.Base(dumpDir),
		CreatedAt:  start,
		SourceDB:   dbName,
		Tables:     tableMetas,
		SchemaFile: "schema.sql",
		Duration:   time.Since(start),
	}

	if err := d.store.WriteMetadata(dumpDir, dumpMeta); err != nil {
		return fmt.Errorf("writing metadata: %w", err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("%d table(s) failed during dump", len(errors))
	}
	return nil
}

func (d *Dumper) exportSchema(ctx context.Context, dumpDir string) error {
	if _, err := exec.LookPath("pg_dump"); err != nil {
		return fmt.Errorf("pg_dump not found in PATH: %w", err)
	}

	outFile := filepath.Join(dumpDir, "schema.sql")
	cmd := exec.CommandContext(ctx, "pg_dump",
		"--format=plain",
		"--schema-only",
		"--no-owner",
		"--no-privileges",
		"--dbname", d.dsn,
	)

	out, err := os.Create(outFile)
	if err != nil {
		return fmt.Errorf("creating schema file: %w", err)
	}
	defer out.Close()

	cmd.Stdout = out
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func (d *Dumper) dumpTable(ctx context.Context, dataDir string, tbl TableInfo) (*storage.TableMeta, error) {
	transforms := make(map[string]string)
	if tblCfg := d.cfg.GetTableConfig(tbl.Name); tblCfg != nil {
		transforms = tblCfg.Columns
	}
	if tblCfg := d.cfg.GetTableConfig(tbl.FullName()); tblCfg != nil {
		transforms = tblCfg.Columns
	}

	result, err := pipeline.CopyTableToFile(ctx, pipeline.CopyToFileParams{
		DSN:         d.dsn,
		Schema:      tbl.Schema,
		Table:       tbl.Name,
		Columns:     tbl.Columns,
		ColumnTypes: tbl.ColumnTypes,
		OutputDir:   dataDir,
		Transforms:  transforms,
	})
	if err != nil {
		return nil, err
	}

	meta := &storage.TableMeta{
		Schema:         tbl.Schema,
		Name:           tbl.Name,
		RowCount:       result.RowCount,
		Columns:        tbl.Columns,
		ColumnTypes:    tbl.ColumnTypes,
		Transformers:   transforms,
		DataFile:       result.DataFile,
		CompressedSize: result.CompressedSize,
	}
	return meta, nil
}
