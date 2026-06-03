package restore

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/bojin/datamask/internal/config"
	"github.com/bojin/datamask/internal/storage"
	"github.com/jackc/pgx/v5"
	"github.com/klauspost/pgzip"
)

type Restorer struct {
	cfg        *config.Config
	store      *storage.Store
	targetDSN  string
	dropSchema bool
}

func New(cfg *config.Config, store *storage.Store, targetDSN string, dropSchema bool) *Restorer {
	return &Restorer{
		cfg:        cfg,
		store:      store,
		targetDSN:  targetDSN,
		dropSchema: dropSchema,
	}
}

func (r *Restorer) Run(ctx context.Context, dumpID string) error {
	start := time.Now()

	meta, err := r.store.LoadMetadata(dumpID)
	if err != nil {
		return fmt.Errorf("loading metadata for %q: %w", dumpID, err)
	}

	if r.dropSchema {
		fmt.Print("Dropping and recreating schema... ")
		if err := r.resetSchema(ctx); err != nil {
			return fmt.Errorf("resetting schema: %w", err)
		}
		fmt.Println("done")
	}

	fmt.Print("Restoring schema via pg_restore... ")
	if err := r.restoreSchema(ctx, dumpID); err != nil {
		return fmt.Errorf("restoring schema: %w", err)
	}
	fmt.Println("done")

	fmt.Printf("Restoring data for %d tables...\n", len(meta.Tables))

	sem := make(chan struct{}, r.cfg.Parallelism)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errors []error
	var totalRows int64

	for _, tbl := range meta.Tables {
		sem <- struct{}{}
		wg.Add(1)
		go func(t storage.TableMeta) {
			defer wg.Done()
			defer func() { <-sem }()

			rows, err := r.restoreTable(ctx, dumpID, t)
			mu.Lock()
			if err != nil {
				errors = append(errors, fmt.Errorf("table %s: %w", t.FullName(), err))
				fmt.Fprintf(os.Stderr, "  ERROR %s: %v\n", t.FullName(), err)
			} else {
				totalRows += rows
				fmt.Printf("  ✓ %s (%d rows)\n", t.FullName(), rows)
			}
			mu.Unlock()
		}(tbl)
	}
	wg.Wait()

	elapsed := time.Since(start)
	fmt.Printf("Restored %d rows across %d tables in %s\n", totalRows, len(meta.Tables)-len(errors), elapsed)

	if len(errors) > 0 {
		return fmt.Errorf("%d table(s) failed during restore", len(errors))
	}
	return nil
}

func (r *Restorer) resetSchema(ctx context.Context) error {
	conn, err := pgx.Connect(ctx, r.targetDSN)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	_, err = conn.Exec(ctx, "DROP SCHEMA IF EXISTS public CASCADE; CREATE SCHEMA public;")
	return err
}

func (r *Restorer) restoreSchema(ctx context.Context, dumpID string) error {
	if _, err := exec.LookPath("pg_restore"); err != nil {
		return fmt.Errorf("pg_restore not found in PATH: %w", err)
	}

	schemaPath := r.store.SchemaPath(dumpID)
	cmd := exec.CommandContext(ctx, "pg_restore",
		"--schema-only",
		"--no-owner",
		"--no-privileges",
		"--dbname", r.targetDSN,
		schemaPath,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func (r *Restorer) restoreTable(ctx context.Context, dumpID string, tbl storage.TableMeta) (int64, error) {
	dataPath := r.store.DataFilePath(dumpID, tbl.DataFile)

	f, err := os.Open(dataPath)
	if err != nil {
		return 0, fmt.Errorf("opening data file: %w", err)
	}
	defer f.Close()

	gzr, err := pgzip.NewReader(f)
	if err != nil {
		return 0, fmt.Errorf("creating gzip reader: %w", err)
	}
	defer gzr.Close()

	conn, err := pgx.Connect(ctx, r.targetDSN)
	if err != nil {
		return 0, fmt.Errorf("connecting to target: %w", err)
	}
	defer conn.Close(ctx)

	tableName := fmt.Sprintf("%q.%q", tbl.Schema, tbl.Name)
	copySQL := fmt.Sprintf("COPY %s FROM STDIN", tableName)

	rawConn := conn.PgConn()
	result, err := rawConn.CopyFrom(ctx, gzr, copySQL)
	if err != nil {
		return 0, fmt.Errorf("COPY FROM: %w", err)
	}

	return result.RowsAffected(), nil
}
