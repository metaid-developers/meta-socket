package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// RespSuccess sends a successful JSON response in idchat-compatible format.
func RespSuccess(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{
		"code":           0,
		"data":           data,
		"message":        "",
		"processingTime": time.Now().UnixMilli(),
	})
}

// RespErr sends an error JSON response in idchat-compatible format.
func RespErr(c *gin.Context, code int, message string) {
	if code == 0 {
		code = 1
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    code,
		"message": message,
	})
}
