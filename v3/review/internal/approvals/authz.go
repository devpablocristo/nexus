package approvals

import (
	"net/http"
	"strings"

	"github.com/devpablocristo/core/http/go/httpjson"
	approvaldomain "github.com/devpablocristo/nexus/v3/review/internal/approvals/usecases/domain"
)

const (
	scopeNexusApprovalsDecide = "nexus:approvals:decide"
	scopeNexusCrossOrg        = "nexus:cross_org"
)

func requireScope(w http.ResponseWriter, r *http.Request, scopes ...string) bool {
	if requestHasNoAuthContext(r) || requestHasScope(r, scopes...) {
		return true
	}
	httpjson.WriteFlatError(w, http.StatusForbidden, "FORBIDDEN", "missing required scope")
	return false
}

func canAccessApprovalOrg(r *http.Request, approval approvaldomain.Approval) bool {
	if requestHasNoAuthContext(r) || requestHasScope(r, scopeNexusCrossOrg) {
		return true
	}
	orgID := strings.TrimSpace(r.Header.Get("X-Org-ID"))
	if orgID == "" {
		return approval.OrgID == nil
	}
	if approval.OrgID == nil {
		return false
	}
	return strings.TrimSpace(*approval.OrgID) == orgID
}

func requestHasNoAuthContext(r *http.Request) bool {
	return strings.TrimSpace(r.Header.Get("X-Auth-Method")) == "" &&
		strings.TrimSpace(r.Header.Get("X-Auth-Scopes")) == ""
}

func requestHasScope(r *http.Request, scopes ...string) bool {
	have := parseHeaderScopes(r.Header.Get("X-Auth-Scopes"))
	for _, scope := range scopes {
		if _, ok := have[scope]; ok {
			return true
		}
	}
	return false
}

func parseHeaderScopes(raw string) map[string]struct{} {
	raw = strings.NewReplacer(",", " ", ";", " ", "+", " ").Replace(raw)
	fields := strings.Fields(raw)
	out := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		if scope := strings.TrimSpace(field); scope != "" {
			out[scope] = struct{}{}
		}
	}
	return out
}
