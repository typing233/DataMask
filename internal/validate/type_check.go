package validate

import (
	"context"
	"fmt"
	"strings"

	"github.com/bojin/datamask/internal/transformer"
)

func (v *Validator) CheckTypes(ctx context.Context) []Finding {
	var findings []Finding

	if v.db == nil {
		return findings
	}

	tables, err := v.db.DiscoverTables(ctx)
	if err != nil {
		return findings
	}

	dbTypes := make(map[string]map[string]string)
	for _, tbl := range tables {
		name := tbl.FullName()
		cols := make(map[string]string)
		for _, col := range tbl.Columns {
			cols[col.Name] = col.DataType
		}
		dbTypes[name] = cols
	}

	for tableName, tblCfg := range v.cfg.Tables {
		cols, ok := dbTypes[tableName]
		if !ok {
			continue
		}
		for colName, txName := range tblCfg.Columns {
			colType, colExists := cols[colName]
			if !colExists {
				continue
			}
			t, err := transformer.Get(txName)
			if err != nil {
				continue
			}
			d, ok := t.(transformer.Described)
			if !ok {
				continue
			}
			supported := d.SupportedTypes()
			if !typeIsCompatible(colType, supported) {
				findings = append(findings, Finding{
					Severity: SeverityWarning,
					Category: "type",
					Table:    tableName,
					Column:   colName,
					Message:  fmt.Sprintf("transformer %q supports types [%s], but column type is %q", txName, strings.Join(supported, ", "), colType),
				})
			}
		}
	}

	return findings
}

func typeIsCompatible(colType string, supported []string) bool {
	normalized := strings.ToLower(strings.TrimSpace(colType))
	for _, s := range supported {
		st := strings.ToLower(strings.TrimSpace(s))
		if normalized == st || strings.HasPrefix(normalized, st) || strings.Contains(normalized, st) {
			return true
		}
	}
	return false
}
