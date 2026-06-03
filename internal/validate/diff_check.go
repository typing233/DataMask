package validate

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/bojin/datamask/internal/pipeline"
)

func (v *Validator) CheckDiff(ctx context.Context) []Finding {
	var findings []Finding

	if v.db == nil {
		findings = append(findings, Finding{
			Severity: SeverityError,
			Category: "diff",
			Message:  "database connection unavailable: cannot perform diff check",
		})
		return findings
	}

	tables, err := v.db.DiscoverTables(ctx)
	if err != nil {
		findings = append(findings, Finding{
			Severity: SeverityError,
			Category: "diff",
			Message:  fmt.Sprintf("failed to discover tables: %v", err),
		})
		return findings
	}

	dbTableInfo := make(map[string]struct {
		columns     []string
		columnTypes []string
	})
	for _, tbl := range tables {
		name := tbl.FullName()
		var cols, types []string
		for _, col := range tbl.Columns {
			cols = append(cols, col.Name)
			types = append(types, col.DataType)
		}
		dbTableInfo[name] = struct {
			columns     []string
			columnTypes []string
		}{cols, types}
	}

	sampleLimit := v.sampleRows
	if sampleLimit <= 0 {
		sampleLimit = 5
	}
	if sampleLimit > 100 {
		sampleLimit = 100
	}

	for tableName, tblCfg := range v.cfg.Tables {
		if tblCfg.Exclude {
			continue
		}
		if len(tblCfg.Columns) == 0 {
			continue
		}

		info, ok := dbTableInfo[tableName]
		if !ok {
			continue
		}

		transformers, err := pipeline.BuildTransformers(info.columns, tblCfg.Columns)
		if err != nil {
			findings = append(findings, Finding{
				Severity: SeverityError,
				Category: "diff",
				Table:    tableName,
				Message:  fmt.Sprintf("failed to build transformers: %v", err),
			})
			continue
		}

		hasAnyTransformer := false
		for _, t := range transformers {
			if t != nil {
				hasAnyTransformer = true
				break
			}
		}
		if !hasAnyTransformer {
			continue
		}

		parts := strings.SplitN(tableName, ".", 2)
		schema := "public"
		table := tableName
		if len(parts) == 2 {
			schema = parts[0]
			table = parts[1]
		}

		// Use a safe integer literal for LIMIT — never interpolate user input
		whereClause := "1=1 LIMIT " + strconv.Itoa(sampleLimit)

		var buf bytes.Buffer
		_, err = v.db.QueryRows(ctx, schema, table, info.columns, whereClause, &buf)
		if err != nil {
			findings = append(findings, Finding{
				Severity: SeverityError,
				Category: "diff",
				Table:    tableName,
				Message:  fmt.Sprintf("failed to sample rows: %v", err),
			})
			continue
		}

		scanner := bufio.NewScanner(&buf)
		rowNum := 0
		for scanner.Scan() {
			rowNum++
			line := scanner.Text()
			if line == "" {
				continue
			}
			transformed, err := pipeline.TransformRow(line, info.columns, info.columnTypes, table, transformers)
			if err != nil {
				findings = append(findings, Finding{
					Severity: SeverityError,
					Category: "diff",
					Table:    tableName,
					Message:  fmt.Sprintf("transform error on row %d: %v", rowNum, err),
				})
				break
			}

			origFields := strings.Split(line, "\t")
			newFields := strings.Split(transformed, "\t")
			for i := range origFields {
				if i >= len(newFields) {
					break
				}
				if origFields[i] != newFields[i] && i < len(info.columns) {
					colName := info.columns[i]
					txName := ""
					if transformers[i] != nil {
						txName = transformers[i].Name()
					}
					findings = append(findings, Finding{
						Severity: SeverityWarning,
						Category: "diff",
						Table:    tableName,
						Column:   colName,
						Message:  fmt.Sprintf("row %d: %q -> %q (transformer: %s)", rowNum, truncate(origFields[i], 30), truncate(newFields[i], 30), txName),
					})
				}
			}
		}
	}

	return findings
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
