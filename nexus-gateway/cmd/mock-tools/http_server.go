package main

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type AuthArtifacts struct {
	KID        string
	PrivateKey *rsa.PrivateKey
}

var transferExecCount atomic.Int64

func NewAuthArtifacts() (*AuthArtifacts, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	return &AuthArtifacts{
		KID:        "mock-tools-kid-1",
		PrivateKey: privateKey,
	}, nil
}

func NewRouter(auth *AuthArtifacts) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	r.GET("/.well-known/jwks.json", func(c *gin.Context) {
		pub := auth.PrivateKey.PublicKey
		c.JSON(http.StatusOK, gin.H{
			"keys": []gin.H{
				{
					"kid": auth.KID,
					"kty": "RSA",
					"alg": "RS256",
					"use": "sig",
					"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
					"e":   base64.RawURLEncoding.EncodeToString([]byte{1, 0, 1}),
				},
			},
		})
	})
	r.GET("/_jwt/issue", func(c *gin.Context) {
		orgID := c.Query("org_id")
		if orgID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "org_id required"})
			return
		}
		sub := c.DefaultQuery("sub", "mock-agent")
		role := c.DefaultQuery("role", "bot")
		claims := jwt.MapClaims{
			"sub":    sub,
			"org_id": orgID,
			"role":   role,
			"scopes": []string{"tools:run", "audit:read", "admin:secrets"},
			"iss":    "nexus-demo-issuer",
			"aud":    "nexus-gateway",
			"exp":    time.Now().Add(15 * time.Minute).Unix(),
			"iat":    time.Now().Unix(),
		}
		t := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		t.Header["kid"] = auth.KID
		raw, err := t.SignedString(auth.PrivateKey)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "sign error"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"token": raw})
	})

	r.POST("/echo", func(c *gin.Context) {
		var v any
		_ = c.ShouldBindJSON(&v)
		authPresent := c.GetHeader("Authorization") != ""
		injectedHeaderPresent := c.GetHeader("X-Injected-Token") != ""
		c.JSON(http.StatusOK, gin.H{
			"received":                 v,
			"server_time":              time.Now().UTC().Format(time.RFC3339),
			"auth_present":             authPresent,
			"x_injected_token_present": injectedHeaderPresent,
		})
	})

	r.POST("/transfer", func(c *gin.Context) {
		var body struct {
			Amount   float64 `json:"amount"`
			SleepMS  int     `json:"sleep_ms"`
			Force5xx bool    `json:"force_5xx"`
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
		if body.SleepMS > 0 {
			time.Sleep(time.Duration(body.SleepMS) * time.Millisecond)
		}
		currentCount := transferExecCount.Add(1)
		if body.Force5xx {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{"code": "UPSTREAM_TEST_5XX", "message": "forced upstream failure"},
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"ok":              true,
			"tx_id":           uuid.NewString(),
			"amount":          body.Amount,
			"execution_count": currentCount,
		})
	})
	r.GET("/_stats/transfer", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"execution_count": transferExecCount.Load()})
	})

	return r
}
