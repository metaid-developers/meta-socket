package push

import (
	"github.com/metaid-developers/meta-socket/internal/groupchat/db"
	"github.com/metaid-developers/meta-socket/internal/socket"
)

type GroupChatItem struct {
	GroupID           string      `json:"groupId"`
	GlobalMetaID      string      `json:"globalMetaId"`
	ChannelID         string      `json:"channelId"`
	MetanetID         string      `json:"metanetId"`
	TxID              string      `json:"txId"`
	PinID             string      `json:"pinId"`
	MetaID            string      `json:"metaId"`
	Address           string      `json:"address"`
	UserInfo          *UserInfo   `json:"userInfo"`
	NickName          string      `json:"nickName"`
	Protocol          string      `json:"protocol"`
	Domain            string      `json:"domain"`
	Content           string      `json:"content"`
	ContentType       string      `json:"contentType"`
	Encryption        string      `json:"encryption"`
	ChatType          int64       `json:"chatType"`
	Data              interface{} `json:"data"`
	ReplyPin          string      `json:"replyPin"`
	ReplyInfo         *ReplyInfo  `json:"replyInfo"`
	ReplyMetaID       string      `json:"replyMetaId"`
	ReplyGlobalMetaID string      `json:"replyGlobalMetaId"`
	Mention           []string    `json:"mention"`
	Timestamp         int64       `json:"timestamp"`
	Params            string      `json:"params"`
	Chain             string      `json:"chain"`
	BlockHeight       int64       `json:"blockHeight"`
	Index             int64       `json:"index"`
}

type PrivateChatItem struct {
	FromGlobalMetaID  string      `json:"fromGlobalMetaId"`
	From              string      `json:"from"`
	FromUserInfo      *UserInfo   `json:"fromUserInfo"`
	ToGlobalMetaID    string      `json:"toGlobalMetaId"`
	To                string      `json:"to"`
	ToUserInfo        *UserInfo   `json:"toUserInfo"`
	TxID              string      `json:"txId"`
	PinID             string      `json:"pinId"`
	GlobalMetaID      string      `json:"globalMetaId"`
	MetaID            string      `json:"metaId"`
	Address           string      `json:"address"`
	UserInfo          *UserInfo   `json:"userInfo"`
	NickName          string      `json:"nickName"`
	Protocol          string      `json:"protocol"`
	Content           string      `json:"content"`
	ContentType       string      `json:"contentType"`
	Encryption        string      `json:"encryption"`
	ChatType          int64       `json:"chatType"`
	Data              interface{} `json:"data"`
	ReplyPin          string      `json:"replyPin"`
	ReplyInfo         *ReplyInfo  `json:"replyInfo"`
	ReplyGlobalMetaID string      `json:"replyGlobalMetaId"`
	ReplyMetaID       string      `json:"replyMetaId"`
	Timestamp         int64       `json:"timestamp"`
	Params            string      `json:"params"`
	Chain             string      `json:"chain"`
	BlockHeight       int64       `json:"blockHeight"`
	Index             int64       `json:"index"`
}

type GroupUserRoleInfo struct {
	GlobalMetaID string    `json:"globalMetaId"`
	MetaID       string    `json:"metaId"`
	Address      string    `json:"address"`
	UserInfo     *UserInfo `json:"userInfo"`
	GroupID      string    `json:"groupId"`
	ChannelID    string    `json:"channelId"`
	IsCreator    bool      `json:"isCreator"`
	IsAdmin      bool      `json:"isAdmin"`
	IsBlocked    bool      `json:"isBlocked"`
	IsWhitelist  bool      `json:"isWhitelist"`
	IsRemoved    bool      `json:"isRemoved,omitempty"`
}

type ReplyInfo struct {
	ChannelID    string    `json:"channelId"`
	PinID        string    `json:"pinId"`
	GlobalMetaID string    `json:"globalMetaId"`
	MetaID       string    `json:"metaId"`
	Address      string    `json:"address"`
	UserInfo     *UserInfo `json:"userInfo"`
	NickName     string    `json:"nickName"`
	Protocol     string    `json:"protocol"`
	Content      string    `json:"content"`
	ContentType  string    `json:"contentType"`
	Encryption   string    `json:"encryption"`
	ChatType     int64     `json:"chatType"`
	Mention      []string  `json:"mention"`
	Timestamp    int64     `json:"timestamp"`
	Chain        string    `json:"chain"`
	BlockHeight  int64     `json:"blockHeight"`
	Index        int64     `json:"index"`
}

type UserInfo struct {
	GlobalMetaID    string `json:"globalMetaId,omitempty"`
	Metaid          string `json:"metaid,omitempty"`
	Address         string `json:"address,omitempty"`
	Name            string `json:"name,omitempty"`
	Avatar          string `json:"avatar,omitempty"`
	AvatarImage     string `json:"avatarImage,omitempty"`
	ChatPublicKey   string `json:"chatPublicKey,omitempty"`
	ChatPublicKeyID string `json:"chatPublicKeyId,omitempty"`
}

func RegisterDBHooks() {
	db.SetHandleGroupChatItem(wsForGroupChatItem)
	db.SetHandlePrivateChatItem(wsForPrivateChatItem)
	db.SetHandleGroupRoleInfoChangeList(wsForGroupRoleInfoChange)
	RegisterRoomJoiner()
}

func wsForGroupChatItem(chat *db.TalkGroupChatV3) error {
	wsPostGroupMsg(chat)
	return nil
}

func wsForPrivateChatItem(chat *db.TalkPrivateChatV3) error {
	wsPostPrivateMsg(chat)
	return nil
}

func wsForGroupRoleInfoChange(roleInfo *db.GroupUserRoleInfo) error {
	wsPostGroupRoleInfo(roleInfo)
	return nil
}

func wsPostGroupMsg(chat *db.TalkGroupChatV3) {
	if chat == nil {
		return
	}

	item := &GroupChatItem{
		GroupID:           chat.GroupID,
		GlobalMetaID:      chat.GlobalMetaID,
		ChannelID:         chat.ChannelID,
		MetanetID:         chat.GroupID,
		TxID:              chat.TxID,
		PinID:             chat.PinID,
		MetaID:            chat.MetaID,
		Address:           chat.Address,
		UserInfo:          toUserInfo(chat.UserInfo),
		NickName:          chat.NickName,
		Protocol:          chat.Protocol,
		Content:           chat.Content,
		ContentType:       chat.ContentType,
		Encryption:        chat.Encryption,
		ChatType:          chat.ChatType,
		ReplyPin:          chat.ReplyPin,
		ReplyInfo:         toReplyInfo(chat.ReplyInfo),
		ReplyMetaID:       chat.ReplyMetaID,
		ReplyGlobalMetaID: chat.ReplyGlobalMetaID,
		Mention:           chat.Mention,
		Timestamp:         chat.Timestamp,
		Chain:             chat.Chain,
		BlockHeight:       chat.BlockHeight,
		Index:             chat.Index,
	}

	method := socket.WS_SERVER_NOTIFY_GROUP_CHAT
	code := socket.WS_CODE_SERVER
	payload := interface{}(item)
	if chat.WrapAsSuccess {
		method = socket.WS_RESPONSE_SUCCESS
		code = socket.WS_CODE_SEND_SUCCESS
		payload = map[string]interface{}{"data": item}
	}

	sentByRoom := false
	if RoomBroadcastEnabled() && chat.GroupID != "" && HasKnownMembers(chat.GroupID) {
		if err := SendGroupMessageToRoom(chat.GroupID, method, code, payload); err == nil {
			sentByRoom = true
		}
	}

	if !sentByRoom {
		targetMetaIDs := mergeUnique(chat.RecipientMetaIDs, []string{chat.MetaID})
		targetGlobalMetaIDs := mergeUnique(chat.RecipientGlobalMetaIDs, []string{chat.GlobalMetaID})
		_ = SendMessageToTargets(targetMetaIDs, targetGlobalMetaIDs, method, code, payload)
	}

	_ = SendAllMessageToExtraPush(
		item,
		chat.RecipientMetaIDs,
		chat.Mention,
		chat.RecipientGlobalMetaIDs,
		nil,
		socket.WS_SERVER_NOTIFY_GROUP_CHAT,
	)
}

func wsPostPrivateMsg(chat *db.TalkPrivateChatV3) {
	if chat == nil {
		return
	}
	if chat.From == chat.To {
		return
	}

	item := &PrivateChatItem{
		FromGlobalMetaID:  chat.FromGlobalMetaID,
		From:              chat.From,
		FromUserInfo:      toUserInfo(chat.FromUserInfo),
		ToGlobalMetaID:    chat.ToGlobalMetaID,
		To:                chat.To,
		ToUserInfo:        toUserInfo(chat.ToUserInfo),
		TxID:              chat.TxID,
		PinID:             chat.PinID,
		GlobalMetaID:      chat.GlobalMetaID,
		MetaID:            chat.MetaID,
		Address:           chat.Address,
		UserInfo:          toUserInfo(chat.UserInfo),
		NickName:          chat.NickName,
		Protocol:          chat.Protocol,
		Content:           chat.Content,
		ContentType:       chat.ContentType,
		Encryption:        chat.Encryption,
		ChatType:          chat.ChatType,
		ReplyPin:          chat.ReplyPin,
		ReplyInfo:         toReplyInfo(chat.ReplyInfo),
		ReplyGlobalMetaID: chat.ReplyGlobalMetaID,
		ReplyMetaID:       chat.ReplyMetaID,
		Timestamp:         chat.Timestamp,
		Chain:             chat.Chain,
		BlockHeight:       chat.BlockHeight,
		Index:             chat.Index,
	}

	targetMetaIDs := mergeUnique(chat.RecipientMetaIDs, []string{chat.From, chat.To})
	targetGlobalMetaIDs := mergeUnique(chat.RecipientGlobalMetaIDs, []string{chat.FromGlobalMetaID, chat.ToGlobalMetaID})
	_ = SendMessageToTargets(targetMetaIDs, targetGlobalMetaIDs, socket.WS_SERVER_NOTIFY_PRIVATE_CHAT, socket.WS_CODE_SERVER, item)
	_ = SendAllMessageToExtraPush(item, targetMetaIDs, nil, targetGlobalMetaIDs, nil, socket.WS_SERVER_NOTIFY_PRIVATE_CHAT)
}

func wsPostGroupRoleInfo(roleInfo *db.GroupUserRoleInfo) {
	if roleInfo == nil {
		return
	}
	item := &GroupUserRoleInfo{
		GlobalMetaID: roleInfo.GlobalMetaID,
		MetaID:       roleInfo.MetaID,
		Address:      roleInfo.Address,
		UserInfo:     toUserInfo(roleInfo.UserInfo),
		GroupID:      roleInfo.GroupID,
		ChannelID:    roleInfo.ChannelID,
		IsCreator:    roleInfo.IsCreator,
		IsAdmin:      roleInfo.IsAdmin,
		IsBlocked:    roleInfo.IsBlocked,
		IsWhitelist:  roleInfo.IsWhitelist,
		IsRemoved:    roleInfo.IsRemoved,
	}

	TrackGroupMembership(roleInfo.MetaID, roleInfo.GlobalMetaID, roleInfo.GroupID, roleInfo.IsRemoved)
	_ = SendGroupRoleInfoToUser(roleInfo.MetaID, roleInfo.GlobalMetaID, item)
	_ = SendAllMessageToExtraPush(
		item,
		[]string{roleInfo.MetaID},
		nil,
		[]string{roleInfo.GlobalMetaID},
		nil,
		socket.WS_SERVER_NOTIFY_GROUP_ROLE,
	)
}

func toUserInfo(info *db.UserInfo) *UserInfo {
	if info == nil {
		return nil
	}
	return &UserInfo{
		GlobalMetaID:    info.GlobalMetaID,
		Metaid:          info.MetaID,
		Address:         info.Address,
		Name:            info.Name,
		Avatar:          info.Avatar,
		AvatarImage:     info.AvatarImage,
		ChatPublicKey:   info.ChatPublicKey,
		ChatPublicKeyID: info.ChatPublicKeyID,
	}
}

func toReplyInfo(reply *db.ReplyInfo) *ReplyInfo {
	if reply == nil {
		return nil
	}
	return &ReplyInfo{
		ChannelID:    reply.ChannelID,
		PinID:        reply.PinID,
		GlobalMetaID: reply.GlobalMetaID,
		MetaID:       reply.MetaID,
		Address:      reply.Address,
		UserInfo:     toUserInfo(reply.UserInfo),
		NickName:     reply.NickName,
		Protocol:     reply.Protocol,
		Content:      reply.Content,
		ContentType:  reply.ContentType,
		Encryption:   reply.Encryption,
		ChatType:     reply.ChatType,
		Mention:      reply.Mention,
		Timestamp:    reply.Timestamp,
		Chain:        reply.Chain,
		BlockHeight:  reply.BlockHeight,
		Index:        reply.Index,
	}
}
