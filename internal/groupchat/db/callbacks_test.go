package db

import "testing"

func TestCallbacks(t *testing.T) {
	groupCalled := false
	privateCalled := false
	roleCalled := false

	SetHandleGroupChatItem(func(chat *TalkGroupChatV3) error {
		groupCalled = chat != nil
		return nil
	})
	SetHandlePrivateChatItem(func(chat *TalkPrivateChatV3) error {
		privateCalled = chat != nil
		return nil
	})
	SetHandleGroupRoleInfoChangeList(func(roleInfo *GroupUserRoleInfo) error {
		roleCalled = roleInfo != nil
		return nil
	})

	_ = dealGroupChatItem(&TalkGroupChatV3{})
	_ = dealPrivateChatItem(&TalkPrivateChatV3{})
	_ = dealGroupRoleInfoChangeList(&GroupUserRoleInfo{})

	if !groupCalled || !privateCalled || !roleCalled {
		t.Fatalf("callbacks not triggered: group=%t private=%t role=%t", groupCalled, privateCalled, roleCalled)
	}
}
