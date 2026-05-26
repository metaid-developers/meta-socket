package main

import (
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/metaid-developers/meta-socket/internal/aggregator"
	"github.com/metaid-developers/meta-socket/internal/aggregator/notify"
	"github.com/metaid-developers/meta-socket/internal/api"
	"github.com/metaid-developers/meta-socket/internal/cache"
	"github.com/metaid-developers/meta-socket/internal/config"
	"github.com/metaid-developers/meta-socket/internal/storage"
)

var version = "dev"

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// --- Pebble store ---
	var store *storage.PebbleStore
	if cfg.Pebble.Enabled {
		store = storage.NewPebbleStore(cfg.Pebble.DataDir)
		log.Printf("pebble store: dataDir=%s", cfg.Pebble.DataDir)
	} else {
		log.Printf("pebble store disabled")
	}

	// --- Cache provider ---
	var cacheProvider *cache.CacheProvider
	if store != nil {
		cacheProvider = cache.New(store)
		log.Printf("cache: maxEntries=%d ttl=%ds", cfg.Cache.MaxEntries, cfg.Cache.DefaultTTLSeconds)
	}

	// --- Aggregator registry ---
	var aggRegistry *aggregator.Registry
	if store != nil && cacheProvider != nil {
		aggRegistry = aggregator.NewRegistry(store, cacheProvider)

		if err := aggRegistry.Register(&notify.Aggregator{}); err != nil {
			log.Printf("WARNING: notify aggregator init failed: %v", err)
		}
		log.Printf("aggregators registered: %d", len(aggRegistry.All()))
	}

	// --- HTTP router ---
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	// Health check (no auth, minimal)
	router.GET(cfg.Service.HealthPath, func(c *gin.Context) {
		api.RespSuccess(c, gin.H{
			"status":  "ok",
			"service": "meta-socket",
			"version": version,
		})
	})

	// Aggregator routes
	if aggRegistry != nil {
		for _, a := range aggRegistry.All() {
			a.RegisterRoutes(router.Group("/"))
		}
	}

	// --- Start HTTP server ---
	srv := &http.Server{
		Addr:              cfg.Service.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	shutdownCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("meta-socket started: %s", cfg.Summary())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-shutdownCtx.Done()

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Service.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
	if store != nil {
		store.Close()
	}
	log.Printf("meta-socket stopped")
}
