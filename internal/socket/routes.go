package socket

import (
	"errors"
	"log"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/metaid-developers/meta-socket/internal/config"
)

func MountRoutes(router gin.IRoutes, manager *Manager, cfg config.SocketConfig) error {
	if router == nil {
		return errors.New("router is nil")
	}
	if manager == nil {
		return errors.New("socket manager is nil")
	}

	handler := manager.Handler()
	paths := uniqueSocketPaths(cfg.PrimaryPath, cfg.LegacyPath)
	if len(paths) == 0 {
		return errors.New("no socket route paths configured")
	}

	for _, path := range paths {
		// Socket.IO polling/websocket requests are routed through catch-all suffix.
		router.GET(path+"/*f", gin.WrapH(handler))
		router.POST(path+"/*f", gin.WrapH(handler))
	}
	log.Printf("socket routes mounted: %v", paths)
	return nil
}

func uniqueSocketPaths(paths ...string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(paths))
	for _, raw := range paths {
		normalized := normalizeSocketPath(raw)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func normalizeSocketPath(raw string) string {
	path := strings.TrimSpace(raw)
	if path == "" {
		return ""
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		return ""
	}
	return path
}
