package pipeline

import (
	"testing"

	"github.com/metaid-developers/meta-socket/internal/config"
)

func TestBuildEnabledAdapters(t *testing.T) {
	cfg := config.ZMQConfig{
		Enabled: true,
		BTC: config.ChainZMQConfig{
			Enabled:  true,
			Endpoint: "tcp://127.0.0.1:28332",
			Topic:    "rawtx",
		},
		MVC: config.ChainZMQConfig{
			Enabled:  false,
			Endpoint: "tcp://127.0.0.1:28333",
			Topic:    "rawtx",
		},
		DOGE: config.ChainZMQConfig{
			Enabled:  true,
			Endpoint: "tcp://127.0.0.1:28334",
			Topic:    "rawtx",
		},
	}

	adapters := BuildEnabledAdapters(cfg)
	if len(adapters) != 2 {
		t.Fatalf("expected 2 enabled adapters, got %d", len(adapters))
	}
	if adapters[0].Name() != "btc" {
		t.Fatalf("first adapter should be btc, got %s", adapters[0].Name())
	}
	if adapters[1].Name() != "doge" {
		t.Fatalf("second adapter should be doge, got %s", adapters[1].Name())
	}
}
