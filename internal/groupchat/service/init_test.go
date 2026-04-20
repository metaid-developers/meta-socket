package service

import (
	"testing"

	"github.com/metaid-developers/meta-socket/internal/config"
)

func TestInit(t *testing.T) {
	cfg := config.Default()
	report, err := Init(cfg)
	if err != nil {
		t.Fatalf("init service failed: %v", err)
	}
	if report == nil || report.DBReport == nil {
		t.Fatalf("expected init report")
	}
	if !report.DBReport.MigrationRan {
		t.Fatalf("expected migration enabled by default")
	}
}
