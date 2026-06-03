package validate

import (
	"context"
	"fmt"
)

func (v *Validator) CheckSchema(ctx context.Context) []Finding {
	var findings []Finding

	if v.db == nil {
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

	dbTables := make(map[string]map[string]string)
	for _, tbl := range tables {
		name := tbl.FullName()
		cols := make(map[string]string)
		for _, col := range tbl.Columns {
			cols[col.Name] = col.DataType
		}
		dbTables[name] = cols
	}

	for tableName, tblCfg := range v.cfg.Tables {
		if tblCfg.Exclude {
			continue
		}
		dbCols, exists := dbTables[tableName]
		if !exists {
			findings = append(findings, Finding{
				Severity: SeverityWarning,
				Category: "schema",
				Table:    tableName,
				Message:  "table referenced in config but not found in database",
			})
			continue
		}

		for colName := range tblCfg.Columns {
			if _, colExists := dbCols[colName]; !colExists {
				findings = append(findings, Finding{
					Severity: SeverityWarning,
					Category: "schema",
					Table:    tableName,
					Column:   colName,
					Message:  "column referenced in config but not found in table",
				})
			}
		}
	}

	for _, inc := range v.cfg.IncludeTables {
		if _, exists := dbTables[inc]; !exists {
			findings = append(findings, Finding{
				Severity: SeverityWarning,
				Category: "schema",
				Table:    inc,
				Message:  "table in include_tables not found in database",
			})
		}
	}

	return findings
}
