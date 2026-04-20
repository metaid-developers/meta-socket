package pipeline

import (
	"context"
	"errors"
	"log"
	"sync"

	"github.com/metaid-developers/meta-socket/internal/adapter"
	"github.com/metaid-developers/meta-socket/internal/config"
)

type ZMQRunner struct {
	router   *PinRouter
	adapters []adapter.ChainZMQAdapter
	inbox    chan adapter.MempoolMessage
	wg       sync.WaitGroup
}

func NewZMQRunner(router *PinRouter) *ZMQRunner {
	return &ZMQRunner{
		router: router,
		inbox:  make(chan adapter.MempoolMessage, 1024),
	}
}

func (r *ZMQRunner) RegisterAdapter(item adapter.ChainZMQAdapter) {
	if item == nil {
		return
	}
	r.adapters = append(r.adapters, item)
}

func (r *ZMQRunner) Start(ctx context.Context) error {
	if r.router == nil {
		return errors.New("pin router is nil")
	}
	if len(r.adapters) == 0 {
		log.Printf("[ZMQ] no chain adapters registered, skip runner startup")
		return nil
	}

	r.wg.Add(1)
	go r.consume(ctx)

	for _, item := range r.adapters {
		adapterItem := item
		r.wg.Add(1)
		go func() {
			defer r.wg.Done()
			if err := adapterItem.Run(ctx, r.inbox); err != nil {
				log.Printf("[ZMQ] adapter stopped with error: chain=%s err=%v", adapterItem.Name(), err)
			} else {
				log.Printf("[ZMQ] adapter stopped: chain=%s", adapterItem.Name())
			}
		}()
	}
	return nil
}

func (r *ZMQRunner) Wait() {
	r.wg.Wait()
}

func (r *ZMQRunner) consume(ctx context.Context) {
	defer r.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-r.inbox:
			for _, pin := range msg.PinList {
				if err := r.router.RoutePin(pin, msg.Tx); err != nil {
					log.Printf("[ZMQ] route pin failed: chain=%s pinId=%s path=%s err=%v", msg.Chain, pin.ID, pin.Path, err)
				}
			}
		}
	}
}

func BuildEnabledAdapters(cfg config.ZMQConfig) []adapter.ChainZMQAdapter {
	items := make([]adapter.ChainZMQAdapter, 0, 3)
	if cfg.BTC.Enabled {
		items = append(items, adapter.NewBTCZMQAdapter(cfg.BTC.Endpoint, cfg.BTC.Topic, buildAdapterOptions("btc", cfg.BTC)...))
	}
	if cfg.MVC.Enabled {
		items = append(items, adapter.NewMVCZMQAdapter(cfg.MVC.Endpoint, cfg.MVC.Topic, buildAdapterOptions("mvc", cfg.MVC)...))
	}
	if cfg.DOGE.Enabled {
		items = append(items, adapter.NewDOGEZMQAdapter(cfg.DOGE.Endpoint, cfg.DOGE.Topic, buildAdapterOptions("doge", cfg.DOGE)...))
	}
	return items
}

func buildAdapterOptions(chain string, item config.ChainZMQConfig) []adapter.JSONZMQAdapterOption {
	options := make([]adapter.JSONZMQAdapterOption, 0, 1)
	lookup, err := adapter.NewRPCPrevoutValueLookup(
		item.RPCHost,
		item.RPCUser,
		item.RPCPass,
		item.RPCHTTPPostMode,
		item.RPCDisableTLS,
	)
	if err != nil {
		log.Printf("[ZMQ] chain=%s init rpc prevout lookup failed, fallback to heuristic owner mapping: %v", chain, err)
		return options
	}
	if lookup != nil {
		options = append(options, adapter.WithPrevoutValueLookup(lookup))
	}
	return options
}
