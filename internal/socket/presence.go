package socket

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

const defaultPresenceSnapshotPath = "/.well-known/metasocket/presence"

// HandleOnlineStats returns the total number of active connections.
func (s *Server) HandleOnlineStats(c *gin.Context) {
	total := s.manager.TotalConnections()
	c.JSON(http.StatusOK, gin.H{
		"code":           0,
		"data":           gin.H{"totalConnections": total},
		"message":        "",
		"processingTime": time.Now().UnixMilli(),
	})
}

// HandleOnlineList returns a paginated list of online connections.
func (s *Server) HandleOnlineList(c *gin.Context) {
	pageStr := c.DefaultQuery("page", "1")
	sizeStr := c.DefaultQuery("size", "20")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	size, err := strconv.Atoi(sizeStr)
	if err != nil || size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}

	items := s.manager.OnlineList(page, size)
	if items == nil {
		items = []OnlineEntry{}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":           0,
		"data":           gin.H{"items": items},
		"message":        "",
		"processingTime": time.Now().UnixMilli(),
	})
}

// HandlePresenceSnapshot returns the local federated presence snapshot.
func (s *Server) HandlePresenceSnapshot(c *gin.Context) {
	provider := s.presenceSnapshotProvider()
	if provider == nil {
		c.Status(http.StatusNotFound)
		return
	}

	snapshot, err := provider.Snapshot()
	if err != nil || snapshot == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "presence snapshot unavailable"})
		return
	}

	c.JSON(http.StatusOK, snapshot)
}

// RegisterPresenceRoutes mounts presence HTTP endpoints on the given Gin router.
func (s *Server) RegisterPresenceRoutes(router *gin.Engine, snapshotPath ...string) {
	socketGroup := router.Group("/socket")
	{
		socketGroup.GET("/online/stats", s.HandleOnlineStats)
		socketGroup.GET("/online/list", s.HandleOnlineList)
	}

	path := defaultPresenceSnapshotPath
	if len(snapshotPath) > 0 && snapshotPath[0] != "" {
		path = snapshotPath[0]
	}
	router.GET(path, s.HandlePresenceSnapshot)
}
