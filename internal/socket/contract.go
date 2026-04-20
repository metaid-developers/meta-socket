package socket

import (
	"encoding/json"
	"errors"
	"fmt"
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

func BuildHeartbeatReplyEnvelope() *SocketData {
	return &SocketData{
		M: HEART_BEAT,
		C: WS_CODE_HEART_BEAT_BACK,
	}
}

func BuildPongReplyEnvelope() *SocketData {
	return &SocketData{
		M: "pong",
		C: WS_CODE_SEND_SUCCESS,
	}
}

func ResolveHandshakeIdentity(auth interface{}, query map[string][]string) (string, string) {
	metaID := ""
	deviceType := DeviceTypePC

	if authMap, ok := auth.(map[string]interface{}); ok {
		if value, ok := authMap["metaid"].(string); ok {
			metaID = value
		}
		if value, ok := authMap["type"].(string); ok && value == DeviceTypeAPP {
			deviceType = DeviceTypeAPP
		}
	}

	if query == nil {
		return metaID, deviceType
	}

	if metaID == "" {
		if values, ok := query["metaid"]; ok && len(values) > 0 {
			metaID = values[0]
		}
	}
	if deviceType == DeviceTypePC {
		if values, ok := query["type"]; ok && len(values) > 0 && values[0] == DeviceTypeAPP {
			deviceType = DeviceTypeAPP
		}
	}

	return metaID, deviceType
}

func ParseEnvelopeString(msg string) (*SocketData, error) {
	payload := strings.TrimSpace(msg)
	if payload == "" {
		return nil, errors.New("empty payload")
	}

	if len(payload) >= 2 && payload[0] == '"' && payload[len(payload)-1] == '"' {
		unquoted, err := strconv.Unquote(payload)
		if err != nil {
			return nil, fmt.Errorf("unquote payload: %w", err)
		}
		payload = unquoted
	}

	var item SocketData
	if err := json.Unmarshal([]byte(payload), &item); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}
	if item.M == "" {
		return nil, errors.New("missing envelope method M")
	}
	return &item, nil
}

func CodeAsInt(code interface{}) (int, bool) {
	switch v := code.(type) {
	case int:
		return v, true
	case int8:
		return int(v), true
	case int16:
		return int(v), true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case uint:
		return int(v), true
	case uint8:
		return int(v), true
	case uint16:
		return int(v), true
	case uint32:
		return int(v), true
	case uint64:
		return int(v), true
	case float32:
		return int(v), true
	case float64:
		return int(v), true
	case json.Number:
		value, err := v.Int64()
		if err != nil {
			return 0, false
		}
		return int(value), true
	case string:
		value, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return 0, false
		}
		return value, true
	default:
		return 0, false
	}
}

func ExtractSuccessWrappedData(item *SocketData) (interface{}, bool) {
	if item == nil || item.M != WS_RESPONSE_SUCCESS {
		return nil, false
	}
	data, ok := item.D.(map[string]interface{})
	if !ok {
		return nil, false
	}
	value, exists := data["data"]
	return value, exists
}

// SocketDataFromStringMsg parses a client message with compatibility for
// quoted-json payloads.
func SocketDataFromStringMsg(msg string) *SocketData {
	item, err := ParseEnvelopeString(msg)
	if err != nil {
		return nil
	}
	return item
}
