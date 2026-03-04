package identity

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	identityoidc "nexus-core/internal/identity/executor/oidc"
	httperr "nexus/pkg/http/errors"
)

// OIDCConfig holds the configuration for the OIDC SSO feature.
type OIDCConfig struct {
	Enabled      bool
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
}

// OIDCHandler exposes OIDC/SSO login endpoints. It is only registered when
// OIDC is enabled. All state is kept in-memory; for production deployments
// with multiple replicas a shared store (Redis, DB) should replace the
// in-memory pendingFlows map.
type OIDCHandler struct {
	cfg       OIDCConfig
	discovery *identityoidc.DiscoveryClient
	exchanger *identityoidc.TokenExchanger
	idSvc     *Usecases

	mu           sync.Mutex
	pendingFlows map[string]*oidcFlowState
}

type oidcFlowState struct {
	CodeVerifier string
	Nonce        string
	CreatedAt    time.Time
}

// NewOIDCHandler creates a new handler for OIDC auth endpoints.
func NewOIDCHandler(cfg OIDCConfig, discovery *identityoidc.DiscoveryClient, exchanger *identityoidc.TokenExchanger, idSvc *Usecases) *OIDCHandler {
	h := &OIDCHandler{
		cfg:          cfg,
		discovery:    discovery,
		exchanger:    exchanger,
		idSvc:        idSvc,
		pendingFlows: make(map[string]*oidcFlowState),
	}
	// Background goroutine to clean up stale flows.
	go h.cleanupLoop()
	return h
}

// Register mounts the OIDC endpoints on the provided router group.
// These routes are public (no auth middleware) because they are the
// entry point for authentication itself.
func (h *OIDCHandler) Register(rg *gin.RouterGroup) {
	rg.GET("/auth/oidc/config", h.configStatus)
	if !h.cfg.Enabled {
		return
	}
	rg.GET("/auth/oidc/authorize", h.authorize)
	rg.GET("/auth/oidc/callback", h.callback)
}

// configStatus returns whether OIDC is enabled and the provider issuer URL.
func (h *OIDCHandler) configStatus(c *gin.Context) {
	resp := gin.H{
		"oidc_enabled": h.cfg.Enabled,
	}
	if h.cfg.Enabled {
		resp["issuer_url"] = h.cfg.IssuerURL
		resp["scopes"] = h.cfg.Scopes
	}
	c.JSON(http.StatusOK, resp)
}

// authorize initiates the OIDC authorization code + PKCE flow.
// It generates a state, nonce, and PKCE code verifier/challenge, stores
// them server-side, and redirects the user to the OIDC provider.
func (h *OIDCHandler) authorize(c *gin.Context) {
	pkce, err := identityoidc.GeneratePKCE()
	if err != nil {
		httperr.Write(c, http.StatusInternalServerError, "INTERNAL", "failed to generate PKCE parameters")
		return
	}
	state, err := identityoidc.GenerateState()
	if err != nil {
		httperr.Write(c, http.StatusInternalServerError, "INTERNAL", "failed to generate state")
		return
	}
	nonce, err := identityoidc.GenerateNonce()
	if err != nil {
		httperr.Write(c, http.StatusInternalServerError, "INTERNAL", "failed to generate nonce")
		return
	}

	authURL, err := h.exchanger.AuthorizationURL(c.Request.Context(), state, nonce, pkce, h.cfg.Scopes)
	if err != nil {
		httperr.Write(c, http.StatusBadGateway, "INTERNAL", "failed to build authorization URL")
		return
	}

	h.mu.Lock()
	h.pendingFlows[state] = &oidcFlowState{
		CodeVerifier: pkce.CodeVerifier,
		Nonce:        nonce,
		CreatedAt:    time.Now(),
	}
	h.mu.Unlock()

	c.Redirect(http.StatusFound, authURL)
}

// callback handles the OIDC provider callback. It validates state, exchanges
// the authorization code for tokens, verifies the ID token via the JWKS
// verifier, resolves the principal, and returns the session information.
func (h *OIDCHandler) callback(c *gin.Context) {
	// Check for errors from the OIDC provider.
	if errCode := c.Query("error"); errCode != "" {
		errDesc := c.DefaultQuery("error_description", "unknown error")
		httperr.Write(c, http.StatusBadRequest, "OIDC_ERROR", errCode+": "+errDesc)
		return
	}

	state := c.Query("state")
	code := c.Query("code")
	if state == "" || code == "" {
		httperr.Write(c, http.StatusBadRequest, "VALIDATION_ERROR", "missing state or code parameter")
		return
	}

	// Look up and consume the pending flow.
	h.mu.Lock()
	flow, ok := h.pendingFlows[state]
	if ok {
		delete(h.pendingFlows, state)
	}
	h.mu.Unlock()

	if !ok {
		httperr.Write(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid or expired state parameter")
		return
	}

	// Reject flows older than 10 minutes.
	if time.Since(flow.CreatedAt) > 10*time.Minute {
		httperr.Write(c, http.StatusBadRequest, "VALIDATION_ERROR", "authorization flow expired")
		return
	}

	// Exchange the authorization code for tokens.
	tokenResp, err := h.exchanger.ExchangeCode(c.Request.Context(), code, flow.CodeVerifier)
	if err != nil {
		httperr.Write(c, http.StatusBadGateway, "OIDC_ERROR", "token exchange failed: "+err.Error())
		return
	}

	// Verify the ID token via the discovered JWKS.
	claims, err := h.discovery.VerifyToken(c.Request.Context(), tokenResp.IDToken)
	if err != nil {
		httperr.Write(c, http.StatusUnauthorized, "UNAUTHORIZED", "id token verification failed: "+err.Error())
		return
	}

	// Validate the nonce claim matches what we sent.
	if nonceClaim, _ := claims["nonce"].(string); nonceClaim != flow.Nonce {
		httperr.Write(c, http.StatusUnauthorized, "UNAUTHORIZED", "id token nonce mismatch")
		return
	}

	// Resolve the principal using the same claim mapping as JWT auth.
	principal, err := h.idSvc.ResolvePrincipal(c.Request.Context(), tokenResp.IDToken)
	if err != nil {
		// If the principal cannot be resolved (e.g. missing org_id claim),
		// return the raw claims so the caller can debug the mapping.
		c.JSON(http.StatusOK, gin.H{
			"auth_method":  "oidc",
			"id_token":     tokenResp.IDToken,
			"access_token": tokenResp.AccessToken,
			"expires_in":   tokenResp.ExpiresIn,
			"claims":       claims,
			"warning":      "principal resolution failed: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"auth_method":  "oidc",
		"id_token":     tokenResp.IDToken,
		"access_token": tokenResp.AccessToken,
		"expires_in":   tokenResp.ExpiresIn,
		"org_id":       principal.OrgID.String(),
		"actor":        principal.Actor,
		"role":         principal.Role,
		"scopes":       principal.Scopes,
	})
}

// cleanupLoop periodically removes stale pending flows to prevent memory leaks.
func (h *OIDCHandler) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		h.mu.Lock()
		now := time.Now()
		for k, v := range h.pendingFlows {
			if now.Sub(v.CreatedAt) > 15*time.Minute {
				delete(h.pendingFlows, k)
			}
		}
		h.mu.Unlock()
	}
}

