package db

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/metaid-developers/meta-socket/internal/adapter"
)

type Processor struct{}

func NewProcessor() *Processor {
	return &Processor{}
}

func (p *Processor) ProcessGroupPin(pin *adapter.PinRecord, tx json.RawMessage) error {
	if pin == nil {
		return nil
	}
	content := parseContentBody(pin.ContentBody)
	groupID := pickString(content, "groupId", "groupID")
	channelID := pickString(content, "channelId", "channelID")
	metaID := firstNonEmpty(pin.MetaID, pin.CreateMetaID, pickString(content, "metaId"))
	globalMetaID := firstNonEmpty(pin.GlobalMetaID, pickString(content, "globalMetaId"))
	address := firstNonEmpty(pin.CreateAddress, pickString(content, "address"))
	now := time.Now().Unix()

	item := &TalkGroupChatV3{
		GroupID:                groupID,
		ChannelID:              channelID,
		TxID:                   firstNonEmpty(pickString(content, "txId"), pin.ID),
		PinID:                  firstNonEmpty(pin.ID, pickString(content, "pinId")),
		GlobalMetaID:           globalMetaID,
		MetaID:                 metaID,
		Address:                address,
		NickName:               pickString(content, "nickName"),
		Protocol:               firstNonEmpty(pin.Path, pickString(content, "protocol")),
		Content:                pickString(content, "content"),
		ContentType:            firstNonEmpty(pickString(content, "contentType"), "text/plain"),
		Encryption:             firstNonEmpty(pickString(content, "encryption"), "none"),
		ChatType:               pickInt64(content["chatType"], 0),
		ReplyPin:               pickString(content, "replyPin"),
		ReplyMetaID:            pickString(content, "replyMetaId"),
		ReplyGlobalMetaID:      pickString(content, "replyGlobalMetaId"),
		Mention:                pickStringSlice(content, "mention"),
		Timestamp:              pickInt64(content["timestamp"], now),
		Chain:                  firstNonEmpty(pin.ChainName, pickString(content, "chain")),
		BlockHeight:            pickInt64(content["blockHeight"], 0),
		Index:                  pickInt64(content["index"], 0),
		RecipientMetaIDs:       mergeUniqueStrings(pickStringSlice(content, "recipientMetaIds"), pickStringSlice(content, "repostMetaIds")),
		RecipientGlobalMetaIDs: mergeUniqueStrings(pickStringSlice(content, "recipientGlobalMetaIds"), pickStringSlice(content, "repostGlobalMetaIds")),
		WrapAsSuccess:          pickBool(content["wrapAsSuccess"], false),
	}

	if nestedData, ok := content["data"].(map[string]interface{}); ok {
		// Compatibility for wrapped payloads where actual chat fields are nested in D.data.
		if item.GroupID == "" {
			item.GroupID = pickString(nestedData, "groupId")
		}
		if item.Content == "" {
			item.Content = pickString(nestedData, "content")
		}
		if item.ContentType == "" {
			item.ContentType = pickString(nestedData, "contentType")
		}
		if item.Protocol == "" {
			item.Protocol = pickString(nestedData, "protocol")
		}
	}

	if item.GroupID == "" {
		// Group chat event without group id is not actionable for room routing.
		return nil
	}

	if item.MetaID != "" && item.GlobalMetaID == "" {
		item.GlobalMetaID = item.MetaID
	}
	return dealGroupChatItem(item)
}

func (p *Processor) ProcessPrivatePin(pin *adapter.PinRecord, tx json.RawMessage) error {
	if pin == nil {
		return nil
	}
	content := parseContentBody(pin.ContentBody)
	from := firstNonEmpty(pickString(content, "from"), pin.MetaID, pin.CreateMetaID)
	to := firstNonEmpty(pickString(content, "to"))
	if from == "" || to == "" || from == to {
		return nil
	}

	fromGlobal := firstNonEmpty(pickString(content, "fromGlobalMetaId"))
	toGlobal := firstNonEmpty(pickString(content, "toGlobalMetaId"))
	if fromGlobal == "" {
		if looksLikeGlobalMetaID(from) {
			fromGlobal = from
		} else {
			fromGlobal = pin.GlobalMetaID
		}
	}
	if toGlobal == "" && looksLikeGlobalMetaID(to) {
		toGlobal = to
	}
	if toGlobal == "" {
		toGlobal = to
	}

	now := time.Now().Unix()
	item := &TalkPrivateChatV3{
		FromGlobalMetaID:       fromGlobal,
		From:                   from,
		FromAddress:            firstNonEmpty(pickString(content, "fromAddress"), pin.CreateAddress),
		ToGlobalMetaID:         toGlobal,
		To:                     to,
		ToAddress:              pickString(content, "toAddress"),
		TxID:                   firstNonEmpty(pickString(content, "txId"), pin.ID),
		PinID:                  firstNonEmpty(pin.ID, pickString(content, "pinId")),
		GlobalMetaID:           fromGlobal,
		MetaID:                 from,
		Address:                firstNonEmpty(pickString(content, "address"), pin.CreateAddress),
		NickName:               pickString(content, "nickName"),
		Protocol:               firstNonEmpty(pin.Path, pickString(content, "protocol")),
		Content:                pickString(content, "content"),
		ContentType:            firstNonEmpty(pickString(content, "contentType"), "text/plain"),
		Encryption:             firstNonEmpty(pickString(content, "encryption"), "none"),
		ChatType:               pickInt64(content["chatType"], 0),
		ReplyPin:               pickString(content, "replyPin"),
		ReplyMetaID:            pickString(content, "replyMetaId"),
		ReplyGlobalMetaID:      pickString(content, "replyGlobalMetaId"),
		Timestamp:              pickInt64(content["timestamp"], now),
		Chain:                  firstNonEmpty(pin.ChainName, pickString(content, "chain")),
		BlockHeight:            pickInt64(content["blockHeight"], 0),
		Index:                  pickInt64(content["index"], 0),
		RecipientMetaIDs:       mergeUniqueStrings([]string{from, to}, pickStringSlice(content, "recipientMetaIds")),
		RecipientGlobalMetaIDs: mergeUniqueStrings([]string{fromGlobal, toGlobal}, pickStringSlice(content, "recipientGlobalMetaIds")),
	}
	return dealPrivateChatItem(item)
}

func (p *Processor) ProcessGroupRolePin(pin *adapter.PinRecord, tx json.RawMessage) error {
	if pin == nil {
		return nil
	}
	content := parseContentBody(pin.ContentBody)
	groupID := pickString(content, "groupId", "groupID")
	if groupID == "" {
		return nil
	}
	channelID := pickString(content, "channelId", "channelID")
	protocol := normalizeProtocol(pin.Path)
	defaultMetaID := firstNonEmpty(pin.MetaID, pin.CreateMetaID)
	defaultGlobal := pin.GlobalMetaID

	targets := extractRoleTargets(content, defaultMetaID)
	if len(targets) == 0 && defaultMetaID == "" && defaultGlobal != "" {
		targets = []string{defaultGlobal}
	}

	for _, target := range targets {
		metaID, globalMetaID := splitIdentity(target, defaultMetaID, defaultGlobal)
		role := &GroupUserRoleInfo{
			MetaID:       metaID,
			GlobalMetaID: globalMetaID,
			Address:      pin.CreateAddress,
			GroupID:      groupID,
			ChannelID:    channelID,
		}
		applyRoleFlags(protocol, content, role)
		if err := dealGroupRoleInfoChangeList(role); err != nil {
			return err
		}
	}
	return nil
}

func parseContentBody(raw json.RawMessage) map[string]interface{} {
	result := make(map[string]interface{})
	if len(raw) == 0 {
		return result
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return result
	}
	if trimmed[0] == '"' {
		var unescaped string
		if err := json.Unmarshal(trimmed, &unescaped); err == nil {
			trimmed = []byte(unescaped)
		}
	}
	_ = json.Unmarshal(trimmed, &result)
	return result
}

func pickString(m map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		value, ok := m[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				return strings.TrimSpace(typed)
			}
		case json.Number:
			return typed.String()
		case float64:
			if typed == float64(int64(typed)) {
				return strconv.FormatInt(int64(typed), 10)
			}
			return strings.TrimSpace(strconv.FormatFloat(typed, 'f', -1, 64))
		}
	}
	return ""
}

func pickInt64(value interface{}, fallback int64) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	case json.Number:
		v, err := typed.Int64()
		if err == nil {
			return v
		}
	case string:
		if typed == "" {
			return fallback
		}
		var n json.Number = json.Number(typed)
		if v, err := n.Int64(); err == nil {
			return v
		}
	}
	return fallback
}

func pickBool(value interface{}, fallback bool) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "1", "true", "yes":
			return true
		case "0", "false", "no":
			return false
		}
	case float64:
		return typed != 0
	case int64:
		return typed != 0
	}
	return fallback
}

func pickStringSlice(m map[string]interface{}, key string) []string {
	value, ok := m[key]
	if !ok || value == nil {
		return nil
	}
	switch typed := value.(type) {
	case []interface{}:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				result = append(result, strings.TrimSpace(s))
			}
		}
		return result
	case []string:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			if strings.TrimSpace(item) != "" {
				result = append(result, strings.TrimSpace(item))
			}
		}
		return result
	default:
		return nil
	}
}

func extractRoleTargets(content map[string]interface{}, fallbackMetaID string) []string {
	targets := make([]string, 0)
	appendUnique := func(values ...string) {
		for _, value := range values {
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				continue
			}
			exists := false
			for _, existing := range targets {
				if existing == trimmed {
					exists = true
					break
				}
			}
			if !exists {
				targets = append(targets, trimmed)
			}
		}
	}
	appendUnique(pickStringSlice(content, "users")...)
	appendUnique(pickStringSlice(content, "metaIds")...)
	appendUnique(pickStringSlice(content, "globalMetaIds")...)
	appendUnique(pickString(content, "metaId"), pickString(content, "globalMetaId"), pickString(content, "to"))
	if len(targets) == 0 {
		appendUnique(fallbackMetaID)
	}
	return targets
}

func applyRoleFlags(protocol string, content map[string]interface{}, role *GroupUserRoleInfo) {
	switch protocol {
	case "simplegroupcreate":
		role.IsCreator = true
	case "simplegroupadmin":
		role.IsAdmin = pickBool(content["state"], true)
	case "simplegroupblock", "simplegroupjoinblock":
		role.IsBlocked = pickBool(content["state"], true)
		if !role.IsBlocked {
			role.IsRemoved = true
		}
	case "simplegroupwhitelist", "simplegroupjoinwhitelist":
		role.IsWhitelist = pickBool(content["state"], true)
	case "simplegroupremoveuser":
		role.IsRemoved = true
	case "simplegroupjoin":
		state := pickInt64(content["state"], 1)
		if state < 0 {
			role.IsRemoved = true
		}
	}
}

func splitIdentity(value, fallbackMetaID, fallbackGlobal string) (string, string) {
	trimmed := strings.TrimSpace(value)
	if looksLikeGlobalMetaID(trimmed) {
		metaID := fallbackMetaID
		if metaID == "" {
			metaID = trimmed
		}
		return metaID, trimmed
	}
	metaID := trimmed
	if metaID == "" {
		metaID = fallbackMetaID
	}
	globalMetaID := fallbackGlobal
	if globalMetaID == "" {
		globalMetaID = metaID
	}
	return metaID, globalMetaID
}

func normalizeProtocol(path string) string {
	value := strings.TrimSpace(strings.ToLower(path))
	value = strings.TrimPrefix(value, "/")
	if strings.HasPrefix(value, "protocols/") {
		value = strings.TrimPrefix(value, "protocols/")
	}
	return value
}

func looksLikeGlobalMetaID(value string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(trimmed, "id")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func mergeUniqueStrings(lists ...[]string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0)
	for _, list := range lists {
		for _, value := range list {
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				continue
			}
			if _, exists := seen[trimmed]; exists {
				continue
			}
			seen[trimmed] = struct{}{}
			result = append(result, trimmed)
		}
	}
	return result
}
