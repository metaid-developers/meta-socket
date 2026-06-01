package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/metaid-developers/meta-socket/internal/aggregator"
	"github.com/metaid-developers/meta-socket/internal/aggregator/groupchat"
	"github.com/metaid-developers/meta-socket/internal/aggregator/notify"
	"github.com/metaid-developers/meta-socket/internal/aggregator/privatechat"
	"github.com/metaid-developers/meta-socket/internal/aggregator/userinfo"
	"github.com/metaid-developers/meta-socket/internal/api"
	"github.com/metaid-developers/meta-socket/internal/cache"
	"github.com/metaid-developers/meta-socket/internal/config"
	"github.com/metaid-developers/meta-socket/internal/storage"
)

// setupFullRouter mirrors cmd/meta-socket/main.go's aggregator wiring so the
// tests below catch real-world routing regressions (e.g. forgetting to register
// privatechat, which causes its routes to fall back to groupchat stubs).
func setupFullRouter(t *testing.T) *gin.Engine {
	t.Helper()

	store := storage.NewPebbleStore(t.TempDir())
	t.Cleanup(func() { store.Close() })
	cacheProvider := cache.New(store)

	reg := aggregator.NewRegistry(store, cacheProvider)
	if err := reg.Register(&notify.Aggregator{}); err != nil {
		t.Fatalf("register notify: %v", err)
	}
	if err := reg.Register(&userinfo.Aggregator{}); err != nil {
		t.Fatalf("register userinfo: %v", err)
	}
	if err := reg.Register(&groupchat.Aggregator{}); err != nil {
		t.Fatalf("register groupchat: %v", err)
	}
	if err := reg.Register(&privatechat.Aggregator{}); err != nil {
		t.Fatalf("register privatechat: %v", err)
	}

	cfg := config.Default()
	// SetupRouter handles nil socketServer gracefully (Socket.IO routes skipped).
	return api.SetupRouter(cfg, store, cacheProvider, reg, nil, "test")
}

func get(t *testing.T, router *gin.Engine, path string) (*httptest.ResponseRecorder, map[string]interface{}) {
	t.Helper()
	req, _ := http.NewRequest("GET", path, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var body map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	return w, body
}

func postJSON(t *testing.T, router *gin.Engine, path string, body string) (*httptest.ResponseRecorder, map[string]interface{}) {
	t.Helper()
	req, _ := http.NewRequest("POST", path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var decoded map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &decoded)
	return w, decoded
}

// TestRouter_AggregatorRegistrationDoesNotPanic ensures all four aggregators
// can be registered together without gin panicking on duplicate routes. This
// is a regression test for the previous state where private-chat routes were
// registered as stubs by groupchat AND by privatechat, which made registering
// both impossible.
func TestRouter_AggregatorRegistrationDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("router setup panicked (likely duplicate route): %v", r)
		}
	}()
	_ = setupFullRouter(t)
}

// TestRouter_PrivateChatRoutesHandledByPrivateChat verifies the four
// /api/group-chat/private-* routes are handled by the privatechat aggregator
// (which returns a real payload) rather than the groupchat handleStub (which
// returns an empty object `{}`).
//
// Each case picks a discriminator that the privatechat handler exposes but the
// stub does not:
//   - "list"   = data is an object with a "list" field.
//   - "object" = data is a plain object that must be non-empty (has at least
//     one of the privatechat fields total/nextCursor/list).
//   - "array"  = data is an array; the stub would have returned {}.
func TestRouter_PrivateChatRoutesHandledByPrivateChat(t *testing.T) {
	router := setupFullRouter(t)

	cases := []struct {
		path  string
		shape string // "object_with_list" | "array"
	}{
		{"/api/group-chat/private-chat-list?metaId=a&otherMetaId=b", "object_with_list"},
		{"/api/group-chat/private-chat-list-by-index?metaId=a&otherMetaId=b", "object_with_list"},
		{"/api/group-chat/private-group-paths?metaId=a", "array"},
		{"/api/group-chat/chat/homes/some_metaid", "object_with_list"},
	}

	for _, tc := range cases {
		w, body := get(t, router, tc.path)
		if w.Code != http.StatusOK {
			t.Errorf("%s: expected 200, got %d body=%s", tc.path, w.Code, w.Body.String())
			continue
		}
		code, _ := body["code"].(float64)
		if int(code) != 0 {
			t.Errorf("%s: expected code=0, got %v body=%s", tc.path, body["code"], w.Body.String())
			continue
		}

		switch tc.shape {
		case "object_with_list":
			data, ok := body["data"].(map[string]interface{})
			if !ok {
				t.Errorf("%s: data is not an object (groupchat stub would be {}): %v",
					tc.path, body["data"])
				continue
			}
			if _, present := data["list"]; !present {
				t.Errorf("%s: expected privatechat field 'list' in data (groupchat stub would have returned empty {}); got data=%v",
					tc.path, data)
			}
		case "array":
			if _, ok := body["data"].([]interface{}); !ok {
				t.Errorf("%s: expected data to be array (privatechat returns []string); groupchat stub would have returned {}; got %T %v",
					tc.path, body["data"], body["data"])
			}
		}
	}
}

func TestRouter_CanonicalPrivateChatRoutesHandledByPrivateChat(t *testing.T) {
	router := setupFullRouter(t)

	cases := []struct {
		path  string
		shape string // "object_with_list" | "array"
	}{
		{"/api/private-chat/messages?metaId=a&otherMetaId=b", "object_with_list"},
		{"/api/private-chat/messages/by-index?metaId=a&otherMetaId=b", "object_with_list"},
		{"/api/private-chat/paths?metaId=a", "array"},
		{"/api/private-chat/homes/some_metaid", "object_with_list"},
	}

	for _, tc := range cases {
		w, body := get(t, router, tc.path)
		if w.Code != http.StatusOK {
			t.Errorf("%s: expected 200, got %d body=%s", tc.path, w.Code, w.Body.String())
			continue
		}
		code, _ := body["code"].(float64)
		if int(code) != 0 {
			t.Errorf("%s: expected code=0, got %v body=%s", tc.path, body["code"], w.Body.String())
			continue
		}

		switch tc.shape {
		case "object_with_list":
			data, ok := body["data"].(map[string]interface{})
			if !ok {
				t.Errorf("%s: data is not an object: %v", tc.path, body["data"])
				continue
			}
			if _, present := data["list"]; !present {
				t.Errorf("%s: expected privatechat field 'list' in data; got data=%v", tc.path, data)
			}
		case "array":
			if _, ok := body["data"].([]interface{}); !ok {
				t.Errorf("%s: expected data to be array; got %T %v", tc.path, body["data"], body["data"])
			}
		}
	}
}

// TestRouter_GroupChatRoutesStillWork verifies the surgical change to
// groupchat/api.go (removing the four private-chat stubs) didn't accidentally
// break other group-chat endpoints.
func TestRouter_GroupChatRoutesStillWork(t *testing.T) {
	router := setupFullRouter(t)

	// Pick three representative group-chat endpoints (community / group / chat).
	paths := []string{
		"/api/group-chat/community/list",
		"/api/group-chat/group-list?metaId=test",
		"/api/group-chat/group-chat-list-v2?groupId=test",
	}
	for _, p := range paths {
		w, body := get(t, router, p)
		if w.Code != http.StatusOK {
			t.Errorf("%s: expected 200, got %d", p, w.Code)
		}
		if _, ok := body["code"]; !ok {
			t.Errorf("%s: missing code field in response: %s", p, w.Body.String())
		}
	}
}

func TestRouter_IDChatChatAPICompatRoutes(t *testing.T) {
	router := setupFullRouter(t)

	cases := []struct {
		path  string
		shape string
	}{
		{"/chat-api/group-chat/community/list", "object_with_results"},
		{"/chat-api/group-chat/group-list?metaId=test", "object_with_list"},
		{"/chat-api/group-chat/private-chat-list?metaId=a&otherMetaId=b", "object_with_list"},
	}

	for _, tc := range cases {
		w, body := get(t, router, tc.path)
		if w.Code != http.StatusOK {
			t.Errorf("%s: expected 200, got %d body=%s", tc.path, w.Code, w.Body.String())
			continue
		}
		code, _ := body["code"].(float64)
		if int(code) != 0 {
			t.Errorf("%s: expected code=0, got %v body=%s", tc.path, body["code"], w.Body.String())
			continue
		}
		data, ok := body["data"].(map[string]interface{})
		if !ok {
			t.Errorf("%s: expected object data, got %T %v", tc.path, body["data"], body["data"])
			continue
		}

		switch tc.shape {
		case "object_with_results":
			if _, present := data["results"]; !present {
				t.Errorf("%s: expected data.results, got data=%v", tc.path, data)
			}
		case "object_with_list":
			if _, present := data["list"]; !present {
				t.Errorf("%s: expected data.list, got data=%v", tc.path, data)
			}
		}
	}
}

func TestRouter_IDChatPushBaseCompatRoutes(t *testing.T) {
	router := setupFullRouter(t)

	w, body := get(t, router, "/push-base/v1/push/get_user_blocked_chats?metaId=test")
	if w.Code != http.StatusOK {
		t.Fatalf("get_user_blocked_chats: expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	code, _ := body["code"].(float64)
	if int(code) != 0 {
		t.Fatalf("get_user_blocked_chats: expected code=0, got %v body=%s", body["code"], w.Body.String())
	}
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("get_user_blocked_chats: expected object data, got %T %v", body["data"], body["data"])
	}
	if _, present := data["blockedChats"]; !present {
		t.Fatalf("get_user_blocked_chats: expected blockedChats in data, got %v", data)
	}

	w, body = postJSON(t, router, "/push-base/v1/push/add_blocked_chat", `{"chatId":"group1","chatType":"group","metaId":"test","reason":"muted"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("add_blocked_chat: expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	code, _ = body["code"].(float64)
	if int(code) != 0 {
		t.Fatalf("add_blocked_chat: expected code=0, got %v body=%s", body["code"], w.Body.String())
	}
}

func TestRouter_IDChatCORSCompat(t *testing.T) {
	router := setupFullRouter(t)

	req, _ := http.NewRequest("GET", "/chat-api/group-chat/group-list?metaId=test", nil)
	req.Header.Set("Origin", "https://idchat.io")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /chat-api/group-chat/group-list: expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("GET /chat-api/group-chat/group-list: expected Access-Control-Allow-Origin *, got %q", got)
	}

	req, _ = http.NewRequest("OPTIONS", "/push-base/v1/push/add_blocked_chat", nil)
	req.Header.Set("Origin", "https://idchat.io")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "X-Signature,X-Public-Key,Content-Type")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("OPTIONS /push-base/v1/push/add_blocked_chat: expected 204, got %d body=%s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("OPTIONS /push-base/v1/push/add_blocked_chat: expected Access-Control-Allow-Origin *, got %q", got)
	}
	allowHeaders := w.Header().Get("Access-Control-Allow-Headers")
	for _, header := range []string{"Content-Type", "X-Signature", "X-Public-Key"} {
		if !strings.Contains(allowHeaders, header) {
			t.Fatalf("OPTIONS /push-base/v1/push/add_blocked_chat: expected Access-Control-Allow-Headers to include %s, got %q", header, allowHeaders)
		}
	}
	allowMethods := w.Header().Get("Access-Control-Allow-Methods")
	for _, method := range []string{"GET", "POST", "OPTIONS"} {
		if !strings.Contains(allowMethods, method) {
			t.Fatalf("OPTIONS /push-base/v1/push/add_blocked_chat: expected Access-Control-Allow-Methods to include %s, got %q", method, allowMethods)
		}
	}
}

// TestRouter_UserInfoMetaFileCompat verifies userinfo still hits both prefixes
// and uses the meta-file-system code convention (40400 not_found), so the
// privatechat fix did not regress the meta-file-system compatibility commit.
func TestRouter_UserInfoMetaFileCompat(t *testing.T) {
	router := setupFullRouter(t)

	for _, p := range []string{
		"/api/info/metaid/nonexistent",
		"/metafile-indexer/api/info/metaid/nonexistent",
	} {
		w, body := get(t, router, p)
		if w.Code != http.StatusOK {
			t.Errorf("%s: expected 200, got %d", p, w.Code)
		}
		code, _ := body["code"].(float64)
		if int(code) != 40400 {
			t.Errorf("%s: expected code=40400, got %v", p, body["code"])
		}
	}
}
