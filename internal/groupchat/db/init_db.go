package db

import (
	"errors"
	"fmt"
	"strings"
)

type InitOptions struct {
	DataDir string

	EnableMigration bool
	EnableBackup    bool

	EnableLuckyBag bool
	EnableGRPC     bool
	EnableHeavyAPI bool
}

type InitReport struct {
	DataDir         string   `json:"dataDir"`
	MigrationRan    bool     `json:"migrationRan"`
	BackupRan       bool     `json:"backupRan"`
	EnabledModules  []string `json:"enabledModules"`
	ExcludedModules []string `json:"excludedModules"`
}

func InitDB(opts InitOptions) (*InitReport, error) {
	if strings.TrimSpace(opts.DataDir) == "" {
		return nil, errors.New("groupchat data dir is required")
	}

	report := &InitReport{
		DataDir: opts.DataDir,
		EnabledModules: []string{
			"groupchat-core-db",
			"socket-push-callbacks",
		},
	}

	if opts.EnableMigration {
		if err := runMigration(opts); err != nil {
			return nil, fmt.Errorf("run migration failed: %w", err)
		}
		report.MigrationRan = true
	}

	if opts.EnableBackup {
		if err := runBackup(opts); err != nil {
			return nil, fmt.Errorf("run backup failed: %w", err)
		}
		report.BackupRan = true
	}

	if !opts.EnableLuckyBag {
		report.ExcludedModules = append(report.ExcludedModules, "luckybag")
	}
	if !opts.EnableGRPC {
		report.ExcludedModules = append(report.ExcludedModules, "grpc")
	}
	if !opts.EnableHeavyAPI {
		report.ExcludedModules = append(report.ExcludedModules, "heavy-db-api")
	}

	return report, nil
}

func runMigration(opts InitOptions) error {
	// Placeholder for migration chain. No-op for current minimal extracted runtime.
	return nil
}

func runBackup(opts InitOptions) error {
	// Placeholder for backup chain. No-op for current minimal extracted runtime.
	return nil
}
