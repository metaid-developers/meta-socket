package db

import "testing"

func TestInitDB_DefaultP0Exclusions(t *testing.T) {
	report, err := InitDB(InitOptions{
		DataDir:         "./data/pebble",
		EnableMigration: true,
		EnableBackup:    false,
		EnableLuckyBag:  false,
		EnableGRPC:      false,
		EnableHeavyAPI:  false,
	})
	if err != nil {
		t.Fatalf("init db failed: %v", err)
	}
	if !report.MigrationRan {
		t.Fatalf("expected migration to run")
	}
	if report.BackupRan {
		t.Fatalf("expected backup to be disabled")
	}
	if len(report.ExcludedModules) != 3 {
		t.Fatalf("expected 3 excluded modules, got %d", len(report.ExcludedModules))
	}
}

func TestInitDB_BackupSwitch(t *testing.T) {
	report, err := InitDB(InitOptions{
		DataDir:         "./data/pebble",
		EnableMigration: false,
		EnableBackup:    true,
		EnableLuckyBag:  false,
		EnableGRPC:      false,
		EnableHeavyAPI:  false,
	})
	if err != nil {
		t.Fatalf("init db failed: %v", err)
	}
	if report.MigrationRan {
		t.Fatalf("expected migration disabled")
	}
	if !report.BackupRan {
		t.Fatalf("expected backup enabled")
	}
}
