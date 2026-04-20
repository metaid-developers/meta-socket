package db

type UserInfo struct {
	GlobalMetaID    string `json:"globalMetaId,omitempty"`
	MetaID          string `json:"metaid,omitempty"`
	Address         string `json:"address,omitempty"`
	Name            string `json:"name,omitempty"`
	Avatar          string `json:"avatar,omitempty"`
	AvatarImage     string `json:"avatarImage,omitempty"`
	ChatPublicKey   string `json:"chatPublicKey,omitempty"`
	ChatPublicKeyID string `json:"chatPublicKeyId,omitempty"`
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

type TalkGroupChatV3 struct {
	CommunityID            string     `json:"communityId"`
	GroupID                string     `json:"groupId"`
	ChannelID              string     `json:"channelId"`
	TxID                   string     `json:"txId"`
	PinID                  string     `json:"pinId"`
	GlobalMetaID           string     `json:"globalMetaId"`
	MetaID                 string     `json:"metaId"`
	Address                string     `json:"address"`
	UserInfo               *UserInfo  `json:"userInfo,omitempty"`
	NickName               string     `json:"nickName"`
	Protocol               string     `json:"protocol"`
	Content                string     `json:"content"`
	ContentType            string     `json:"contentType"`
	Encryption             string     `json:"encryption"`
	ChatType               int64      `json:"chatType"`
	ReplyPin               string     `json:"replyPin"`
	ReplyInfo              *ReplyInfo `json:"replyInfo"`
	ReplyMetaID            string     `json:"replyMetaId"`
	ReplyGlobalMetaID      string     `json:"replyGlobalMetaId"`
	Mention                []string   `json:"mention"`
	Timestamp              int64      `json:"timestamp"`
	Chain                  string     `json:"chain"`
	BlockHeight            int64      `json:"blockHeight"`
	Index                  int64      `json:"index"`
	RecipientMetaIDs       []string   `json:"recipientMetaIds,omitempty"`
	RecipientGlobalMetaIDs []string   `json:"recipientGlobalMetaIds,omitempty"`
	WrapAsSuccess          bool       `json:"wrapAsSuccess,omitempty"`
}

type TalkPrivateChatV3 struct {
	FromGlobalMetaID       string     `json:"fromGlobalMetaId"`
	From                   string     `json:"from"`
	FromAddress            string     `json:"fromAddress"`
	FromUserInfo           *UserInfo  `json:"fromUserInfo,omitempty"`
	ToGlobalMetaID         string     `json:"toGlobalMetaId"`
	To                     string     `json:"to"`
	ToAddress              string     `json:"toAddress"`
	ToUserInfo             *UserInfo  `json:"toUserInfo,omitempty"`
	TxID                   string     `json:"txId"`
	PinID                  string     `json:"pinId"`
	GlobalMetaID           string     `json:"globalMetaId"`
	MetaID                 string     `json:"metaId"`
	Address                string     `json:"address"`
	UserInfo               *UserInfo  `json:"userInfo,omitempty"`
	NickName               string     `json:"nickName"`
	Protocol               string     `json:"protocol"`
	Content                string     `json:"content"`
	ContentType            string     `json:"contentType"`
	Encryption             string     `json:"encryption"`
	ChatType               int64      `json:"chatType"`
	ReplyPin               string     `json:"replyPin"`
	ReplyInfo              *ReplyInfo `json:"replyInfo"`
	ReplyMetaID            string     `json:"replyMetaId"`
	ReplyGlobalMetaID      string     `json:"replyGlobalMetaId"`
	Timestamp              int64      `json:"timestamp"`
	Chain                  string     `json:"chain"`
	BlockHeight            int64      `json:"blockHeight"`
	Index                  int64      `json:"index"`
	RecipientMetaIDs       []string   `json:"recipientMetaIds,omitempty"`
	RecipientGlobalMetaIDs []string   `json:"recipientGlobalMetaIds,omitempty"`
}

type GroupUserRoleInfo struct {
	MetaID       string    `json:"metaId"`
	GlobalMetaID string    `json:"globalMetaId"`
	Address      string    `json:"address"`
	UserInfo     *UserInfo `json:"userInfo,omitempty"`
	GroupID      string    `json:"groupId"`
	ChannelID    string    `json:"channelId"`
	IsCreator    bool      `json:"isCreator"`
	IsAdmin      bool      `json:"isAdmin"`
	IsBlocked    bool      `json:"isBlocked"`
	IsWhitelist  bool      `json:"isWhitelist"`
	IsRemoved    bool      `json:"isRemoved,omitempty"`
}
