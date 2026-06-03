package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/bojin/datamask/internal/config"
	"github.com/bojin/datamask/internal/database"
	"github.com/bojin/datamask/internal/pipeline"
	"github.com/bojin/datamask/internal/storage"
	"github.com/bojin/datamask/internal/subset"
	"github.com/klauspost/pgzip"
	"github.com/spf13/cobra"
)

var subsetCmd = &cobra.Command{
	Use:   "subset",
	Short: "Extract a subset of data based on conditions",
	Long: `Extract a consistent subset of database records based on user-defined SQL WHERE
conditions. Automatically resolves foreign key dependencies to ensure referential
integrity of the extracted subset. The result is stored as a standard dump.`,
	RunE: runSubset,
}

func init() {
	rootCmd.AddCommand(subsetCmd)
	subsetCmd.Flags().String("name", "", "optional dump name suffix")
	subsetCmd.Flags().String("dsn", "", "override connection DSN from config")
	subsetCmd.Flags().String("description", "", "description for this subset dump")
}

func runSubset(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if cfg.Subset == nil || len(cfg.Subset.Tables) == 0 {
		return fmt.Errorf("no subset configuration found; define subset.tables in config")
	}

	dsn, _ := cmd.Flags().GetString("dsn")
	if dsn == "" {
		dsn = cfg.Connection.DSN()
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	db, err := database.GetDriver("postgres")
	if err != nil {
		return err
	}
	if err := db.Connect(ctx, dsn); err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer db.Close(ctx)

	subsetTables := make(map[string]subset.TableSubset)
	for name, ts := range cfg.Subset.Tables {
		subsetTables[name] = subset.TableSubset{Where: ts.Where}
	}

	maxDepth := cfg.Subset.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 10
	}

	subCfg := subset.SubsetConfig{
		Tables:          subsetTables,
		ResolveParents:  cfg.Subset.ResolveParents,
		ResolveChildren: cfg.Subset.ResolveChildren,
		MaxDepth:        maxDepth,
	}

	fmt.Print("Planning subset extraction... ")
	extractor := subset.NewExtractor(db, subCfg)
	plan, err := extractor.Plan(ctx)
	if err != nil {
		return fmt.Errorf("planning subset: %w", err)
	}
	fmt.Printf("done (%d seed tables, %d dependency paths)\n", len(plan.SeedTables), len(plan.Dependencies))

	fmt.Print("Resolving data dependencies... ")
	resolver := subset.NewResolver(db, plan, subCfg)
	results, err := resolver.Resolve(ctx)
	if err != nil {
		return fmt.Errorf("resolving subset: %w", err)
	}
	fmt.Println("done")

	store, err := storage.New(cfg.StorageDir)
	if err != nil {
		return fmt.Errorf("initializing storage: %w", err)
	}

	start := time.Now()
	nameSuffix, _ := cmd.Flags().GetString("name")
	if nameSuffix == "" {
		nameSuffix = "subset"
	}
	description, _ := cmd.Flags().GetString("description")

	dumpDir, err := store.CreateDump(cfg.Connection.DBName, nameSuffix)
	if err != nil {
		return fmt.Errorf("creating dump directory: %w", err)
	}
	fmt.Printf("Dump directory: %s\n", dumpDir)

	fmt.Print("Exporting schema... ")
	schemaPath := filepath.Join(dumpDir, "schema.dump")
	if err := db.ExportSchema(ctx, schemaPath); err != nil {
		return fmt.Errorf("exporting schema: %w", err)
	}
	fmt.Println("done")

	dataDir := filepath.Join(dumpDir, "data")
	var tableMetas []storage.TableMeta

	for tableName, buf := range results {
		if buf.Len() == 0 {
			continue
		}

		parts := strings.SplitN(tableName, ".", 2)
		schema := "public"
		table := tableName
		if len(parts) == 2 {
			schema = parts[0]
			table = parts[1]
		}

		dataFileName := fmt.Sprintf("%s.%s.csv.gz", schema, table)
		dataFilePath := filepath.Join(dataDir, dataFileName)

		f, err := os.Create(dataFilePath)
		if err != nil {
			return fmt.Errorf("creating data file for %s: %w", tableName, err)
		}

		gzw, _ := pgzip.NewWriterLevel(f, pgzip.BestSpeed)

		transforms := make(map[string]string)
		if tblCfg := cfg.GetTableConfig(table); tblCfg != nil {
			transforms = tblCfg.Columns
		}
		if tblCfg := cfg.GetTableConfig(tableName); tblCfg != nil {
			transforms = tblCfg.Columns
		}

		cols := plan.TableColumns[tableName]
		colNames := make([]string, len(cols))
		colTypes := make([]string, len(cols))
		for i, c := range cols {
			colNames[i] = c.Name
			colTypes[i] = c.DataType
		}

		transformers, err := pipeline.BuildTransformers(colNames, transforms)
		if err != nil {
			f.Close()
			return fmt.Errorf("building transformers for %s: %w", tableName, err)
		}

		var rowCount int64
		scanner := bufio.NewScanner(buf)
		scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			transformed, err := pipeline.TransformRow(line, colNames, colTypes, table, transformers)
			if err != nil {
				gzw.Close()
				f.Close()
				return fmt.Errorf("transforming row in %s: %w", tableName, err)
			}
			gzw.Write([]byte(transformed))
			gzw.Write([]byte("\n"))
			rowCount++
		}
		gzw.Close()
		f.Close()

		stat, _ := os.Stat(dataFilePath)
		compressedSize := int64(0)
		if stat != nil {
			compressedSize = stat.Size()
		}

		tableMetas = append(tableMetas, storage.TableMeta{
			Schema:         schema,
			Name:           table,
			RowCount:       rowCount,
			Columns:        colNames,
			ColumnTypes:    colTypes,
			Transformers:   transforms,
			DataFile:       dataFileName,
			CompressedSize: compressedSize,
		})
		fmt.Printf("  ✓ %s (%d rows)\n", tableName, rowCount)
	}

	dumpMeta := &storage.DumpMetadata{
		ID:          filepath.Base(dumpDir),
		CreatedAt:   start,
		SourceDB:    cfg.Connection.DBName,
		Tables:      tableMetas,
		SchemaFile:  "schema.dump",
		Duration:    time.Since(start),
		Description: description,
		Version:     "0.2.0",
		TotalRows:   computeSubsetTotalRows(tableMetas),
		TotalSize:   computeSubsetTotalSize(tableMetas),
		PGVersion:   db.ServerVersion(ctx),
	}

	if err := store.WriteMetadata(dumpDir, dumpMeta); err != nil {
		return fmt.Errorf("writing metadata: %w", err)
	}

	fmt.Printf("\nSubset dump completed: %d tables, %d total rows\n", len(tableMetas), dumpMeta.TotalRows)
	return nil
}

func computeSubsetTotalRows(tables []storage.TableMeta) int64 {
	var total int64
	for _, t := range tables {
		total += t.RowCount
	}
	return total
}

func computeSubsetTotalSize(tables []storage.TableMeta) int64 {
	var total int64
	for _, t := range tables {
		total += t.CompressedSize
	}
	return total
}
