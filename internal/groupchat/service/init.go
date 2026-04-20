package service

import (
	"fmt"
	"log"

	"github.com/metaid-developers/meta-socket/internal/config"
	"github.com/metaid-developers/meta-socket/internal/groupchat/db"
	"github.com/metaid-developers/meta-socket/internal/groupchat/push"
)

type InitReport struct {
	DBReport        *db.InitReport `json:"dbReport"`
	ExcludedModules []string       `json:"excludedModules"`
}

func Init(cfg config.Config) (*InitReport, error) {
	dbReport, err := db.InitDB(db.InitOptions{
		DataDir:         cfg.Pebble.DataDir,
		EnableMigration: cfg.GroupChat.MigrationEnabled,
		EnableBackup:    cfg.GroupChat.BackupEnabled,
		EnableLuckyBag:  cfg.GroupChat.LuckyBagEnabled,
		EnableGRPC:      cfg.GroupChat.GRPCEnabled,
		EnableHeavyAPI:  cfg.GroupChat.HeavyAPIEnabled,
	})
	if err != nil {
		return nil, fmt.Errorf("groupchat db init failed: %w", err)
	}

	push.Configure(push.ServiceConfig{
		RoomBroadcastEnabled: cfg.Socket.RoomBroadcastEnabled,
	})
	push.RegisterDBHooks()

	report := &InitReport{
		DBReport:        dbReport,
		ExcludedModules: append([]string{}, dbReport.ExcludedModules...),
	}

	log.Printf(
		"[GROUPCHAT-INIT] migration=%t backup=%t excluded=%v",
		dbReport.MigrationRan,
		dbReport.BackupRan,
		dbReport.ExcludedModules,
	)

	return report, nil
}
