package contract_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	groupchatdb "github.com/metaid-developers/meta-socket/internal/groupchat/db"
	groupchatpush "github.com/metaid-developers/meta-socket/internal/groupchat/push"
	"github.com/metaid-developers/meta-socket/internal/socket"
)

// Replay baseline: fixtures are treated as legacy-service outputs.
// For identical synthetic inputs, meta-socket output must stay byte-shape compatible.
func TestReplayCompareWithLegacyFixtures(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		build   func() *socket.SocketData
	}{
		{
			name:    "group event",
			fixture: "group_chat_event.json",
			build: func() *socket.SocketData {
				return groupchatpush.BuildGroupChatEnvelope(newGroupChatInput(false))
			},
		},
		{
			name:    "private event",
			fixture: "private_chat_event.json",
			build: func() *socket.SocketData {
				return groupchatpush.BuildPrivateChatEnvelope(newPrivateChatInput())
			},
		},
		{
			name:    "group role event",
			fixture: "group_role_event.json",
			build: func() *socket.SocketData {
				return groupchatpush.BuildGroupRoleEnvelope(newGroupRoleInput())
			},
		},
		{
			name:    "response success wrapped group",
			fixture: "response_success_wrapped_group.json",
			build: func() *socket.SocketData {
				return groupchatpush.BuildGroupChatEnvelope(newGroupChatInput(true))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			expected := mustLoadFixtureMap(t, tc.fixture)
			actualEnvelope := tc.build()
			if actualEnvelope == nil {
				t.Fatalf("builder returned nil envelope")
			}
			actual := mustEnvelopeToMap(t, actualEnvelope)
			assertSubsetMatch(t, expected, actual, "$")
		})
	}
}

func TestWrappedSuccessDataContract(t *testing.T) {
	envelope := groupchatpush.BuildGroupChatEnvelope(newGroupChatInput(true))
	if envelope == nil {
		t.Fatalf("expected envelope")
	}
	if envelope.M != socket.WS_RESPONSE_SUCCESS {
		t.Fatalf("expected method %s, got %s", socket.WS_RESPONSE_SUCCESS, envelope.M)
	}
	if code, ok := socket.CodeAsInt(envelope.C); !ok || code != socket.WS_CODE_SEND_SUCCESS {
		t.Fatalf("expected success code %d, got %v", socket.WS_CODE_SEND_SUCCESS, envelope.C)
	}

	wrapped, ok := socket.ExtractSuccessWrappedData(envelope)
	if !ok {
		t.Fatalf("expected wrapped data payload")
	}
	switch payload := wrapped.(type) {
	case map[string]interface{}:
		if _, exists := payload["groupId"]; !exists {
			t.Fatalf("wrapped data missing groupId")
		}
	case *groupchatpush.GroupChatItem:
		if payload.GroupID == "" {
			t.Fatalf("wrapped data missing groupId")
		}
	default:
		t.Fatalf("unexpected wrapped payload type %T", wrapped)
	}
}

func newGroupChatInput(wrapAsSuccess bool) *groupchatdb.TalkGroupChatV3 {
	content := "hello group"
	txID := "tx_group_001"
	pinID := "pin_group_001"
	globalMetaID := "id_sender_001"
	metaID := "meta_sender_001"
	address := "1ExampleSenderAddress"
	timestamp := int64(1760000000)
	blockHeight := int64(100)
	index := int64(1)
	if wrapAsSuccess {
		content = "wrapped group payload"
		txID = "tx_group_002"
		pinID = "pin_group_002"
		globalMetaID = "id_sender_002"
		metaID = "meta_sender_002"
		address = "1ExampleSenderAddress2"
		timestamp = int64(1760000002)
		blockHeight = 0
		index = 0
	}

	return &groupchatdb.TalkGroupChatV3{
		GroupID:      "group_01",
		ChannelID:    "",
		TxID:         txID,
		PinID:        pinID,
		GlobalMetaID: globalMetaID,
		MetaID:       metaID,
		Address:      address,
		UserInfo: &groupchatdb.UserInfo{
			Name:          "sender",
			Avatar:        "/avatar/sender.png",
			ChatPublicKey: "02abcdef",
		},
		NickName:          "sender",
		Protocol:          "/protocols/simplegroupchat",
		Content:           content,
		ContentType:       "text/plain",
		Encryption:        "none",
		ChatType:          0,
		ReplyPin:          "",
		ReplyInfo:         nil,
		ReplyMetaID:       "",
		ReplyGlobalMetaID: "",
		Mention:           []string{},
		Timestamp:         timestamp,
		Chain:             "mvc",
		BlockHeight:       blockHeight,
		Index:             index,
		WrapAsSuccess:     wrapAsSuccess,
	}
}

func newPrivateChatInput() *groupchatdb.TalkPrivateChatV3 {
	return &groupchatdb.TalkPrivateChatV3{
		FromGlobalMetaID: "id_from_001",
		From:             "meta_from_001",
		FromUserInfo: &groupchatdb.UserInfo{
			Name:          "alice",
			Avatar:        "/avatar/alice.png",
			ChatPublicKey: "03aaaa",
		},
		ToGlobalMetaID: "id_to_001",
		To:             "meta_to_001",
		ToUserInfo: &groupchatdb.UserInfo{
			Name:          "bob",
			Avatar:        "/avatar/bob.png",
			ChatPublicKey: "03bbbb",
		},
		TxID:         "tx_private_001",
		PinID:        "pin_private_001",
		GlobalMetaID: "meta_from_001",
		MetaID:       "meta_from_001",
		Address:      "1ExampleFromAddress",
		UserInfo: &groupchatdb.UserInfo{
			Name:          "alice",
			Avatar:        "/avatar/alice.png",
			ChatPublicKey: "03aaaa",
		},
		NickName:          "",
		Protocol:          "/protocols/simplemsg",
		Content:           "hello private",
		ContentType:       "text/plain",
		Encryption:        "none",
		ChatType:          0,
		ReplyPin:          "",
		ReplyInfo:         nil,
		ReplyMetaID:       "",
		ReplyGlobalMetaID: "",
		Timestamp:         1760000001,
		Chain:             "mvc",
		BlockHeight:       101,
		Index:             2,
	}
}

func newGroupRoleInput() *groupchatdb.GroupUserRoleInfo {
	return &groupchatdb.GroupUserRoleInfo{
		GlobalMetaID: "id_user_001",
		MetaID:       "meta_user_001",
		Address:      "1ExampleUserAddress",
		UserInfo: &groupchatdb.UserInfo{
			Name:          "member",
			Avatar:        "/avatar/member.png",
			ChatPublicKey: "02cccc",
		},
		GroupID:     "group_01",
		ChannelID:   "",
		IsCreator:   false,
		IsAdmin:     true,
		IsBlocked:   false,
		IsWhitelist: true,
		IsRemoved:   false,
	}
}

func mustLoadFixtureMap(t *testing.T, name string) map[string]interface{} {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "fixtures", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	var data map[string]interface{}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("unmarshal fixture %s: %v", name, err)
	}
	return data
}

func mustEnvelopeToMap(t *testing.T, item *socket.SocketData) map[string]interface{} {
	t.Helper()
	raw, err := item.ToString()
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	return data
}

func assertSubsetMatch(t *testing.T, expected, actual interface{}, path string) {
	t.Helper()

	switch exp := expected.(type) {
	case map[string]interface{}:
		act, ok := actual.(map[string]interface{})
		if !ok {
			t.Fatalf("%s expected object, got %T", path, actual)
		}
		for key, expValue := range exp {
			actValue, exists := act[key]
			if !exists {
				t.Fatalf("%s missing key %q", path, key)
			}
			assertSubsetMatch(t, expValue, actValue, path+"."+key)
		}
	case []interface{}:
		act, ok := actual.([]interface{})
		if !ok {
			t.Fatalf("%s expected array, got %T", path, actual)
		}
		if len(exp) != len(act) {
			t.Fatalf("%s expected array length %d, got %d", path, len(exp), len(act))
		}
		for i := range exp {
			assertSubsetMatch(t, exp[i], act[i], path)
		}
	default:
		if !reflect.DeepEqual(exp, actual) {
			t.Fatalf("%s expected %v (%T), got %v (%T)", path, exp, exp, actual, actual)
		}
	}
}
