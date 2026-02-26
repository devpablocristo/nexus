package toolab

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"gopkg.in/yaml.v3"

	domain "nexus-core/internal/toolab/usecases/domain"
)

// Service defines the business operations for the toolab adapter.
type Service interface {
	Manifest(baseURL string) domain.Manifest
	Fingerprint(ctx context.Context) (string, error)
	Snapshot(ctx context.Context, label string) (*domain.SnapshotMeta, error)
	Restore(ctx context.Context, snapshotID string) (*domain.SnapshotMeta, error)
	Reset(ctx context.Context) (string, error)
	Metrics(ctx context.Context) ([]domain.MetricItem, error)
	Schema(ctx context.Context) (*domain.SchemaResponse, error)
	SuggestedFlows(ctx context.Context) (*domain.SuggestedFlowsResponse, error)
	Invariants() *domain.InvariantsResponse
	Limits() *domain.LimitsResponse
	Environment() *domain.EnvironmentResponse
	Profile(ctx context.Context, baseURL string) (*domain.ProfileResponse, error)
	OpenAPIDocument(ctx context.Context) ([]byte, error)
	OpenAPIInfo(ctx context.Context, baseURL string) (*domain.OpenAPIInfo, error)
}

// Config holds adapter configuration provided at startup.
type Config struct {
	AppVersion string

	Environment string
	ReadOnly    bool
	GitSHA      string

	OpenAPIPath string

	DefaultRateRPS       float64
	DefaultRateBurst     int
	DefaultTimeoutMS     int
	MaxTimeoutMS         int
	MaxInflight          int
	MaxQueue             int
	MaxRequestBodyBytes  int64
	MaxResponseBodyBytes int64
	MaxLogsLines         int
	MaxTracesSpans       int
}

type service struct {
	repo RepositoryPort
	cfg  Config

	mu        sync.RWMutex
	snapshots map[string]domain.SnapshotMeta
}

// NewService creates the toolab adapter service.
func NewService(repo RepositoryPort, cfg Config) Service {
	if cfg.AppVersion == "" {
		cfg.AppVersion = "1.0.0"
	}
	if cfg.Environment == "" {
		cfg.Environment = envOrDefault("NEXUS_ENV", "dev")
	}
	if cfg.OpenAPIPath == "" {
		cfg.OpenAPIPath = "docs/openapi.yaml"
	}
	if cfg.DefaultRateRPS <= 0 {
		cfg.DefaultRateRPS = 20
	}
	if cfg.DefaultRateBurst <= 0 {
		cfg.DefaultRateBurst = 40
	}
	if cfg.DefaultTimeoutMS <= 0 {
		cfg.DefaultTimeoutMS = 5000
	}
	if cfg.MaxTimeoutMS <= 0 {
		cfg.MaxTimeoutMS = 30000
	}
	if cfg.MaxInflight <= 0 {
		cfg.MaxInflight = 100
	}
	if cfg.MaxQueue <= 0 {
		cfg.MaxQueue = 1000
	}
	if cfg.MaxRequestBodyBytes <= 0 {
		cfg.MaxRequestBodyBytes = 262144
	}
	if cfg.MaxResponseBodyBytes <= 0 {
		cfg.MaxResponseBodyBytes = 1048576
	}
	if cfg.MaxLogsLines <= 0 {
		cfg.MaxLogsLines = 500
	}
	if cfg.MaxTracesSpans <= 0 {
		cfg.MaxTracesSpans = 5000
	}

	return &service{
		repo:      repo,
		cfg:       cfg,
		snapshots: make(map[string]domain.SnapshotMeta),
	}
}

func (s *service) Manifest(baseURL string) domain.Manifest {
	baseURL = strings.TrimRight(baseURL, "/")
	links := map[string]string{}
	if baseURL != "" {
		links = map[string]string{
			"openapi_url":         baseURL + "/openapi.yaml",
			"schema_url":          baseURL + "/_toolab/schema",
			"profile_url":         baseURL + "/_toolab/profile",
			"suggested_flows_url": baseURL + "/_toolab/suggested_flows",
			"invariants_url":      baseURL + "/_toolab/invariants",
			"limits_url":          baseURL + "/_toolab/limits",
			"environment_url":     baseURL + "/_toolab/environment",
		}
	}
	return domain.Manifest{
		AdapterVersion:  "1",
		StandardVersion: "1.1",
		AppName:         "nexus",
		AppVersion:      s.cfg.AppVersion,
		Capabilities:    append([]string(nil), s.capabilities()...),
		Links:           links,
	}
}

func (s *service) capabilities() []string {
	return []string{
		"state.fingerprint",
		"state.snapshot",
		"state.restore",
		"state.reset",
		"metrics",
		"profile",
		"schema",
		"openapi",
		"suggested_flows",
		"invariants",
		"limits",
		"environment",
	}
}

func (s *service) Fingerprint(ctx context.Context) (string, error) {
	return s.repo.Fingerprint(ctx)
}

func (s *service) Snapshot(ctx context.Context, label string) (*domain.SnapshotMeta, error) {
	fp, err := s.repo.Fingerprint(ctx)
	if err != nil {
		return nil, err
	}

	id := fmt.Sprintf("snap_%s", time.Now().UTC().Format("20060102_150405"))
	if err := s.repo.CreateSavepoint(ctx, id); err != nil {
		return nil, err
	}

	meta := domain.SnapshotMeta{
		ID:          id,
		Fingerprint: fp,
		Label:       label,
		CreatedAt:   time.Now().UTC(),
	}

	s.mu.Lock()
	s.snapshots[id] = meta
	s.mu.Unlock()

	return &meta, nil
}

func (s *service) Restore(ctx context.Context, snapshotID string) (*domain.SnapshotMeta, error) {
	s.mu.RLock()
	meta, ok := s.snapshots[snapshotID]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("snapshot %s not found", snapshotID)
	}

	if err := s.repo.RollbackToSavepoint(ctx, snapshotID); err != nil {
		return nil, err
	}
	return &meta, nil
}

func (s *service) Reset(ctx context.Context) (string, error) {
	if err := s.repo.TruncateAll(ctx); err != nil {
		return "", err
	}
	fp, _ := s.repo.Fingerprint(ctx)
	return fp, nil
}

func (s *service) Metrics(_ context.Context) ([]domain.MetricItem, error) {
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		return nil, err
	}

	var items []domain.MetricItem
	for _, mf := range families {
		for _, m := range mf.GetMetric() {
			labels := make(map[string]string, len(m.GetLabel()))
			for _, lp := range m.GetLabel() {
				labels[lp.GetName()] = lp.GetValue()
			}

			item := domain.MetricItem{
				Name:   mf.GetName(),
				Type:   metricTypeName(mf.GetType()),
				Labels: labels,
			}

			switch mf.GetType() {
			case dto.MetricType_COUNTER:
				item.Value = m.GetCounter().GetValue()
			case dto.MetricType_GAUGE:
				item.Value = m.GetGauge().GetValue()
			case dto.MetricType_HISTOGRAM:
				h := m.GetHistogram()
				buckets := make([]map[string]any, 0, len(h.GetBucket()))
				for _, b := range h.GetBucket() {
					buckets = append(buckets, map[string]any{
						"upper_bound":      b.GetUpperBound(),
						"cumulative_count": b.GetCumulativeCount(),
					})
				}
				item.Value = map[string]any{
					"count":   h.GetSampleCount(),
					"sum":     h.GetSampleSum(),
					"buckets": buckets,
				}
			case dto.MetricType_SUMMARY:
				sm := m.GetSummary()
				quantiles := make([]map[string]any, 0, len(sm.GetQuantile()))
				for _, q := range sm.GetQuantile() {
					quantiles = append(quantiles, map[string]any{
						"quantile": q.GetQuantile(),
						"value":    q.GetValue(),
					})
				}
				item.Value = map[string]any{
					"count":     sm.GetSampleCount(),
					"sum":       sm.GetSampleSum(),
					"quantiles": quantiles,
				}
			default:
				item.Value = m.GetUntyped().GetValue()
			}

			items = append(items, item)
		}
	}
	return items, nil
}

func (s *service) Schema(ctx context.Context) (*domain.SchemaResponse, error) {
	return s.repo.Schema(ctx)
}

func (s *service) SuggestedFlows(ctx context.Context) (*domain.SuggestedFlowsResponse, error) {
	doc, err := s.loadOpenAPI(ctx)
	if err != nil {
		return nil, err
	}

	methodOrder := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	paths := make([]string, 0, len(doc.Paths))
	for path := range doc.Paths {
		if strings.HasPrefix(path, "/_toolab/") {
			continue
		}
		paths = append(paths, path)
	}
	sort.Strings(paths)

	flows := make([]domain.SuggestedFlow, 0, len(paths))
	seenIDs := map[string]bool{}
	for _, path := range paths {
		opMap := doc.Paths[path]
		for _, method := range methodOrder {
			op, ok := opMap[strings.ToLower(method)]
			if !ok {
				continue
			}
			flowID := sanitizeFlowID(op.OperationID)
			if flowID == "" {
				flowID = sanitizeFlowID(method + "_" + strings.ReplaceAll(strings.Trim(path, "/"), "/", "_"))
			}
			if flowID == "" {
				flowID = "flow"
			}
			if seenIDs[flowID] {
				flowID = fmt.Sprintf("%s_%s", flowID, shortHash(method+":"+path))
			}
			seenIDs[flowID] = true

			req := domain.SuggestedFlowRequest{
				Method:    method,
				Path:      path,
				TimeoutMS: 5000,
				Weight:    1,
			}
			if method == "POST" || method == "PUT" || method == "PATCH" {
				req.JSONBody = map[string]any{"placeholder": "{{fill_me}}"}
			} else {
				empty := ""
				req.Body = &empty
			}
			flows = append(flows, domain.SuggestedFlow{
				ID:          flowID,
				Description: strings.TrimSpace(op.Summary),
				Weight:      1,
				Requests:    []domain.SuggestedFlowRequest{req},
			})
		}
	}

	sort.SliceStable(flows, func(i, j int) bool {
		return flows[i].ID < flows[j].ID
	})
	return &domain.SuggestedFlowsResponse{
		Flows: flows,
		DefaultHeaders: map[string]string{
			"X-NEXUS-GATEWAY-KEY": "{{NEXUS_GATEWAY_KEY}}",
		},
	}, nil
}

func (s *service) Invariants() *domain.InvariantsResponse {
	max4xx := 0.10
	max429 := 0.05
	status429 := 429
	return &domain.InvariantsResponse{
		Invariants: []domain.Invariant{
			{ID: "inv_no_5xx", Type: "no_5xx_allowed", Description: "No server errors should occur."},
			{ID: "inv_max_4xx", Type: "max_4xx_rate", Description: "Client errors must stay below threshold.", Max: &max4xx},
			{ID: "inv_status_429", Type: "status_code_rate", Description: "Rate-limit responses must stay bounded.", Status: &status429, Max: &max429},
			{ID: "inv_idempotency", Type: "idempotent_key_identical_response", Description: "Idempotent requests should be stable."},
		},
	}
}

func (s *service) Limits() *domain.LimitsResponse {
	return &domain.LimitsResponse{
		Rate: &domain.RateLimits{
			RequestsPerSecond: s.cfg.DefaultRateRPS,
			Burst:             s.cfg.DefaultRateBurst,
			WindowSeconds:     1,
		},
		Timeouts: &domain.TimeoutLimits{
			RequestDefaultMS: s.cfg.DefaultTimeoutMS,
			RequestMaxMS:     s.cfg.MaxTimeoutMS,
		},
		Concurrency: &domain.ConcurrencyLimits{
			MaxInflight: s.cfg.MaxInflight,
			MaxQueue:    s.cfg.MaxQueue,
		},
		Payload: &domain.PayloadLimits{
			MaxRequestBodyBytes:  s.cfg.MaxRequestBodyBytes,
			MaxResponseBodyBytes: s.cfg.MaxResponseBodyBytes,
			MaxLogsLines:         s.cfg.MaxLogsLines,
			MaxTracesSpans:       s.cfg.MaxTracesSpans,
		},
	}
}

func (s *service) Environment() *domain.EnvironmentResponse {
	return &domain.EnvironmentResponse{
		Mode:     s.cfg.Environment,
		ReadOnly: s.cfg.ReadOnly,
		Features: map[string]bool{
			"toolab_standard_v1_1": true,
			"profile":              true,
			"schema":               true,
			"openapi":              true,
			"suggested_flows":      true,
			"invariants":           true,
			"limits":               true,
			"environment":          true,
			"state_snapshot":       true,
			"metrics":              true,
		},
		Metadata: map[string]any{
			"service": "nexus",
		},
		Release: &domain.ReleaseInfo{
			Version: s.cfg.AppVersion,
			GitSHA:  s.cfg.GitSHA,
		},
	}
}

func (s *service) Profile(ctx context.Context, baseURL string) (*domain.ProfileResponse, error) {
	profile := &domain.ProfileResponse{
		StandardVersion: "1.1",
		ProfileVersion:  "1",
		Manifest:        ptrManifest(s.Manifest(baseURL)),
		Unknowns:        []string{},
		Hashes:          map[string]string{},
	}

	schema, err := s.Schema(ctx)
	if err == nil {
		profile.Schema = schema
		profile.Hashes["schema_sha256"] = hashObject(schema)
	} else {
		profile.Unknowns = append(profile.Unknowns, "schema unavailable")
	}

	flows, err := s.SuggestedFlows(ctx)
	if err == nil {
		profile.SuggestedFlows = flows
		profile.Hashes["suggested_flows_sha256"] = hashObject(flows)
	} else {
		profile.Unknowns = append(profile.Unknowns, "suggested_flows unavailable")
	}

	invariants := s.Invariants()
	profile.Invariants = invariants
	profile.Hashes["invariants_sha256"] = hashObject(invariants)

	limits := s.Limits()
	profile.Limits = limits
	profile.Hashes["limits_sha256"] = hashObject(limits)

	env := s.Environment()
	profile.Environment = env
	profile.Hashes["environment_sha256"] = hashObject(env)

	openapiInfo, openapiErr := s.OpenAPIInfo(ctx, baseURL)
	if openapiErr == nil {
		profile.OpenAPI = openapiInfo
		profile.Hashes["openapi_sha256"] = openapiInfo.SHA256
	} else {
		profile.Unknowns = append(profile.Unknowns, "openapi metadata unavailable")
	}

	sort.Strings(profile.Unknowns)
	if len(profile.Unknowns) == 0 {
		profile.Unknowns = nil
	}
	if len(profile.Hashes) == 0 {
		profile.Hashes = nil
	}
	return profile, nil
}

func (s *service) OpenAPIDocument(_ context.Context) ([]byte, error) {
	raw, err := os.ReadFile(s.cfg.OpenAPIPath)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func (s *service) OpenAPIInfo(ctx context.Context, baseURL string) (*domain.OpenAPIInfo, error) {
	raw, err := s.OpenAPIDocument(ctx)
	if err != nil {
		return nil, err
	}
	doc, err := parseOpenAPIDoc(raw)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)
	sha := hex.EncodeToString(sum[:])
	etag := fmt.Sprintf("\"%s\"", sha[:16])
	return &domain.OpenAPIInfo{
		URL:         strings.TrimRight(baseURL, "/") + "/openapi.yaml",
		ContentType: "application/yaml",
		Version:     doc.OpenAPI,
		ETag:        etag,
		SHA256:      sha,
	}, nil
}

func metricTypeName(t dto.MetricType) string {
	switch t {
	case dto.MetricType_COUNTER:
		return "counter"
	case dto.MetricType_GAUGE:
		return "gauge"
	case dto.MetricType_HISTOGRAM:
		return "histogram"
	case dto.MetricType_SUMMARY:
		return "summary"
	default:
		return "untyped"
	}
}

type openAPIDoc struct {
	OpenAPI string                          `yaml:"openapi"`
	Paths   map[string]map[string]openAPIOp `yaml:"paths"`
}

type openAPIOp struct {
	OperationID string `yaml:"operationId"`
	Summary     string `yaml:"summary"`
}

func (s *service) loadOpenAPI(_ context.Context) (*openAPIDoc, error) {
	raw, err := os.ReadFile(s.cfg.OpenAPIPath)
	if err != nil {
		return nil, err
	}
	return parseOpenAPIDoc(raw)
}

func parseOpenAPIDoc(raw []byte) (*openAPIDoc, error) {
	var doc openAPIDoc
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	if doc.OpenAPI == "" {
		return nil, fmt.Errorf("openapi version missing")
	}
	if doc.Paths == nil {
		doc.Paths = map[string]map[string]openAPIOp{}
	}
	return &doc, nil
}

func ptrManifest(m domain.Manifest) *domain.Manifest {
	v := m
	return &v
}

func hashObject(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func sanitizeFlowID(v string) string {
	v = strings.TrimSpace(v)
	v = strings.ToLower(v)
	replacer := strings.NewReplacer(" ", "_", "/", "_", "-", "_", "{", "", "}", "")
	v = replacer.Replace(v)
	for strings.Contains(v, "__") {
		v = strings.ReplaceAll(v, "__", "_")
	}
	v = strings.Trim(v, "_")
	return v
}

func shortHash(v string) string {
	sum := sha256.Sum256([]byte(v))
	return hex.EncodeToString(sum[:3])
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
