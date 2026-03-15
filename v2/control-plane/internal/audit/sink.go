package audit

import (
	"context"

	sharedaudit "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/audit"
)

type Sink interface {
	Write(ctx context.Context, req sharedaudit.WriteRequest) error
}

type SinkAdapter struct {
	uc *Usecases
}

func NewSinkAdapter(uc *Usecases) SinkAdapter {
	return SinkAdapter{uc: uc}
}

func (a SinkAdapter) Write(ctx context.Context, req sharedaudit.WriteRequest) error {
	_, err := a.uc.Create(ctx, req)
	return err
}
