package model

import "time"

type Run struct {
	RunID      string
	OrgID      string
	Seed       int64
	ConfigHash string
	ConfigJSON []byte
	CreatedAt  time.Time
}

type Event struct {
	ID        int64
	RunID     string
	StepID    int64
	Seq       int64
	OrgID     string
	AgentID   string
	ToolName  string
	Payload   []byte
	RequestID string
	CreatedAt time.Time
}

type Snapshot struct {
	RunID     string
	StepID    int64
	StateJSON []byte
	StateHash string
	CreatedAt time.Time
}
