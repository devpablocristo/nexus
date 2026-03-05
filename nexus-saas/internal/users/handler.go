package users

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"nexus-saas/internal/shared/authz"
	userdto "nexus-saas/internal/users/handler/dto"
	userdomain "nexus-saas/internal/users/usecases/domain"
	httperr "nexus/pkg/http/errors"
	"nexus/pkg/types"
)

type Handler struct {
	uc *Usecases
}

func NewHandler(uc *Usecases) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.GET("/users/me", h.me)
	rg.GET("/orgs/:org_id/members", h.listOrgMembers)
	rg.GET("/orgs/:org_id/api-keys", h.listAPIKeys)
	rg.POST("/orgs/:org_id/api-keys", h.createAPIKey)
	rg.DELETE("/orgs/:org_id/api-keys/:id", h.deleteAPIKey)
	rg.POST("/orgs/:org_id/api-keys/:id/rotate", h.rotateAPIKey)
}

func (h *Handler) me(c *gin.Context) {
	profile, err := h.uc.GetMe(
		c.Request.Context(),
		mustOrgID(c),
		actorFromCtx(c),
		roleFromCtx(c),
		scopesFromCtx(c),
	)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	resp := userdto.MeResponse{
		OrgID:      profile.OrgID.String(),
		ExternalID: profile.ExternalID,
		Role:       profile.Role,
		Scopes:     profile.Scopes,
	}
	if profile.User != nil {
		u := profile.User
		resp.User = &userdto.UserInfo{
			ID:         u.ID.String(),
			ExternalID: u.ExternalID,
			Email:      u.Email,
			Name:       u.Name,
			AvatarURL:  u.AvatarURL,
			CreatedAt:  toRFC3339(u.CreatedAt),
			UpdatedAt:  toRFC3339(u.UpdatedAt),
		}
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) listOrgMembers(c *gin.Context) {
	if !canReadMembers(scopesFromCtx(c), roleFromCtx(c)) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, "admin/secops or admin:console:read required")
		return
	}
	orgID, err := uuid.Parse(strings.TrimSpace(c.Param("org_id")))
	if err != nil {
		httperr.BadRequest(c, "invalid org_id")
		return
	}
	if err := EnsureOrgMatch(orgID, mustOrgID(c)); err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	items, err := h.uc.ListOrgMembers(c.Request.Context(), orgID)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	resp := userdto.ListOrgMembersResponse{Items: make([]userdto.OrgMemberItem, 0, len(items))}
	for _, item := range items {
		resp.Items = append(resp.Items, toMemberDTO(item))
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) listAPIKeys(c *gin.Context) {
	if !canReadAPIKeys(scopesFromCtx(c), roleFromCtx(c)) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, "admin/secops or admin:console:read required")
		return
	}
	orgID, err := uuid.Parse(strings.TrimSpace(c.Param("org_id")))
	if err != nil {
		httperr.BadRequest(c, "invalid org_id")
		return
	}
	if err := EnsureOrgMatch(orgID, mustOrgID(c)); err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	items, err := h.uc.ListAPIKeys(c.Request.Context(), orgID)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	resp := userdto.ListAPIKeysResponse{Items: make([]userdto.APIKeyItem, 0, len(items))}
	for _, item := range items {
		resp.Items = append(resp.Items, toAPIKeyDTO(item))
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) createAPIKey(c *gin.Context) {
	if !canWriteAPIKeys(scopesFromCtx(c), roleFromCtx(c)) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, "admin or admin:console:write required")
		return
	}
	orgID, err := uuid.Parse(strings.TrimSpace(c.Param("org_id")))
	if err != nil {
		httperr.BadRequest(c, "invalid org_id")
		return
	}
	if err := EnsureOrgMatch(orgID, mustOrgID(c)); err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	var req userdto.CreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid json")
		return
	}
	created, err := h.uc.CreateAPIKey(c.Request.Context(), orgID, req.Name, req.Scopes)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusCreated, userdto.CreateAPIKeyResponse{
		ID:        created.Key.ID.String(),
		OrgID:     created.Key.OrgID.String(),
		Name:      created.Key.Name,
		Scopes:    created.Key.Scopes,
		APIKey:    created.Raw,
		CreatedAt: toRFC3339(created.Key.CreatedAt),
	})
}

func (h *Handler) deleteAPIKey(c *gin.Context) {
	if !canWriteAPIKeys(scopesFromCtx(c), roleFromCtx(c)) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, "admin or admin:console:write required")
		return
	}
	orgID, err := uuid.Parse(strings.TrimSpace(c.Param("org_id")))
	if err != nil {
		httperr.BadRequest(c, "invalid org_id")
		return
	}
	if err := EnsureOrgMatch(orgID, mustOrgID(c)); err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	keyID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		httperr.BadRequest(c, "invalid id")
		return
	}
	if err := h.uc.DeleteAPIKey(c.Request.Context(), orgID, keyID); err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) rotateAPIKey(c *gin.Context) {
	if !canWriteAPIKeys(scopesFromCtx(c), roleFromCtx(c)) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, "admin or admin:console:write required")
		return
	}
	orgID, err := uuid.Parse(strings.TrimSpace(c.Param("org_id")))
	if err != nil {
		httperr.BadRequest(c, "invalid org_id")
		return
	}
	if err := EnsureOrgMatch(orgID, mustOrgID(c)); err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	keyID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		httperr.BadRequest(c, "invalid id")
		return
	}
	raw, err := h.uc.RotateAPIKey(c.Request.Context(), orgID, keyID)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, userdto.RotateAPIKeyResponse{
		ID:        keyID.String(),
		OrgID:     orgID.String(),
		APIKey:    raw,
		RotatedAt: time.Now().UTC().Format(time.RFC3339),
	})
}

func canReadMembers(scopes []string, role string) bool {
	return authz.IsRole(ptrRole(role), "admin", "secops") || authz.HasScope(scopes, authz.ScopeAdminConsoleRead)
}

func canReadAPIKeys(scopes []string, role string) bool {
	return authz.IsRole(ptrRole(role), "admin", "secops") || authz.HasScope(scopes, authz.ScopeAdminConsoleRead)
}

func canWriteAPIKeys(scopes []string, role string) bool {
	return authz.IsRole(ptrRole(role), "admin") || authz.HasScope(scopes, authz.ScopeAdminConsoleWrite)
}

func toMemberDTO(in userdomain.OrgMember) userdto.OrgMemberItem {
	return userdto.OrgMemberItem{
		ID:       in.ID.String(),
		OrgID:    in.OrgID.String(),
		UserID:   in.UserID.String(),
		Role:     in.Role,
		JoinedAt: toRFC3339(in.JoinedAt),
		User: userdto.UserInfo{
			ID:         in.User.ID.String(),
			ExternalID: in.User.ExternalID,
			Email:      in.User.Email,
			Name:       in.User.Name,
			AvatarURL:  in.User.AvatarURL,
			CreatedAt:  toRFC3339(in.User.CreatedAt),
			UpdatedAt:  toRFC3339(in.User.UpdatedAt),
		},
	}
}

func toAPIKeyDTO(in userdomain.APIKey) userdto.APIKeyItem {
	return userdto.APIKeyItem{
		ID:        in.ID.String(),
		OrgID:     in.OrgID.String(),
		Name:      in.Name,
		Scopes:    in.Scopes,
		CreatedAt: toRFC3339(in.CreatedAt),
	}
}

func mustOrgID(c *gin.Context) uuid.UUID {
	v, _ := c.Get(string(types.CtxKeyOrgID))
	id, _ := v.(uuid.UUID)
	return id
}

func actorFromCtx(c *gin.Context) string {
	if v, ok := c.Get(string(types.CtxKeyActor)); ok {
		if s, ok := v.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func roleFromCtx(c *gin.Context) string {
	if v, ok := c.Get(string(types.CtxKeyRole)); ok {
		if s, ok := v.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func scopesFromCtx(c *gin.Context) []string {
	if v, ok := c.Get(string(types.CtxKeyScopes)); ok {
		if scopes, ok := v.([]string); ok {
			return scopes
		}
	}
	return nil
}

func ptrRole(role string) *string {
	role = strings.TrimSpace(role)
	if role == "" {
		return nil
	}
	return &role
}

func toRFC3339(v time.Time) string {
	if v.IsZero() {
		return ""
	}
	return v.UTC().Format(time.RFC3339)
}
