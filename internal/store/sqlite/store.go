package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"fab/internal/domain"
)

type Store struct{ db *sql.DB }

func New(db *sql.DB) *Store { return &Store{db: db} }

func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, coreMigration); err != nil {
		return fmt.Errorf("apply core migration: %w", err)
	}
	return nil
}

func (s *Store) CreateTask(ctx context.Context, t domain.Task) (domain.Task, error) {
	if t.ID == "" {
		return t, errors.New("task id is required")
	}
	now := time.Now().UTC()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	if t.UpdatedAt.IsZero() {
		t.UpdatedAt = t.CreatedAt
	}
	if t.Status == "" {
		t.Status = domain.TaskQueued
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO tasks(id,repo_path,base_commit,prompt,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, t.ID, t.RepoPath, t.BaseCommit, t.Prompt, t.Status, t.CreatedAt.Format(time.RFC3339Nano), t.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return t, fmt.Errorf("create task: %w", err)
	}
	return t, nil
}

func (s *Store) Task(ctx context.Context, id domain.TaskID) (domain.Task, error) {
	var t domain.Task
	var created, updated string
	err := s.db.QueryRowContext(ctx, `SELECT id,repo_path,base_commit,prompt,status,created_at,updated_at FROM tasks WHERE id=?`, id).Scan(&t.ID, &t.RepoPath, &t.BaseCommit, &t.Prompt, &t.Status, &created, &updated)
	if err != nil {
		return t, err
	}
	t.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	t.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	return t, nil
}

func (s *Store) CreateArtifact(ctx context.Context, a domain.Artifact) (domain.Artifact, error) {
	return a, errors.New("CreateArtifact not implemented in core scaffold")
}
func (s *Store) Artifact(ctx context.Context, ref domain.ArtifactRef) (domain.Artifact, error) {
	return domain.Artifact{}, errors.New("Artifact not implemented in core scaffold")
}
func (s *Store) CreatePatch(ctx context.Context, p domain.Patch) (domain.Patch, error) {
	return p, errors.New("CreatePatch not implemented in core scaffold")
}
func (s *Store) Patch(ctx context.Context, taskID domain.TaskID, number uint64) (domain.Patch, error) {
	return domain.Patch{}, errors.New("Patch not implemented in core scaffold")
}
func (s *Store) TransitionPatch(ctx context.Context, taskID domain.TaskID, number uint64, to domain.PatchStatus) error {
	return errors.New("TransitionPatch not implemented in core scaffold")
}

// AllocatePatchNumber increments the task-local counter in the same transaction
// that returns the allocated value. Numbers are never reused after commit.
func (s *Store) AllocatePatchNumber(ctx context.Context, taskID domain.TaskID) (uint64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var next uint64
	if err := tx.QueryRowContext(ctx, `SELECT next_patch_number FROM tasks WHERE id=?`, taskID).Scan(&next); err != nil {
		return 0, fmt.Errorf("read patch counter: %w", err)
	}
	res, err := tx.ExecContext(ctx, `UPDATE tasks SET next_patch_number=next_patch_number+1, updated_at=? WHERE id=? AND next_patch_number=?`, time.Now().UTC().Format(time.RFC3339Nano), taskID, next)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	if n != 1 {
		return 0, errors.New("patch counter changed concurrently")
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return next, nil
}
