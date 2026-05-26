package api

import (
	"github.com/gin-gonic/gin"

	"github.com/metaid-developers/meta-socket/internal/aggregator"
	"github.com/metaid-developers/meta-socket/internal/cache"
	"github.com/metaid-developers/meta-socket/internal/config"
	"github.com/metaid-developers/meta-socket/internal/socket"
	"github.com/metaid-developers/meta-socket/internal/storage"
)

// SetupRouter creates and configures the Gin router with all routes.
// It centralizes route registration for health checks, socket.io, aggregator APIs, and presence.
func SetupRouter(
	cfg config.Config,
	store *storage.PebbleStore,
	cacheProvider *cache.CacheProvider,
	aggRegistry *aggregator.Registry,
	socketServer *socket.Server,
	version string,
) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	// Health check
	router.GET(cfg.Service.HealthPath, func(c *gin.Context) {
		RespSuccess(c, gin.H{
			"status":  "ok",
			"service": "meta-socket",
			"version": version,
		})
	})

	// Socket.IO routes
	if socketServer != nil {
		handler := socketServer.Handler()

		// Primary path: /socket/socket.io
		router.Any(cfg.Socket.PrimaryPath+"/*any", handler)

		// Legacy path: /socket.io
		router.Any(cfg.Socket.LegacyPath+"/*any", handler)

		// Presence routes
		socketServer.RegisterPresenceRoutes(router)
	}

	// Aggregator routes (mounted under /api/ prefix per idchat API contract).
	if aggRegistry != nil {
		for _, a := range aggRegistry.All() {
			a.RegisterRoutes(router.Group("/api"))
		}

		// Also expose the userinfo aggregator under /metafile-indexer/api so
		// idchat's `metafileIndexerApi` client (configured as
		// `<metaFSBaseURL>/metafile-indexer/api`) can target meta-socket as a
		// drop-in replacement for the meta-file-system user info subset
		// without any frontend code changes. Only `/info/*` routes are
		// duplicated here; file upload / avatar content stay with
		// meta-file-system.
		metafileGroup := router.Group("/metafile-indexer/api")
		for _, a := range aggRegistry.All() {
			if a.Name() == "userinfo" {
				a.RegisterRoutes(metafileGroup)
			}
		}
	}

	return router
}
