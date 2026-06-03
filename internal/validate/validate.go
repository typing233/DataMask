package validate

import (
	"context"
	"fmt"

	"github.com/bojin/datamask/internal/config"
	"github.com/bojin/datamask/internal/database"
)

type Severity int

const (
	SeverityWarning Severity = iota
	SeverityError
)

func (s Severity) String() string {
	if s == SeverityError {
		return "ERROR"
	}
	return "WARNING"
}

type Finding struct {
	Severity Severity
	Category string
	Table    string
	Column   string
	Message  string
}

func (f Finding) String() string {
	loc := ""
	if f.Table != "" {
		loc = f.Table
		if f.Column != "" {
			loc += "." + f.Column
		}
		loc += ": "
	}
	return fmt.Sprintf("[%s] %s: %s%s", f.Severity, f.Category, loc, f.Message)
}

type Validator struct {
	cfg        *config.Config
	db         database.Database
	dsn        string
	sampleRows int
}

func New(cfg *config.Config, db database.Database, dsn string, sampleRows int) *Validator {
	return &Validator{
		cfg:        cfg,
		db:         db,
		dsn:        dsn,
		sampleRows: sampleRows,
	}
}

func (v *Validator) RunAll(ctx context.Context) []Finding {
	var findings []Finding
	findings = append(findings, v.CheckConfig()...)
	findings = append(findings, v.CheckSchema(ctx)...)
	findings = append(findings, v.CheckTypes(ctx)...)
	findings = append(findings, v.CheckDiff(ctx)...)
	return findings
}

func (v *Validator) RunChecks(ctx context.Context, checks []string) []Finding {
	var findings []Finding
	for _, check := range checks {
		switch check {
		case "config":
			findings = append(findings, v.CheckConfig()...)
		case "schema":
			findings = append(findings, v.CheckSchema(ctx)...)
		case "types":
			findings = append(findings, v.CheckTypes(ctx)...)
		case "diff":
			findings = append(findings, v.CheckDiff(ctx)...)
		}
	}
	return findings
}
