package validate

import (
	"fmt"

	"github.com/bojin/datamask/internal/transformer"
)

func (v *Validator) CheckConfig() []Finding {
	var findings []Finding

	if v.cfg.StorageDir == "" {
		findings = append(findings, Finding{
			Severity: SeverityWarning,
			Category: "config",
			Message:  "storage_dir is empty, will use default ./dumps",
		})
	}

	if v.cfg.Parallelism <= 0 {
		findings = append(findings, Finding{
			Severity: SeverityWarning,
			Category: "config",
			Message:  "parallelism not set or invalid, will default to 4",
		})
	}

	if v.cfg.Connection.Host == "" {
		findings = append(findings, Finding{
			Severity: SeverityError,
			Category: "config",
			Message:  "connection.host is required",
		})
	}

	if v.cfg.Connection.DBName == "" {
		findings = append(findings, Finding{
			Severity: SeverityError,
			Category: "config",
			Message:  "connection.dbname is required",
		})
	}

	if v.cfg.Connection.User == "" {
		findings = append(findings, Finding{
			Severity: SeverityError,
			Category: "config",
			Message:  "connection.user is required",
		})
	}

	for tableName, tblCfg := range v.cfg.Tables {
		for colName, txName := range tblCfg.Columns {
			if _, err := transformer.Get(txName); err != nil {
				findings = append(findings, Finding{
					Severity: SeverityError,
					Category: "config",
					Table:    tableName,
					Column:   colName,
					Message:  fmt.Sprintf("unknown transformer: %q", txName),
				})
			}
		}
	}

	for _, inc := range v.cfg.IncludeTables {
		for _, exc := range v.cfg.ExcludeTables {
			if inc == exc {
				findings = append(findings, Finding{
					Severity: SeverityWarning,
					Category: "config",
					Table:    inc,
					Message:  "table appears in both include_tables and exclude_tables",
				})
			}
		}
	}

	return findings
}
