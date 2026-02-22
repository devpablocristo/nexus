package httpapi

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"world-sim/internal/config"
	"world-sim/internal/model"
	"world-sim/internal/sim"
	"world-sim/internal/store"
)

type Server struct {
	cfg   config.Config
	store *store.Store
	mux   *http.ServeMux
}

type toolError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

type toolResponse struct {
	RequestID string     `json:"request_id"`
	Status    string     `json:"status"`
	Error     *toolError `json:"error,omitempty"`
	Data      any        `json:"data,omitempty"`
}

func New(cfg config.Config, st *store.Store) *Server {
	s := &Server{cfg: cfg, store: st, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) routes() {
	s.mux.HandleFunc("/healthz", s.handleHealthz)
	s.mux.HandleFunc("/tools/world.observe", s.withAuth(s.handleWorldObserve))
	s.mux.HandleFunc("/tools/world.move", s.withAuth(s.handleWorldMove))
	s.mux.HandleFunc("/admin/run/runs", s.withAuth(s.handleListRuns))
	s.mux.HandleFunc("/admin/run/create", s.withAuth(s.handleCreateRun))
	s.mux.HandleFunc("/admin/run/replay", s.withAuth(s.handleReplayRun))
	s.mux.HandleFunc("/admin/run/state", s.withAuth(s.handleGetState))
	s.mux.HandleFunc("/admin/run/events", s.withAuth(s.handleGetEvents))
}

func (s *Server) withAuth(next func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimSpace(s.cfg.InternalKey) != "" {
			if r.Header.Get("X-WorldSim-Internal-Key") != s.cfg.InternalKey {
				writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]any{"code": "UNAUTHORIZED", "message": "invalid internal key"}})
				return
			}
		}
		next(w, r)
	}
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

type observeRequest struct {
	OrgID     string `json:"org_id"`
	AgentID   string `json:"agent_id"`
	RunID     string `json:"run_id"`
	StepID    int64  `json:"step_id"`
	RequestID string `json:"request_id"`
}

type moveRequest struct {
	OrgID     string    `json:"org_id"`
	AgentID   string    `json:"agent_id"`
	RunID     string    `json:"run_id"`
	StepID    int64     `json:"step_id"`
	RequestID string    `json:"request_id"`
	Direction *sim.Vec2 `json:"direction,omitempty"`
	Target    *sim.Vec2 `json:"target,omitempty"`
	Speed     float64   `json:"speed,omitempty"`
}

func (s *Server) handleWorldObserve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": map[string]any{"code": "INVALID_INPUT", "message": "method not allowed"}})
		return
	}
	var req observeRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeFailure(w, requestIDFrom(r, ""), http.StatusBadRequest, "INVALID_INPUT", "invalid json", false)
		return
	}
	requestID := requestIDFrom(r, req.RequestID)
	if req.OrgID == "" || req.RunID == "" || req.AgentID == "" {
		s.writeFailure(w, requestID, http.StatusBadRequest, "INVALID_INPUT", "org_id, run_id, agent_id are required", false)
		return
	}
	_, err := s.store.GetRun(r.Context(), req.OrgID, req.RunID)
	if err != nil {
		s.writeFailure(w, requestID, http.StatusNotFound, "NOT_FOUND", "run not found", false)
		return
	}
	snap, err := s.store.LatestSnapshot(r.Context(), req.RunID, &req.StepID)
	if err != nil {
		s.writeFailure(w, requestID, http.StatusNotFound, "NOT_FOUND", "snapshot not found", false)
		return
	}
	st, err := sim.DecodeState(snap.StateJSON)
	if err != nil {
		s.writeFailure(w, requestID, http.StatusInternalServerError, "PROVIDER_ERROR", "failed to decode state", true)
		return
	}
	ag, ok := st.Agents[req.AgentID]
	if !ok {
		s.writeFailure(w, requestID, http.StatusNotFound, "NOT_FOUND", "agent not found", false)
		return
	}

	seq, err := s.store.NextSeq(r.Context(), req.RunID)
	if err == nil {
		payload := baseEventPayload("tool.called", req.OrgID, req.RunID, req.StepID, seq, req.AgentID, "world.observe", requestID)
		payload["input"] = req
		_ = s.store.InsertEvent(r.Context(), model.Event{RunID: req.RunID, StepID: req.StepID, Seq: seq, OrgID: req.OrgID, AgentID: req.AgentID, ToolName: "world.observe", Payload: mustJSON(payload), RequestID: requestID})
	}

	nearby := make([]sim.Agent, 0, 12)
	for id, other := range st.Agents {
		if id == req.AgentID {
			continue
		}
		if distance(ag.X, ag.Y, other.X, other.Y) <= 5 {
			nearby = append(nearby, *other)
		}
	}
	events, _ := s.store.ListEvents(r.Context(), req.OrgID, req.RunID, 0, 50)
	last := make([]any, 0, 8)
	for i := len(events) - 1; i >= 0 && len(last) < 8; i-- {
		if events[i].AgentID != req.AgentID {
			continue
		}
		var payload map[string]any
		_ = json.Unmarshal(events[i].Payload, &payload)
		last = append(last, payload)
	}
	writeJSON(w, http.StatusOK, toolResponse{
		RequestID: requestID,
		Status:    "success",
		Data: map[string]any{
			"position":        map[string]any{"x": ag.X, "y": ag.Y},
			"velocity":        map[string]any{"x": ag.VX, "y": ag.VY},
			"heading":         ag.Heading,
			"nearby_entities": nearby,
			"last_events":     last,
		},
	})
}

func (s *Server) handleWorldMove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": map[string]any{"code": "INVALID_INPUT", "message": "method not allowed"}})
		return
	}
	var req moveRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeFailure(w, requestIDFrom(r, ""), http.StatusBadRequest, "INVALID_INPUT", "invalid json", false)
		return
	}
	requestID := requestIDFrom(r, req.RequestID)
	if req.OrgID == "" || req.RunID == "" || req.AgentID == "" {
		s.writeFailure(w, requestID, http.StatusBadRequest, "INVALID_INPUT", "org_id, run_id, agent_id are required", false)
		return
	}
	if req.StepID < 0 {
		s.writeFailure(w, requestID, http.StatusBadRequest, "INVALID_INPUT", "step_id must be >= 0", false)
		return
	}
	_, err := s.store.GetRun(r.Context(), req.OrgID, req.RunID)
	if err != nil {
		s.writeFailure(w, requestID, http.StatusNotFound, "NOT_FOUND", "run not found", false)
		return
	}
	snap, err := s.store.LatestSnapshot(r.Context(), req.RunID, &req.StepID)
	if err != nil {
		s.writeFailure(w, requestID, http.StatusNotFound, "NOT_FOUND", "snapshot not found", false)
		return
	}
	st, err := sim.DecodeState(snap.StateJSON)
	if err != nil {
		s.writeFailure(w, requestID, http.StatusInternalServerError, "PROVIDER_ERROR", "failed to decode snapshot", true)
		return
	}
	startSeq, err := s.store.NextSeq(r.Context(), req.RunID)
	if err != nil {
		s.writeFailure(w, requestID, http.StatusInternalServerError, "PROVIDER_ERROR", "failed to allocate sequence", true)
		return
	}

	calledPayload := baseEventPayload("tool.called", req.OrgID, req.RunID, req.StepID, startSeq, req.AgentID, "world.move", requestID)
	calledPayload["input"] = req
	moveRes := sim.ApplyMove(&st, req.AgentID, sim.MoveIntent{Direction: req.Direction, Target: req.Target, Speed: req.Speed})
	eventType := "agent.moved"
	if !moveRes.OK {
		if moveRes.CollisionWith != "" {
			eventType = "agent.collided"
		} else {
			eventType = "agent.blocked"
		}
	}
	agentPayload := baseEventPayload(eventType, req.OrgID, req.RunID, req.StepID, startSeq+1, req.AgentID, "world.move", requestID)
	agentPayload["result"] = moveRes
	rawState, stateHash, err := sim.EncodeState(st)
	if err != nil {
		s.writeFailure(w, requestID, http.StatusInternalServerError, "PROVIDER_ERROR", "failed to encode state", true)
		return
	}
	snapshotPayload := baseEventPayload("world.snapshot", req.OrgID, req.RunID, req.StepID, startSeq+2, req.AgentID, "world.move", requestID)
	snapshotPayload["state_hash"] = stateHash

	err = s.store.InsertEvents(r.Context(), []model.Event{
		{RunID: req.RunID, StepID: req.StepID, Seq: startSeq, OrgID: req.OrgID, AgentID: req.AgentID, ToolName: "world.move", Payload: mustJSON(calledPayload), RequestID: requestID},
		{RunID: req.RunID, StepID: req.StepID, Seq: startSeq + 1, OrgID: req.OrgID, AgentID: req.AgentID, ToolName: eventType, Payload: mustJSON(agentPayload), RequestID: requestID},
		{RunID: req.RunID, StepID: req.StepID, Seq: startSeq + 2, OrgID: req.OrgID, AgentID: req.AgentID, ToolName: "world.snapshot", Payload: mustJSON(snapshotPayload), RequestID: requestID},
	})
	if err != nil {
		s.writeFailure(w, requestID, http.StatusInternalServerError, "PROVIDER_ERROR", "failed to persist events", true)
		return
	}
	if err := s.store.SaveSnapshot(r.Context(), store.BuildSnapshot(req.RunID, req.StepID, rawState, stateHash)); err != nil {
		s.writeFailure(w, requestID, http.StatusInternalServerError, "PROVIDER_ERROR", "failed to persist snapshot", true)
		return
	}
	collisions := []string{}
	if moveRes.CollisionWith != "" {
		collisions = append(collisions, moveRes.CollisionWith)
	}
	writeJSON(w, http.StatusOK, toolResponse{
		RequestID: requestID,
		Status:    "success",
		Data: map[string]any{
			"ok":         moveRes.OK,
			"new_state":  moveRes.NewState,
			"collisions": collisions,
			"state_hash": stateHash,
		},
	})
}

type createRunRequest struct {
	OrgID      string `json:"org_id"`
	RunID      string `json:"run_id"`
	Seed       *int64 `json:"seed"`
	AgentCount *int   `json:"agent_count"`
}

func (s *Server) handleCreateRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": map[string]any{"code": "INVALID_INPUT", "message": "method not allowed"}})
		return
	}
	var req createRunRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": "INVALID_INPUT", "message": "invalid json"}})
		return
	}
	if req.OrgID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": "INVALID_INPUT", "message": "org_id is required"}})
		return
	}
	seed := time.Now().UnixNano()
	if req.Seed != nil {
		seed = *req.Seed
	}
	runID := strings.TrimSpace(req.RunID)
	if runID == "" {
		runID = newID()
	}
	agentCount := s.cfg.DefaultAgentCount
	if req.AgentCount != nil && *req.AgentCount > 0 {
		agentCount = *req.AgentCount
	}
	cfg := sim.DefaultConfig(agentCount)
	cfgRaw, _ := json.Marshal(cfg)
	h := sha256.Sum256(cfgRaw)
	configHash := hex.EncodeToString(h[:])
	state := sim.NewState(seed, cfg)
	rawState, stateHash, err := sim.EncodeState(state)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"code": "PROVIDER_ERROR", "message": "failed to encode initial state"}})
		return
	}
	run := store.BuildRun(seed, req.OrgID, runID, configHash, cfgRaw)
	if err := s.store.CreateRun(r.Context(), run); err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": map[string]any{"code": "CONFLICT", "message": "run already exists"}})
		return
	}
	_ = s.store.SaveSnapshot(r.Context(), store.BuildSnapshot(runID, 0, rawState, stateHash))

	requestID := requestIDFrom(r, "")
	seq := int64(1)
	events := make([]model.Event, 0, cfg.AgentCount+2)
	seededPayload := baseEventPayload("world.seeded", req.OrgID, runID, 0, seq, "", "world.run.create", requestID)
	seededPayload["seed"] = seed
	seededPayload["config_hash"] = configHash
	events = append(events, model.Event{RunID: runID, StepID: 0, Seq: seq, OrgID: req.OrgID, AgentID: "", ToolName: "world.seeded", Payload: mustJSON(seededPayload), RequestID: requestID})
	for _, a := range state.Agents {
		seq++
		p := baseEventPayload("agent.spawned", req.OrgID, runID, 0, seq, a.ID, "world.run.create", requestID)
		p["position"] = map[string]any{"x": a.X, "y": a.Y}
		events = append(events, model.Event{RunID: runID, StepID: 0, Seq: seq, OrgID: req.OrgID, AgentID: a.ID, ToolName: "agent.spawned", Payload: mustJSON(p), RequestID: requestID})
	}
	seq++
	snapPayload := baseEventPayload("world.snapshot", req.OrgID, runID, 0, seq, "", "world.run.create", requestID)
	snapPayload["state_hash"] = stateHash
	events = append(events, model.Event{RunID: runID, StepID: 0, Seq: seq, OrgID: req.OrgID, AgentID: "", ToolName: "world.snapshot", Payload: mustJSON(snapPayload), RequestID: requestID})
	_ = s.store.InsertEvents(r.Context(), events)
	writeJSON(w, http.StatusOK, map[string]any{"run_id": runID, "seed": seed, "config_hash": configHash, "state_hash": stateHash})
}

type replayRunRequest struct {
	OrgID string `json:"org_id"`
	RunID string `json:"run_id"`
}

func (s *Server) handleReplayRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": map[string]any{"code": "INVALID_INPUT", "message": "method not allowed"}})
		return
	}
	var req replayRunRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": "INVALID_INPUT", "message": "invalid json"}})
		return
	}
	if req.OrgID == "" || req.RunID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": "INVALID_INPUT", "message": "org_id and run_id are required"}})
		return
	}
	run, err := s.store.GetRun(r.Context(), req.OrgID, req.RunID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"code": "NOT_FOUND", "message": "run not found"}})
		return
	}
	cfg, err := store.DecodeRunConfig(run)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"code": "PROVIDER_ERROR", "message": "invalid run config"}})
		return
	}
	state := sim.NewState(run.Seed, cfg)
	if err := s.store.DeleteSnapshots(r.Context(), req.RunID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"code": "PROVIDER_ERROR", "message": "failed to clear snapshots"}})
		return
	}
	raw0, hash0, _ := sim.EncodeState(state)
	_ = s.store.SaveSnapshot(r.Context(), store.BuildSnapshot(req.RunID, 0, raw0, hash0))
	calls, err := s.store.ListMoveCalls(r.Context(), req.OrgID, req.RunID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"code": "PROVIDER_ERROR", "message": "failed to load events"}})
		return
	}
	replayed := 0
	finalHash := hash0
	for _, ev := range calls {
		var payload map[string]any
		_ = json.Unmarshal(ev.Payload, &payload)
		in, ok := payload["input"].(map[string]any)
		if !ok {
			continue
		}
		agentID, _ := in["agent_id"].(string)
		stepID := toInt64(in["step_id"])
		intent := sim.MoveIntent{Speed: toFloat64(in["speed"])}
		if d, ok := in["direction"].(map[string]any); ok {
			intent.Direction = &sim.Vec2{X: toFloat64(d["x"]), Y: toFloat64(d["y"])}
		}
		if t, ok := in["target"].(map[string]any); ok {
			intent.Target = &sim.Vec2{X: toFloat64(t["x"]), Y: toFloat64(t["y"])}
		}
		_ = sim.ApplyMove(&state, agentID, intent)
		raw, h, err := sim.EncodeState(state)
		if err != nil {
			continue
		}
		finalHash = h
		_ = s.store.SaveSnapshot(r.Context(), store.BuildSnapshot(req.RunID, stepID, raw, h))
		replayed++
	}
	requestID := requestIDFrom(r, "")
	nextSeq, _ := s.store.NextSeq(r.Context(), req.RunID)
	replayedPayload := baseEventPayload("world.replayed", req.OrgID, req.RunID, 0, nextSeq, "", "world.run.replay", requestID)
	replayedPayload["replayed_moves"] = replayed
	replayedPayload["state_hash"] = finalHash
	_ = s.store.InsertEvent(r.Context(), model.Event{RunID: req.RunID, StepID: 0, Seq: nextSeq, OrgID: req.OrgID, AgentID: "", ToolName: "world.replayed", Payload: mustJSON(replayedPayload), RequestID: requestID})
	writeJSON(w, http.StatusOK, map[string]any{"run_id": req.RunID, "replayed_moves": replayed, "state_hash": finalHash})
}

func (s *Server) handleGetState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": map[string]any{"code": "INVALID_INPUT", "message": "method not allowed"}})
		return
	}
	orgID := strings.TrimSpace(r.URL.Query().Get("org_id"))
	runID := strings.TrimSpace(r.URL.Query().Get("run_id"))
	if orgID == "" || runID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": "INVALID_INPUT", "message": "org_id and run_id are required"}})
		return
	}
	_, err := s.store.GetRun(r.Context(), orgID, runID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"code": "NOT_FOUND", "message": "run not found"}})
		return
	}
	var stepID *int64
	if raw := strings.TrimSpace(r.URL.Query().Get("step_id")); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || v < 0 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": "INVALID_INPUT", "message": "step_id must be >= 0"}})
			return
		}
		stepID = &v
	}
	snap, err := s.store.LatestSnapshot(r.Context(), runID, stepID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"code": "NOT_FOUND", "message": "snapshot not found"}})
		return
	}
	var state map[string]any
	_ = json.Unmarshal(snap.StateJSON, &state)
	writeJSON(w, http.StatusOK, map[string]any{"run_id": runID, "step_id": snap.StepID, "state_hash": snap.StateHash, "state": state})
}

func (s *Server) handleGetEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": map[string]any{"code": "INVALID_INPUT", "message": "method not allowed"}})
		return
	}
	orgID := strings.TrimSpace(r.URL.Query().Get("org_id"))
	runID := strings.TrimSpace(r.URL.Query().Get("run_id"))
	if orgID == "" || runID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": "INVALID_INPUT", "message": "org_id and run_id are required"}})
		return
	}
	fromSeq, _ := strconv.ParseInt(r.URL.Query().Get("from_seq"), 10, 64)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	rows, err := s.store.ListEvents(r.Context(), orgID, runID, fromSeq, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"code": "PROVIDER_ERROR", "message": "failed to query events"}})
		return
	}
	items := make([]map[string]any, 0, len(rows))
	nextSeq := fromSeq
	for _, row := range rows {
		var payload map[string]any
		_ = json.Unmarshal(row.Payload, &payload)
		items = append(items, map[string]any{
			"id":         row.ID,
			"run_id":     row.RunID,
			"step_id":    row.StepID,
			"seq":        row.Seq,
			"org_id":     row.OrgID,
			"agent_id":   row.AgentID,
			"tool_name":  row.ToolName,
			"request_id": row.RequestID,
			"created_at": row.CreatedAt.UTC().Format(time.RFC3339),
			"payload":    payload,
		})
		nextSeq = row.Seq
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "next_seq": nextSeq})
}

func (s *Server) handleListRuns(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": map[string]any{"code": "INVALID_INPUT", "message": "method not allowed"}})
		return
	}
	orgID := strings.TrimSpace(r.URL.Query().Get("org_id"))
	if orgID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": "INVALID_INPUT", "message": "org_id is required"}})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	cursor := strings.TrimSpace(r.URL.Query().Get("cursor"))
	rows, err := s.store.ListRuns(r.Context(), orgID, limit, cursor)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"code": "PROVIDER_ERROR", "message": "failed to query runs"}})
		return
	}
	items := make([]map[string]any, 0, len(rows))
	nextCursor := ""
	for _, row := range rows {
		items = append(items, map[string]any{"run_id": row.RunID, "org_id": row.OrgID, "seed": row.Seed, "config_hash": row.ConfigHash, "created_at": row.CreatedAt.UTC().Format(time.RFC3339)})
		nextCursor = row.RunID
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "next_cursor": nextCursor})
}

func (s *Server) writeFailure(w http.ResponseWriter, requestID string, status int, code, msg string, retryable bool) {
	writeJSON(w, status, toolResponse{RequestID: requestID, Status: "failure", Error: &toolError{Code: code, Message: msg, Retryable: retryable}})
}

func requestIDFrom(r *http.Request, bodyRequestID string) string {
	if rid := strings.TrimSpace(r.Header.Get("X-Nexus-Request-Id")); rid != "" {
		return rid
	}
	if rid := strings.TrimSpace(bodyRequestID); rid != "" {
		return rid
	}
	return newID()
}

func baseEventPayload(eventType, orgID, runID string, stepID, seq int64, agentID, toolName, requestID string) map[string]any {
	return map[string]any{
		"event_version": 1,
		"event_type":    eventType,
		"timestamp":     time.Now().UTC().Format(time.RFC3339),
		"run_id":        runID,
		"step_id":       stepID,
		"seq":           seq,
		"org_id":        orgID,
		"agent_id":      agentID,
		"tool_name":     toolName,
		"policy_id":     nil,
		"request_id":    requestID,
	}
}

func mustJSON(v map[string]any) []byte {
	raw, _ := json.Marshal(v)
	return raw
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return err
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	return dec.Decode(dst)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func toInt64(v any) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int:
		return int64(t)
	case int64:
		return t
	case json.Number:
		x, _ := t.Int64()
		return x
	default:
		return 0
	}
}

func toFloat64(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case json.Number:
		x, _ := t.Float64()
		return x
	default:
		return 0
	}
}

func distance(x1, y1, x2, y2 float64) float64 {
	return math.Hypot(x2-x1, y2-y1)
}

func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return hex.EncodeToString(b[:])
}
