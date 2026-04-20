package db

var (
	handleGroupChatItem           func(chat *TalkGroupChatV3) error
	handlePrivateChatItem         func(chat *TalkPrivateChatV3) error
	handleGroupRoleInfoChangeList func(roleInfo *GroupUserRoleInfo) error
)

func SetHandleGroupChatItem(handle func(chat *TalkGroupChatV3) error) {
	handleGroupChatItem = handle
}

func dealGroupChatItem(chat *TalkGroupChatV3) error {
	if handleGroupChatItem != nil {
		return handleGroupChatItem(chat)
	}
	return nil
}

func SetHandlePrivateChatItem(handle func(chat *TalkPrivateChatV3) error) {
	handlePrivateChatItem = handle
}

func dealPrivateChatItem(chat *TalkPrivateChatV3) error {
	if handlePrivateChatItem != nil {
		return handlePrivateChatItem(chat)
	}
	return nil
}

func SetHandleGroupRoleInfoChangeList(handle func(roleInfo *GroupUserRoleInfo) error) {
	handleGroupRoleInfoChangeList = handle
}

func dealGroupRoleInfoChangeList(roleInfo *GroupUserRoleInfo) error {
	if handleGroupRoleInfoChangeList != nil {
		return handleGroupRoleInfoChangeList(roleInfo)
	}
	return nil
}
