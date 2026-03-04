package org

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"

	orgdto "nexus-saas/internal/org/handler/dto"
	"nexus/pkg/utils"
)

type Handler struct {
	repo *Repository
}

func NewHandler(repo *Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) Register(r *gin.RouterGroup) {
	r.POST("/orgs", h.CreateOrg)
}

func (h *Handler) CreateOrg(c *gin.Context) {
	var req orgdto.CreateOrgRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION", "message": err.Error()}})
		return
	}

	orgID, err := h.repo.UpsertOrgByName(c.Request.Context(), req.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL", "message": "failed to create org"}})
		return
	}

	rawKey := generateAPIKey()
	keyHash := utils.SHA256Hex(rawKey)
	keyName := req.Name + "-key"

	if err := h.repo.UpsertAPIKey(c.Request.Context(), orgID, keyHash, keyName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL", "message": "failed to create api key"}})
		return
	}

	scopes := req.Scopes
	if len(scopes) == 0 {
		scopes = []string{"admin:full"}
	}
	if err := h.repo.ReplaceAPIKeyScopes(c.Request.Context(), keyHash, scopes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL", "message": "failed to set scopes"}})
		return
	}

	c.JSON(http.StatusCreated, orgdto.CreateOrgResponse{
		OrgID:  orgID,
		APIKey: rawKey,
		Name:   req.Name,
	})
}

func generateAPIKey() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed")
	}
	return "nxk_" + hex.EncodeToString(b)
}
