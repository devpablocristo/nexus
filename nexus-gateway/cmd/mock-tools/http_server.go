package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func NewRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	r.POST("/echo", func(c *gin.Context) {
		var v any
		_ = c.ShouldBindJSON(&v)
		c.JSON(http.StatusOK, gin.H{
			"received":    v,
			"server_time": time.Now().UTC().Format(time.RFC3339),
		})
	})

	r.POST("/transfer", func(c *gin.Context) {
		var body struct {
			Amount float64 `json:"amount"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{"code": "INVALID_JSON", "message": "invalid json"},
			})
			return
		}
		if body.Amount <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{"code": "INVALID_AMOUNT", "message": "amount must be > 0"},
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"ok":     true,
			"tx_id":  uuid.NewString(),
			"amount": body.Amount,
		})
	})

	return r
}
