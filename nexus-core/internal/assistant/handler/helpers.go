package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"nexus/pkg/types"
)

// MustOrgID extrae org_id del contexto (inyectado por auth middleware).
func MustOrgID(c *gin.Context) uuid.UUID {
	v, _ := c.Get(string(types.CtxKeyOrgID))
	id, _ := v.(uuid.UUID)
	return id
}

// ActorFromCtx extrae el actor del contexto.
func ActorFromCtx(c *gin.Context) *string {
	if v, ok := c.Get(string(types.CtxKeyActor)); ok {
		if s, ok := v.(string); ok && s != "" {
			return &s
		}
	}
	return nil
}

// RoleFromCtx extrae el role del contexto.
func RoleFromCtx(c *gin.Context) *string {
	if v, ok := c.Get(string(types.CtxKeyRole)); ok {
		if s, ok := v.(string); ok && s != "" {
			return &s
		}
	}
	return nil
}

// ScopesFromCtx extrae los scopes del contexto.
func ScopesFromCtx(c *gin.Context) []string {
	if v, ok := c.Get(string(types.CtxKeyScopes)); ok {
		if scopes, ok := v.([]string); ok {
			return scopes
		}
	}
	return nil
}
