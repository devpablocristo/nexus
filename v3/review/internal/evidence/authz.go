package evidence

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/devpablocristo/core/http/go/httpjson"
	evidencedomain "github.com/devpablocristo/nexus/v3/review/internal/evidence/usecases/domain"
)

const (
	scopeNexusRequestsRead = "nexus:requests:read"
	scopeNexusCrossOrg     = "nexus:cross_org"
)

func requireScope(w http.ResponseWriter, r *http.Request, scopes ...string) bool {
	if requestHasNoAuthContext(r) || requestHasScope(r, scopes...) {
		return true
	}
	httpjson.WriteFlatError(w, http.StatusForbidden, "FORBIDDEN", "missing required scope")
	return false
}

func canAccessEvidenceOrg(r *http.Request, pack evidencedomain.EvidencePack) bool {
	if requestHasNoAuthContext(r) || requestHasScope(r, scopeNexusCrossOrg) {
		return true
	}
	orgID := strings.TrimSpace(r.Header.Get("X-Org-ID"))
	packOrg := strings.TrimSpace(rawToString(pack.Request.Params["org_id"]))
	if orgID == "" {
		return packOrg == ""
	}
	return packOrg == "" || packOrg == orgID
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

func rawToString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}
