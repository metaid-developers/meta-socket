package socket

import (
	"errors"
	"net/http"
	"sort"
	"strconv"

	"github.com/gin-gonic/gin"
)

const (
	defaultOnlinePage = 1
	defaultOnlineSize = 20
	maxOnlineSize     = 200
)

type OnlineConnection struct {
	SocketID    string `json:"socketId"`
	DeviceType  string `json:"deviceType"`
	ConnectTime int64  `json:"connectTime"`
	LastActive  int64  `json:"lastActive"`
}

type OnlineMetaBot struct {
	MetaID          string             `json:"metaId"`
	ConnectionCount int                `json:"connectionCount"`
	LastActive      int64              `json:"lastActive"`
	Connections     []OnlineConnection `json:"connections"`
}

func MountOnlineRoutes(router gin.IRoutes, manager *Manager) error {
	if router == nil {
		return errors.New("router is nil")
	}
	if manager == nil {
		return errors.New("socket manager is nil")
	}

	router.GET("/socket/online/stats", func(c *gin.Context) {
		totalMetaBots, totalConnections := manager.OnlineStats()
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "ok",
			"data": gin.H{
				"totalMetaBots":    totalMetaBots,
				"totalConnections": totalConnections,
			},
		})
	})

	router.GET("/socket/online/list", func(c *gin.Context) {
		page := normalizePage(c.Query("page"))
		size := normalizeSize(c.Query("size"))

		items, totalMetaBots, totalConnections := manager.ListOnlineMetaBots(page, size)
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "ok",
			"data": gin.H{
				"page":             page,
				"size":             size,
				"total":            totalMetaBots,
				"totalConnections": totalConnections,
				"items":            items,
			},
		})
	})

	return nil
}

func (m *Manager) OnlineStats() (int, int) {
	if m == nil {
		return 0, 0
	}
	items := m.snapshotOnlineMetaBots()
	totalConnections := 0
	for _, item := range items {
		totalConnections += item.ConnectionCount
	}
	return len(items), totalConnections
}

func (m *Manager) ListOnlineMetaBots(page, size int) ([]OnlineMetaBot, int, int) {
	if m == nil {
		return nil, 0, 0
	}

	normalizedPage := normalizePage(strconv.Itoa(page))
	normalizedSize := normalizeSize(strconv.Itoa(size))
	items := m.snapshotOnlineMetaBots()

	totalMetaBots := len(items)
	totalConnections := 0
	for _, item := range items {
		totalConnections += item.ConnectionCount
	}

	offset := (normalizedPage - 1) * normalizedSize
	if offset >= totalMetaBots {
		return []OnlineMetaBot{}, totalMetaBots, totalConnections
	}
	end := offset + normalizedSize
	if end > totalMetaBots {
		end = totalMetaBots
	}

	return items[offset:end], totalMetaBots, totalConnections
}

func (m *Manager) snapshotOnlineMetaBots() []OnlineMetaBot {
	if m == nil {
		return nil
	}

	items := make([]OnlineMetaBot, 0)
	m.connections.Range(func(key, value interface{}) bool {
		metaID, ok := key.(string)
		if !ok || metaID == "" {
			return true
		}
		userConn, ok := value.(*UserConnections)
		if !ok || userConn == nil {
			return true
		}

		connections := make([]OnlineConnection, 0, len(userConn.Devices))
		lastActive := int64(0)
		for _, device := range userConn.Devices {
			if device == nil || !device.IsActive {
				continue
			}
			connection := OnlineConnection{
				SocketID:    device.SocketID,
				DeviceType:  device.DeviceType,
				ConnectTime: device.ConnectTime.Unix(),
				LastActive:  device.LastActive.Unix(),
			}
			if connection.LastActive > lastActive {
				lastActive = connection.LastActive
			}
			connections = append(connections, connection)
		}
		if len(connections) == 0 {
			return true
		}

		sort.Slice(connections, func(i, j int) bool {
			if connections[i].LastActive == connections[j].LastActive {
				return connections[i].SocketID < connections[j].SocketID
			}
			return connections[i].LastActive > connections[j].LastActive
		})

		items = append(items, OnlineMetaBot{
			MetaID:          metaID,
			ConnectionCount: len(connections),
			LastActive:      lastActive,
			Connections:     connections,
		})
		return true
	})

	sort.Slice(items, func(i, j int) bool {
		if items[i].LastActive == items[j].LastActive {
			return items[i].MetaID < items[j].MetaID
		}
		return items[i].LastActive > items[j].LastActive
	})

	return items
}

func normalizePage(raw string) int {
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return defaultOnlinePage
	}
	return value
}

func normalizeSize(raw string) int {
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return defaultOnlineSize
	}
	if value > maxOnlineSize {
		return maxOnlineSize
	}
	return value
}
