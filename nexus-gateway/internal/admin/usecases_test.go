package admin

import (
	"context"
	"testing"

	"github.com/google/uuid"

	admindomain "nexus-gateway/internal/admin/usecases/domain"
)

type repoStub struct {
	settings admindomain.TenantSettings
	has      bool
	events   []admindomain.AdminActivityEvent
}

func (r *repoStub) GetTenantSettings(_ context.Context, _ uuid.UUID) (admindomain.TenantSettings, bool, error) {
	return r.settings, r.has, nil
}

func (r *repoStub) UpsertTenantSettings(_ context.Context, s admindomain.TenantSettings) (admindomain.TenantSettings, error) {
	r.settings = s
	r.has = true
	return s, nil
}

func (r *repoStub) CreateAdminActivityEvent(_ context.Context, ev admindomain.AdminActivityEvent) error {
	r.events = append(r.events, ev)
	return nil
}

func (r *repoStub) ListAdminActivityEvents(_ context.Context, _ uuid.UUID, _ int) ([]admindomain.AdminActivityEvent, error) {
	return r.events, nil
}

func TestUpsertTenantSettingsRequiresWritePermission(t *testing.T) {
	svc := NewService(&repoStub{})
	orgID := uuid.New()
	_, err := svc.UpsertTenantSettings(context.Background(), orgID, nil, nil, []string{"admin:console:read"}, UpsertTenantSettingsRequest{PlanCode: "growth"})
	if err == nil {
		t.Fatalf("expected forbidden")
	}
}

func TestBootstrapWithAdminScope(t *testing.T) {
	repo := &repoStub{}
	svc := NewService(repo)
	orgID := uuid.New()
	resp, err := svc.GetBootstrap(context.Background(), orgID, nil, nil, []string{"admin:console:read"}, "api_key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.CanReadAdmin {
		t.Fatalf("expected read permission")
	}
	if resp.Settings.PlanCode == "" {
		t.Fatalf("expected default settings")
	}
}

func TestGetTenantSettingsRequiresReadPermission(t *testing.T) {
	svc := NewService(&repoStub{})
	orgID := uuid.New()
	_, err := svc.GetTenantSettings(context.Background(), orgID, nil, nil, []string{"tools:read"})
	if err == nil {
		t.Fatalf("expected forbidden")
	}
}

func TestListActivityRequiresReadPermission(t *testing.T) {
	svc := NewService(&repoStub{})
	orgID := uuid.New()
	_, err := svc.ListActivity(context.Background(), orgID, nil, nil, []string{"tools:read"}, 10)
	if err == nil {
		t.Fatalf("expected forbidden")
	}
}
