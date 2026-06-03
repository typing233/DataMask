package validate

import (
	"context"
	"fmt"
	"strings"

	"github.com/bojin/datamask/internal/storage"
)

func (v *Validator) CheckSchema(ctx context.Context) []Finding {
	var findings []Finding

	if v.db == nil {
		findings = append(findings, Finding{
			Severity: SeverityError,
			Category: "schema",
			Message:  "database connection unavailable: cannot perform schema validation",
		})
		return findings
	}

	tables, err := v.db.DiscoverTables(ctx)
	if err != nil {
		findings = append(findings, Finding{
			Severity: SeverityError,
			Category: "schema",
			Message:  fmt.Sprintf("failed to discover tables: %v", err),
		})
		return findings
	}

	// Build current DB schema map: table -> column -> type
	dbSchema := make(map[string]map[string]string)
	for _, tbl := range tables {
		name := tbl.FullName()
		cols := make(map[string]string)
		for _, col := range tbl.Columns {
			cols[col.Name] = col.DataType
		}
		dbSchema[name] = cols
	}

	// Check config references against DB
	for tableName, tblCfg := range v.cfg.Tables {
		if tblCfg.Exclude {
			continue
		}
		dbCols, exists := dbSchema[tableName]
		if !exists {
			findings = append(findings, Finding{
				Severity: SeverityError,
				Category: "schema",
				Table:    tableName,
				Message:  "table configured for masking does not exist in database",
			})
			continue
		}

		for colName := range tblCfg.Columns {
			if _, colExists := dbCols[colName]; !colExists {
				findings = append(findings, Finding{
					Severity: SeverityError,
					Category: "schema",
					Table:    tableName,
					Column:   colName,
					Message:  "column configured for masking does not exist in table",
				})
			}
		}
	}

	for _, inc := range v.cfg.IncludeTables {
		if _, exists := dbSchema[inc]; !exists {
			findings = append(findings, Finding{
				Severity: SeverityWarning,
				Category: "schema",
				Table:    inc,
				Message:  "table in include_tables not found in database",
			})
		}
	}

	// If a dump is available, diff the dump's stored schema against the live DB
	if v.store != nil && v.dumpID != "" {
		findings = append(findings, v.diffDumpSchema(dbSchema)...)
	}

	return findings
}

// diffDumpSchema compares a stored dump's schema snapshot against the current DB schema.
func (v *Validator) diffDumpSchema(dbSchema map[string]map[string]string) []Finding {
	var findings []Finding

	meta, err := v.store.LoadMetadata(v.dumpID)
	if err != nil {
		findings = append(findings, Finding{
			Severity: SeverityWarning,
			Category: "schema",
			Message:  fmt.Sprintf("cannot load dump metadata for schema diff: %v", err),
		})
		return findings
	}

	// Build dump schema from metadata
	dumpSchema := buildSchemaFromMeta(meta)

	// Compare: find tables/columns added, removed, or changed
	for tableName, dumpCols := range dumpSchema {
		dbCols, exists := dbSchema[tableName]
		if !exists {
			findings = append(findings, Finding{
				Severity: SeverityWarning,
				Category: "schema",
				Table:    tableName,
				Message:  "table present in dump but removed from database",
			})
			continue
		}

		// Columns removed from DB since dump
		for colName, dumpType := range dumpCols {
			dbType, colExists := dbCols[colName]
			if !colExists {
				findings = append(findings, Finding{
					Severity: SeverityWarning,
					Category: "schema",
					Table:    tableName,
					Column:   colName,
					Message:  fmt.Sprintf("column present in dump (type: %s) but removed from database", dumpType),
				})
				continue
			}
			// Type changed
			if !typesEqual(dumpType, dbType) {
				findings = append(findings, Finding{
					Severity: SeverityWarning,
					Category: "schema",
					Table:    tableName,
					Column:   colName,
					Message:  fmt.Sprintf("column type changed: dump has %q, database has %q", dumpType, dbType),
				})
			}
		}

		// Columns added to DB since dump
		for colName, dbType := range dbCols {
			if _, inDump := dumpCols[colName]; !inDump {
				findings = append(findings, Finding{
					Severity: SeverityWarning,
					Category: "schema",
					Table:    tableName,
					Column:   colName,
					Message:  fmt.Sprintf("column added to database since dump (type: %s)", dbType),
				})
			}
		}
	}

	// Tables added to DB since dump
	for tableName := range dbSchema {
		if _, inDump := dumpSchema[tableName]; !inDump {
			// Only report if this table is relevant (in config)
			if _, inConfig := v.cfg.Tables[tableName]; inConfig {
				findings = append(findings, Finding{
					Severity: SeverityWarning,
					Category: "schema",
					Table:    tableName,
					Message:  "table added to database since dump (configured for masking)",
				})
			}
		}
	}

	return findings
}

func buildSchemaFromMeta(meta *storage.DumpMetadata) map[string]map[string]string {
	schema := make(map[string]map[string]string)
	for _, tbl := range meta.Tables {
		name := tbl.FullName()
		cols := make(map[string]string)
		for i, colName := range tbl.Columns {
			colType := ""
			if i < len(tbl.ColumnTypes) {
				colType = tbl.ColumnTypes[i]
			}
			cols[colName] = colType
		}
		schema[name] = cols
	}
	return schema
}

func typesEqual(a, b string) bool {
	na := strings.ToLower(strings.TrimSpace(a))
	nb := strings.ToLower(strings.TrimSpace(b))
	return na == nb
}

// CheckSchemaAgainstDump explicitly diffs a specific dump against the live database.
// Used when the user wants to detect schema drift before restoring.
func (v *Validator) CheckSchemaAgainstDump(ctx context.Context, store *storage.Store, dumpID string) []Finding {
	var findings []Finding

	if v.db == nil {
		findings = append(findings, Finding{
			Severity: SeverityError,
			Category: "schema",
			Message:  "database connection unavailable: cannot perform schema diff",
		})
		return findings
	}

	tables, err := v.db.DiscoverTables(ctx)
	if err != nil {
		findings = append(findings, Finding{
			Severity: SeverityError,
			Category: "schema",
			Message:  fmt.Sprintf("failed to discover tables: %v", err),
		})
		return findings
	}

	dbSchema := make(map[string]map[string]string)
	for _, tbl := range tables {
		name := tbl.FullName()
		cols := make(map[string]string)
		for _, col := range tbl.Columns {
			cols[col.Name] = col.DataType
		}
		dbSchema[name] = cols
	}

	meta, err := store.LoadMetadata(dumpID)
	if err != nil {
		findings = append(findings, Finding{
			Severity: SeverityError,
			Category: "schema",
			Message:  fmt.Sprintf("cannot load dump metadata: %v", err),
		})
		return findings
	}

	dumpSchema := buildSchemaFromMeta(meta)
	return diffSchemas(dumpSchema, dbSchema)
}

func diffSchemas(dumpSchema, dbSchema map[string]map[string]string) []Finding {
	var findings []Finding

	for tableName, dumpCols := range dumpSchema {
		dbCols, exists := dbSchema[tableName]
		if !exists {
			findings = append(findings, Finding{
				Severity: SeverityError,
				Category: "schema",
				Table:    tableName,
				Message:  "table in dump does not exist in target database",
			})
			continue
		}

		for colName, dumpType := range dumpCols {
			dbType, colExists := dbCols[colName]
			if !colExists {
				findings = append(findings, Finding{
					Severity: SeverityError,
					Category: "schema",
					Table:    tableName,
					Column:   colName,
					Message:  fmt.Sprintf("column in dump (type: %s) does not exist in target", dumpType),
				})
				continue
			}
			if !typesEqual(dumpType, dbType) {
				findings = append(findings, Finding{
					Severity: SeverityWarning,
					Category: "schema",
					Table:    tableName,
					Column:   colName,
					Message:  fmt.Sprintf("type mismatch: dump=%q, target=%q", dumpType, dbType),
				})
			}
		}

		for colName, dbType := range dbCols {
			if _, inDump := dumpCols[colName]; !inDump {
				findings = append(findings, Finding{
					Severity: SeverityWarning,
					Category: "schema",
					Table:    tableName,
					Column:   colName,
					Message:  fmt.Sprintf("column exists in target (type: %s) but not in dump — will be NULL after restore", dbType),
				})
			}
		}
	}

	return findings
}
