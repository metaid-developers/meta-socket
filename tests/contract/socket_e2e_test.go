package contract_test

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/metaid-developers/meta-socket/internal/config"
	"github.com/metaid-developers/meta-socket/internal/socket"
)

func TestSocketE2EPathsAndHeartbeat(t *testing.T) {
	t.Parallel()

	serverURL, cfg := startSocketTestServer(t)
	paths := []string{cfg.Socket.PrimaryPath, cfg.Socket.LegacyPath}

	for idx, path := range paths {
		path := path
		t.Run(path, func(t *testing.T) {
			metaID := fmt.Sprintf("meta_e2e_%d", idx)
			wsURL := buildSocketWSURL(t, serverURL, path, map[string]string{
				"metaid": metaID,
				"type":   "pc",
			})
			probe, err := runSocketProbe(wsURL)
			if err != nil {
				t.Fatalf("run local socket probe: %v", err)
			}

			assertEnvelopeMethodCode(t, probe.ConnectAck, socket.WS_RESPONSE_SUCCESS, socket.WS_CODE_SEND_SUCCESS)
			assertEnvelopeMethodCode(t, probe.PongAck, "pong", socket.WS_CODE_SEND_SUCCESS)
			assertEnvelopeMethodCode(t, probe.HeartbeatAck, socket.HEART_BEAT, socket.WS_CODE_HEART_BEAT_BACK)
		})
	}
}

func TestSocketE2EMissingMetaIDRejected(t *testing.T) {
	t.Parallel()

	serverURL, cfg := startSocketTestServer(t)
	wsURL := buildSocketWSURL(t, serverURL, cfg.Socket.PrimaryPath, map[string]string{
		"type": "pc",
	})

	conn, err := dialSocketConn(wsURL)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	if err := completeSocketIOHandshake(conn, 8*time.Second); err != nil {
		t.Fatalf("complete socket.io handshake: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))

	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			// Connection closed / deadline exceeded before success ack is expected.
			return
		}

		envelope, ok := parseEnvelopeFromSocketPacket(string(payload))
		if !ok {
			continue
		}
		if envelope.M == socket.WS_RESPONSE_SUCCESS {
			t.Fatalf("unexpected success ack when metaid missing: %s", mustEnvelopeString(t, envelope))
		}
	}
}

func TestLegacyDualRunCompareConnectionAndHeartbeat(t *testing.T) {
	legacyBase := strings.TrimSpace(os.Getenv("META_SOCKET_LEGACY_WS_URL"))
	if legacyBase == "" {
		t.Skip("META_SOCKET_LEGACY_WS_URL is not set; skip legacy dual-run compare")
	}

	serverURL, cfg := startSocketTestServer(t)
	metaID := fmt.Sprintf("meta_dualrun_%d", time.Now().UnixNano())

	localURL := buildSocketWSURL(t, serverURL, cfg.Socket.PrimaryPath, map[string]string{
		"metaid": metaID,
		"type":   "pc",
	})
	legacyURL := appendSocketQuery(t, legacyBase, map[string]string{
		"metaid": metaID,
		"type":   "pc",
	})

	localProbe, err := runSocketProbe(localURL)
	if err != nil {
		t.Fatalf("run local socket probe: %v", err)
	}
	legacyProbe, err := runSocketProbe(legacyURL)
	if err != nil {
		t.Skipf("legacy endpoint unavailable for dual-run compare: %v", err)
	}

	assertMethodCodeEqual(t, "connect ack", localProbe.ConnectAck, legacyProbe.ConnectAck)
	assertMethodCodeEqual(t, "pong ack", localProbe.PongAck, legacyProbe.PongAck)
	assertMethodCodeEqual(t, "heartbeat ack", localProbe.HeartbeatAck, legacyProbe.HeartbeatAck)
}

type socketProbeResult struct {
	ConnectAck   *socket.SocketData
	PongAck      *socket.SocketData
	HeartbeatAck *socket.SocketData
}

func runSocketProbe(wsURL string) (*socketProbeResult, error) {
	conn, err := dialSocketConn(wsURL)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if err := completeSocketIOHandshake(conn, 8*time.Second); err != nil {
		return nil, fmt.Errorf("complete socket.io handshake: %w", err)
	}
	connectAck, err := waitEnvelopeByMethod(conn, socket.WS_RESPONSE_SUCCESS, 8*time.Second)
	if err != nil {
		return nil, fmt.Errorf("wait connect ack: %w", err)
	}

	if err := sendSocketIOEvent(conn, "ping"); err != nil {
		return nil, fmt.Errorf("send ping event: %w", err)
	}
	pongAck, err := waitEnvelopeByMethod(conn, "pong", 8*time.Second)
	if err != nil {
		return nil, fmt.Errorf("wait pong ack: %w", err)
	}

	heartbeatMsg, err := (&socket.SocketData{M: socket.HEART_BEAT}).ToString()
	if err != nil {
		return nil, fmt.Errorf("marshal heartbeat payload: %w", err)
	}
	if err := sendSocketIOEvent(conn, "message", heartbeatMsg); err != nil {
		return nil, fmt.Errorf("send heartbeat event: %w", err)
	}
	heartbeatAck, err := waitEnvelopeByMethod(conn, socket.HEART_BEAT, 8*time.Second)
	if err != nil {
		return nil, fmt.Errorf("wait heartbeat ack: %w", err)
	}

	return &socketProbeResult{
		ConnectAck:   connectAck,
		PongAck:      pongAck,
		HeartbeatAck: heartbeatAck,
	}, nil
}

func startSocketTestServer(t *testing.T) (string, config.Config) {
	t.Helper()

	gin.SetMode(gin.ReleaseMode)
	cfg := config.Default()
	manager, err := socket.NewManager(cfg.Socket)
	if err != nil {
		t.Fatalf("new socket manager: %v", err)
	}

	router := gin.New()
	if err := socket.MountRoutes(router, manager, cfg.Socket); err != nil {
		t.Fatalf("mount socket routes: %v", err)
	}

	ts := httptest.NewServer(router)
	t.Cleanup(ts.Close)
	return ts.URL, cfg
}

func buildSocketWSURL(t *testing.T, baseURL, path string, params map[string]string) string {
	t.Helper()

	u, err := url.Parse(baseURL)
	if err != nil {
		t.Fatalf("parse base URL: %v", err)
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	}
	u.Path = strings.TrimSuffix(path, "/") + "/"
	q := u.Query()
	q.Set("EIO", "4")
	q.Set("transport", "websocket")
	for key, value := range params {
		q.Set(key, value)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func appendSocketQuery(t *testing.T, base string, params map[string]string) string {
	t.Helper()

	u, err := url.Parse(base)
	if err != nil {
		t.Fatalf("parse legacy URL: %v", err)
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	}
	q := u.Query()
	q.Set("EIO", "4")
	q.Set("transport", "websocket")
	for key, value := range params {
		q.Set(key, value)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func dialSocketConn(wsURL string) (*websocket.Conn, error) {
	dialer := websocket.Dialer{
		HandshakeTimeout: 8 * time.Second,
	}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("dial websocket %s: %w", wsURL, err)
	}
	return conn, nil
}

func sendSocketIOEvent(conn *websocket.Conn, event string, payload ...interface{}) error {
	items := make([]interface{}, 0, 1+len(payload))
	items = append(items, event)
	items = append(items, payload...)
	body, err := json.Marshal(items)
	if err != nil {
		return fmt.Errorf("marshal socket.io event %s: %w", event, err)
	}
	return sendSocketIOPacket(conn, "42"+string(body))
}

func sendSocketIOPacket(conn *websocket.Conn, packet string) error {
	if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return fmt.Errorf("set write deadline: %w", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, []byte(packet)); err != nil {
		return fmt.Errorf("write socket packet %s: %w", packet, err)
	}
	return nil
}

func completeSocketIOHandshake(conn *websocket.Conn, timeout time.Duration) error {
	if conn == nil {
		return fmt.Errorf("websocket conn is nil")
	}
	if err := waitEngineIOOpen(conn, timeout); err != nil {
		return err
	}
	if err := sendSocketIOPacket(conn, "40"); err != nil {
		return fmt.Errorf("send connect packet: %w", err)
	}
	return nil
}

func waitEngineIOOpen(conn *websocket.Conn, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting engine.io open packet")
		}

		if err := conn.SetReadDeadline(deadline); err != nil {
			return fmt.Errorf("set read deadline: %w", err)
		}
		_, payload, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read websocket frame: %w", err)
		}
		frame := string(payload)

		if frame == "2" {
			if err := sendSocketIOPacket(conn, "3"); err != nil {
				return fmt.Errorf("write engine.io pong: %w", err)
			}
			continue
		}

		if strings.HasPrefix(frame, "0{") {
			return nil
		}
	}
}

func waitEnvelopeByMethod(conn *websocket.Conn, method string, timeout time.Duration) (*socket.SocketData, error) {
	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout waiting method=%s", method)
		}

		if err := conn.SetReadDeadline(deadline); err != nil {
			return nil, fmt.Errorf("set read deadline: %w", err)
		}
		_, payload, err := conn.ReadMessage()
		if err != nil {
			return nil, fmt.Errorf("read websocket frame: %w", err)
		}

		frame := string(payload)
		if frame == "2" {
			// engine.io ping
			if err := sendSocketIOPacket(conn, "3"); err != nil {
				return nil, fmt.Errorf("write engine.io pong: %w", err)
			}
			continue
		}

		envelope, ok := parseEnvelopeFromSocketPacket(frame)
		if !ok {
			continue
		}
		if envelope.M == method {
			return envelope, nil
		}
	}
}

func parseEnvelopeFromSocketPacket(frame string) (*socket.SocketData, bool) {
	idx := strings.Index(frame, "[")
	if idx < 0 {
		return nil, false
	}

	var eventPacket []json.RawMessage
	if err := json.Unmarshal([]byte(frame[idx:]), &eventPacket); err != nil {
		return nil, false
	}
	if len(eventPacket) < 2 {
		return nil, false
	}

	var eventName string
	if err := json.Unmarshal(eventPacket[0], &eventName); err != nil {
		return nil, false
	}
	if eventName != "message" {
		return nil, false
	}

	var messagePayload string
	if err := json.Unmarshal(eventPacket[1], &messagePayload); err != nil {
		return nil, false
	}
	envelope, err := socket.ParseEnvelopeString(messagePayload)
	if err != nil {
		return nil, false
	}
	return envelope, true
}

func assertEnvelopeMethodCode(t *testing.T, item *socket.SocketData, method string, code int) {
	t.Helper()
	if item == nil {
		t.Fatalf("envelope is nil")
	}
	if item.M != method {
		t.Fatalf("expected method %s, got %s", method, item.M)
	}
	actualCode, ok := socket.CodeAsInt(item.C)
	if !ok || actualCode != code {
		t.Fatalf("expected code %d, got %v", code, item.C)
	}
}

func assertMethodCodeEqual(t *testing.T, label string, left, right *socket.SocketData) {
	t.Helper()
	if left == nil || right == nil {
		t.Fatalf("%s has nil envelope: left=%v right=%v", label, left, right)
	}
	if left.M != right.M {
		t.Fatalf("%s method mismatch: left=%s right=%s", label, left.M, right.M)
	}
	leftCode, lok := socket.CodeAsInt(left.C)
	rightCode, rok := socket.CodeAsInt(right.C)
	if !lok || !rok || leftCode != rightCode {
		t.Fatalf("%s code mismatch: left=%v right=%v", label, left.C, right.C)
	}
}

func mustEnvelopeString(t *testing.T, item *socket.SocketData) string {
	t.Helper()
	if item == nil {
		return "<nil>"
	}
	raw, err := item.ToString()
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	return raw
}
