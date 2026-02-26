// Package toollab wires the TOOLLAB adapter library into Nexus routes.
package toollab

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	adapterlib "github.com/toollab/toollab-adapter-go"
)

// Handler mounts the external TOOLLAB adapter library under /_toollab.
type Handler struct {
	httpHandler http.Handler
}

// NewHandler creates a new handler backed by github.com/toollab/toollab-adapter-go.
func NewHandler(svc Service) *Handler {
	manifest := svc.Manifest("")
	adapter := adapterlib.NewAdapter(adapterlib.Config{
		AppName:                "nexus",
		AppVersion:             manifest.AppVersion,
		StandardVersion:        manifest.StandardVersion,
		StateProvider:          &stateProviderBridge{svc: svc},
		MetricsProvider:        &metricsProviderBridge{svc: svc},
		SchemaProvider:         &schemaProviderBridge{svc: svc},
		SuggestedFlowsProvider: &suggestedFlowsProviderBridge{svc: svc},
		InvariantsProvider:     &invariantsProviderBridge{svc: svc},
		LimitsProvider:         &limitsProviderBridge{svc: svc},
		EnvironmentProvider:    &environmentProviderBridge{svc: svc},
		OpenAPIProvider:        &openAPIProviderBridge{svc: svc},
	})
	return &Handler{httpHandler: adapter.Handler()}
}

// Register mounts all adapter routes on the given router group.
func (h *Handler) Register(rg *gin.RouterGroup) {
	wrapped := http.StripPrefix("/_toollab", h.httpHandler)
	rg.Any("/*path", gin.WrapH(wrapped))
}

type stateProviderBridge struct{ svc Service }

func (b *stateProviderBridge) Fingerprint(ctx context.Context) (string, error) {
	return b.svc.Fingerprint(ctx)
}

func (b *stateProviderBridge) Snapshot(ctx context.Context, label string) (string, string, error) {
	meta, err := b.svc.Snapshot(ctx, label)
	if err != nil {
		return "", "", err
	}
	return meta.ID, meta.Fingerprint, nil
}

func (b *stateProviderBridge) Restore(ctx context.Context, snapshotID string) error {
	_, err := b.svc.Restore(ctx, snapshotID)
	return err
}

func (b *stateProviderBridge) Reset(ctx context.Context) error {
	_, err := b.svc.Reset(ctx)
	return err
}

type metricsProviderBridge struct{ svc Service }

func (b *metricsProviderBridge) Snapshot(ctx context.Context) ([]adapterlib.Metric, error) {
	items, err := b.svc.Metrics(ctx)
	if err != nil {
		return nil, err
	}
	metrics := make([]adapterlib.Metric, 0, len(items))
	for _, item := range items {
		metrics = append(metrics, adapterlib.Metric{
			Name:   item.Name,
			Type:   item.Type,
			Value:  item.Value,
			Labels: item.Labels,
		})
	}
	return metrics, nil
}

type schemaProviderBridge struct{ svc Service }

func (b *schemaProviderBridge) Schema(ctx context.Context) (any, error) {
	return b.svc.Schema(ctx)
}

type suggestedFlowsProviderBridge struct{ svc Service }

func (b *suggestedFlowsProviderBridge) SuggestedFlows(ctx context.Context) (any, error) {
	return b.svc.SuggestedFlows(ctx)
}

type invariantsProviderBridge struct{ svc Service }

func (b *invariantsProviderBridge) Invariants(ctx context.Context) (any, error) {
	_ = ctx
	return b.svc.Invariants(), nil
}

type limitsProviderBridge struct{ svc Service }

func (b *limitsProviderBridge) Limits(ctx context.Context) (any, error) {
	_ = ctx
	return b.svc.Limits(), nil
}

type environmentProviderBridge struct{ svc Service }

func (b *environmentProviderBridge) Environment(ctx context.Context) (any, error) {
	_ = ctx
	return b.svc.Environment(), nil
}

type openAPIProviderBridge struct{ svc Service }

func (b *openAPIProviderBridge) OpenAPIDocument(ctx context.Context) (string, []byte, error) {
	raw, err := b.svc.OpenAPIDocument(ctx)
	if err != nil {
		return "", nil, err
	}
	return "application/yaml", raw, nil
}

func (b *openAPIProviderBridge) OpenAPIInfo(ctx context.Context) (*adapterlib.OpenAPIInfo, error) {
	info, err := b.svc.OpenAPIInfo(ctx, "")
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, nil
	}
	return &adapterlib.OpenAPIInfo{
		URL:         info.URL,
		ContentType: info.ContentType,
		Version:     info.Version,
		ETag:        info.ETag,
		SHA256:      info.SHA256,
	}, nil
}
