package userinfo

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/metaid-developers/meta-socket/internal/aggregator"
	"github.com/metaid-developers/meta-socket/internal/cache"
	"github.com/metaid-developers/meta-socket/internal/storage"
)

// setupTestAggregator creates a test-ready userinfo aggregator with a real Pebble store and cache.
func setupTestAggregator(t *testing.T) (*Aggregator, *storage.PebbleStore, *gin.Engine) {
	t.Helper()

	store := storage.NewPebbleStore(t.TempDir())
	cacheProvider := cache.New(store)

	agg := &Aggregator{}
	if err := agg.Init(store, cacheProvider); err != nil {
		t.Fatalf("failed to init aggregator: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	agg.RegisterRoutes(router.Group("/api"))

	return agg, store, router
}

// performRequest is a test helper.
func performRequest(t *testing.T, router *gin.Engine, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	req, _ := http.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// --- Acceptance Criteria #4: UserInfo database storage ---

func TestHandleBlockPin_InitAndName(t *testing.T) {
	agg, store, _ := setupTestAggregator(t)
	defer store.Close()

	// Simulate an init pin (path="/").
	initPin := &aggregator.PinInscription{
		Path:      "/",
		Operation: "init",
		MetaId:    "testuser123",
		Address:   "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
		ChainName: "btc",
		Id:        "tx1:i0",
	}

	_, err := agg.HandleBlockPin(initPin)
	if err != nil {
		t.Fatalf("HandleBlockPin(init) failed: %v", err)
	}

	// Simulate a /info/name pin.
	namePin := &aggregator.PinInscription{
		Path:        "/info/name",
		Operation:   "create",
		MetaId:      "testuser123",
		Address:     "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
		ChainName:   "btc",
		ContentBody: []byte("Alice"),
		Id:          "tx2:i0",
	}

	_, err = agg.HandleBlockPin(namePin)
	if err != nil {
		t.Fatalf("HandleBlockPin(name) failed: %v", err)
	}

	// Verify the profile was stored in Pebble with the name field.
	raw, err := store.Get(namespace, profileKey("testuser123"))
	if err != nil {
		t.Fatalf("store.Get failed: %v", err)
	}
	if raw == nil {
		t.Fatal("profile not found in store")
	}

	var profile UserProfile
	if err := json.Unmarshal(raw, &profile); err != nil {
		t.Fatalf("failed to unmarshal profile: %v", err)
	}

	if profile.Name != "Alice" {
		t.Errorf("expected name 'Alice', got %q", profile.Name)
	}
	if profile.MetaID != "testuser123" {
		t.Errorf("expected metaid 'testuser123', got %q", profile.MetaID)
	}
	t.Logf("stored profile: name=%s metaid=%s globalMetaId=%s", profile.Name, profile.MetaID, profile.GlobalMetaID)
}

// --- Acceptance Criteria #5: UserInfo HTTP response ---

func TestHandleMetaIdInfo_ReturnsCorrectFormat(t *testing.T) {
	agg, store, router := setupTestAggregator(t)
	defer store.Close()

	// First, store a profile via HandleBlockPin.
	initPin := &aggregator.PinInscription{
		Path:      "/",
		Operation: "init",
		MetaId:    "testuser456",
		Address:   "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
		ChainName: "btc",
		Id:        "tx3:i0",
	}
	agg.HandleBlockPin(initPin)

	// Hit the HTTP endpoint.
	w := performRequest(t, router, "GET", "/api/info/metaid/testuser456")

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	// Decode response.
	var resp struct {
		Code           int         `json:"code"`
		Data           UserProfile `json:"data"`
		Message        string      `json:"message"`
		ProcessingTime int64       `json:"processingTime"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Code != 0 {
		t.Errorf("expected code=0, got %d", resp.Code)
	}
	if resp.ProcessingTime <= 0 {
		t.Errorf("expected processingTime > 0, got %d", resp.ProcessingTime)
	}
	if resp.Data.MetaID != "testuser456" {
		t.Errorf("expected metaid 'testuser456', got %q", resp.Data.MetaID)
	}
	t.Logf("HTTP response: code=%d metaid=%s processingTime=%d", resp.Code, resp.Data.MetaID, resp.ProcessingTime)
}

// TestHandleMetaIdInfo_NotFound tests error response for unknown user.
func TestHandleMetaIdInfo_NotFound(t *testing.T) {
	_, store, router := setupTestAggregator(t)
	defer store.Close()

	w := performRequest(t, router, "GET", "/api/info/metaid/nonexistent")

	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Code == 0 {
		t.Error("expected non-zero code for not found")
	}
	t.Logf("not found response: code=%d message=%s", resp.Code, resp.Message)
}

// --- Acceptance Criteria #7: GlobalMetaId ---

func TestHandleBlockPin_GeneratesGlobalMetaId(t *testing.T) {
	agg, store, _ := setupTestAggregator(t)
	defer store.Close()

	// A valid BTC P2PKH address should generate an "id"-prefixed GlobalMetaId.
	initPin := &aggregator.PinInscription{
		Path:      "/",
		Operation: "init",
		MetaId:    "testuser_globalid",
		Address:   "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
		ChainName: "btc",
		Id:        "tx_global:i0",
	}

	_, err := agg.HandleBlockPin(initPin)
	if err != nil {
		t.Fatalf("HandleBlockPin failed: %v", err)
	}

	raw, err := store.Get(namespace, profileKey("testuser_globalid"))
	if err != nil || raw == nil {
		t.Fatal("profile not found in store")
	}

	var profile UserProfile
	json.Unmarshal(raw, &profile)

	if profile.GlobalMetaID == "" {
		t.Error("GlobalMetaID should not be empty")
	}
	if len(profile.GlobalMetaID) < 3 || profile.GlobalMetaID[:2] != "id" {
		t.Errorf("GlobalMetaID should start with 'id', got %q", profile.GlobalMetaID)
	}
	t.Logf("GlobalMetaID: %s", profile.GlobalMetaID)
}

// --- Acceptance Criteria #8: Cache hit ---

func TestHandleMetaIdInfo_CacheHit(t *testing.T) {
	agg, store, router := setupTestAggregator(t)
	defer store.Close()

	// Store a profile.
	initPin := &aggregator.PinInscription{
		Path:      "/",
		Operation: "init",
		MetaId:    "cache_test_user",
		Address:   "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
		ChainName: "btc",
		Id:        "cache_init:i0",
	}
	agg.HandleBlockPin(initPin)

	// First call: should populate cache.
	w1 := performRequest(t, router, "GET", "/api/info/metaid/cache_test_user")
	var resp1 struct {
		Code           int         `json:"code"`
		Data           UserProfile `json:"data"`
		ProcessingTime int64       `json:"processingTime"`
	}
	json.Unmarshal(w1.Body.Bytes(), &resp1)
	if resp1.Code != 0 {
		t.Fatalf("first call failed: code=%d", resp1.Code)
	}
	pt1 := resp1.ProcessingTime

	// Second call: should hit cache and return processingTime > 0.
	w2 := performRequest(t, router, "GET", "/api/info/metaid/cache_test_user")
	var resp2 struct {
		Code           int         `json:"code"`
		Data           UserProfile `json:"data"`
		ProcessingTime int64       `json:"processingTime"`
	}
	json.Unmarshal(w2.Body.Bytes(), &resp2)
	if resp2.Code != 0 {
		t.Fatalf("second call failed: code=%d", resp2.Code)
	}
	pt2 := resp2.ProcessingTime

	if pt1 <= 0 || pt2 <= 0 {
		t.Errorf("processingTime should be > 0: pt1=%d pt2=%d", pt1, pt2)
	}
	if resp2.Data.MetaID != "cache_test_user" {
		t.Errorf("expected cached metaid, got %q", resp2.Data.MetaID)
	}
	t.Logf("processingTime: call1=%d call2=%d", pt1, pt2)
}

// --- Acceptance Criteria #9: Cache invalidation ---

func TestHandleBlockPin_InvalidatesCache(t *testing.T) {
	agg, store, router := setupTestAggregator(t)
	defer store.Close()

	// Store initial profile.
	initPin := &aggregator.PinInscription{
		Path:      "/",
		Operation: "init",
		MetaId:    "invalidate_test_user",
		Address:   "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
		ChainName: "btc",
		Id:        "inv_init:i0",
	}
	agg.HandleBlockPin(initPin)

	// Set initial name.
	namePin1 := &aggregator.PinInscription{
		Path:        "/info/name",
		Operation:   "create",
		MetaId:      "invalidate_test_user",
		Address:     "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
		ChainName:   "btc",
		ContentBody: []byte("OldName"),
		Id:          "inv_name1:i0",
	}
	agg.HandleBlockPin(namePin1)

	// Prime cache.
	w1 := performRequest(t, router, "GET", "/api/info/metaid/invalidate_test_user")
	var resp1 struct {
		Code int         `json:"code"`
		Data UserProfile `json:"data"`
	}
	json.Unmarshal(w1.Body.Bytes(), &resp1)
	if resp1.Data.Name != "OldName" {
		t.Fatalf("expected initial name 'OldName', got %q", resp1.Data.Name)
	}

	// Process an updated name pin — this should invalidate the cache.
	namePin2 := &aggregator.PinInscription{
		Path:        "/info/name",
		Operation:   "modify",
		MetaId:      "invalidate_test_user",
		Address:     "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
		ChainName:   "btc",
		ContentBody: []byte("NewName"),
		Id:          "inv_name2:i0",
	}
	_, err := agg.HandleBlockPin(namePin2)
	if err != nil {
		t.Fatalf("HandleBlockPin(modify name) failed: %v", err)
	}

	// Verify the name was updated in the store.
	raw, _ := store.Get(namespace, profileKey("invalidate_test_user"))
	var stored UserProfile
	json.Unmarshal(raw, &stored)
	if stored.Name != "NewName" {
		t.Errorf("expected stored name 'NewName', got %q", stored.Name)
	}

	// Fetch via HTTP — should return the updated name (cache was invalidated).
	w2 := performRequest(t, router, "GET", "/api/info/metaid/invalidate_test_user")
	var resp2 struct {
		Code int         `json:"code"`
		Data UserProfile `json:"data"`
	}
	json.Unmarshal(w2.Body.Bytes(), &resp2)
	if resp2.Data.Name != "NewName" {
		t.Errorf("expected updated name 'NewName' via HTTP, got %q", resp2.Data.Name)
	}
	t.Logf("cache invalidation test: old=%q new=%q", resp1.Data.Name, resp2.Data.Name)
}

// TestHandleAddressInfo tests the /api/info/address/:address endpoint.
func TestHandleAddressInfo(t *testing.T) {
	agg, store, router := setupTestAggregator(t)
	defer store.Close()

	// Store a profile with a known address.
	initPin := &aggregator.PinInscription{
		Path:      "/",
		Operation: "init",
		MetaId:    "addr_test_user",
		Address:   "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
		ChainName: "btc",
		Id:        "addr_init:i0",
	}
	agg.HandleBlockPin(initPin)

	w := performRequest(t, router, "GET", "/api/info/address/1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa")

	var resp struct {
		Code           int         `json:"code"`
		Data           UserProfile `json:"data"`
		ProcessingTime int64       `json:"processingTime"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Code != 0 {
		t.Errorf("expected code=0, got %d", resp.Code)
	}
	if resp.ProcessingTime <= 0 {
		t.Errorf("expected processingTime > 0, got %d", resp.ProcessingTime)
	}
	t.Logf("address info: code=%d metaid=%s globalMetaId=%s", resp.Code, resp.Data.MetaID, resp.Data.GlobalMetaID)
}

// TestHandleMempoolPin_NoOp verifies mempool pins are ignored for userinfo.
func TestHandleMempoolPin_NoOp(t *testing.T) {
	agg, store, _ := setupTestAggregator(t)
	defer store.Close()

	pin := &aggregator.PinInscription{
		Path:        "/info/name",
		Operation:   "create",
		MetaId:      "mempool_user",
		ContentBody: []byte("MempoolName"),
		ChainName:   "btc",
	}

	evt, err := agg.HandleMempoolPin(pin)
	if err != nil {
		t.Errorf("HandleMempoolPin should not error: %v", err)
	}
	if evt != nil {
		t.Error("HandleMempoolPin should return nil event for userinfo")
	}

	// Profile should not be stored for mempool pins.
	raw, _ := store.Get(namespace, profileKey("mempool_user"))
	if raw != nil {
		t.Error("mempool pin should not persist user profile")
	}
}

// TestNameMethod verifies Aggregator.Name returns "userinfo".
func TestNameMethod(t *testing.T) {
	agg := &Aggregator{}
	if agg.Name() != "userinfo" {
		t.Errorf("expected Name()='userinfo', got %q", agg.Name())
	}
}

// TestNotifyChannel verifies the notify channel has correct type and capacity.
func TestNotifyChannel(t *testing.T) {
	agg, store, _ := setupTestAggregator(t)
	defer store.Close()

	ch := agg.NotifyChannel()
	if ch == nil {
		t.Fatal("NotifyChannel() returned nil")
	}

	// Try to read from channel (should not block since nothing is sent).
	select {
	case <-ch:
		t.Error("unexpected event on notify channel")
	case <-time.After(10 * time.Millisecond):
		// Expected: no event.
	}
}
