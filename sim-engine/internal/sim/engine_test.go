package sim

import "testing"

func TestDeterministicReplaySameSeedSameMoves(t *testing.T) {
	cfg := DefaultConfig(50)
	seed := int64(12345)

	build := func() string {
		st := NewState(seed, cfg)
		moves := []struct {
			agent string
			step  int64
			dir   Vec2
			speed float64
		}{
			{"agent-001", 1, Vec2{X: 1, Y: 0}, 1},
			{"agent-002", 1, Vec2{X: 1, Y: 0.1}, 0.9},
			{"agent-003", 2, Vec2{X: 1, Y: -0.2}, 1},
			{"agent-010", 2, Vec2{X: 1, Y: 0}, 1},
		}
		for _, m := range moves {
			_ = ApplyMove(&st, m.agent, MoveIntent{Direction: &m.dir, Speed: m.speed})
		}
		_, h, err := EncodeState(st)
		if err != nil {
			t.Fatalf("encode: %v", err)
		}
		return h
	}

	h1 := build()
	h2 := build()
	if h1 != h2 {
		t.Fatalf("expected equal hashes got %s vs %s", h1, h2)
	}
}

func TestApplyMoveCollisionTargetDeterministic(t *testing.T) {
	cfg := DefaultConfig(0)
	cfg.AgentRadius = 0.45
	st := RuntimeState{
		Config: cfg,
		Agents: map[string]*Agent{
			"agent-001": {ID: "agent-001", X: 1.0, Y: 1.0},
			"agent-002": {ID: "agent-002", X: 2.0, Y: 1.0},
			"agent-003": {ID: "agent-003", X: 2.0, Y: 1.0},
		},
	}

	result := ApplyMove(&st, "agent-001", MoveIntent{
		Direction: &Vec2{X: 1, Y: 0},
		Speed:     1,
	})
	if result.OK {
		t.Fatalf("expected blocked collision")
	}
	if result.CollisionWith != "agent-002" {
		t.Fatalf("expected deterministic collision target agent-002 got %s", result.CollisionWith)
	}
}
