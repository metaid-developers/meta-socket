package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Service ServiceConfig `json:"service"`
	Socket  SocketConfig  `json:"socket"`
	ZMQ     ZMQConfig     `json:"zmq"`
	Pebble  PebbleConfig  `json:"pebble"`
	Profile ProfileConfig `json:"profile"`
}

type ServiceConfig struct {
	HTTPAddr        string        `json:"httpAddr"`
	HealthPath      string        `json:"healthPath"`
	ShutdownTimeout time.Duration `json:"shutdownTimeout"`
}

type SocketConfig struct {
	Enabled     bool   `json:"enabled"`
	PrimaryPath string `json:"primaryPath"`
	LegacyPath  string `json:"legacyPath"`
}

type ZMQConfig struct {
	Enabled bool           `json:"enabled"`
	BTC     ChainZMQConfig `json:"btc"`
	MVC     ChainZMQConfig `json:"mvc"`
	DOGE    ChainZMQConfig `json:"doge"`
}

type ChainZMQConfig struct {
	Enabled  bool   `json:"enabled"`
	Endpoint string `json:"endpoint"`
	Topic    string `json:"topic"`
}

type PebbleConfig struct {
	Enabled bool   `json:"enabled"`
	DataDir string `json:"dataDir"`
}

type ProfileConfig struct {
	Enabled             bool   `json:"enabled"`
	Mode                string `json:"mode"`
	RemoteBaseURL       string `json:"remoteBaseURL"`
	AllowRemoteFallback bool   `json:"allowRemoteFallback"`
}

func Default() Config {
	return Config{
		Service: ServiceConfig{
			HTTPAddr:        ":8080",
			HealthPath:      "/healthz",
			ShutdownTimeout: 10 * time.Second,
		},
		Socket: SocketConfig{
			Enabled:     true,
			PrimaryPath: "/socket/socket.io",
			LegacyPath:  "/socket.io",
		},
		ZMQ: ZMQConfig{
			Enabled: false,
			BTC: ChainZMQConfig{
				Enabled:  false,
				Endpoint: "",
				Topic:    "hashblock",
			},
			MVC: ChainZMQConfig{
				Enabled:  false,
				Endpoint: "",
				Topic:    "hashblock",
			},
			DOGE: ChainZMQConfig{
				Enabled:  false,
				Endpoint: "",
				Topic:    "hashblock",
			},
		},
		Pebble: PebbleConfig{
			Enabled: true,
			DataDir: "./data/pebble",
		},
		Profile: ProfileConfig{
			Enabled:             true,
			Mode:                "local-first",
			RemoteBaseURL:       "",
			AllowRemoteFallback: true,
		},
	}
}

func Load() (Config, error) {
	cfg := Default()

	applyStringEnv("META_SOCKET_HTTP_ADDR", &cfg.Service.HTTPAddr)
	applyStringEnv("META_SOCKET_HEALTH_PATH", &cfg.Service.HealthPath)
	applyDurationEnv("META_SOCKET_SHUTDOWN_TIMEOUT", &cfg.Service.ShutdownTimeout)

	applyBoolEnv("META_SOCKET_SOCKET_ENABLED", &cfg.Socket.Enabled)
	applyStringEnv("META_SOCKET_SOCKET_PATH", &cfg.Socket.PrimaryPath)
	applyStringEnv("META_SOCKET_SOCKET_LEGACY_PATH", &cfg.Socket.LegacyPath)

	applyBoolEnv("META_SOCKET_ZMQ_ENABLED", &cfg.ZMQ.Enabled)
	applyBoolEnv("META_SOCKET_ZMQ_BTC_ENABLED", &cfg.ZMQ.BTC.Enabled)
	applyStringEnv("META_SOCKET_ZMQ_BTC_ENDPOINT", &cfg.ZMQ.BTC.Endpoint)
	applyStringEnv("META_SOCKET_ZMQ_BTC_TOPIC", &cfg.ZMQ.BTC.Topic)
	applyBoolEnv("META_SOCKET_ZMQ_MVC_ENABLED", &cfg.ZMQ.MVC.Enabled)
	applyStringEnv("META_SOCKET_ZMQ_MVC_ENDPOINT", &cfg.ZMQ.MVC.Endpoint)
	applyStringEnv("META_SOCKET_ZMQ_MVC_TOPIC", &cfg.ZMQ.MVC.Topic)
	applyBoolEnv("META_SOCKET_ZMQ_DOGE_ENABLED", &cfg.ZMQ.DOGE.Enabled)
	applyStringEnv("META_SOCKET_ZMQ_DOGE_ENDPOINT", &cfg.ZMQ.DOGE.Endpoint)
	applyStringEnv("META_SOCKET_ZMQ_DOGE_TOPIC", &cfg.ZMQ.DOGE.Topic)

	applyBoolEnv("META_SOCKET_PEBBLE_ENABLED", &cfg.Pebble.Enabled)
	applyStringEnv("META_SOCKET_PEBBLE_DATA_DIR", &cfg.Pebble.DataDir)

	applyBoolEnv("META_SOCKET_PROFILE_ENABLED", &cfg.Profile.Enabled)
	applyStringEnv("META_SOCKET_PROFILE_MODE", &cfg.Profile.Mode)
	applyStringEnv("META_SOCKET_PROFILE_REMOTE_BASE_URL", &cfg.Profile.RemoteBaseURL)
	applyBoolEnv("META_SOCKET_PROFILE_ALLOW_REMOTE_FALLBACK", &cfg.Profile.AllowRemoteFallback)

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Service.HTTPAddr) == "" {
		return errors.New("service.httpAddr is required")
	}
	if !strings.HasPrefix(c.Service.HealthPath, "/") {
		return errors.New("service.healthPath must start with '/'")
	}
	if strings.TrimSpace(c.Socket.PrimaryPath) == "" {
		return errors.New("socket.primaryPath is required")
	}
	if strings.TrimSpace(c.Socket.LegacyPath) == "" {
		return errors.New("socket.legacyPath is required")
	}
	if c.Service.ShutdownTimeout <= 0 {
		return errors.New("service.shutdownTimeout must be greater than zero")
	}
	return nil
}

func applyStringEnv(name string, target *string) {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		*target = value
	}
}

func applyBoolEnv(name string, target *bool) {
	value, ok := os.LookupEnv(name)
	if !ok {
		return
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return
	}
	*target = parsed
}

func applyDurationEnv(name string, target *time.Duration) {
	value, ok := os.LookupEnv(name)
	if !ok {
		return
	}
	parsed, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil {
		return
	}
	*target = parsed
}

func (c Config) Summary() string {
	return fmt.Sprintf(
		"listen=%s health=%s socket_enabled=%t socket_path=%s socket_legacy_path=%s zmq_enabled=%t pebble_enabled=%t profile_enabled=%t",
		c.Service.HTTPAddr,
		c.Service.HealthPath,
		c.Socket.Enabled,
		c.Socket.PrimaryPath,
		c.Socket.LegacyPath,
		c.ZMQ.Enabled,
		c.Pebble.Enabled,
		c.Profile.Enabled,
	)
}
