package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/metaid-developers/meta-socket/internal/adapter"
	"github.com/metaid-developers/meta-socket/internal/config"
	"github.com/metaid-developers/meta-socket/internal/pipeline"
	metasocket "github.com/metaid-developers/meta-socket/internal/socket"
)

var version = "dev"

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	var zmqRunner *pipeline.ZMQRunner
	var stopPipeline context.CancelFunc

	if cfg.Socket.Enabled {
		socketManager, err := metasocket.NewManager(cfg.Socket)
		if err != nil {
			log.Fatalf("failed to initialize socket manager: %v", err)
		}
		metasocket.SetGlobalManager(socketManager)
		if err := metasocket.MountRoutes(router, socketManager, cfg.Socket); err != nil {
			log.Fatalf("failed to mount socket routes: %v", err)
		}
		log.Printf("socket service enabled on paths: %s, %s", cfg.Socket.PrimaryPath, cfg.Socket.LegacyPath)
	} else {
		log.Printf("socket service disabled by config")
	}

	if cfg.ZMQ.Enabled {
		handlers := pipeline.RouterHandlers{
			OnGroup: func(pin *adapter.PinRecord, tx json.RawMessage) error {
				log.Printf("[PIPELINE] group pin routed: chain=%s pinId=%s path=%s globalMetaId=%s", pin.ChainName, pin.ID, pin.Path, pin.GlobalMetaID)
				return nil
			},
			OnPrivate: func(pin *adapter.PinRecord, tx json.RawMessage) error {
				log.Printf("[PIPELINE] private pin routed: chain=%s pinId=%s path=%s globalMetaId=%s", pin.ChainName, pin.ID, pin.Path, pin.GlobalMetaID)
				return nil
			},
			OnGroupRole: func(pin *adapter.PinRecord, tx json.RawMessage) error {
				log.Printf("[PIPELINE] group-role pin routed: chain=%s pinId=%s path=%s globalMetaId=%s", pin.ChainName, pin.ID, pin.Path, pin.GlobalMetaID)
				return nil
			},
		}
		pinRouter := pipeline.NewPinRouter(nil, handlers)
		zmqRunner = pipeline.NewZMQRunner(pinRouter)

		enabledAdapters := pipeline.BuildEnabledAdapters(cfg.ZMQ)
		for _, item := range enabledAdapters {
			zmqRunner.RegisterAdapter(item)
		}

		runCtx, cancel := context.WithCancel(context.Background())
		stopPipeline = cancel
		if err := zmqRunner.Start(runCtx); err != nil {
			log.Fatalf("failed to start zmq runner: %v", err)
		}
		log.Printf("zmq pipeline enabled: adapters=%d", len(enabledAdapters))
	} else {
		log.Printf("zmq pipeline disabled by config")
	}

	router.GET(cfg.Service.HealthPath, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "meta-socket",
			"version": version,
		})
	})

	srv := &http.Server{
		Addr:              cfg.Service.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	shutdownCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serverErrCh := make(chan error, 1)
	go func() {
		log.Printf("meta-socket bootstrap service started: %s", cfg.Summary())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrCh <- err
		}
		close(serverErrCh)
	}()

	select {
	case err := <-serverErrCh:
		if err != nil {
			log.Fatalf("server exited unexpectedly: %v", err)
		}
	case <-shutdownCtx.Done():
	}

	if stopPipeline != nil {
		stopPipeline()
	}
	if zmqRunner != nil {
		zmqRunner.Wait()
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Service.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("graceful shutdown failed: %v", err)
	}
	log.Printf("meta-socket stopped cleanly")
}
