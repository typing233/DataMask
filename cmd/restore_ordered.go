package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bojin/datamask/internal/config"
	"github.com/bojin/datamask/internal/database"
	"github.com/bojin/datamask/internal/depgraph"
	"github.com/bojin/datamask/internal/storage"
	"github.com/jackc/pgx/v5"
	pgzip "github.com/klauspost/pgzip"
	"github.com/spf13/cobra"
)

var restoreOrderedCmd = &cobra.Command{
	Use:   "restore-in-order [dump-id]",
	Short: "Restore tables in foreign key dependency order",
	Long: `Restore tables in topological order based on foreign key relationships.
Tables are restored in dependency layers - parent tables before child tables.
Circular references are handled by temporarily disabling triggers.`,
	Args: cobra.ExactArgs(1),
	RunE: runRestoreOrdered,
}

func init() {
	rootCmd.AddCommand(restoreOrderedCmd)
	restoreOrderedCmd.Flags().String("target-dsn", "", "target database connection string (required)")
	restoreOrderedCmd.Flags().Bool("drop-schema", false, "drop and recreate public schema before restore")
	restoreOrderedCmd.Flags().Bool("disable-triggers", true, "disable triggers for circular dependency groups")
	restoreOrderedCmd.MarkFlagRequired("target-dsn")
}

func runRestoreOrdered(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	targetDSN, _ := cmd.Flags().GetString("target-dsn")
	dropSchema, _ := cmd.Flags().GetBool("drop-schema")
	disableTriggers, _ := cmd.Flags().GetBool("disable-triggers")

	store, err := storage.New(cfg.StorageDir)
	if err != nil {
		return fmt.Errorf("initializing storage: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	dumpID := args[0]
	meta, err := store.LoadMetadata(dumpID)
	if err != nil {
		return fmt.Errorf("loading metadata: %w", err)
	}

	if dropSchema {
		fmt.Print("Dropping and recreating schema... ")
		if err := resetTargetSchema(ctx, targetDSN); err != nil {
			return fmt.Errorf("resetting schema: %w", err)
		}
		fmt.Println("done")
	}

	fmt.Print("Restoring schema... ")
	schemaPath := store.SchemaPath(dumpID)
	db, err := database.GetDriver("postgres")
	if err != nil {
		return err
	}
	if err := db.Connect(ctx, targetDSN); err != nil {
		return fmt.Errorf("connecting to target: %w", err)
	}
	defer db.Close(ctx)

	if err := db.RestoreSchema(ctx, schemaPath, targetDSN); err != nil {
		return fmt.Errorf("restoring schema: %w", err)
	}
	fmt.Println("done")

	fmt.Print("Discovering foreign key relationships... ")
	fks, err := db.DiscoverForeignKeys(ctx)
	if err != nil {
		return fmt.Errorf("discovering FKs: %w", err)
	}
	fmt.Printf("found %d foreign keys\n", len(fks))

	graph := depgraph.New()
	tableIndex := make(map[string]storage.TableMeta)
	for _, tbl := range meta.Tables {
		name := tbl.FullName()
		graph.AddNode(name)
		tableIndex[name] = tbl
	}

	for _, fk := range fks {
		from := fk.FromFullName()
		to := fk.ToFullName()
		if _, ok := tableIndex[from]; !ok {
			continue
		}
		if _, ok := tableIndex[to]; !ok {
			continue
		}
		graph.AddEdge(from, to)
	}

	cycles := graph.FindCycles()
	if len(cycles) > 0 {
		fmt.Printf("Detected %d circular dependency group(s)\n", len(cycles))
	}

	cycleNodes := make(map[string]bool)
	for _, cycle := range cycles {
		for _, node := range cycle {
			cycleNodes[node] = true
		}
	}

	layers, _ := graph.TopologicalSort()

	start := time.Now()
	var totalRows int64
	var restoredCount int

	for _, layer := range layers {
		for _, tableName := range layer {
			tbl, ok := tableIndex[tableName]
			if !ok {
				continue
			}
			rows, err := restoreTableOrdered(ctx, store, dumpID, tbl, targetDSN)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  ERROR %s: %v\n", tableName, err)
				continue
			}
			totalRows += rows
			restoredCount++
			fmt.Printf("  ✓ %s (%d rows)\n", tableName, rows)
		}
	}

	if disableTriggers && len(cycles) > 0 {
		fmt.Println("Restoring circular dependency tables with triggers disabled...")
		for _, cycle := range cycles {
			if err := restoreCycleGroup(ctx, store, dumpID, cycle, tableIndex, targetDSN); err != nil {
				fmt.Fprintf(os.Stderr, "  ERROR restoring cycle group: %v\n", err)
			} else {
				for _, name := range cycle {
					if tbl, ok := tableIndex[name]; ok {
						totalRows += tbl.RowCount
						restoredCount++
						fmt.Printf("  ✓ %s (cycle, %d rows)\n", name, tbl.RowCount)
					}
				}
			}
		}
	}

	elapsed := time.Since(start)
	fmt.Printf("\nRestored %d rows across %d tables in %s\n", totalRows, restoredCount, elapsed)
	return nil
}

func resetTargetSchema(ctx context.Context, dsn string) error {
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, "DROP SCHEMA IF EXISTS public CASCADE; CREATE SCHEMA public;")
	return err
}

func restoreTableOrdered(ctx context.Context, store *storage.Store, dumpID string, tbl storage.TableMeta, targetDSN string) (int64, error) {
	dataPath := store.DataFilePath(dumpID, tbl.DataFile)

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

	conn, err := pgx.Connect(ctx, targetDSN)
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

func restoreCycleGroup(ctx context.Context, store *storage.Store, dumpID string, tables []string, tableIndex map[string]storage.TableMeta, targetDSN string) error {
	conn, err := pgx.Connect(ctx, targetDSN)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	for _, name := range tables {
		tbl := tableIndex[name]
		tableName := fmt.Sprintf("%q.%q", tbl.Schema, tbl.Name)
		_, err := conn.Exec(ctx, fmt.Sprintf("ALTER TABLE %s DISABLE TRIGGER ALL", tableName))
		if err != nil {
			return fmt.Errorf("disabling triggers on %s: %w", name, err)
		}
	}

	for _, name := range tables {
		tbl := tableIndex[name]
		if _, err := restoreTableOrdered(ctx, store, dumpID, tbl, targetDSN); err != nil {
			return fmt.Errorf("restoring %s: %w", name, err)
		}
	}

	for _, name := range tables {
		tbl := tableIndex[name]
		tableName := fmt.Sprintf("%q.%q", tbl.Schema, tbl.Name)
		_, err := conn.Exec(ctx, fmt.Sprintf("ALTER TABLE %s ENABLE TRIGGER ALL", tableName))
		if err != nil {
			return fmt.Errorf("re-enabling triggers on %s: %w", name, err)
		}
	}

	return nil
}
