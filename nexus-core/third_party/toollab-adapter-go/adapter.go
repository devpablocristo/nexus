// Package toollab provides a Toollab Adapter SDK that any Go HTTP application
// can mount to become toollab-ready.
//
// Usage with net/http:
//
//	adapter := toollab.NewAdapter(toollab.Config{
//	    AppName:    "my-app",
//	    AppVersion: "1.0.0",
//	})
//	http.Handle("/_toollab/", http.StripPrefix("/_toollab", adapter.Handler()))
package toollab

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Config configures the adapter. Only AppName and AppVersion are required.
// Optional providers enable additional capabilities.
type Config struct {
	AppName    string
	AppVersion string

	// StandardVersion defaults to "1.1".
	StandardVersion string

	// BaseURL is used to generate absolute links in manifest/profile.
	// If empty, base URL is derived from the incoming request.
	BaseURL string

	// DB enables state.fingerprint, state.snapshot, state.restore, state.reset.
	// If nil, state capabilities are not advertised.
	DB *sql.DB

	// StateProvider overrides the default DB-based state implementation.
	// If set, DB is ignored for state operations.
	StateProvider StateProvider

	// MetricsProvider enables the "metrics" capability.
	MetricsProvider MetricsProvider

	// LogsProvider enables the "logs" capability.
	LogsProvider LogsProvider

	// TracesProvider enables the "traces" capability.
	TracesProvider TracesProvider

	// SeedProvider enables the "seed" capability.
	SeedProvider SeedProvider

	// Standard v1.1 discovery providers.
	SchemaProvider         SchemaProvider
	SuggestedFlowsProvider SuggestedFlowsProvider
	InvariantsProvider     InvariantsProvider
	LimitsProvider         LimitsProvider
	EnvironmentProvider    EnvironmentProvider
	OpenAPIProvider        OpenAPIProvider

	// Standard v1.2 — rich service description for comprehension reports.
	ServiceDescriptionProvider ServiceDescriptionProvider
}

// Adapter is the toollab adapter instance.
type Adapter struct {
	cfg       Config
	mu        sync.RWMutex
	snapshots map[string]snapshotMeta
}

type snapshotMeta struct {
	ID          string
	Fingerprint string
	Label       string
	CreatedAt   time.Time
}

// NewAdapter creates a new adapter with the given configuration.
func NewAdapter(cfg Config) *Adapter {
	if cfg.StandardVersion == "" {
		cfg.StandardVersion = "1.1"
	}
	return &Adapter{
		cfg:       cfg,
		snapshots: make(map[string]snapshotMeta),
	}
}

// Handler returns an http.Handler that serves all adapter endpoints.
// Mount it at /_toollab/:
//
//	http.Handle("/_toollab/", http.StripPrefix("/_toollab", adapter.Handler()))
func (a *Adapter) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/manifest", a.handleManifest)

	if a.hasProfile() {
		mux.HandleFunc("/profile", a.handleProfile)
	}
	if a.cfg.SchemaProvider != nil {
		mux.HandleFunc("/schema", a.handleSchema)
	}
	if a.cfg.OpenAPIProvider != nil {
		mux.HandleFunc("/openapi", a.handleOpenAPI)
	}
	if a.cfg.SuggestedFlowsProvider != nil {
		mux.HandleFunc("/suggested_flows", a.handleSuggestedFlows)
	}
	if a.cfg.InvariantsProvider != nil {
		mux.HandleFunc("/invariants", a.handleInvariants)
	}
	if a.cfg.LimitsProvider != nil {
		mux.HandleFunc("/limits", a.handleLimits)
	}
	if a.cfg.EnvironmentProvider != nil {
		mux.HandleFunc("/environment", a.handleEnvironment)
	}
	if a.cfg.ServiceDescriptionProvider != nil {
		mux.HandleFunc("/description", a.handleDescription)
	}

	if a.hasState() {
		mux.HandleFunc("/state/fingerprint", a.handleStateFingerprint)
		mux.HandleFunc("/state/snapshot", a.handleStateSnapshot)
		mux.HandleFunc("/state/restore", a.handleStateRestore)
		mux.HandleFunc("/state/reset", a.handleStateReset)
	}
	if a.cfg.SeedProvider != nil {
		mux.HandleFunc("/seed", a.handleSeed)
	}
	if a.cfg.MetricsProvider != nil {
		mux.HandleFunc("/metrics", a.handleMetrics)
	}
	if a.cfg.TracesProvider != nil {
		mux.HandleFunc("/traces", a.handleTraces)
	}
	if a.cfg.LogsProvider != nil {
		mux.HandleFunc("/logs", a.handleLogs)
	}
	return mux
}

func (a *Adapter) hasState() bool {
	return a.cfg.StateProvider != nil || a.cfg.DB != nil
}

func (a *Adapter) hasProfile() bool {
	return a.cfg.SchemaProvider != nil ||
		a.cfg.SuggestedFlowsProvider != nil ||
		a.cfg.InvariantsProvider != nil ||
		a.cfg.LimitsProvider != nil ||
		a.cfg.EnvironmentProvider != nil ||
		a.cfg.OpenAPIProvider != nil ||
		a.cfg.ServiceDescriptionProvider != nil
}

func (a *Adapter) stateProvider() StateProvider {
	if a.cfg.StateProvider != nil {
		return a.cfg.StateProvider
	}
	if a.cfg.DB != nil {
		return &defaultDBState{db: a.cfg.DB}
	}
	return nil
}

func (a *Adapter) capabilities() []string {
	caps := make([]string, 0)
	if a.hasState() {
		caps = append(caps, "state.fingerprint", "state.snapshot", "state.restore", "state.reset")
	}
	if a.cfg.SeedProvider != nil {
		caps = append(caps, "seed")
	}
	if a.cfg.MetricsProvider != nil {
		caps = append(caps, "metrics")
	}
	if a.cfg.TracesProvider != nil {
		caps = append(caps, "traces")
	}
	if a.cfg.LogsProvider != nil {
		caps = append(caps, "logs")
	}
	if a.hasProfile() {
		caps = append(caps, "profile")
	}
	if a.cfg.SchemaProvider != nil {
		caps = append(caps, "schema")
	}
	if a.cfg.OpenAPIProvider != nil {
		caps = append(caps, "openapi")
	}
	if a.cfg.SuggestedFlowsProvider != nil {
		caps = append(caps, "suggested_flows")
	}
	if a.cfg.InvariantsProvider != nil {
		caps = append(caps, "invariants")
	}
	if a.cfg.LimitsProvider != nil {
		caps = append(caps, "limits")
	}
	if a.cfg.EnvironmentProvider != nil {
		caps = append(caps, "environment")
	}
	if a.cfg.ServiceDescriptionProvider != nil {
		caps = append(caps, "description")
	}
	sort.Strings(caps)
	return caps
}

func (a *Adapter) baseURL(r *http.Request) string {
	if strings.TrimSpace(a.cfg.BaseURL) != "" {
		return strings.TrimRight(strings.TrimSpace(a.cfg.BaseURL), "/")
	}
	scheme := "http"
	if r != nil && r.TLS != nil {
		scheme = "https"
	}
	if r != nil {
		if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); forwarded != "" {
			scheme = strings.ToLower(forwarded)
		}
		host := strings.TrimSpace(r.Host)
		if host == "" {
			host = "localhost"
		}
		return scheme + "://" + host
	}
	return ""
}

func (a *Adapter) manifestPayload(r *http.Request) map[string]any {
	base := a.baseURL(r)
	caps := a.capabilities()
	out := map[string]any{
		"adapter_version":  "1",
		"standard_version": a.cfg.StandardVersion,
		"app_name":         a.cfg.AppName,
		"app_version":      a.cfg.AppVersion,
		"capabilities":     caps,
	}
	if base != "" {
		links := map[string]string{}
		if a.cfg.OpenAPIProvider != nil {
			links["openapi_url"] = base + "/openapi.yaml"
		}
		if a.cfg.SchemaProvider != nil {
			links["schema_url"] = base + "/_toollab/schema"
		}
		if a.hasProfile() {
			links["profile_url"] = base + "/_toollab/profile"
		}
		if a.cfg.SuggestedFlowsProvider != nil {
			links["suggested_flows_url"] = base + "/_toollab/suggested_flows"
		}
		if a.cfg.InvariantsProvider != nil {
			links["invariants_url"] = base + "/_toollab/invariants"
		}
		if a.cfg.LimitsProvider != nil {
			links["limits_url"] = base + "/_toollab/limits"
		}
		if a.cfg.EnvironmentProvider != nil {
			links["environment_url"] = base + "/_toollab/environment"
		}
		if a.cfg.ServiceDescriptionProvider != nil {
			links["description_url"] = base + "/_toollab/description"
		}
		if len(links) > 0 {
			out["links"] = links
		}
	}
	return out
}

// --- Handlers ---

func (a *Adapter) handleManifest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
		return
	}
	writeJSON(w, http.StatusOK, a.manifestPayload(r))
}

func (a *Adapter) handleProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
		return
	}
	if !a.hasProfile() {
		writeError(w, http.StatusNotImplemented, "not_supported", "profile capability not supported")
		return
	}

	profile := map[string]any{
		"standard_version": a.cfg.StandardVersion,
		"profile_version":  "1",
		"manifest":         a.manifestPayload(r),
	}

	unknowns := make([]string, 0)
	hashes := map[string]string{}

	if a.cfg.SchemaProvider != nil {
		value, err := a.cfg.SchemaProvider.Schema(r.Context())
		if err != nil {
			unknowns = append(unknowns, "schema unavailable")
		} else {
			profile["schema"] = value
			if h := hashAny(value); h != "" {
				hashes["schema_sha256"] = h
			}
		}
	}
	if a.cfg.SuggestedFlowsProvider != nil {
		value, err := a.cfg.SuggestedFlowsProvider.SuggestedFlows(r.Context())
		if err != nil {
			unknowns = append(unknowns, "suggested_flows unavailable")
		} else {
			profile["suggested_flows"] = value
			if h := hashAny(value); h != "" {
				hashes["suggested_flows_sha256"] = h
			}
		}
	}
	if a.cfg.InvariantsProvider != nil {
		value, err := a.cfg.InvariantsProvider.Invariants(r.Context())
		if err != nil {
			unknowns = append(unknowns, "invariants unavailable")
		} else {
			profile["invariants"] = value
			if h := hashAny(value); h != "" {
				hashes["invariants_sha256"] = h
			}
		}
	}
	if a.cfg.LimitsProvider != nil {
		value, err := a.cfg.LimitsProvider.Limits(r.Context())
		if err != nil {
			unknowns = append(unknowns, "limits unavailable")
		} else {
			profile["limits"] = value
			if h := hashAny(value); h != "" {
				hashes["limits_sha256"] = h
			}
		}
	}
	if a.cfg.EnvironmentProvider != nil {
		value, err := a.cfg.EnvironmentProvider.Environment(r.Context())
		if err != nil {
			unknowns = append(unknowns, "environment unavailable")
		} else {
			profile["environment"] = value
			if h := hashAny(value); h != "" {
				hashes["environment_sha256"] = h
			}
		}
	}
	if a.cfg.ServiceDescriptionProvider != nil {
		value, err := a.cfg.ServiceDescriptionProvider.ServiceDescription(r.Context())
		if err != nil {
			unknowns = append(unknowns, "description unavailable")
		} else {
			profile["description"] = value
			if h := hashAny(value); h != "" {
				hashes["description_sha256"] = h
			}
		}
	}
	if a.cfg.OpenAPIProvider != nil {
		info, err := a.cfg.OpenAPIProvider.OpenAPIInfo(r.Context())
		if err != nil {
			unknowns = append(unknowns, "openapi metadata unavailable")
		} else if info != nil {
			if info.URL == "" {
				info.URL = a.baseURL(r) + "/openapi.yaml"
			}
			profile["openapi"] = info
			if info.SHA256 != "" {
				hashes["openapi_sha256"] = info.SHA256
			}
		}
	}

	if len(hashes) > 0 {
		profile["hashes"] = hashes
	}
	if len(unknowns) > 0 {
		sort.Strings(unknowns)
		profile["unknowns"] = unknowns
	}
	writeJSON(w, http.StatusOK, profile)
}

func (a *Adapter) handleSchema(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
		return
	}
	if a.cfg.SchemaProvider == nil {
		writeError(w, http.StatusNotImplemented, "not_supported", "schema capability not supported")
		return
	}
	value, err := a.cfg.SchemaProvider.Schema(r.Context())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "schema_not_available", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (a *Adapter) handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
		return
	}
	if a.cfg.OpenAPIProvider == nil {
		writeError(w, http.StatusNotImplemented, "not_supported", "openapi capability not supported")
		return
	}
	contentType, raw, err := a.cfg.OpenAPIProvider.OpenAPIDocument(r.Context())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "openapi_not_available", err.Error())
		return
	}
	if contentType == "" {
		contentType = "application/yaml"
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}

func (a *Adapter) handleSuggestedFlows(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
		return
	}
	if a.cfg.SuggestedFlowsProvider == nil {
		writeError(w, http.StatusNotImplemented, "not_supported", "suggested_flows capability not supported")
		return
	}
	value, err := a.cfg.SuggestedFlowsProvider.SuggestedFlows(r.Context())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "suggested_flows_not_available", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (a *Adapter) handleInvariants(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
		return
	}
	if a.cfg.InvariantsProvider == nil {
		writeError(w, http.StatusNotImplemented, "not_supported", "invariants capability not supported")
		return
	}
	value, err := a.cfg.InvariantsProvider.Invariants(r.Context())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "invariants_not_available", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (a *Adapter) handleLimits(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
		return
	}
	if a.cfg.LimitsProvider == nil {
		writeError(w, http.StatusNotImplemented, "not_supported", "limits capability not supported")
		return
	}
	value, err := a.cfg.LimitsProvider.Limits(r.Context())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "limits_not_available", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (a *Adapter) handleEnvironment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
		return
	}
	if a.cfg.EnvironmentProvider == nil {
		writeError(w, http.StatusNotImplemented, "not_supported", "environment capability not supported")
		return
	}
	value, err := a.cfg.EnvironmentProvider.Environment(r.Context())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "environment_not_available", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (a *Adapter) handleDescription(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
		return
	}
	if a.cfg.ServiceDescriptionProvider == nil {
		writeError(w, http.StatusNotImplemented, "not_supported", "description capability not supported")
		return
	}
	value, err := a.cfg.ServiceDescriptionProvider.ServiceDescription(r.Context())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "description_not_available", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (a *Adapter) handleStateFingerprint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
		return
	}
	sp := a.stateProvider()
	fp, err := sp.Fingerprint(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"fingerprint": fp,
		"scope":       "full",
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
	})
}

func (a *Adapter) handleStateSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use POST")
		return
	}
	var body struct {
		Label string `json:"label"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	sp := a.stateProvider()
	id, fp, err := sp.Snapshot(r.Context(), body.Label)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	now := time.Now().UTC()
	a.mu.Lock()
	a.snapshots[id] = snapshotMeta{ID: id, Fingerprint: fp, Label: body.Label, CreatedAt: now}
	a.mu.Unlock()

	writeJSON(w, http.StatusCreated, map[string]any{
		"snapshot_id": id,
		"fingerprint": fp,
		"label":       body.Label,
		"created_at":  now.Format(time.RFC3339),
	})
}

func (a *Adapter) handleStateRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use POST")
		return
	}
	var body struct {
		SnapshotID string `json:"snapshot_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.SnapshotID == "" {
		writeError(w, http.StatusBadRequest, "seed_invalid", "snapshot_id is required")
		return
	}

	a.mu.RLock()
	meta, ok := a.snapshots[body.SnapshotID]
	a.mu.RUnlock()
	if !ok {
		writeError(w, http.StatusNotFound, "snapshot_not_found", "snapshot "+body.SnapshotID+" does not exist")
		return
	}

	sp := a.stateProvider()
	if err := sp.Restore(r.Context(), body.SnapshotID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"restored":    true,
		"snapshot_id": body.SnapshotID,
		"fingerprint": meta.Fingerprint,
	})
}

func (a *Adapter) handleStateReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use POST")
		return
	}
	sp := a.stateProvider()
	if err := sp.Reset(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	fp, _ := sp.Fingerprint(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"reset":       true,
		"fingerprint": fp,
	})
}

func (a *Adapter) handleSeed(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var body struct {
			RunSeed string   `json:"run_seed"`
			Scope   []string `json:"scope"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.RunSeed == "" {
			writeError(w, http.StatusBadRequest, "seed_invalid", "run_seed is required")
			return
		}
		result, err := a.cfg.SeedProvider.Apply(r.Context(), body.RunSeed, body.Scope)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"applied":       true,
			"run_seed":      body.RunSeed,
			"scope_applied": result.Applied,
			"scope_ignored": result.Ignored,
		})
	case http.MethodDelete:
		if err := a.cfg.SeedProvider.Clear(r.Context()); err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"cleared": true})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use POST or DELETE")
	}
}

func (a *Adapter) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
		return
	}
	metrics, err := a.cfg.MetricsProvider.Snapshot(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"collected_at": time.Now().UTC().Format(time.RFC3339),
		"metrics":      metrics,
	})
}

func (a *Adapter) handleTraces(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
		return
	}
	since := parseSince(r.URL.Query().Get("since"))
	limit := parseInt(r.URL.Query().Get("limit"), 100)

	traces, err := a.cfg.TracesProvider.Collect(r.Context(), since, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"collected_at": time.Now().UTC().Format(time.RFC3339),
		"traces":       traces,
	})
}

func (a *Adapter) handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
		return
	}
	since := parseSince(r.URL.Query().Get("since"))
	limit := parseInt(r.URL.Query().Get("limit"), 500)
	level := r.URL.Query().Get("level")

	lines, err := a.cfg.LogsProvider.Collect(r.Context(), since, limit, level)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"collected_at": time.Now().UTC().Format(time.RFC3339),
		"lines":        lines,
	})
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": code, "message": message})
}

func parseSince(s string) time.Time {
	if s == "" {
		return time.Now().UTC().Add(-5 * time.Minute)
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Now().UTC().Add(-5 * time.Minute)
	}
	return t
}

func parseInt(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

func hashAny(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
