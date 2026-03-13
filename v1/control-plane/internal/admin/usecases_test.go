package admin

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	admindomain "control-plane/internal/admin/usecases/domain"
)

type repoStub struct {
	settings           admindomain.TenantSettings
	protectedResources []admindomain.ProtectedResource
	restoreEvidence    []admindomain.RestoreEvidence
	has                bool
	events             []admindomain.AdminActivityEvent
}

func (r *repoStub) GetTenantSettings(_ context.Context, _ uuid.UUID) (admindomain.TenantSettings, bool, error) {
	return r.settings, r.has, nil
}

func (r *repoStub) UpsertTenantSettings(_ context.Context, s admindomain.TenantSettings) (admindomain.TenantSettings, error) {
	r.settings = s
	r.has = true
	return s, nil
}

func (r *repoStub) UpdateTenantLifecycle(_ context.Context, _ uuid.UUID, status string, deletedAt *time.Time, updatedBy *string) (admindomain.TenantSettings, error) {
	r.settings.Status = status
	r.settings.DeletedAt = deletedAt
	r.settings.UpdatedBy = updatedBy
	r.has = true
	return r.settings, nil
}

func (r *repoStub) CreateAdminActivityEvent(_ context.Context, ev admindomain.AdminActivityEvent) error {
	r.events = append(r.events, ev)
	return nil
}

func (r *repoStub) ListAdminActivityEvents(_ context.Context, _ uuid.UUID, _ int) ([]admindomain.AdminActivityEvent, error) {
	return r.events, nil
}

func (r *repoStub) ListProtectedResources(_ context.Context, _ uuid.UUID) ([]admindomain.ProtectedResource, error) {
	return append([]admindomain.ProtectedResource{}, r.protectedResources...), nil
}

func (r *repoStub) CreateProtectedResource(_ context.Context, resource admindomain.ProtectedResource) (admindomain.ProtectedResource, error) {
	resource.CreatedAt = time.Now().UTC()
	resource.UpdatedAt = resource.CreatedAt
	r.protectedResources = append(r.protectedResources, resource)
	return resource, nil
}

func (r *repoStub) DeleteProtectedResource(_ context.Context, _ uuid.UUID, resourceID uuid.UUID) error {
	for i, item := range r.protectedResources {
		if item.ID == resourceID {
			r.protectedResources = append(r.protectedResources[:i], r.protectedResources[i+1:]...)
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (r *repoStub) ListRestoreEvidence(_ context.Context, _ uuid.UUID, environment string, _ int) ([]admindomain.RestoreEvidence, error) {
	if environment == "" {
		return append([]admindomain.RestoreEvidence{}, r.restoreEvidence...), nil
	}
	out := make([]admindomain.RestoreEvidence, 0, len(r.restoreEvidence))
	for _, item := range r.restoreEvidence {
		if item.Environment == environment {
			out = append(out, item)
		}
	}
	return out, nil
}

func (r *repoStub) CreateRestoreEvidence(_ context.Context, evidence admindomain.RestoreEvidence) (admindomain.RestoreEvidence, error) {
	evidence.CreatedAt = time.Now().UTC()
	r.restoreEvidence = append(r.restoreEvidence, evidence)
	return evidence, nil
}

func TestUpsertTenantSettingsRequiresWritePermission(t *testing.T) {
	svc := NewUsecases(&repoStub{})
	orgID := uuid.New()
	_, err := svc.UpsertTenantSettings(context.Background(), orgID, nil, nil, []string{"admin:console:read"}, UpsertTenantSettingsRequest{PlanCode: "growth"})
	if err == nil {
		t.Fatalf("expected forbidden")
	}
}

func TestBootstrapWithAdminScope(t *testing.T) {
	repo := &repoStub{}
	svc := NewUsecases(repo)
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
	svc := NewUsecases(&repoStub{})
	orgID := uuid.New()
	_, err := svc.GetTenantSettings(context.Background(), orgID, nil, nil, []string{"tools:read"})
	if err == nil {
		t.Fatalf("expected forbidden")
	}
}

func TestListActivityRequiresReadPermission(t *testing.T) {
	svc := NewUsecases(&repoStub{})
	orgID := uuid.New()
	_, err := svc.ListActivity(context.Background(), orgID, nil, nil, []string{"tools:read"}, 10)
	if err == nil {
		t.Fatalf("expected forbidden")
	}
}

func TestSuspendAndReactivateTenant(t *testing.T) {
	repo := &repoStub{
		settings: admindomain.TenantSettings{
			PlanCode:   "growth",
			Status:     admindomain.TenantStatusActive,
			HardLimits: map[string]any{"run_rpm": 1200},
		},
		has: true,
	}
	svc := NewUsecases(repo)
	orgID := uuid.New()
	role := "admin"

	suspended, err := svc.SuspendTenant(context.Background(), orgID, orgID, nil, &role, nil)
	if err != nil {
		t.Fatalf("SuspendTenant error: %v", err)
	}
	if suspended.Status != admindomain.TenantStatusSuspended {
		t.Fatalf("expected suspended status, got %s", suspended.Status)
	}

	reactivated, err := svc.ReactivateTenant(context.Background(), orgID, orgID, nil, &role, nil)
	if err != nil {
		t.Fatalf("ReactivateTenant error: %v", err)
	}
	if reactivated.Status != admindomain.TenantStatusActive {
		t.Fatalf("expected active status, got %s", reactivated.Status)
	}
}

func TestCreateProtectedResourceRequiresWritePermission(t *testing.T) {
	svc := NewUsecases(&repoStub{})
	orgID := uuid.New()
	_, err := svc.CreateProtectedResource(context.Background(), orgID, nil, nil, []string{"admin:console:read"}, CreateProtectedResourceRequest{
		Name:         "state bucket",
		ResourceType: "terraform_address",
		MatchValue:   "aws_s3_bucket.state",
	})
	if err == nil {
		t.Fatalf("expected forbidden")
	}
}

func TestCreateAndListProtectedResources(t *testing.T) {
	repo := &repoStub{}
	svc := NewUsecases(repo)
	orgID := uuid.New()
	role := "admin"
	created, err := svc.CreateProtectedResource(context.Background(), orgID, nil, &role, nil, CreateProtectedResourceRequest{
		Name:         "prod-state-bucket",
		ResourceType: "terraform_address",
		MatchValue:   "aws_s3_bucket.prod_state",
		Environment:  "prod",
		Reason:       "crown jewel",
	})
	if err != nil {
		t.Fatalf("CreateProtectedResource error: %v", err)
	}
	if created.MatchMode != admindomain.ProtectedResourceMatchExact {
		t.Fatalf("expected exact match mode, got %s", created.MatchMode)
	}
	items, err := svc.ListProtectedResources(context.Background(), orgID, nil, &role, nil)
	if err != nil {
		t.Fatalf("ListProtectedResources error: %v", err)
	}
	if len(items) != 1 || items[0].Name != "prod-state-bucket" {
		t.Fatalf("unexpected resources: %#v", items)
	}
}

func TestRecordAndListRestoreEvidence(t *testing.T) {
	repo := &repoStub{}
	svc := NewUsecases(repo)
	orgID := uuid.New()
	actor := "system:dr"
	completedAt := time.Now().UTC()
	item, err := svc.RecordRestoreEvidence(context.Background(), orgID, &actor, RecordRestoreEvidenceRequest{
		Environment:    "prod",
		System:         "database",
		Status:         admindomain.RestoreEvidenceStatusPassed,
		SnapshotID:     "snap-123",
		RestoreTarget:  "restore-temp",
		CompletedAt:    &completedAt,
		Source:         "dr.test_restore.sh",
		ArtifactSHA256: "sha-1",
		Summary:        map[string]any{"core_ok": true},
	})
	if err != nil {
		t.Fatalf("RecordRestoreEvidence error: %v", err)
	}
	if item.Environment != "prod" || item.Status != admindomain.RestoreEvidenceStatusPassed {
		t.Fatalf("unexpected item: %#v", item)
	}
	items, err := svc.ListRestoreEvidence(context.Background(), orgID, nil, nil, []string{"admin:console:read"}, "prod", 10)
	if err != nil {
		t.Fatalf("ListRestoreEvidence error: %v", err)
	}
	if len(items) != 1 || items[0].SnapshotID != "snap-123" {
		t.Fatalf("unexpected restore evidence: %#v", items)
	}
}
