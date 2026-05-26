package groupchat

import (
	"github.com/gin-gonic/gin"
	"github.com/metaid-developers/meta-socket/internal/aggregator"
	"github.com/metaid-developers/meta-socket/internal/cache"
	"github.com/metaid-developers/meta-socket/internal/storage"
)

// Aggregator wraps the existing groupchat processing with the Aggregator interface.
// In v1, it delegates to the existing callback-based processor for Pin handling
// and adds Pebble persistence + HTTP query APIs.
type Aggregator struct {
	store    *storage.PebbleStore
	cache    *cache.Cache[[]byte]
	notifyCh chan *aggregator.NotifyEvent
}

const (
	namespace      = "groupchat"
	cacheMaxEntries = 2000
	cacheTTL        = 5 * 60 // 5 min default (seconds)
)

func (a *Aggregator) Name() string { return "groupchat" }

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
	// In v1, block pin handling is not yet implemented.
	// The existing ZMQ pipeline handles mempool pins via the callback system.
	// Block scanning will populate Pebble for query APIs.
	return nil, nil
}

func (a *Aggregator) HandleMempoolPin(pin *aggregator.PinInscription) (*aggregator.NotifyEvent, error) {
	// Delegated to existing pipeline via callbacks.
	// This aggregator adds persistence for later HTTP queries.
	return nil, nil
}

// RegisterRoutes mounts groupchat HTTP query endpoints.
// In v1, this is a placeholder — the existing pipeline does not persist data.
// Routes will be populated once Pebble-backed block scanning is added.
func (a *Aggregator) RegisterRoutes(router *gin.RouterGroup) {
	// Placeholder: group chat query routes will be added here
	// when block scanning + persistence are implemented.
}
