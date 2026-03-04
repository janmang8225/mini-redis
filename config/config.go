package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Port      int    `yaml:"port"`
	LogLevel  string `yaml:"log_level"`
	MaxMemory string `yaml:"max_memory"` // e.g. "256mb", "1gb"

	Persistence PersistenceConfig `yaml:"persistence"`
	Dashboard   DashboardConfig   `yaml:"dashboard"`
}

type PersistenceConfig struct {
	AOF              bool   `yaml:"aof"`
	AOFFile          string `yaml:"aof_file"`
	Snapshot         bool   `yaml:"snapshot"`
	SnapshotFile     string `yaml:"snapshot_file"`
	SnapshotInterval int    `yaml:"snapshot_interval_seconds"`
}

type DashboardConfig struct {
	Enabled bool `yaml:"enabled"`
	Port    int  `yaml:"port"`
}

func Load(path string) (*Config, error) {
	// sensible defaults
	cfg := &Config{
		Port:     6379,
		LogLevel: "info",
		Persistence: PersistenceConfig{
			AOF:              true,
			AOFFile:          "miniredis.aof",
			Snapshot:         true,
			SnapshotFile:     "miniredis.rdb",
			SnapshotInterval: 300,
		},
		Dashboard: DashboardConfig{
			Enabled: true,
			Port:    8080,
		},
	}

	data, err := os.ReadFile(path)
	if err != nil {
		// config file optional — just use defaults
		return cfg, nil
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}