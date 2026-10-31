package sqlite_test

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"fab/internal/domain"
	storedb "fab/internal/store/sqlite"

	_ "modernc.org/sqlite"
)

func openStore(t *testing.T, path string) (*sql.DB, *storedb.Store) {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_busy_timeout=5000&_journal_mode=WAL", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	// agentd is the sole database writer. Serialising connections here mirrors
	// that ownership model while still exercising concurrent callers.
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })

	s := storedb.New(db)
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	return db, s
}

func createTask(t *testing.T, s *storedb.Store, id domain.TaskID) domain.Task {
	t.Helper()
	task, err := s.CreateTask(context.Background(), domain.Task{
		ID:         id,
		RepoPath:   "/tmp/repo",
		BaseCommit: "0123456789abcdef",
		Prompt:     "exercise durable storage",
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	return task
}

func TestArtifactRoundTripPreservesProvenanceAndParents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agentd.db")
	_, s := openStore(t, path)
	createTask(t, s, "T001")

	ctx := context.Background()
	parent, err := s.CreateArtifact(ctx, domain.Artifact{
		Ref:  domain.ArtifactRef{TaskID: "T001"},
		Type: domain.ArtifactInvestigation,
		Provenance: domain.ArtifactProvenance{
			Role:           "investigator",
			Harness:        "pi",
			HarnessVersion: "0.1.0",
			ModelProvider:  "local",
			Model:          "test-model",
			SessionID:      "session-1",
			BaseCommit:     "0123456789abcdef",
			ConfigRevision: "config-1",
		},
	})
	if err != nil {
		t.Fatalf("create parent artifact: %v", err)
	}

	child, err := s.CreateArtifact(ctx, domain.Artifact{
		Ref:        domain.ArtifactRef{TaskID: "T001"},
		Type:       domain.ArtifactPlan,
		ContentRef: "sha256:abc",
		Parents:    []domain.ArtifactRef{parent.Ref},
		Provenance: domain.ArtifactProvenance{
			Role:           "planner",
			Harness:        "adk",
			HarnessVersion: "2.6.0",
			ModelProvider:  "google",
			Model:          "test-model",
			SessionID:      "session-2",
			BaseCommit:     "0123456789abcdef",
			ConfigRevision: "config-2",
		},
	})
	if err != nil {
		t.Fatalf("create child artifact: %v", err)
	}

	got, err := s.Artifact(ctx, child.Ref)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	if got.Ref != child.Ref || got.Type != child.Type || got.ContentRef != child.ContentRef {
		t.Fatalf("artifact round trip mismatch: got %+v want %+v", got, child)
	}
	if got.Provenance != child.Provenance {
		t.Fatalf("provenance changed: got %+v want %+v", got.Provenance, child.Provenance)
	}
	if len(got.Parents) != 1 || got.Parents[0] != parent.Ref {
		t.Fatalf("parents changed: got %+v want [%+v]", got.Parents, parent.Ref)
	}
}

func TestStoreSurvivesDatabaseReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agentd.db")
	db, s := openStore(t, path)
	createTask(t, s, "T002")

	artifact, err := s.CreateArtifact(context.Background(), domain.Artifact{
		Ref:        domain.ArtifactRef{TaskID: "T002"},
		Type:       domain.ArtifactPlan,
		ContentRef: "sha256:durable",
		Provenance: domain.ArtifactProvenance{Role: "planner", Harness: "fake"},
	})
	if err != nil {
		t.Fatalf("create artifact: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}

	_, reopened := openStore(t, path)
	got, err := reopened.Artifact(context.Background(), artifact.Ref)
	if err != nil {
		t.Fatalf("read artifact after reopen: %v", err)
	}
	if got.ContentRef != artifact.ContentRef || got.Provenance != artifact.Provenance {
		t.Fatalf("artifact changed after reopen: got %+v want %+v", got, artifact)
	}
}

func TestArtifactIDIsNotReusedAfterFailedWrite(t *testing.T) {
	_, s := openStore(t, filepath.Join(t.TempDir(), "agentd.db"))
	createTask(t, s, "T003")
	ctx := context.Background()

	_, err := s.CreateArtifact(ctx, domain.Artifact{
		Ref:     domain.ArtifactRef{TaskID: "T003"},
		Type:    domain.ArtifactPlan,
		Parents: []domain.ArtifactRef{{TaskID: "T003", ID: "A9999"}},
	})
	if err == nil {
		t.Fatal("expected artifact creation with missing parent to fail")
	}

	artifact, err := s.CreateArtifact(ctx, domain.Artifact{
		Ref:  domain.ArtifactRef{TaskID: "T003"},
		Type: domain.ArtifactPlan,
	})
	if err != nil {
		t.Fatalf("create artifact after failed write: %v", err)
	}
	if artifact.Ref.ID != "A0002" {
		t.Fatalf("artifact ID was reused: got %s want A0002", artifact.Ref.ID)
	}
}

func TestConcurrentPatchAllocationIsMonotonicAndUnique(t *testing.T) {
	_, s := openStore(t, filepath.Join(t.TempDir(), "agentd.db"))
	createTask(t, s, "T004")

	const count = 32
	numbers := make(chan uint64, count)
	errs := make(chan error, count)
	var wg sync.WaitGroup
	wg.Add(count)
	for range count {
		go func() {
			defer wg.Done()
			n, err := s.AllocatePatchNumber(context.Background(), "T004")
			if err != nil {
				errs <- err
				return
			}
			numbers <- n
		}()
	}
	wg.Wait()
	close(numbers)
	close(errs)

	for err := range errs {
		t.Errorf("allocate patch number: %v", err)
	}
	if t.Failed() {
		return
	}

	got := make([]int, 0, count)
	for n := range numbers {
		got = append(got, int(n))
	}
	sort.Ints(got)
	if len(got) != count {
		t.Fatalf("allocated %d numbers, want %d", len(got), count)
	}
	for i, n := range got {
		want := i + 1
		if n != want {
			t.Fatalf("allocation[%d] = %d, want %d; all=%v", i, n, want, got)
		}
	}
}

func TestPatchRoundTripAndStateTransitions(t *testing.T) {
	_, s := openStore(t, filepath.Join(t.TempDir(), "agentd.db"))
	createTask(t, s, "T005")
	ctx := context.Background()

	artifact, err := s.CreateArtifact(ctx, domain.Artifact{
		Ref:  domain.ArtifactRef{TaskID: "T005"},
		Type: domain.ArtifactPatch,
		Provenance: domain.ArtifactProvenance{
			Role:       "coder",
			Harness:    "pi",
			BaseCommit: "deadbeef",
		},
	})
	if err != nil {
		t.Fatalf("create patch artifact: %v", err)
	}

	patch, err := s.CreatePatch(ctx, domain.Patch{
		Artifact:   artifact,
		BaseCommit: "deadbeef",
		Type:       domain.CommitFix,
		Scope:      "store",
		Subject:    "preserve patch state",
		Intent:     "Make patch state durable.",
		Rationale:  "Recovery depends on persisted state.",
		Diff:       []byte("diff --git a/a b/a\n"),
	})
	if err != nil {
		t.Fatalf("create patch: %v", err)
	}
	if patch.Number != 1 || patch.Status != domain.PatchProposed {
		t.Fatalf("unexpected initial patch: %+v", patch)
	}

	if err := s.TransitionPatch(ctx, "T005", patch.Number, domain.PatchValidated); err == nil {
		t.Fatal("expected illegal proposed -> validated transition to fail")
	}
	if err := s.TransitionPatch(ctx, "T005", patch.Number, domain.PatchValidating); err != nil {
		t.Fatalf("transition to validating: %v", err)
	}
	if err := s.TransitionPatch(ctx, "T005", patch.Number, domain.PatchValidated); err != nil {
		t.Fatalf("transition to validated: %v", err)
	}

	got, err := s.Patch(ctx, "T005", patch.Number)
	if err != nil {
		t.Fatalf("read patch: %v", err)
	}
	if got.Status != domain.PatchValidated {
		t.Fatalf("patch status = %s, want %s", got.Status, domain.PatchValidated)
	}
	if got.Artifact.Provenance != artifact.Provenance {
		t.Fatalf("patch provenance changed: got %+v want %+v", got.Artifact.Provenance, artifact.Provenance)
	}
	if string(got.Diff) != string(patch.Diff) {
		t.Fatalf("patch diff changed: got %q want %q", got.Diff, patch.Diff)
	}
}

func TestPatchNumbersContinueAfterReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agentd.db")
	db, s := openStore(t, path)
	createTask(t, s, "T006")
	ctx := context.Background()

	first, err := s.AllocatePatchNumber(ctx, "T006")
	if err != nil {
		t.Fatalf("allocate first patch number: %v", err)
	}
	if first != 1 {
		t.Fatalf("first patch number = %d, want 1", first)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}

	_, reopened := openStore(t, path)
	second, err := reopened.AllocatePatchNumber(ctx, "T006")
	if err != nil {
		t.Fatalf("allocate patch number after reopen: %v", err)
	}
	if second != 2 {
		t.Fatalf("patch number after reopen = %d, want 2", second)
	}
}

func TestArtifactTimestampsRoundTripAtNanosecondPrecision(t *testing.T) {
	_, s := openStore(t, filepath.Join(t.TempDir(), "agentd.db"))
	createTask(t, s, "T007")
	created := time.Date(2026, 10, 31, 14, 0, 1, 123456789, time.UTC)

	artifact, err := s.CreateArtifact(context.Background(), domain.Artifact{
		Ref:       domain.ArtifactRef{TaskID: "T007"},
		Type:      domain.ArtifactPlan,
		CreatedAt: created,
		Provenance: domain.ArtifactProvenance{
			CreatedAt: created,
		},
	})
	if err != nil {
		t.Fatalf("create artifact: %v", err)
	}
	got, err := s.Artifact(context.Background(), artifact.Ref)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	if !got.CreatedAt.Equal(created) || !got.Provenance.CreatedAt.Equal(created) {
		t.Fatalf("timestamp changed: got %s/%s want %s", got.CreatedAt, got.Provenance.CreatedAt, created)
	}
}
