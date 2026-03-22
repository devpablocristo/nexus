package tasks

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/devpablocristo/nexus/v3/companion/internal/reviewclient"
	domain "github.com/devpablocristo/nexus/v3/companion/internal/tasks/usecases/domain"
)

type fakeRepo struct {
	tasks map[uuid.UUID]domain.Task
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
	return uuid.Nil, ErrNotFound
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

type stubReview struct{}

func (s *stubReview) SubmitRequest(ctx context.Context, idempotencyKey string, body reviewclient.SubmitRequestBody) (reviewclient.SubmitResponse, error) {
	return reviewclient.SubmitResponse{}, nil
}

func (s *stubReview) GetRequest(ctx context.Context, id uuid.UUID) (reviewclient.RequestSummary, int, error) {
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
