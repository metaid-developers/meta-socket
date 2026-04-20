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
}
