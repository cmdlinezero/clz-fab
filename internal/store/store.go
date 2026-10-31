package store

import (
	"context"
	"fab/internal/domain"
)

type Store interface {
	CreateTask(context.Context, domain.Task) (domain.Task, error)
	Task(context.Context, domain.TaskID) (domain.Task, error)
	CreateArtifact(context.Context, domain.Artifact) (domain.Artifact, error)
	Artifact(context.Context, domain.ArtifactRef) (domain.Artifact, error)
	CreatePatch(context.Context, domain.Patch) (domain.Patch, error)
	Patch(context.Context, domain.TaskID, uint64) (domain.Patch, error)
	TransitionPatch(context.Context, domain.TaskID, uint64, domain.PatchStatus) error
}
