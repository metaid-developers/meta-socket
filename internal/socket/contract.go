package socket

import (
	"encoding/json"
	"strconv"
	"strings"
)

// SocketData is the message envelope consumed by existing IDBots clients.
type SocketData struct {
	M string      `json:"M"`
	C interface{} `json:"C"`
	D interface{} `json:"D,omitempty"`
}

const (
	HEART_BEAT                    = "HEART_BEAT"
	WS_SERVER_NOTIFY_PRIVATE_CHAT = "WS_SERVER_NOTIFY_PRIVATE_CHAT"
	WS_SERVER_NOTIFY_GROUP_CHAT   = "WS_SERVER_NOTIFY_GROUP_CHAT"
	WS_SERVER_NOTIFY_GROUP_ROLE   = "WS_SERVER_NOTIFY_GROUP_ROLE"
	WS_RESPONSE_SUCCESS           = "WS_RESPONSE_SUCCESS"
	WS_RESPONSE_ERROR             = "WS_RESPONSE_ERROR"
)

const (
	WS_CODE_HEART_BEAT      = 10
	WS_CODE_HEART_BEAT_BACK = 10
	WS_CODE_SERVER          = 0
	WS_CODE_SEND_SUCCESS    = 200
	WS_CODE_SEND_ERROR      = 400
)

func (w *SocketData) ToString() (string, error) {
	data, err := json.Marshal(w)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SocketDataFromStringMsg parses a client message with compatibility for
// quoted-json payloads.
func SocketDataFromStringMsg(msg string) *SocketData {
	payload := strings.TrimSpace(msg)
	if payload == "" {
		return nil
	}

	if len(payload) >= 2 && payload[0] == '"' && payload[len(payload)-1] == '"' {
		if unquoted, err := strconv.Unquote(payload); err == nil {
			payload = unquoted
		}
	}

	var item SocketData
	if err := json.Unmarshal([]byte(payload), &item); err != nil {
		return nil
	}
	if item.M == "" {
		return nil
	}
	return &item
}
