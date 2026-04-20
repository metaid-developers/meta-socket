package contract_test

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/metaid-developers/meta-socket/internal/socket"
)

func TestResolveHandshakeIdentity(t *testing.T) {
	t.Run("auth preferred", func(t *testing.T) {
		metaID, deviceType := socket.ResolveHandshakeIdentity(
			map[string]interface{}{"metaid": "meta_auth", "type": "app"},
			map[string][]string{"metaid": {"meta_query"}, "type": {"pc"}},
		)
		if metaID != "meta_auth" {
			t.Fatalf("expected auth metaid, got %s", metaID)
		}
		if deviceType != socket.DeviceTypeAPP {
			t.Fatalf("expected app device type, got %s", deviceType)
		}
	})

	t.Run("query fallback", func(t *testing.T) {
		metaID, deviceType := socket.ResolveHandshakeIdentity(
			nil,
			map[string][]string{"metaid": {"meta_query"}, "type": {"app"}},
		)
		if metaID != "meta_query" {
			t.Fatalf("expected query metaid, got %s", metaID)
		}
		if deviceType != socket.DeviceTypeAPP {
			t.Fatalf("expected app device type, got %s", deviceType)
		}
	})
}

func TestSocketFixturesEnvelopeCompatibility(t *testing.T) {
	cases := []struct {
		name          string
		fixture       string
		expectMethod  string
		requiredDKeys []string
	}{
		{
			name:          "connection ack",
			fixture:       "connection_success.json",
			expectMethod:  socket.WS_RESPONSE_SUCCESS,
			requiredDKeys: nil,
		},
		{
			name:          "group chat event",
			fixture:       "group_chat_event.json",
			expectMethod:  socket.WS_SERVER_NOTIFY_GROUP_CHAT,
			requiredDKeys: []string{"groupId", "pinId", "protocol", "content"},
		},
		{
			name:          "private chat event",
			fixture:       "private_chat_event.json",
			expectMethod:  socket.WS_SERVER_NOTIFY_PRIVATE_CHAT,
			requiredDKeys: []string{"fromGlobalMetaId", "toGlobalMetaId", "pinId", "protocol"},
		},
		{
			name:          "group role event",
			fixture:       "group_role_event.json",
			expectMethod:  socket.WS_SERVER_NOTIFY_GROUP_ROLE,
			requiredDKeys: []string{"groupId", "isAdmin", "isBlocked"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			envelope := mustLoadFixtureEnvelope(t, tc.fixture)
			if envelope.M != tc.expectMethod {
				t.Fatalf("expected method %s, got %s", tc.expectMethod, envelope.M)
			}

			code, ok := socket.CodeAsInt(envelope.C)
			if !ok {
				t.Fatalf("expected numeric C code, got %T", envelope.C)
			}
			if code == 0 && tc.expectMethod != socket.WS_SERVER_NOTIFY_GROUP_CHAT && tc.expectMethod != socket.WS_SERVER_NOTIFY_PRIVATE_CHAT && tc.expectMethod != socket.WS_SERVER_NOTIFY_GROUP_ROLE {
				t.Fatalf("unexpected zero code for method %s", tc.expectMethod)
			}

			if len(tc.requiredDKeys) == 0 {
				return
			}

			dMap, ok := envelope.D.(map[string]interface{})
			if !ok {
				t.Fatalf("expected D as object, got %T", envelope.D)
			}
			for _, key := range tc.requiredDKeys {
				if _, exists := dMap[key]; !exists {
					t.Fatalf("missing field %q in D", key)
				}
			}
		})
	}
}

func TestHeartbeatAndPingCompatibility(t *testing.T) {
	pong := mustLoadFixtureEnvelope(t, "pong_message.json")
	if pong.M != "pong" {
		t.Fatalf("expected pong fixture method, got %s", pong.M)
	}
	pongCode, ok := socket.CodeAsInt(pong.C)
	if !ok || pongCode != socket.WS_CODE_SEND_SUCCESS {
		t.Fatalf("expected pong code %d, got %v", socket.WS_CODE_SEND_SUCCESS, pong.C)
	}

	heartbeatAck := socket.BuildHeartbeatReplyEnvelope()
	if heartbeatAck.M != socket.HEART_BEAT {
		t.Fatalf("expected HEART_BEAT ack, got %s", heartbeatAck.M)
	}
	ackCode, ok := socket.CodeAsInt(heartbeatAck.C)
	if !ok || ackCode != socket.WS_CODE_HEART_BEAT_BACK {
		t.Fatalf("expected heartbeat code %d, got %v", socket.WS_CODE_HEART_BEAT_BACK, heartbeatAck.C)
	}

	pongAck := socket.BuildPongReplyEnvelope()
	if pongAck.M != "pong" {
		t.Fatalf("expected pong ack, got %s", pongAck.M)
	}
	pongAckCode, ok := socket.CodeAsInt(pongAck.C)
	if !ok || pongAckCode != socket.WS_CODE_SEND_SUCCESS {
		t.Fatalf("expected pong ack code %d, got %v", socket.WS_CODE_SEND_SUCCESS, pongAck.C)
	}
}

func mustLoadFixtureEnvelope(t *testing.T, name string) *socket.SocketData {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "fixtures", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	envelope, err := socket.ParseEnvelopeString(string(raw))
	if err != nil {
		t.Fatalf("parse fixture %s: %v", name, err)
	}
	if envelope.M == "" {
		t.Fatalf("fixture %s has empty M", name)
	}
	if _, ok := socket.CodeAsInt(envelope.C); !ok {
		t.Fatalf("fixture %s has non-numeric C: %T", name, envelope.C)
	}
	if envelope.D == nil && name != "pong_message.json" && name != "connection_success.json" {
		t.Fatalf("fixture %s has empty D", name)
	}
	return envelope
}

func TestResponseSuccessWrappedFixture(t *testing.T) {
	envelope := mustLoadFixtureEnvelope(t, "response_success_wrapped_group.json")
	if envelope.M != socket.WS_RESPONSE_SUCCESS {
		t.Fatalf("expected method %s, got %s", socket.WS_RESPONSE_SUCCESS, envelope.M)
	}

	wrapped, ok := socket.ExtractSuccessWrappedData(envelope)
	if !ok {
		t.Fatalf("expected wrapped D.data payload")
	}
	dataMap, ok := wrapped.(map[string]interface{})
	if !ok {
		t.Fatalf("expected wrapped payload object, got %T", wrapped)
	}
	for _, key := range []string{"groupId", "pinId", "protocol", "content"} {
		if _, exists := dataMap[key]; !exists {
			t.Fatalf("wrapped payload missing field %s", key)
		}
	}

	normalized, err := socket.ParseEnvelopeString(strconv.Quote(mustToJSON(t, envelope)))
	if err != nil {
		t.Fatalf("parse quoted envelope: %v", err)
	}
	if normalized.M != envelope.M {
		t.Fatalf("quoted parse changed method: %s vs %s", normalized.M, envelope.M)
	}
}

func mustToJSON(t *testing.T, data interface{}) string {
	t.Helper()
	item, ok := data.(*socket.SocketData)
	if !ok {
		t.Fatalf("expected *socket.SocketData, got %T", data)
	}
	raw, err := item.ToString()
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	return raw
}
