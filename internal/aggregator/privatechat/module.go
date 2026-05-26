package privatechat

import (
	"github.com/gin-gonic/gin"
	"github.com/metaid-developers/meta-socket/internal/aggregator"
	"github.com/metaid-developers/meta-socket/internal/cache"
	"github.com/metaid-developers/meta-socket/internal/storage"
)

// Aggregator wraps private chat processing with the Aggregator interface.
// In v1, it delegates to the existing callback-based processor for Pin handling
// and adds Pebble persistence + HTTP query APIs.
type Aggregator struct {
	store    *storage.PebbleStore
	cache    *cache.Cache[[]byte]
	notifyCh chan *aggregator.NotifyEvent
}

const (
	namespace       = "privatechat"
	cacheMaxEntries = 2000
	cacheTTL        = 5 * 60 // 5 min
)

func (a *Aggregator) Name() string { return "privatechat" }

func (a *Aggregator) Init(store *storage.PebbleStore, cacheProvider *cache.CacheProvider) error {
	a.store = store
	a.cache = cacheProvider.Namespace(namespace, cacheMaxEntries, cacheTTL)
	a.notifyCh = make(chan *aggregator.NotifyEvent, 256)
	return nil
}

func (a *Aggregator) NotifyChannel() <-chan *aggregator.NotifyEvent {
	return a.notifyCh
}

func (a *Aggregator) HandleBlockPin(pin *aggregator.PinInscription) (*aggregator.NotifyEvent, error) {
	return nil, nil
}

func (a *Aggregator) HandleMempoolPin(pin *aggregator.PinInscription) (*aggregator.NotifyEvent, error) {
	return nil, nil
}

func (a *Aggregator) RegisterRoutes(router *gin.RouterGroup) {
	// Placeholder: private chat query routes will be added here
}
