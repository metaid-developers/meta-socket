package pipeline

import (
	"encoding/json"
	"testing"

	"github.com/metaid-developers/meta-socket/internal/adapter"
)

type testResolver struct {
	value string
}

func (r *testResolver) ResolveByAddress(address string) (string, error) {
	return r.value, nil
}

func TestClassify(t *testing.T) {
	router := NewPinRouter(nil, RouterHandlers{})
	cases := []struct {
		path string
		want PinKind
	}{
		{path: "/protocols/simplegroupchat", want: PinKindGroup},
		{path: "/protocols/simplemsg", want: PinKindPrivate},
		{path: "/protocols/simplegroupadmin", want: PinKindGroupRole},
		{path: "/protocols/unknown", want: PinKindUnknown},
	}
	for _, item := range cases {
		if got := router.Classify(item.path); got != item.want {
			t.Fatalf("path=%s got=%s want=%s", item.path, got, item.want)
		}
	}
}

func TestRoutePinEnsuresGlobalMetaIDFromResolver(t *testing.T) {
	var seen string
	router := NewPinRouter(&testResolver{value: "id_resolved_1"}, RouterHandlers{
		OnGroup: func(pin *adapter.PinRecord, tx json.RawMessage) error {
			seen = pin.GlobalMetaID
			return nil
		},
	})

	pin := &adapter.PinRecord{
		Path:          "/protocols/simplegroupchat",
		CreateMetaID:  "meta_local_1",
		CreateAddress: "1abc",
	}

	if err := router.RoutePin(pin, nil); err != nil {
		t.Fatalf("route pin failed: %v", err)
	}
	if seen != "id_resolved_1" {
		t.Fatalf("expected resolved globalMetaId, got %s", seen)
	}
}

func TestRoutePinReusesMetaMapping(t *testing.T) {
	var firstSeen string
	var secondSeen string
	router := NewPinRouter(nil, RouterHandlers{
		OnPrivate: func(pin *adapter.PinRecord, tx json.RawMessage) error {
			if firstSeen == "" {
				firstSeen = pin.GlobalMetaID
			} else {
				secondSeen = pin.GlobalMetaID
			}
			return nil
		},
	})

	first := &adapter.PinRecord{
		Path:         "/protocols/simplemsg",
		MetaID:       "meta_1",
		GlobalMetaID: "id_meta_1",
	}
	second := &adapter.PinRecord{
		Path:   "/protocols/simplemsg",
		MetaID: "meta_1",
	}

	if err := router.RoutePin(first, nil); err != nil {
		t.Fatalf("first route failed: %v", err)
	}
	if err := router.RoutePin(second, nil); err != nil {
		t.Fatalf("second route failed: %v", err)
	}
	if firstSeen != "id_meta_1" || secondSeen != "id_meta_1" {
		t.Fatalf("mapping reuse failed: first=%s second=%s", firstSeen, secondSeen)
	}
}
