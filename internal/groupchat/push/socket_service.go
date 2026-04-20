package push

import (
	"fmt"
	"log"
	"sync"

	"github.com/metaid-developers/meta-socket/internal/socket"
)

type ServiceConfig struct {
	RoomBroadcastEnabled bool
}

var (
	serviceConfig = ServiceConfig{
		RoomBroadcastEnabled: true,
	}
	serviceConfigMu sync.RWMutex
)

const groupRoomPrefix = "group:"

func Configure(cfg ServiceConfig) {
	serviceConfigMu.Lock()
	defer serviceConfigMu.Unlock()
	serviceConfig = cfg
}

func RoomBroadcastEnabled() bool {
	serviceConfigMu.RLock()
	defer serviceConfigMu.RUnlock()
	return serviceConfig.RoomBroadcastEnabled
}

func GroupRoomName(groupID string) string {
	if groupID == "" {
		return ""
	}
	return groupRoomPrefix + groupID
}

func SendGroupMessageToUser(metaid, globalMetaID string, message interface{}) error {
	return sendMessageToIdentities(
		[]string{metaid},
		[]string{globalMetaID},
		socket.WS_SERVER_NOTIFY_GROUP_CHAT,
		socket.WS_CODE_SERVER,
		message,
	)
}

func SendGroupRoleInfoToUser(metaid, globalMetaID string, message interface{}) error {
	return sendMessageToIdentities(
		[]string{metaid},
		[]string{globalMetaID},
		socket.WS_SERVER_NOTIFY_GROUP_ROLE,
		socket.WS_CODE_SERVER,
		message,
	)
}

func SendPrivateMessageToUser(metaid, globalMetaID string, message interface{}) error {
	return sendMessageToIdentities(
		[]string{metaid},
		[]string{globalMetaID},
		socket.WS_SERVER_NOTIFY_PRIVATE_CHAT,
		socket.WS_CODE_SERVER,
		message,
	)
}

func SendMessageToTargets(metaIDs, globalMetaIDs []string, method string, code int, message interface{}) error {
	return sendMessageToIdentities(metaIDs, globalMetaIDs, method, code, message)
}

func SendGroupMessageToRoom(groupID string, method string, code int, message interface{}) error {
	manager := socket.GetManager()
	if manager == nil {
		return fmt.Errorf("socket manager not initialized")
	}
	room := GroupRoomName(groupID)
	if room == "" {
		return fmt.Errorf("invalid group room")
	}
	return manager.SendMessageToRoom(room, &socket.SocketData{
		M: method,
		C: code,
		D: message,
	})
}

func JoinGroupRoomForUser(identity, groupID string) int {
	if !RoomBroadcastEnabled() {
		return 0
	}
	manager := socket.GetManager()
	if manager == nil {
		return 0
	}
	return manager.JoinRoomForUser(identity, GroupRoomName(groupID))
}

func LeaveGroupRoomForUser(identity, groupID string) int {
	if !RoomBroadcastEnabled() {
		return 0
	}
	manager := socket.GetManager()
	if manager == nil {
		return 0
	}
	return manager.LeaveRoomForUser(identity, GroupRoomName(groupID))
}

func SendAllMessageToExtraPush(message interface{}, repostMetaIDs, mentionMetaIDs, repostGlobalMetaIDs, mentionGlobalMetaIDs []string, method string) error {
	manager := socket.GetManager()
	if manager == nil {
		return nil
	}

	type ExtraPushMessage struct {
		Message              interface{} `json:"message"`
		RepostMetaIDs        []string    `json:"repostMetaIds"`
		MentionMetaIDs       []string    `json:"mentionMetaIds"`
		RepostGlobalMetaIDs  []string    `json:"repostGlobalMetaIds"`
		MentionGlobalMetaIDs []string    `json:"mentionGlobalMetaIds"`
	}

	payload := &ExtraPushMessage{
		Message:              message,
		RepostMetaIDs:        repostMetaIDs,
		MentionMetaIDs:       mentionMetaIDs,
		RepostGlobalMetaIDs:  repostGlobalMetaIDs,
		MentionGlobalMetaIDs: mentionGlobalMetaIDs,
	}

	return manager.SendMessageToExtraPush(&socket.SocketData{
		M: method,
		C: socket.WS_CODE_SERVER,
		D: payload,
	})
}

func sendMessageToIdentities(metaIDs, globalMetaIDs []string, method string, code int, message interface{}) error {
	manager := socket.GetManager()
	if manager == nil {
		log.Printf("socket manager not initialized")
		return nil
	}

	payload := &socket.SocketData{
		M: method,
		C: code,
		D: message,
	}

	targets := mergeUnique(metaIDs, globalMetaIDs)
	for _, identity := range targets {
		if err := manager.SendMessageToUserAllDevices(identity, payload); err != nil {
			return err
		}
	}
	return nil
}

func mergeUnique(lists ...[]string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0)
	for _, list := range lists {
		for _, item := range list {
			if item == "" {
				continue
			}
			if _, exists := seen[item]; exists {
				continue
			}
			seen[item] = struct{}{}
			result = append(result, item)
		}
	}
	return result
}
