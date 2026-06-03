package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	StorageDir         string                 `yaml:"storage_dir"`
	Parallelism        int                    `yaml:"parallelism"`
	DefaultTransformer string                 `yaml:"default_transformer,omitempty"`
	Tables             map[string]TableConfig `yaml:"tables"`
	ExcludeTables      []string               `yaml:"exclude_tables,omitempty"`
	IncludeTables      []string               `yaml:"include_tables,omitempty"`
	Connection         ConnectionConfig       `yaml:"connection"`
	Subset             *SubsetConfig          `yaml:"subset,omitempty"`
}

type TableConfig struct {
	Columns map[string]string `yaml:"columns"`
	Exclude bool              `yaml:"exclude,omitempty"`
}

type SubsetConfig struct {
	Tables          map[string]SubsetTableConfig `yaml:"tables"`
	ResolveParents  bool                         `yaml:"resolve_parents"`
	ResolveChildren bool                         `yaml:"resolve_children"`
	MaxDepth        int                          `yaml:"max_depth"`
}

type SubsetTableConfig struct {
	Where string `yaml:"where"`
}

type ConnectionConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
	SSLMode  string `yaml:"sslmode"`
}

func (c ConnectionConfig) DSN() string {
	port := c.Port
	if port == 0 {
		port = 5432
	}
	sslmode := c.SSLMode
	if sslmode == "" {
		sslmode = "disable"
	}
	dsn := fmt.Sprintf("host=%s port=%d user=%s dbname=%s sslmode=%s",
		c.Host, port, c.User, c.DBName, sslmode)
	if c.Password != "" {
		dsn += fmt.Sprintf(" password=%s", c.Password)
	}
	return dsn
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	cfg.applyDefaults()
	return &cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Parallelism <= 0 {
		c.Parallelism = 4
	}
	if c.StorageDir == "" {
		c.StorageDir = "./dumps"
	}
}

func (c *Config) ShouldIncludeTable(schema, table string) bool {
	fullName := table
	if schema != "" && schema != "public" {
		fullName = schema + "." + table
	}

	if tblCfg, ok := c.Tables[fullName]; ok && tblCfg.Exclude {
		return false
	}
	if tblCfg, ok := c.Tables[table]; ok && tblCfg.Exclude {
		return false
	}

	for _, excluded := range c.ExcludeTables {
		if excluded == fullName || excluded == table {
			return false
		}
	}

	if len(c.IncludeTables) > 0 {
		for _, included := range c.IncludeTables {
			if included == fullName || included == table {
				return true
			}
		}
		return false
	}

	return true
}

func (c *Config) GetTableConfig(table string) *TableConfig {
	if tblCfg, ok := c.Tables[table]; ok {
		return &tblCfg
	}
	return nil
}
