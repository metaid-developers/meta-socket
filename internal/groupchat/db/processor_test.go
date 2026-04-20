package db

import (
	"encoding/json"
	"testing"

	"github.com/metaid-developers/meta-socket/internal/adapter"
)

func TestProcessGroupPin(t *testing.T) {
	p := NewProcessor()

	var got *TalkGroupChatV3
	SetHandleGroupChatItem(func(chat *TalkGroupChatV3) error {
		got = chat
		return nil
	})

	body := map[string]interface{}{
		"groupId":     "group-1",
		"channelId":   "channel-1",
		"content":     "hello",
		"contentType": "text/plain",
		"timestamp":   int64(1760000000),
	}
	raw, _ := json.Marshal(body)
	pin := &adapter.PinRecord{
		ID:            "pin-1",
		Path:          "/protocols/simplegroupchat",
		MetaID:        "meta_sender",
		GlobalMetaID:  "id_sender",
		CreateAddress: "1abc",
		ChainName:     "mvc",
		ContentBody:   raw,
	}

	if err := p.ProcessGroupPin(pin, nil); err != nil {
		t.Fatalf("process group pin failed: %v", err)
	}
	if got == nil {
		t.Fatalf("expected group callback")
	}
	if got.GroupID != "group-1" || got.MetaID != "meta_sender" || got.GlobalMetaID != "id_sender" {
		t.Fatalf("unexpected parsed group item: %+v", got)
	}
}

func TestProcessPrivatePin(t *testing.T) {
	p := NewProcessor()

	var got *TalkPrivateChatV3
	SetHandlePrivateChatItem(func(chat *TalkPrivateChatV3) error {
		got = chat
		return nil
	})

	body := map[string]interface{}{
		"from":        "meta_a",
		"to":          "meta_b",
		"content":     "hi",
		"contentType": "text/plain",
		"timestamp":   int64(1760000001),
	}
	raw, _ := json.Marshal(body)
	pin := &adapter.PinRecord{
		ID:            "pin-p-1",
		Path:          "/protocols/simplemsg",
		GlobalMetaID:  "id_a",
		CreateAddress: "1from",
		ChainName:     "mvc",
		ContentBody:   raw,
	}

	if err := p.ProcessPrivatePin(pin, nil); err != nil {
		t.Fatalf("process private pin failed: %v", err)
	}
	if got == nil {
		t.Fatalf("expected private callback")
	}
	if got.From != "meta_a" || got.To != "meta_b" {
		t.Fatalf("unexpected private item: %+v", got)
	}
}

func TestProcessGroupRolePin(t *testing.T) {
	p := NewProcessor()

	var got []*GroupUserRoleInfo
	SetHandleGroupRoleInfoChangeList(func(roleInfo *GroupUserRoleInfo) error {
		got = append(got, roleInfo)
		return nil
	})

	body := map[string]interface{}{
		"groupId": "group-r-1",
		"users":   []string{"meta_u1", "meta_u2"},
	}
	raw, _ := json.Marshal(body)
	pin := &adapter.PinRecord{
		ID:            "pin-r-1",
		Path:          "/protocols/simplegroupadmin",
		GlobalMetaID:  "id_admin",
		CreateAddress: "1admin",
		ChainName:     "mvc",
		ContentBody:   raw,
	}

	if err := p.ProcessGroupRolePin(pin, nil); err != nil {
		t.Fatalf("process role pin failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 role callbacks, got %d", len(got))
	}
	if !got[0].IsAdmin {
		t.Fatalf("expected role isAdmin=true: %+v", got[0])
	}
}
