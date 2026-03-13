package secrets

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"data-plane/internal/secrets/handler/dto"
	secretdomain "data-plane/internal/secrets/usecases/domain"
	"data-plane/internal/shared/authz"
	httperr "nexus/pkg/http/errors"
	"nexus/pkg/types"
)

type secretsUsecase interface {
	UpsertForTool(ctx context.Context, orgID uuid.UUID, toolName, secretType, keyName, value string, enabled bool) (secretdomain.ToolSecret, error)
	ListForTool(ctx context.Context, orgID uuid.UUID, toolName string) ([]secretdomain.ToolSecret, error)
	DeleteForTool(ctx context.Context, orgID uuid.UUID, toolName, keyName string) error
}

type Handler struct {
	uc secretsUsecase
}

func NewHandler(uc secretsUsecase) *Handler { return &Handler{uc: uc} }

func AsSecretsUsecase(uc *Usecases) secretsUsecase { return uc }

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.POST("/tools/:name/secrets", h.upsert)
	rg.GET("/tools/:name/secrets", h.list)
	rg.DELETE("/tools/:name/secrets", h.delete)
}

func (h *Handler) upsert(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeSecretsAdmin) {
		httperr.Write(c, 403, types.ErrCodeSecretDenied, "secret admin scope required")
		return
	}
	var req dto.UpsertSecretRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid json")
		return
	}
	orgID := mustOrgID(c)
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	created, err := h.uc.UpsertForTool(c.Request.Context(), orgID, c.Param("name"), req.SecretType, req.KeyName, req.Value, enabled)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(200, dto.SecretResponse{ID: created.ID.String(), SecretType: created.SecretType, KeyName: created.KeyName, Enabled: created.Enabled, CreatedAt: created.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"), UpdatedAt: created.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00")})
}

func (h *Handler) list(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeSecretsAdmin) {
		httperr.Write(c, 403, types.ErrCodeSecretDenied, "secret admin scope required")
		return
	}
	orgID := mustOrgID(c)
	items, err := h.uc.ListForTool(c.Request.Context(), orgID, c.Param("name"))
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	resp := make([]dto.SecretResponse, 0, len(items))
	for _, item := range items {
		resp = append(resp, dto.SecretResponse{ID: item.ID.String(), SecretType: item.SecretType, KeyName: item.KeyName, Enabled: item.Enabled, CreatedAt: item.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"), UpdatedAt: item.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00")})
	}
	c.JSON(200, gin.H{"items": resp})
}

func (h *Handler) delete(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeSecretsAdmin) {
		httperr.Write(c, 403, types.ErrCodeSecretDenied, "secret admin scope required")
		return
	}
	orgID := mustOrgID(c)
	if err := h.uc.DeleteForTool(c.Request.Context(), orgID, c.Param("name"), c.Query("key_name")); err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.Status(204)
}

func mustOrgID(c *gin.Context) uuid.UUID {
	v, _ := c.Get(string(types.CtxKeyOrgID))
	id, _ := v.(uuid.UUID)
	return id
}

func roleFromCtx(c *gin.Context) *string {
	if v, ok := c.Get(string(types.CtxKeyRole)); ok {
		if s, ok := v.(string); ok && s != "" {
			return &s
		}
	}
	return nil
}

func scopesFromCtx(c *gin.Context) []string {
	if v, ok := c.Get(string(types.CtxKeyScopes)); ok {
		if scopes, ok := v.([]string); ok {
			return scopes
		}
	}
	return nil
}
