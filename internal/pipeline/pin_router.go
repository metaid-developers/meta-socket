package pipeline

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/metaid-developers/meta-socket/internal/adapter"
)

type PinKind string

const (
	PinKindUnknown   PinKind = "unknown"
	PinKindGroup     PinKind = "group"
	PinKindPrivate   PinKind = "private"
	PinKindGroupRole PinKind = "group_role"
)

type PinHandler func(pin *adapter.PinRecord, tx json.RawMessage) error

type GlobalMetaIDResolver interface {
	ResolveByAddress(address string) (string, error)
}

type RouterHandlers struct {
	OnGroup     PinHandler
	OnPrivate   PinHandler
	OnGroupRole PinHandler
}

type PinRouter struct {
	resolver      GlobalMetaIDResolver
	handlers      RouterHandlers
	metaIDMapping sync.Map // map[metaid]globalMetaId
}

func NewPinRouter(resolver GlobalMetaIDResolver, handlers RouterHandlers) *PinRouter {
	return &PinRouter{
		resolver: resolver,
		handlers: handlers,
	}
}

func (r *PinRouter) RoutePin(pin *adapter.PinRecord, tx json.RawMessage) error {
	if pin == nil {
		return nil
	}
	r.ensureGlobalMetaID(pin)

	switch r.Classify(pin.Path) {
	case PinKindGroup:
		if r.handlers.OnGroup != nil {
			return r.handlers.OnGroup(pin, tx)
		}
	case PinKindPrivate:
		if r.handlers.OnPrivate != nil {
			return r.handlers.OnPrivate(pin, tx)
		}
	case PinKindGroupRole:
		if r.handlers.OnGroupRole != nil {
			return r.handlers.OnGroupRole(pin, tx)
		}
	}
	return nil
}

func (r *PinRouter) Classify(path string) PinKind {
	protocol := normalizeProtocol(path)
	switch protocol {
	case "simplegroupchat", "simplefilegroupchat", "simplegroupluckybag", "simplegroupopenluckybag", "simplegroupresidueluckybag":
		return PinKindGroup
	case "simplemsg", "simplefilemsg", "simplecardmsg", "simpleblock":
		return PinKindPrivate
	case "simplegroupjoin", "simplegroupremoveuser", "simplegroupadmin", "simplegroupblock", "simplegroupwhitelist", "simplegroupjoinwhitelist", "simplegroupjoinblock", "simplegroupcreate", "simplegroupchannel":
		return PinKindGroupRole
	default:
		return PinKindUnknown
	}
}

func (r *PinRouter) ensureGlobalMetaID(pin *adapter.PinRecord) {
	if pin == nil {
		return
	}
	if pin.GlobalMetaID != "" {
		r.rememberMapping(pin.CreateMetaID, pin.GlobalMetaID)
		r.rememberMapping(pin.MetaID, pin.GlobalMetaID)
		return
	}

	if value, ok := r.lookupMapping(pin.CreateMetaID); ok {
		pin.GlobalMetaID = value
		return
	}
	if value, ok := r.lookupMapping(pin.MetaID); ok {
		pin.GlobalMetaID = value
		return
	}
	if looksLikeGlobalMetaID(pin.CreateMetaID) {
		pin.GlobalMetaID = pin.CreateMetaID
		r.rememberMapping(pin.CreateMetaID, pin.GlobalMetaID)
		return
	}
	if looksLikeGlobalMetaID(pin.MetaID) {
		pin.GlobalMetaID = pin.MetaID
		r.rememberMapping(pin.MetaID, pin.GlobalMetaID)
		return
	}
	if r.resolver != nil && pin.CreateAddress != "" {
		if value, err := r.resolver.ResolveByAddress(pin.CreateAddress); err == nil && value != "" {
			pin.GlobalMetaID = value
			r.rememberMapping(pin.CreateMetaID, value)
			r.rememberMapping(pin.MetaID, value)
			return
		}
	}
	if pin.CreateMetaID != "" {
		pin.GlobalMetaID = pin.CreateMetaID
		r.rememberMapping(pin.CreateMetaID, pin.GlobalMetaID)
	}
}

func (r *PinRouter) rememberMapping(metaID, globalMetaID string) {
	if metaID == "" || globalMetaID == "" {
		return
	}
	r.metaIDMapping.Store(metaID, globalMetaID)
}

func (r *PinRouter) lookupMapping(metaID string) (string, bool) {
	if metaID == "" {
		return "", false
	}
	value, ok := r.metaIDMapping.Load(metaID)
	if !ok {
		return "", false
	}
	globalMetaID, ok := value.(string)
	if !ok || globalMetaID == "" {
		return "", false
	}
	return globalMetaID, true
}

func normalizeProtocol(path string) string {
	value := strings.TrimSpace(strings.ToLower(path))
	value = strings.TrimPrefix(value, "/")
	if strings.HasPrefix(value, "protocols/") {
		value = strings.TrimPrefix(value, "protocols/")
	}
	return value
}

func looksLikeGlobalMetaID(value string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	return strings.HasPrefix(value, "id")
}
