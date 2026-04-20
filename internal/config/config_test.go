package config

import "testing"

func TestDefaultZMQTopicIsRawTx(t *testing.T) {
	cfg := Default()
	if cfg.ZMQ.BTC.Topic != "rawtx" {
		t.Fatalf("expected btc topic rawtx, got %s", cfg.ZMQ.BTC.Topic)
	}
	if cfg.ZMQ.MVC.Topic != "rawtx" {
		t.Fatalf("expected mvc topic rawtx, got %s", cfg.ZMQ.MVC.Topic)
	}
	if cfg.ZMQ.DOGE.Topic != "rawtx" {
		t.Fatalf("expected doge topic rawtx, got %s", cfg.ZMQ.DOGE.Topic)
	}
	if !cfg.ZMQ.BTC.RPCHTTPPostMode || !cfg.ZMQ.MVC.RPCHTTPPostMode || !cfg.ZMQ.DOGE.RPCHTTPPostMode {
		t.Fatalf("expected default rpc http-post-mode=true for all chains")
	}
	if !cfg.ZMQ.BTC.RPCDisableTLS || !cfg.ZMQ.MVC.RPCDisableTLS || !cfg.ZMQ.DOGE.RPCDisableTLS {
		t.Fatalf("expected default rpc disable-tls=true for all chains")
	}
}
