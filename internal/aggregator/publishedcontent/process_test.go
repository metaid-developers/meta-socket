package publishedcontent

import (
	"encoding/json"
	"testing"

	"github.com/metaid-developers/metaso-p2p/internal/aggregator"
	"github.com/metaid-developers/metaso-p2p/internal/cache"
	"github.com/metaid-developers/metaso-p2p/internal/storage"
)

func setupTestAggregator(t *testing.T) (*Aggregator, *storage.PebbleStore) {
	t.Helper()

	store := storage.NewPebbleStore(t.TempDir())
	cacheProvider := cache.New(store)
	agg := &Aggregator{}
	if err := agg.Init(store, cacheProvider); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return agg, store
}

type contentPinOpts struct {
	PinId          string
	Path           string
	Operation      string
	ChainName      string
	OriginalId     string
	Timestamp      int64
	ContentBody    []byte
	ContentSummary string
	ContentType    string
	GlobalMetaId   string
	MetaId         string
	Address        string
}

func makeContentPin(opts contentPinOpts) *aggregator.PinInscription {
	if opts.Path == "" {
		opts.Path = PathSimpleBuzz
	}
	if opts.Operation == "" {
		opts.Operation = OperationCreate
	}
	if opts.ChainName == "" {
		opts.ChainName = "mvc"
	}
	if opts.GlobalMetaId == "" {
		opts.GlobalMetaId = "gid-user"
	}
	if opts.MetaId == "" {
		opts.MetaId = "meta-user"
	}
	if opts.Address == "" {
		opts.Address = "addr-user"
	}
	if opts.ContentType == "" {
		opts.ContentType = "text/plain"
	}
	return &aggregator.PinInscription{
		Id:             opts.PinId,
		Path:           opts.Path,
		Operation:      opts.Operation,
		ContentBody:    opts.ContentBody,
		ContentSummary: opts.ContentSummary,
		ContentType:    opts.ContentType,
		ChainName:      opts.ChainName,
		GlobalMetaId:   opts.GlobalMetaId,
		MetaId:         opts.MetaId,
		CreateMetaId:   opts.MetaId,
		Address:        opts.Address,
		CreateAddress:  opts.Address,
		Timestamp:      opts.Timestamp,
		OriginalId:     opts.OriginalId,
	}
}

func mustProcess(t *testing.T, agg *Aggregator, pin *aggregator.PinInscription) {
	t.Helper()
	if _, err := agg.HandleBlockPin(pin); err != nil {
		t.Fatalf("HandleBlockPin(%s): %v", pin.Id, err)
	}
}

func mustLoadRecord(t *testing.T, agg *Aggregator, chainName, protocolPath, sourcePinId string) *Record {
	t.Helper()
	rec, err := agg.loadRecord(chainName, protocolPath, sourcePinId)
	if err != nil {
		t.Fatalf("loadRecord(%s/%s/%s): %v", chainName, protocolPath, sourcePinId, err)
	}
	if rec == nil {
		t.Fatalf("expected record %s/%s/%s", chainName, protocolPath, sourcePinId)
	}
	return rec
}

func TestProcessCreateModifyRevokeFoldsCurrentRecord(t *testing.T) {
	agg, store := setupTestAggregator(t)
	defer store.Close()

	mustProcess(t, agg, makeContentPin(contentPinOpts{
		PinId:       "buzz-create:i0",
		Operation:   OperationCreate,
		Timestamp:   1000,
		ContentBody: []byte("hello world"),
	}))
	mustProcess(t, agg, makeContentPin(contentPinOpts{
		PinId:       "buzz-modify:i0",
		Path:        PathSimpleBuzz + "@buzz-create:i0",
		Operation:   OperationModify,
		Timestamp:   2000,
		ContentBody: []byte("edited world"),
	}))
	mustProcess(t, agg, makeContentPin(contentPinOpts{
		PinId:       "buzz-revoke:i0",
		Path:        PathSimpleBuzz + "@buzz-modify:i0",
		Operation:   OperationRevoke,
		Timestamp:   3000,
		ContentBody: []byte("ignored"),
	}))

	rec := mustLoadRecord(t, agg, "mvc", PathSimpleBuzz, "buzz-create:i0")
	if rec.SourcePinId != "buzz-create:i0" {
		t.Fatalf("SourcePinId: got %q", rec.SourcePinId)
	}
	if rec.CurrentPinId != "buzz-revoke:i0" {
		t.Fatalf("CurrentPinId: got %q", rec.CurrentPinId)
	}
	if rec.Operation != OperationRevoke {
		t.Fatalf("Operation: got %q", rec.Operation)
	}
	if !rec.Hidden {
		t.Fatalf("revoked record should be hidden")
	}
	if rec.CreatedAt != 1000 || rec.UpdatedAt != 3000 {
		t.Fatalf("timestamps: createdAt=%d updatedAt=%d", rec.CreatedAt, rec.UpdatedAt)
	}
	if rec.PayloadText != "edited world" {
		t.Fatalf("revoke should preserve last exposed payload, got %q", rec.PayloadText)
	}

	result, err := agg.List(ListParams{
		ProtocolPath:          PathSimpleBuzz,
		PublisherGlobalMetaId: "gid-user",
		Size:                  5,
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(result.Items) != 0 {
		t.Fatalf("revoked records should be hidden by default, got %d", len(result.Items))
	}
}

func TestPayloadFallsBackToContentSummary(t *testing.T) {
	agg, store := setupTestAggregator(t)
	defer store.Close()

	mustProcess(t, agg, makeContentPin(contentPinOpts{
		PinId:          "summary-create:i0",
		Operation:      OperationCreate,
		Timestamp:      1000,
		ContentBody:    []byte(" \n\t "),
		ContentSummary: `{"title":"from summary","count":2}`,
		ContentType:    "application/json",
	}))

	rec := mustLoadRecord(t, agg, "mvc", PathSimpleBuzz, "summary-create:i0")
	if !rec.PayloadExposed {
		t.Fatalf("expected fallback payload to be exposed")
	}
	if rec.PayloadText != "" {
		t.Fatalf("JSON object fallback should not populate PayloadText: %q", rec.PayloadText)
	}
	if rec.PayloadJSON == nil {
		t.Fatal("expected PayloadJSON from contentSummary fallback")
	}
	raw, _ := json.Marshal(rec.PayloadJSON)
	if string(raw) != `{"count":2,"title":"from summary"}` {
		t.Fatalf("PayloadJSON: %s", raw)
	}
}

func TestBinaryPayloadIsNotExposed(t *testing.T) {
	agg, store := setupTestAggregator(t)
	defer store.Close()

	mustProcess(t, agg, makeContentPin(contentPinOpts{
		PinId:       "binary-create:i0",
		Operation:   OperationCreate,
		Timestamp:   1000,
		ContentType: "image/png",
		ContentBody: []byte{0x89, 'P', 'N', 'G', 0x00},
	}))

	rec := mustLoadRecord(t, agg, "mvc", PathSimpleBuzz, "binary-create:i0")
	if rec.PayloadExposed {
		t.Fatal("binary payload should not be exposed")
	}
	if rec.PayloadText != "" || rec.PayloadJSON != nil {
		t.Fatalf("binary payload leaked: text=%q json=%v", rec.PayloadText, rec.PayloadJSON)
	}
}
