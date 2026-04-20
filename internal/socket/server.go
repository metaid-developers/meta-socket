package socket

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/zishang520/engine.io/v2/types"
	socketio "github.com/zishang520/socket.io/v2/socket"

	"github.com/metaid-developers/meta-socket/internal/config"
)

const (
	DeviceTypePC  = "pc"
	DeviceTypeAPP = "app"

	ExtraPushServiceMetaID = "extra_push_service"
)

type ConnectionInfo struct {
	SocketID    string
	MetaID      string
	DeviceType  string
	ConnectTime time.Time
	LastActive  time.Time
	IsActive    bool
}

type UserConnections struct {
	MetaID  string
	Devices []*ConnectionInfo
}

type Manager struct {
	server          *socketio.Server
	socketConfig    config.SocketConfig
	connections     sync.Map // map[metaid]*UserConnections
	socketIndex     sync.Map // map[socketID]*socketio.Socket
	mutex           sync.Mutex
	extraConnection *ConnectionInfo
	connectHandler  func(metaid string, client *socketio.Socket)
}

var (
	globalManager *Manager
	globalMu      sync.RWMutex
)

func NewManager(cfg config.SocketConfig) (*Manager, error) {
	server := socketio.NewServer(nil, nil)
	if server == nil {
		return nil, errors.New("socket server creation failed")
	}

	manager := &Manager{
		server:       server,
		socketConfig: cfg,
	}
	manager.setupAutoListeners()
	return manager, nil
}

func SetGlobalManager(manager *Manager) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalManager = manager
}

func GetManager() *Manager {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalManager
}

func (m *Manager) SetOnClientConnected(handler func(metaid string, client *socketio.Socket)) {
	m.connectHandler = handler
}

func (m *Manager) Handler() http.Handler {
	options := socketio.DefaultServerOptions()
	options.SetServeClient(true)
	options.SetPingInterval(m.socketConfig.PingInterval)
	options.SetPingTimeout(m.socketConfig.PingTimeout)
	options.SetConnectTimeout(10 * time.Second)
	options.SetUpgradeTimeout(10 * time.Second)
	options.SetMaxHttpBufferSize(1_000_000)
	options.SetTransports(types.NewSet("polling", "websocket"))
	options.SetAllowEIO3(m.socketConfig.AllowEIO3)
	options.SetCors(&types.Cors{
		Origin:      "*",
		Credentials: true,
	})
	return m.server.ServeHandler(options)
}

func (m *Manager) setupAutoListeners() {
	m.server.On("connection", func(args ...interface{}) {
		if len(args) == 0 {
			return
		}
		client, ok := args[0].(*socketio.Socket)
		if !ok || client == nil {
			return
		}
		m.handleClientConnect(client)
	})
}

func (m *Manager) handleClientConnect(client *socketio.Socket) {
	if m.socketConfig.ExtraPushAuthKey != "" {
		extraAuthKey := m.getExtraPushAuthKeyFromSocket(client)
		if extraAuthKey != "" && extraAuthKey == m.socketConfig.ExtraPushAuthKey {
			m.storeSocket(client)
			m.extraConnection = &ConnectionInfo{
				SocketID:    string(client.Id()),
				MetaID:      ExtraPushServiceMetaID,
				DeviceType:  DeviceTypePC,
				ConnectTime: time.Now(),
				LastActive:  time.Now(),
				IsActive:    true,
			}
			_ = m.sendMessage(client, &SocketData{
				M: WS_RESPONSE_SUCCESS,
				C: WS_CODE_SEND_SUCCESS,
				D: "Extra push connection successful",
			})
			m.setupExtraPushListeners(client)
			return
		}
	}

	metaid, deviceType := m.getMetaIDAndDeviceTypeFromSocket(client)
	if metaid == "" {
		client.Disconnect(true)
		return
	}

	if m.socketConfig.MaxConnections > 0 && m.getConnectionCount() >= m.socketConfig.MaxConnections {
		log.Printf("[SOCKET] rejected connection because max limit reached: socketID=%s metaid=%s", client.Id(), metaid)
		client.Disconnect(true)
		return
	}

	evictedSocketID := m.addDeviceConnection(metaid, deviceType, string(client.Id()))
	m.storeSocket(client)

	if evictedSocketID != "" {
		go m.disconnectBySocketID(evictedSocketID)
	}

	_ = m.sendMessage(client, &SocketData{
		M: WS_RESPONSE_SUCCESS,
		C: WS_CODE_SEND_SUCCESS,
		D: "Connection successful",
	})

	if m.connectHandler != nil {
		m.connectHandler(metaid, client)
	}

	client.On("message", func(args ...interface{}) {
		if len(args) == 0 {
			return
		}
		m.handleClientMessage(client, args[0])
	})

	client.On("ping", func(args ...interface{}) {
		m.handleClientPing(client)
	})

	client.On("disconnect", func(args ...interface{}) {
		reason := "unknown"
		if len(args) > 0 {
			if s, ok := args[0].(string); ok {
				reason = s
			}
		}
		m.handleClientDisconnect(client, reason)
	})
}

func (m *Manager) handleClientDisconnect(client *socketio.Socket, reason string) {
	if client == nil {
		return
	}
	log.Printf("[SOCKET] disconnected: socketID=%s reason=%s", client.Id(), reason)
	m.removeSocket(string(client.Id()))
	m.removeDeviceConnection(string(client.Id()))
}

func (m *Manager) handleClientMessage(client *socketio.Socket, payload interface{}) {
	if client == nil {
		return
	}
	m.updateDeviceConnectionActivity(string(client.Id()))

	msg, ok := payload.(string)
	if !ok {
		data, err := json.Marshal(payload)
		if err != nil {
			_ = m.sendError(client, "Invalid message format", WS_CODE_SEND_ERROR)
			return
		}
		msg = string(data)
	}

	socketData := SocketDataFromStringMsg(msg)
	if socketData == nil {
		_ = m.sendError(client, "Invalid message format", WS_CODE_SEND_ERROR)
		return
	}

	if socketData.M == HEART_BEAT {
		_ = m.sendMessage(client, BuildHeartbeatReplyEnvelope())
		return
	}
}

func (m *Manager) handleClientPing(client *socketio.Socket) {
	if client == nil {
		return
	}
	m.updateDeviceConnectionActivity(string(client.Id()))
	_ = m.sendMessage(client, BuildPongReplyEnvelope())
}

func (m *Manager) getMetaIDAndDeviceTypeFromSocket(client *socketio.Socket) (string, string) {
	if client == nil || client.Handshake() == nil {
		return "", DeviceTypePC
	}

	handshake := client.Handshake()
	return ResolveHandshakeIdentity(handshake.Auth, handshake.Query)
}

func (m *Manager) getExtraPushAuthKeyFromSocket(client *socketio.Socket) string {
	if client == nil || client.Handshake() == nil {
		return ""
	}

	handshake := client.Handshake()
	if handshake.Auth != nil {
		if authMap, ok := handshake.Auth.(map[string]interface{}); ok {
			if value, ok := authMap["extraPushAuthKey"].(string); ok {
				return value
			}
		}
	}
	if handshake.Query != nil {
		if values, ok := handshake.Query["extraPushAuthKey"]; ok && len(values) > 0 {
			return values[0]
		}
	}
	return ""
}

func (m *Manager) setupExtraPushListeners(client *socketio.Socket) {
	client.On("message", func(args ...interface{}) {
		if len(args) == 0 {
			return
		}
		m.handleExtraPushMessage(client, args[0])
	})

	client.On("ping", func(args ...interface{}) {
		m.handleExtraPushPing(client)
	})

	client.On("disconnect", func(args ...interface{}) {
		reason := "unknown"
		if len(args) > 0 {
			if s, ok := args[0].(string); ok {
				reason = s
			}
		}
		m.handleExtraPushDisconnect(client, reason)
	})
}

func (m *Manager) handleExtraPushMessage(client *socketio.Socket, payload interface{}) {
	if client == nil {
		return
	}
	if m.extraConnection != nil {
		m.extraConnection.LastActive = time.Now()
	}

	msg, ok := payload.(string)
	if !ok {
		data, err := json.Marshal(payload)
		if err != nil {
			_ = m.sendError(client, "Invalid message format", WS_CODE_SEND_ERROR)
			return
		}
		msg = string(data)
	}
	socketData := SocketDataFromStringMsg(msg)
	if socketData == nil {
		_ = m.sendError(client, "Invalid message format", WS_CODE_SEND_ERROR)
		return
	}
	if socketData.M == HEART_BEAT {
		_ = m.sendMessage(client, BuildHeartbeatReplyEnvelope())
	}
}

func (m *Manager) handleExtraPushPing(client *socketio.Socket) {
	if client == nil {
		return
	}
	if m.extraConnection != nil {
		m.extraConnection.LastActive = time.Now()
	}
	_ = m.sendMessage(client, BuildPongReplyEnvelope())
}

func (m *Manager) handleExtraPushDisconnect(client *socketio.Socket, reason string) {
	if client == nil {
		m.extraConnection = nil
		return
	}
	log.Printf("[SOCKET] extra push disconnected: socketID=%s reason=%s", client.Id(), reason)
	m.removeSocket(string(client.Id()))
	m.extraConnection = nil
}

func (m *Manager) addDeviceConnection(metaid, deviceType, socketID string) string {
	newDevice := &ConnectionInfo{
		SocketID:    socketID,
		MetaID:      metaid,
		DeviceType:  deviceType,
		ConnectTime: time.Now(),
		LastActive:  time.Now(),
		IsActive:    true,
	}

	limit := m.socketConfig.MaxPCPerUser
	if deviceType == DeviceTypeAPP {
		limit = m.socketConfig.MaxAppPerUser
	}

	evictedSocketID := ""

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if value, ok := m.connections.Load(metaid); ok {
		userConn := value.(*UserConnections)

		typeCount := 0
		oldestIndex := -1
		var oldestAt time.Time
		for i, device := range userConn.Devices {
			if device.DeviceType != deviceType {
				continue
			}
			typeCount++
			if oldestIndex == -1 || device.ConnectTime.Before(oldestAt) {
				oldestIndex = i
				oldestAt = device.ConnectTime
			}
		}

		if limit > 0 && typeCount >= limit && oldestIndex >= 0 {
			evictedSocketID = userConn.Devices[oldestIndex].SocketID
			userConn.Devices = append(userConn.Devices[:oldestIndex], userConn.Devices[oldestIndex+1:]...)
		}

		userConn.Devices = append(userConn.Devices, newDevice)
		return evictedSocketID
	}

	m.connections.Store(metaid, &UserConnections{
		MetaID:  metaid,
		Devices: []*ConnectionInfo{newDevice},
	})
	return evictedSocketID
}

func (m *Manager) removeDeviceConnection(socketID string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.connections.Range(func(key, value interface{}) bool {
		userConn := value.(*UserConnections)
		for i, device := range userConn.Devices {
			if device.SocketID != socketID {
				continue
			}
			userConn.Devices = append(userConn.Devices[:i], userConn.Devices[i+1:]...)
			if len(userConn.Devices) == 0 {
				m.connections.Delete(key)
			}
			return false
		}
		return true
	})
}

func (m *Manager) updateDeviceConnectionActivity(socketID string) {
	m.connections.Range(func(key, value interface{}) bool {
		userConn := value.(*UserConnections)
		for _, device := range userConn.Devices {
			if device.SocketID != socketID {
				continue
			}
			device.LastActive = time.Now()
			return false
		}
		return true
	})
}

func (m *Manager) getConnectionCount() int {
	count := 0
	m.connections.Range(func(key, value interface{}) bool {
		userConn := value.(*UserConnections)
		count += len(userConn.Devices)
		return true
	})
	return count
}

func (m *Manager) storeSocket(client *socketio.Socket) {
	if client == nil {
		return
	}
	m.socketIndex.Store(string(client.Id()), client)
}

func (m *Manager) removeSocket(socketID string) {
	if socketID == "" {
		return
	}
	m.socketIndex.Delete(socketID)
}

func (m *Manager) getSocketByID(socketID string) *socketio.Socket {
	if socketID == "" {
		return nil
	}
	if value, ok := m.socketIndex.Load(socketID); ok {
		if client, ok := value.(*socketio.Socket); ok {
			return client
		}
	}
	return nil
}

func (m *Manager) disconnectBySocketID(socketID string) {
	client := m.getSocketByID(socketID)
	if client != nil {
		client.Disconnect(true)
	}
}

func (m *Manager) sendError(client *socketio.Socket, message string, code int) error {
	return m.sendMessage(client, &SocketData{
		M: WS_RESPONSE_ERROR,
		C: code,
		D: message,
	})
}

func (m *Manager) sendMessage(client *socketio.Socket, socketData *SocketData) error {
	if client == nil {
		return errors.New("client is nil")
	}
	if socketData == nil {
		return errors.New("socketData is nil")
	}
	msg, err := socketData.ToString()
	if err != nil {
		return err
	}
	return client.Emit("message", msg)
}

func (m *Manager) SendMessageToUserAllDevices(metaid string, socketData *SocketData) error {
	if socketData == nil {
		return errors.New("socketData is nil")
	}
	devices, exists := m.GetUserAllConnections(metaid)
	if !exists {
		return nil
	}
	for _, device := range devices {
		client := m.getSocketByID(device.SocketID)
		if client == nil {
			continue
		}
		if err := m.sendMessage(client, socketData); err != nil {
			log.Printf("[SOCKET] failed to send message: metaid=%s socketID=%s err=%v", metaid, device.SocketID, err)
		}
	}
	return nil
}

func (m *Manager) SendMessageToRoom(room string, socketData *SocketData) error {
	if room == "" {
		return errors.New("room is empty")
	}
	if socketData == nil {
		return errors.New("socketData is nil")
	}
	msg, err := socketData.ToString()
	if err != nil {
		return err
	}
	if m.server == nil {
		return fmt.Errorf("socket server not initialized")
	}
	m.server.To(socketio.Room(room)).Emit("message", msg)
	return nil
}

func (m *Manager) SendMessageToExtraPush(socketData *SocketData) error {
	if socketData == nil {
		return errors.New("socketData is nil")
	}
	if m.extraConnection == nil || !m.extraConnection.IsActive {
		return errors.New("extra push connection not available")
	}
	client := m.getSocketByID(m.extraConnection.SocketID)
	if client == nil {
		return fmt.Errorf("extra push socket not found: socketID=%s", m.extraConnection.SocketID)
	}
	return m.sendMessage(client, socketData)
}

func (m *Manager) JoinRoomForUser(metaid, room string) int {
	if metaid == "" || room == "" {
		return 0
	}
	devices, exists := m.GetUserAllConnections(metaid)
	if !exists {
		return 0
	}
	count := 0
	for _, device := range devices {
		client := m.getSocketByID(device.SocketID)
		if client == nil {
			continue
		}
		client.Join(socketio.Room(room))
		count++
	}
	return count
}

func (m *Manager) LeaveRoomForUser(metaid, room string) int {
	if metaid == "" || room == "" {
		return 0
	}
	devices, exists := m.GetUserAllConnections(metaid)
	if !exists {
		return 0
	}
	count := 0
	for _, device := range devices {
		client := m.getSocketByID(device.SocketID)
		if client == nil {
			continue
		}
		client.Leave(socketio.Room(room))
		count++
	}
	return count
}

func (m *Manager) GetUserAllConnections(metaid string) ([]*ConnectionInfo, bool) {
	value, exists := m.connections.Load(metaid)
	if !exists {
		return nil, false
	}
	userConn := value.(*UserConnections)
	items := make([]*ConnectionInfo, 0, len(userConn.Devices))
	for _, device := range userConn.Devices {
		if device.IsActive {
			items = append(items, device)
		}
	}
	return items, len(items) > 0
}
