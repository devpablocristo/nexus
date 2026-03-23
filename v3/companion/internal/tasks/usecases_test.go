package tasks

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/devpablocristo/core/governance/go/reviewclient"
	domain "github.com/devpablocristo/nexus/v3/companion/internal/tasks/usecases/domain"
)

type fakeRepo struct {
	tasks       map[uuid.UUID]domain.Task
	lastPropose map[uuid.UUID]uuid.UUID
}

func (f *fakeRepo) CreateTask(ctx context.Context, t domain.Task) (domain.Task, error) {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	if f.tasks == nil {
		f.tasks = make(map[uuid.UUID]domain.Task)
	}
	f.tasks[t.ID] = t
	return t, nil
}

func (f *fakeRepo) GetTaskByID(ctx context.Context, id uuid.UUID) (domain.Task, error) {
	t, ok := f.tasks[id]
	if !ok {
		return domain.Task{}, ErrNotFound
	}
	return t, nil
}

func (f *fakeRepo) ListTasks(ctx context.Context, limit int) ([]domain.Task, error) {
	var out []domain.Task
	for _, t := range f.tasks {
		out = append(out, t)
	}
	return out, nil
}

func (f *fakeRepo) UpdateTask(ctx context.Context, t domain.Task) (domain.Task, error) {
	if _, ok := f.tasks[t.ID]; !ok {
		return domain.Task{}, ErrNotFound
	}
	f.tasks[t.ID] = t
	return t, nil
}

func (f *fakeRepo) ListTasksByStatus(ctx context.Context, status string, limit int) ([]domain.Task, error) {
	var out []domain.Task
	for _, t := range f.tasks {
		if t.Status == status {
			out = append(out, t)
		}
	}
	return out, nil
}

func (f *fakeRepo) LatestProposeReviewRequestID(ctx context.Context, taskID uuid.UUID) (uuid.UUID, error) {
	if f.lastPropose == nil {
		return uuid.Nil, ErrNotFound
	}
	rid, ok := f.lastPropose[taskID]
	if !ok {
		return uuid.Nil, ErrNotFound
	}
	return rid, nil
}

func (f *fakeRepo) InsertMessage(ctx context.Context, m domain.TaskMessage) (domain.TaskMessage, error) {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return m, nil
}

func (f *fakeRepo) ListMessagesByTaskID(ctx context.Context, taskID uuid.UUID) ([]domain.TaskMessage, error) {
	return nil, nil
}

func (f *fakeRepo) InsertAction(ctx context.Context, a domain.TaskAction) (domain.TaskAction, error) {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return a, nil
}

func (f *fakeRepo) UpdateActionReviewResult(ctx context.Context, actionID uuid.UUID, reviewRequestID *uuid.UUID, errMsg string) error {
	return nil
}

func (f *fakeRepo) ListActionsByTaskID(ctx context.Context, taskID uuid.UUID) ([]domain.TaskAction, error) {
	return nil, nil
}

func (f *fakeRepo) ListArtifactsByTaskID(ctx context.Context, taskID uuid.UUID) ([]domain.TaskArtifact, error) {
	return nil, nil
}

type stubReview struct {
	getFn func(ctx context.Context, id string) (reviewclient.RequestSummary, int, error)
}

func (s *stubReview) SubmitRequest(ctx context.Context, idempotencyKey string, body reviewclient.SubmitRequestBody) (reviewclient.SubmitResponse, error) {
	return reviewclient.SubmitResponse{}, nil
}

func (s *stubReview) GetRequest(ctx context.Context, id string) (reviewclient.RequestSummary, int, error) {
	if s.getFn != nil {
		return s.getFn(ctx, id)
	}
	return reviewclient.RequestSummary{}, 404, nil
}

func TestUsecases_Create_requiresTitle(t *testing.T) {
	t.Parallel()
	uc := NewUsecases(&fakeRepo{}, &stubReview{})
	_, err := uc.Create(context.Background(), CreateTaskInput{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUsecases_Create_ok(t *testing.T) {
	t.Parallel()
	r := &fakeRepo{}
	uc := NewUsecases(r, &stubReview{})
	out, err := uc.Create(context.Background(), CreateTaskInput{Title: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Title != "x" || out.Status != domain.TaskStatusNew {
		t.Fatalf("task %+v", out)
	}
}

func TestUsecases_SyncTaskReview_approvedToDone(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rid := uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
	r := &fakeRepo{lastPropose: make(map[uuid.UUID]uuid.UUID)}
	rev := &stubReview{
		getFn: func(ctx context.Context, id string) (reviewclient.RequestSummary, int, error) {
			if id != rid.String() {
				return reviewclient.RequestSummary{}, http.StatusNotFound, nil
			}
			return reviewclient.RequestSummary{ID: rid.String(), Status: "approved"}, http.StatusOK, nil
		},
	}
	uc := NewUsecases(r, rev)
	created, err := uc.Create(ctx, CreateTaskInput{Title: "sync-test"})
	if err != nil {
		t.Fatal(err)
	}
	tid := created.ID
	created.Status = domain.TaskStatusWaitingForApproval
	if _, err := r.UpdateTask(ctx, created); err != nil {
		t.Fatal(err)
	}
	r.lastPropose[tid] = rid

	out, err := uc.SyncTaskReview(ctx, tid)
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != domain.TaskStatusDone {
		t.Fatalf("expected done, got %q", out.Status)
	}
	if out.ClosedAt == nil {
		t.Fatal("expected ClosedAt on terminal state")
	}
}
