// Package gateway contains the controlled execution runtime and preflight logic.
package gateway

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	gwdomain "data-plane/internal/gateway/usecases/domain"
	tooldomain "data-plane/internal/tool/usecases/domain"
	"nexus/pkg/types"
	"nexus/pkg/utils"
)

type preflightEvaluation struct {
	required       bool
	status         gwdomain.PreflightStatus
	summary        map[string]any
	artifactSHA256 *string
	failureReason  string
	failureCode    string
	failureHTTP    int
}

func evaluateDeterministicPreflight(tool tooldomain.Tool, input, contextMap map[string]any, protectedResources []ProtectedResource, restoreEvidence []RestoreEvidence) preflightEvaluation {
	switch {
	case requiresTerraformAWSPrefight(tool, input, contextMap):
		return evaluateTerraformAWSPrefight(tool, input, contextMap, protectedResources, restoreEvidence)
	case requiresKubectlPreflight(tool, input, contextMap):
		return evaluateKubectlPreflight(tool, input, contextMap, protectedResources, restoreEvidence)
	case requiresSensitiveBashPreflight(tool, input, contextMap):
		return evaluateSensitiveBashPreflight(tool, input, contextMap, protectedResources, restoreEvidence)
	default:
		return preflightEvaluation{
			required: false,
			status:   gwdomain.PreflightStatusNotRequired,
			summary: map[string]any{
				"required": false,
				"family":   "generic",
			},
		}
	}
}

func evaluateTerraformAWSPrefight(_ tooldomain.Tool, input, contextMap map[string]any, protectedResources []ProtectedResource, restoreEvidence []RestoreEvidence) preflightEvaluation {
	result := preflightEvaluation{
		required:    true,
		status:      gwdomain.PreflightStatusFailed,
		failureCode: types.ErrCodePreflightFailed,
		failureHTTP: http.StatusBadRequest,
		summary: map[string]any{
			"required": true,
			"family":   "terraform_aws",
		},
	}

	artifact, artifactKind, ok := firstValue(input, contextMap,
		"terraform_plan_json", "plan_json", "plan_artifact", "terraform_plan")
	if !ok {
		result.failureReason = "terraform/aws preflight requires plan artifact"
		result.summary["check_plan_artifact"] = "missing"
		return result
	}
	hash, err := artifactSHA256(artifact)
	if err != nil {
		result.failureReason = "terraform/aws preflight artifact is not hashable"
		result.summary["check_plan_artifact"] = "invalid"
		return result
	}
	result.artifactSHA256 = &hash
	result.summary["artifact_kind"] = artifactKind
	result.summary["artifact_sha256"] = hash
	result.summary["check_plan_artifact"] = "ok"

	backend := firstString(input, contextMap, "backend", "backend_type")
	if backend == "" {
		backend = "unknown"
	}
	result.summary["backend"] = backend
	if !looksLikeRemoteTerraformBackend(backend) {
		result.failureReason = "terraform/aws preflight requires remote backend"
		result.summary["check_remote_backend"] = "failed"
		return result
	}
	result.summary["check_remote_backend"] = "ok"

	lockingOK := boolLike(firstAny(input, contextMap, "state_lock", "backend_locking", "locking")) ||
		firstString(input, contextMap, "dynamodb_table", "lock_table") != ""
	result.summary["locking_present"] = lockingOK
	if !lockingOK {
		result.failureReason = "terraform/aws preflight requires state locking"
		result.summary["check_state_locking"] = "failed"
		return result
	}
	result.summary["check_state_locking"] = "ok"

	targetEnv := firstString(input, contextMap, "target_env", "target_environment", "environment", "env")
	workspace := firstString(input, contextMap, "workspace")
	result.summary["target_env"] = targetEnv
	result.summary["workspace"] = workspace
	if strings.TrimSpace(targetEnv) == "" || strings.TrimSpace(workspace) == "" {
		result.failureReason = "terraform/aws preflight requires target_env and workspace"
		result.summary["check_environment_binding"] = "failed"
		return result
	}
	result.summary["check_environment_binding"] = "ok"

	createCount := firstInt(input, contextMap, "create_count")
	updateCount := firstInt(input, contextMap, "update_count")
	deleteCount := firstInt(input, contextMap, "delete_count")
	replaceCount := firstInt(input, contextMap, "replace_count")
	if planSummary, ok := firstMap(input, contextMap, "plan_summary"); ok {
		createCount = chooseInt(createCount, anyToInt(planSummary["create"]))
		updateCount = chooseInt(updateCount, anyToInt(planSummary["update"]))
		deleteCount = chooseInt(deleteCount, anyToInt(planSummary["delete"]))
		replaceCount = chooseInt(replaceCount, anyToInt(planSummary["replace"]))
	}
	result.summary["blast_radius"] = map[string]any{
		"create":  createCount,
		"update":  updateCount,
		"delete":  deleteCount,
		"replace": replaceCount,
	}
	result.summary["check_blast_radius"] = "ok"

	if isProductionTarget(input, contextMap) && (deleteCount > 0 || replaceCount > 0) {
		result.failureReason = "terraform/aws preflight blocked destructive changes in production"
		result.failureHTTP = http.StatusForbidden
		result.summary["check_destructive_prod"] = "failed"
		return result
	}
	result.summary["check_destructive_prod"] = "ok"
	if blocked := applyProtectedResources(&result, input, contextMap, protectedResources); blocked {
		return result
	}
	if blocked := applyRestoreEvidence(&result, input, contextMap, restoreEvidence); blocked {
		return result
	}

	now := time.Now().UTC()
	result.summary["completed_at"] = now.Format(time.RFC3339)
	result.status = gwdomain.PreflightStatusPassed
	return result
}

func requiresTerraformAWSPrefight(tool tooldomain.Tool, input, contextMap map[string]any) bool {
	combined := strings.ToLower(strings.Join([]string{
		tool.Name,
		tool.URL,
		firstString(input, contextMap, "provider", "tool_family", "executor"),
		firstString(input, contextMap, "backend", "backend_type"),
	}, " "))
	return strings.Contains(combined, "terraform")
}

func requiresKubectlPreflight(tool tooldomain.Tool, input, contextMap map[string]any) bool {
	combined := strings.ToLower(strings.Join([]string{
		tool.Name,
		tool.URL,
		firstString(input, contextMap, "provider", "tool_family", "executor"),
		firstString(input, contextMap, "command", "verb"),
	}, " "))
	return strings.Contains(combined, "kubectl")
}

func evaluateKubectlPreflight(_ tooldomain.Tool, input, contextMap map[string]any, protectedResources []ProtectedResource, restoreEvidence []RestoreEvidence) preflightEvaluation {
	result := preflightEvaluation{
		required:    true,
		status:      gwdomain.PreflightStatusFailed,
		failureCode: types.ErrCodePreflightFailed,
		failureHTTP: http.StatusBadRequest,
		summary: map[string]any{
			"required": true,
			"family":   "kubectl",
		},
	}

	cluster := firstString(input, contextMap, "cluster", "cluster_name", "target_cluster")
	namespace := firstString(input, contextMap, "namespace", "target_namespace")
	result.summary["cluster"] = cluster
	result.summary["namespace"] = namespace
	if cluster == "" || namespace == "" {
		result.failureReason = "kubectl preflight requires cluster and namespace"
		result.summary["check_cluster_namespace"] = "failed"
		return result
	}
	result.summary["check_cluster_namespace"] = "ok"

	artifact, artifactKind, ok := firstValue(input, contextMap,
		"manifest", "manifest_yaml", "manifest_json", "command", "kubectl_args")
	if !ok {
		result.failureReason = "kubectl preflight requires manifest or command artifact"
		result.summary["check_artifact"] = "missing"
		return result
	}
	hash, err := artifactSHA256(artifact)
	if err != nil {
		result.failureReason = "kubectl preflight artifact is not hashable"
		result.summary["check_artifact"] = "invalid"
		return result
	}
	result.artifactSHA256 = &hash
	result.summary["artifact_kind"] = artifactKind
	result.summary["artifact_sha256"] = hash
	result.summary["check_artifact"] = "ok"

	verb := strings.ToLower(firstString(input, contextMap, "verb", "operation"))
	if verb == "" {
		verb = inferVerbFromCommand(firstString(input, contextMap, "command"))
	}
	result.summary["verb"] = verb
	if verb == "" {
		result.failureReason = "kubectl preflight requires an explicit verb"
		result.summary["check_verb"] = "failed"
		return result
	}
	result.summary["check_verb"] = "ok"

	if isProductionTarget(input, contextMap) && (verb == "delete" || verb == "replace") {
		result.failureReason = "kubectl preflight blocked destructive prod command"
		result.failureHTTP = http.StatusForbidden
		result.summary["check_destructive_prod"] = "failed"
		return result
	}
	result.summary["check_destructive_prod"] = "ok"
	if blocked := applyProtectedResources(&result, input, contextMap, protectedResources); blocked {
		return result
	}
	if blocked := applyRestoreEvidence(&result, input, contextMap, restoreEvidence); blocked {
		return result
	}
	now := time.Now().UTC()
	result.summary["completed_at"] = now.Format(time.RFC3339)
	result.status = gwdomain.PreflightStatusPassed
	return result
}

func requiresSensitiveBashPreflight(tool tooldomain.Tool, input, contextMap map[string]any) bool {
	combined := strings.ToLower(strings.Join([]string{
		tool.Name,
		tool.URL,
		firstString(input, contextMap, "provider", "tool_family", "executor"),
		firstString(input, contextMap, "shell", "interpreter"),
	}, " "))
	return strings.Contains(combined, "bash") || strings.Contains(combined, "shell")
}

func evaluateSensitiveBashPreflight(_ tooldomain.Tool, input, contextMap map[string]any, protectedResources []ProtectedResource, restoreEvidence []RestoreEvidence) preflightEvaluation {
	result := preflightEvaluation{
		required:    true,
		status:      gwdomain.PreflightStatusFailed,
		failureCode: types.ErrCodePreflightFailed,
		failureHTTP: http.StatusBadRequest,
		summary: map[string]any{
			"required": true,
			"family":   "bash",
		},
	}

	artifact, artifactKind, ok := firstValue(input, contextMap, "script", "command", "script_body")
	if !ok {
		result.failureReason = "bash preflight requires script or command artifact"
		result.summary["check_artifact"] = "missing"
		return result
	}
	hash, err := artifactSHA256(artifact)
	if err != nil {
		result.failureReason = "bash preflight artifact is not hashable"
		result.summary["check_artifact"] = "invalid"
		return result
	}
	result.artifactSHA256 = &hash
	result.summary["artifact_kind"] = artifactKind
	result.summary["artifact_sha256"] = hash
	result.summary["check_artifact"] = "ok"

	targetHost := firstString(input, contextMap, "target_host", "host", "hostname")
	result.summary["target_host"] = targetHost
	if targetHost == "" {
		result.failureReason = "bash preflight requires target_host"
		result.summary["check_target_host"] = "failed"
		return result
	}
	result.summary["check_target_host"] = "ok"

	commandText := strings.ToLower(firstString(input, contextMap, "command", "script", "script_body"))
	result.summary["destructive_signal"] = containsDestructiveShellSignal(commandText)
	if isProductionTarget(input, contextMap) && containsDestructiveShellSignal(commandText) {
		result.failureReason = "bash preflight blocked destructive prod command"
		result.failureHTTP = http.StatusForbidden
		result.summary["check_destructive_prod"] = "failed"
		return result
	}
	result.summary["check_destructive_prod"] = "ok"
	if blocked := applyProtectedResources(&result, input, contextMap, protectedResources); blocked {
		return result
	}
	if blocked := applyRestoreEvidence(&result, input, contextMap, restoreEvidence); blocked {
		return result
	}
	now := time.Now().UTC()
	result.summary["completed_at"] = now.Format(time.RFC3339)
	result.status = gwdomain.PreflightStatusPassed
	return result
}

func applyProtectedResources(result *preflightEvaluation, input, contextMap map[string]any, protectedResources []ProtectedResource) bool {
	result.summary["protected_resources_registered"] = len(protectedResources)
	if len(protectedResources) == 0 {
		result.summary["check_protected_resources"] = "ok"
		return false
	}
	targetIsProd := isProductionTarget(input, contextMap)
	candidates := collectProtectedResourceCandidates(input, contextMap)
	hits := make([]map[string]any, 0, 2)
	for _, resource := range protectedResources {
		if !resourceAppliesToTarget(resource.Environment, targetIsProd) {
			continue
		}
		if !matchesProtectedResource(resource, candidates) {
			continue
		}
		hits = append(hits, map[string]any{
			"id":            resource.ID.String(),
			"name":          resource.Name,
			"resource_type": resource.ResourceType,
			"match_value":   resource.MatchValue,
			"match_mode":    resource.MatchMode,
			"environment":   resource.Environment,
			"reason":        resource.Reason,
		})
	}
	if len(hits) == 0 {
		result.summary["check_protected_resources"] = "ok"
		return false
	}
	firstName := strings.TrimSpace(anyToString(hits[0]["name"]))
	if firstName == "" {
		firstName = "protected resource"
	}
	result.failureReason = "preflight blocked protected resource: " + firstName
	result.failureCode = types.ErrCodePreflightFailed
	result.failureHTTP = http.StatusForbidden
	result.summary["check_protected_resources"] = "failed"
	result.summary["protected_resource_hits"] = hits
	return true
}

func applyRestoreEvidence(result *preflightEvaluation, input, contextMap map[string]any, restoreEvidence []RestoreEvidence) bool {
	if !isProductionTarget(input, contextMap) {
		result.summary["restore_evidence_required"] = false
		result.summary["check_restore_evidence"] = "skipped_nonprod"
		return false
	}
	maxAgeHours := restoreEvidenceMaxAgeHours(input, contextMap)
	result.summary["restore_evidence_required"] = true
	result.summary["restore_evidence_max_age_hours"] = maxAgeHours
	if len(restoreEvidence) == 0 {
		result.failureReason = "preflight requires recent restore evidence for production"
		result.summary["check_restore_evidence"] = "missing"
		return true
	}
	latest := latestSuccessfulRestoreEvidence(restoreEvidence)
	if latest == nil || latest.CompletedAt == nil || latest.CompletedAt.IsZero() {
		result.failureReason = "preflight requires recent successful restore evidence for production"
		result.summary["check_restore_evidence"] = "missing"
		return true
	}
	result.summary["restore_evidence_latest_completed_at"] = latest.CompletedAt.UTC().Format(time.RFC3339)
	result.summary["restore_evidence_latest_snapshot_id"] = latest.SnapshotID
	result.summary["restore_evidence_latest_system"] = latest.System
	result.summary["restore_evidence_latest_source"] = latest.Source
	if time.Since(latest.CompletedAt.UTC()) > time.Duration(maxAgeHours)*time.Hour {
		result.failureReason = "preflight requires newer restore evidence for production"
		result.summary["check_restore_evidence"] = "stale"
		return true
	}
	result.summary["check_restore_evidence"] = "ok"
	return false
}

func latestSuccessfulRestoreEvidence(items []RestoreEvidence) *RestoreEvidence {
	var latest *RestoreEvidence
	for i := range items {
		item := items[i]
		if strings.TrimSpace(strings.ToLower(item.Status)) != "passed" {
			continue
		}
		if item.CompletedAt == nil || item.CompletedAt.IsZero() {
			continue
		}
		if latest == nil || latest.CompletedAt.Before(item.CompletedAt.UTC()) {
			copy := item
			latest = &copy
		}
	}
	return latest
}

func restoreEvidenceMaxAgeHours(input, contextMap map[string]any) int {
	value := firstInt(input, contextMap, "restore_evidence_max_age_hours")
	if value <= 0 {
		return 720
	}
	return value
}

func resourceAppliesToTarget(environment string, targetIsProd bool) bool {
	switch strings.ToLower(strings.TrimSpace(environment)) {
	case "", "*":
		return true
	case "prod":
		return targetIsProd
	case "nonprod":
		return !targetIsProd
	default:
		return true
	}
}

func matchesProtectedResource(resource ProtectedResource, candidates []string) bool {
	needle := strings.ToLower(strings.TrimSpace(resource.MatchValue))
	if needle == "" {
		return false
	}
	matchMode := strings.ToLower(strings.TrimSpace(resource.MatchMode))
	if matchMode == "" {
		matchMode = "exact"
	}
	for _, candidate := range candidates {
		switch matchMode {
		case "contains":
			if strings.Contains(candidate, needle) {
				return true
			}
		default:
			if candidate == needle {
				return true
			}
		}
	}
	return false
}

func collectProtectedResourceCandidates(input, contextMap map[string]any) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(v string) {
		value := strings.ToLower(strings.TrimSpace(v))
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	var visit func(v any)
	visit = func(v any) {
		switch t := v.(type) {
		case string:
			add(t)
		case []string:
			for _, item := range t {
				add(item)
			}
		case []any:
			for _, item := range t {
				visit(item)
			}
		case map[string]any:
			for _, item := range t {
				visit(item)
			}
			if raw, err := json.Marshal(t); err == nil {
				add(string(raw))
			}
		default:
			if raw, err := json.Marshal(t); err == nil {
				add(string(raw))
			}
		}
	}
	visit(input)
	visit(contextMap)
	return out
}

func anyToString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	default:
		return ""
	}
}

func artifactSHA256(v any) (string, error) {
	switch t := v.(type) {
	case string:
		return utils.SHA256Hex(t), nil
	default:
		raw, err := json.Marshal(t)
		if err != nil {
			return "", err
		}
		return utils.SHA256Hex(string(raw)), nil
	}
}

func looksLikeRemoteTerraformBackend(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "s3", "remote", "gcs", "azurerm":
		return true
	default:
		return false
	}
}

func inferVerbFromCommand(command string) string {
	tokens := strings.Fields(strings.ToLower(strings.TrimSpace(command)))
	for i, tok := range tokens {
		if tok == "kubectl" && i+1 < len(tokens) {
			return tokens[i+1]
		}
	}
	if len(tokens) > 0 {
		return tokens[0]
	}
	return ""
}

func containsDestructiveShellSignal(command string) bool {
	for _, marker := range []string{"rm -rf", "terraform destroy", "drop database", "kubectl delete", "shutdown "} {
		if strings.Contains(command, marker) {
			return true
		}
	}
	return false
}

func firstValue(input, contextMap map[string]any, keys ...string) (any, string, bool) {
	for _, src := range []map[string]any{input, contextMap} {
		for _, key := range keys {
			if src == nil {
				continue
			}
			if v, ok := src[key]; ok && v != nil {
				return v, key, true
			}
		}
	}
	return nil, "", false
}

func firstAny(input, contextMap map[string]any, keys ...string) any {
	v, _, _ := firstValue(input, contextMap, keys...)
	return v
}

func firstString(input, contextMap map[string]any, keys ...string) string {
	v, _, ok := firstValue(input, contextMap, keys...)
	if !ok {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	default:
		return ""
	}
}

func firstInt(input, contextMap map[string]any, keys ...string) int {
	return anyToInt(firstAny(input, contextMap, keys...))
}

func firstMap(input, contextMap map[string]any, keys ...string) (map[string]any, bool) {
	v, _, ok := firstValue(input, contextMap, keys...)
	if !ok {
		return nil, false
	}
	m, ok := v.(map[string]any)
	return m, ok
}

func anyToInt(v any) int {
	switch t := v.(type) {
	case int:
		return t
	case int64:
		return int(t)
	case float64:
		return int(t)
	case json.Number:
		i, _ := t.Int64()
		return int(i)
	default:
		return 0
	}
}

func chooseInt(primary, fallback int) int {
	if primary != 0 {
		return primary
	}
	return fallback
}

func nowPtrIfRequired(required bool) *time.Time {
	if !required {
		return nil
	}
	now := time.Now().UTC()
	return &now
}
