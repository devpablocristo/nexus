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
