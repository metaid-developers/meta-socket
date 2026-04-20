package socket

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/metaid-developers/meta-socket/internal/config"
)

func TestMountOnlineRoutesRequiresManager(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	if err := MountOnlineRoutes(router, nil); err == nil {
		t.Fatalf("expected error when manager is nil")
	}
}

func TestOnlineRoutesStatsAndList(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	cfg := config.Default()
	manager, err := NewManager(cfg.Socket)
	if err != nil {
		t.Fatalf("new manager failed: %v", err)
	}

	manager.addDeviceConnection("bot_alpha", DeviceTypePC, "sock_1")
	manager.addDeviceConnection("bot_alpha", DeviceTypeAPP, "sock_2")
	manager.addDeviceConnection("bot_beta", DeviceTypePC, "sock_3")

	router := gin.New()
	if err := MountOnlineRoutes(router, manager); err != nil {
		t.Fatalf("mount online routes failed: %v", err)
	}

	statsReq := httptest.NewRequest(http.MethodGet, "/socket/online/stats", nil)
	statsW := httptest.NewRecorder()
	router.ServeHTTP(statsW, statsReq)

	if statsW.Code != http.StatusOK {
		t.Fatalf("stats status=%d body=%s", statsW.Code, statsW.Body.String())
	}

	var statsResp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			TotalMetaBots    int `json:"totalMetaBots"`
			TotalConnections int `json:"totalConnections"`
		} `json:"data"`
	}
	if err := json.Unmarshal(statsW.Body.Bytes(), &statsResp); err != nil {
		t.Fatalf("unmarshal stats response: %v", err)
	}
	if statsResp.Data.TotalMetaBots != 2 {
		t.Fatalf("expected 2 online metabot, got %d", statsResp.Data.TotalMetaBots)
	}
	if statsResp.Data.TotalConnections != 3 {
		t.Fatalf("expected 3 online connections, got %d", statsResp.Data.TotalConnections)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/socket/online/list?page=1&size=1", nil)
	listW := httptest.NewRecorder()
	router.ServeHTTP(listW, listReq)
	if listW.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", listW.Code, listW.Body.String())
	}

	var listResp struct {
		Code int `json:"code"`
		Data struct {
			Page             int `json:"page"`
			Size             int `json:"size"`
			Total            int `json:"total"`
			TotalConnections int `json:"totalConnections"`
			Items            []struct {
				MetaID          string `json:"metaId"`
				ConnectionCount int    `json:"connectionCount"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(listW.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("unmarshal list response: %v", err)
	}

	if listResp.Data.Page != 1 || listResp.Data.Size != 1 {
		t.Fatalf("unexpected pagination page=%d size=%d", listResp.Data.Page, listResp.Data.Size)
	}
	if listResp.Data.Total != 2 {
		t.Fatalf("expected total=2, got %d", listResp.Data.Total)
	}
	if listResp.Data.TotalConnections != 3 {
		t.Fatalf("expected totalConnections=3, got %d", listResp.Data.TotalConnections)
	}
	if len(listResp.Data.Items) != 1 {
		t.Fatalf("expected 1 item in page, got %d", len(listResp.Data.Items))
	}
}

func TestOnlineListRouteNormalizesInvalidPagination(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	cfg := config.Default()
	manager, err := NewManager(cfg.Socket)
	if err != nil {
		t.Fatalf("new manager failed: %v", err)
	}
	manager.addDeviceConnection("bot_gamma", DeviceTypePC, "sock_9")

	router := gin.New()
	if err := MountOnlineRoutes(router, manager); err != nil {
		t.Fatalf("mount online routes failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/socket/online/list?page=0&size=9999", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Data struct {
			Page int `json:"page"`
			Size int `json:"size"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if resp.Data.Page != 1 {
		t.Fatalf("expected default page=1, got %d", resp.Data.Page)
	}
	if resp.Data.Size != 200 {
		t.Fatalf("expected normalized size=200, got %d", resp.Data.Size)
	}
}
