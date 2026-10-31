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
	if a.Ref.TaskID == "" {
		return a, errors.New("artifact task id is required")
	}
	if a.Ref.ID == "" {
		id, err := s.AllocateArtifactID(ctx, a.Ref.TaskID)
		if err != nil {
			return a, err
		}
		a.Ref.ID = id
	}
	if a.Type == "" {
		return a, errors.New("artifact type is required")
	}

	now := time.Now().UTC()
	if a.CreatedAt.IsZero() {
		a.CreatedAt = now
	}
	if a.Provenance.CreatedAt.IsZero() {
		a.Provenance.CreatedAt = a.CreatedAt
	}
	if a.Provenance.TaskID == "" {
		a.Provenance.TaskID = a.Ref.TaskID
	}
	if a.Provenance.TaskID != a.Ref.TaskID {
		return a, errors.New("artifact provenance task id must match artifact task id")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return a, fmt.Errorf("begin artifact transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `INSERT INTO artifacts(
		task_id,id,type,content_ref,role,harness,harness_version,model_provider,model,
		session_id,base_commit,config_revision,created_at
	) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		a.Ref.TaskID, a.Ref.ID, a.Type, a.ContentRef,
		a.Provenance.Role, a.Provenance.Harness, a.Provenance.HarnessVersion,
		a.Provenance.ModelProvider, a.Provenance.Model, a.Provenance.SessionID,
		a.Provenance.BaseCommit, a.Provenance.ConfigRevision,
		a.Provenance.CreatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return a, fmt.Errorf("create artifact: %w", err)
	}

	for i, parent := range a.Parents {
		if parent.TaskID == "" || parent.ID == "" {
			return a, fmt.Errorf("artifact parent %d is incomplete", i)
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO artifact_parents(
			task_id,artifact_id,parent_task_id,parent_artifact_id,ordinal
		) VALUES(?,?,?,?,?)`, a.Ref.TaskID, a.Ref.ID, parent.TaskID, parent.ID, i)
		if err != nil {
			return a, fmt.Errorf("create artifact parent %d: %w", i, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return a, fmt.Errorf("commit artifact: %w", err)
	}
	return a, nil
}

func (s *Store) Artifact(ctx context.Context, ref domain.ArtifactRef) (domain.Artifact, error) {
	var a domain.Artifact
	var created string
	err := s.db.QueryRowContext(ctx, `SELECT
		task_id,id,type,content_ref,role,harness,harness_version,model_provider,model,
		session_id,base_commit,config_revision,created_at
		FROM artifacts WHERE task_id=? AND id=?`, ref.TaskID, ref.ID).Scan(
		&a.Ref.TaskID, &a.Ref.ID, &a.Type, &a.ContentRef,
		&a.Provenance.Role, &a.Provenance.Harness, &a.Provenance.HarnessVersion,
		&a.Provenance.ModelProvider, &a.Provenance.Model, &a.Provenance.SessionID,
		&a.Provenance.BaseCommit, &a.Provenance.ConfigRevision, &created,
	)
	if err != nil {
		return a, err
	}
	a.Provenance.TaskID = a.Ref.TaskID
	a.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return a, fmt.Errorf("parse artifact creation time: %w", err)
	}
	a.Provenance.CreatedAt = a.CreatedAt

	rows, err := s.db.QueryContext(ctx, `SELECT parent_task_id,parent_artifact_id
		FROM artifact_parents WHERE task_id=? AND artifact_id=? ORDER BY ordinal`, ref.TaskID, ref.ID)
	if err != nil {
		return a, fmt.Errorf("read artifact parents: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var parent domain.ArtifactRef
		if err := rows.Scan(&parent.TaskID, &parent.ID); err != nil {
			return a, fmt.Errorf("scan artifact parent: %w", err)
		}
		a.Parents = append(a.Parents, parent)
	}
	if err := rows.Err(); err != nil {
		return a, fmt.Errorf("read artifact parents: %w", err)
	}
	return a, nil
}

func (s *Store) CreatePatch(ctx context.Context, p domain.Patch) (domain.Patch, error) {
	if p.Artifact.Ref.TaskID == "" || p.Artifact.Ref.ID == "" {
		return p, errors.New("patch artifact reference is required")
	}
	if p.Type == "" || !p.Type.Valid() {
		return p, fmt.Errorf("invalid commit type %q", p.Type)
	}
	if p.Subject == "" {
		return p, errors.New("patch subject is required")
	}
	if p.BaseCommit == "" {
		return p, errors.New("patch base commit is required")
	}
	if p.Number == 0 {
		number, err := s.AllocatePatchNumber(ctx, p.Artifact.Ref.TaskID)
		if err != nil {
			return p, err
		}
		p.Number = number
	}
	if p.Status == "" {
		p.Status = domain.PatchProposed
	}
	now := time.Now().UTC()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	if p.UpdatedAt.IsZero() {
		p.UpdatedAt = p.CreatedAt
	}

	_, err := s.db.ExecContext(ctx, `INSERT INTO patches(
		task_id,number,artifact_id,base_commit,commit_type,scope,subject,intent,rationale,
		diff,supersedes,status,created_at,updated_at
	) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		p.Artifact.Ref.TaskID, p.Number, p.Artifact.Ref.ID, p.BaseCommit, p.Type, p.Scope,
		p.Subject, p.Intent, p.Rationale, p.Diff, p.Supersedes, p.Status,
		p.CreatedAt.Format(time.RFC3339Nano), p.UpdatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return p, fmt.Errorf("create patch: %w", err)
	}
	return p, nil
}

func (s *Store) Patch(ctx context.Context, taskID domain.TaskID, number uint64) (domain.Patch, error) {
	var p domain.Patch
	var artifactID domain.ArtifactID
	var supersedes sql.NullInt64
	var created, updated string
	err := s.db.QueryRowContext(ctx, `SELECT
		artifact_id,base_commit,commit_type,scope,subject,intent,rationale,diff,supersedes,
		status,created_at,updated_at
		FROM patches WHERE task_id=? AND number=?`, taskID, number).Scan(
		&artifactID, &p.BaseCommit, &p.Type, &p.Scope, &p.Subject, &p.Intent, &p.Rationale,
		&p.Diff, &supersedes, &p.Status, &created, &updated,
	)
	if err != nil {
		return p, err
	}
	p.Number = number
	if supersedes.Valid {
		value := uint64(supersedes.Int64)
		p.Supersedes = &value
	}
	p.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return p, fmt.Errorf("parse patch creation time: %w", err)
	}
	p.UpdatedAt, err = time.Parse(time.RFC3339Nano, updated)
	if err != nil {
		return p, fmt.Errorf("parse patch update time: %w", err)
	}
	p.Artifact, err = s.Artifact(ctx, domain.ArtifactRef{TaskID: taskID, ID: artifactID})
	if err != nil {
		return p, fmt.Errorf("read patch artifact: %w", err)
	}
	return p, nil
}

func (s *Store) TransitionPatch(ctx context.Context, taskID domain.TaskID, number uint64, to domain.PatchStatus) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin patch transition: %w", err)
	}
	defer tx.Rollback()

	var from domain.PatchStatus
	if err := tx.QueryRowContext(ctx, `SELECT status FROM patches WHERE task_id=? AND number=?`, taskID, number).Scan(&from); err != nil {
		return err
	}
	if err := domain.ValidatePatchTransition(from, to); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, `UPDATE patches SET status=?,updated_at=?
		WHERE task_id=? AND number=? AND status=?`, to, time.Now().UTC().Format(time.RFC3339Nano), taskID, number, from)
	if err != nil {
		return fmt.Errorf("transition patch: %w", err)
	}
	changed, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("read patch transition result: %w", err)
	}
	if changed != 1 {
		return errors.New("patch status changed concurrently")
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit patch transition: %w", err)
	}
	return nil
}

// AllocateArtifactID increments the task-local artifact counter in its own
// committed transaction. Once returned, an ID is never reused even if the
// subsequent artifact write fails.
func (s *Store) AllocateArtifactID(ctx context.Context, taskID domain.TaskID) (domain.ArtifactID, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	var next uint64
	if err := tx.QueryRowContext(ctx, `SELECT next_artifact_number FROM tasks WHERE id=?`, taskID).Scan(&next); err != nil {
		return "", fmt.Errorf("read artifact counter: %w", err)
	}
	res, err := tx.ExecContext(ctx, `UPDATE tasks SET next_artifact_number=next_artifact_number+1,updated_at=?
		WHERE id=? AND next_artifact_number=?`, time.Now().UTC().Format(time.RFC3339Nano), taskID, next)
	if err != nil {
		return "", fmt.Errorf("advance artifact counter: %w", err)
	}
	changed, err := res.RowsAffected()
	if err != nil {
		return "", err
	}
	if changed != 1 {
		return "", errors.New("artifact counter changed concurrently")
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return domain.ArtifactID(fmt.Sprintf("A%04d", next)), nil
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
