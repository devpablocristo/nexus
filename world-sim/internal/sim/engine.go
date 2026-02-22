package sim

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"math/rand"
	"sort"
)

type Vec2 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type Agent struct {
	ID         string  `json:"id"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	VX         float64 `json:"vx"`
	VY         float64 `json:"vy"`
	Heading    float64 `json:"heading"`
	IntentionX float64 `json:"intention_x"`
	IntentionY float64 `json:"intention_y"`
}

type WorldConfig struct {
	Width       float64 `json:"width"`
	Height      float64 `json:"height"`
	DoorX       float64 `json:"door_x"`
	DoorMinY    float64 `json:"door_min_y"`
	DoorMaxY    float64 `json:"door_max_y"`
	AgentRadius float64 `json:"agent_radius"`
	AgentCount  int     `json:"agent_count"`
}

type RuntimeState struct {
	Config WorldConfig
	Agents map[string]*Agent
}

type MoveIntent struct {
	Direction *Vec2
	Target    *Vec2
	Speed     float64
}

type MoveResult struct {
	OK            bool    `json:"ok"`
	CollisionWith string  `json:"collision_with,omitempty"`
	Reason        string  `json:"reason,omitempty"`
	Distance      float64 `json:"distance"`
	NewState      Agent   `json:"new_state"`
}

func DefaultConfig(agentCount int) WorldConfig {
	if agentCount <= 0 {
		agentCount = 50
	}
	return WorldConfig{
		Width:       60,
		Height:      30,
		DoorX:       30,
		DoorMinY:    13,
		DoorMaxY:    17,
		AgentRadius: 0.45,
		AgentCount:  agentCount,
	}
}

func NewState(seed int64, cfg WorldConfig) RuntimeState {
	r := rand.New(rand.NewSource(seed))
	agents := make(map[string]*Agent, cfg.AgentCount)
	cols := 10
	if cfg.AgentCount < cols {
		cols = cfg.AgentCount
	}
	spacingX := 1.1
	spacingY := 1.1
	for i := 0; i < cfg.AgentCount; i++ {
		id := AgentID(i)
		row := i / cols
		col := i % cols
		jitter := (r.Float64() - 0.5) * 0.2
		x := 2 + float64(col)*spacingX
		y := 2 + float64(row)*spacingY + jitter
		if y > cfg.Height-2 {
			y = cfg.Height - 2
		}
		a := &Agent{
			ID:      id,
			X:       x,
			Y:       y,
			Heading: 0,
		}
		a.IntentionX = cfg.DoorX
		a.IntentionY = (cfg.DoorMinY + cfg.DoorMaxY) / 2
		agents[id] = a
	}
	return RuntimeState{Config: cfg, Agents: agents}
}

func AgentID(i int) string {
	return "agent-" + leftPad(i+1, 3)
}

func leftPad(n int, width int) string {
	s := ""
	for n > 0 {
		s = string('0'+rune(n%10)) + s
		n /= 10
	}
	if s == "" {
		s = "0"
	}
	for len(s) < width {
		s = "0" + s
	}
	return s
}

func ApplyMove(st *RuntimeState, agentID string, intent MoveIntent) MoveResult {
	ag, ok := st.Agents[agentID]
	if !ok {
		return MoveResult{OK: false, Reason: "agent_not_found"}
	}
	vx, vy := resolveDirection(*ag, intent)
	ag.IntentionX = ag.X + vx
	ag.IntentionY = ag.Y + vy

	speed := intent.Speed
	if speed <= 0 {
		speed = 1
	}
	if speed > 1 {
		speed = 1
	}
	candX := ag.X + vx*speed
	candY := ag.Y + vy*speed
	candX = clamp(candX, st.Config.AgentRadius, st.Config.Width-st.Config.AgentRadius)
	candY = clamp(candY, st.Config.AgentRadius, st.Config.Height-st.Config.AgentRadius)

	if crossesWall(st.Config, ag.X, ag.Y, candX, candY) {
		ag.VX, ag.VY = 0, 0
		return MoveResult{OK: false, Reason: "blocked_wall", Distance: 0, NewState: *ag}
	}

	for id, other := range st.Agents {
		if id == agentID {
			continue
		}
		if dist(candX, candY, other.X, other.Y) < 2*st.Config.AgentRadius {
			ag.VX, ag.VY = 0, 0
			return MoveResult{OK: false, Reason: "blocked_agent", CollisionWith: id, Distance: 0, NewState: *ag}
		}
	}

	d := dist(ag.X, ag.Y, candX, candY)
	ag.VX, ag.VY = candX-ag.X, candY-ag.Y
	ag.X, ag.Y = candX, candY
	if d > 0 {
		ag.Heading = math.Atan2(ag.VY, ag.VX)
	}
	return MoveResult{OK: true, Distance: d, NewState: *ag}
}

func resolveDirection(a Agent, intent MoveIntent) (float64, float64) {
	if intent.Target != nil {
		dx := intent.Target.X - a.X
		dy := intent.Target.Y - a.Y
		return normalize(dx, dy)
	}
	if intent.Direction != nil {
		return normalize(intent.Direction.X, intent.Direction.Y)
	}
	dx := a.IntentionX - a.X
	dy := a.IntentionY - a.Y
	if dx == 0 && dy == 0 {
		return 1, 0
	}
	return normalize(dx, dy)
}

func normalize(x, y float64) (float64, float64) {
	n := math.Hypot(x, y)
	if n == 0 {
		return 0, 0
	}
	return x / n, y / n
}

func crossesWall(cfg WorldConfig, x1, y1, x2, y2 float64) bool {
	crosses := (x1 < cfg.DoorX && x2 >= cfg.DoorX) || (x1 > cfg.DoorX && x2 <= cfg.DoorX)
	if !crosses {
		return false
	}
	midY := (y1 + y2) / 2
	return midY < cfg.DoorMinY || midY > cfg.DoorMaxY
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func dist(x1, y1, x2, y2 float64) float64 {
	return math.Hypot(x2-x1, y2-y1)
}

func EncodeState(st RuntimeState) ([]byte, string, error) {
	type stateDTO struct {
		Config WorldConfig `json:"config"`
		Agents []Agent     `json:"agents"`
	}
	agents := make([]Agent, 0, len(st.Agents))
	for _, a := range st.Agents {
		agents = append(agents, *a)
	}
	sort.Slice(agents, func(i, j int) bool { return agents[i].ID < agents[j].ID })
	raw, err := json.Marshal(stateDTO{Config: st.Config, Agents: agents})
	if err != nil {
		return nil, "", err
	}
	h := sha256.Sum256(raw)
	return raw, hex.EncodeToString(h[:]), nil
}

func DecodeState(raw []byte) (RuntimeState, error) {
	type stateDTO struct {
		Config WorldConfig `json:"config"`
		Agents []Agent     `json:"agents"`
	}
	var dto stateDTO
	if err := json.Unmarshal(raw, &dto); err != nil {
		return RuntimeState{}, err
	}
	st := RuntimeState{Config: dto.Config, Agents: map[string]*Agent{}}
	for i := range dto.Agents {
		a := dto.Agents[i]
		cp := a
		st.Agents[a.ID] = &cp
	}
	return st, nil
}
