package domain

import "time"

type TaskID string
type ArtifactID string

type ArtifactRef struct {
	TaskID TaskID
	ID     ArtifactID
}

type TaskStatus string

const (
	TaskQueued    TaskStatus = "queued"
	TaskRunning   TaskStatus = "running"
	TaskWaiting   TaskStatus = "waiting"
	TaskCompleted TaskStatus = "completed"
	TaskFailed    TaskStatus = "failed"
	TaskCancelled TaskStatus = "cancelled"
)

type ArtifactType string

const (
	ArtifactInvestigation ArtifactType = "investigation"
	ArtifactPlan          ArtifactType = "plan"
	ArtifactPatch         ArtifactType = "patch"
	ArtifactValidation    ArtifactType = "validation"
	ArtifactReview        ArtifactType = "review"
	ArtifactCommit        ArtifactType = "commit"
)

type CommitType string

const (
	CommitFeat     CommitType = "feat"
	CommitFix      CommitType = "fix"
	CommitRefactor CommitType = "refactor"
	CommitChore    CommitType = "chore"
	CommitPerf     CommitType = "perf"
	CommitCI       CommitType = "ci"
	CommitOps      CommitType = "ops"
	CommitBuild    CommitType = "build"
	CommitDocs     CommitType = "docs"
	CommitStyle    CommitType = "style"
	CommitRevert   CommitType = "revert"
	CommitTest     CommitType = "test"
)

type PatchStatus string

const (
	PatchProposed     PatchStatus = "proposed"
	PatchValidating   PatchStatus = "validating"
	PatchInvalid      PatchStatus = "invalid"
	PatchValidated    PatchStatus = "validated"
	PatchReviewing    PatchStatus = "reviewing"
	PatchRejected     PatchStatus = "rejected"
	PatchAccepted     PatchStatus = "accepted"
	PatchCommitting   PatchStatus = "committing"
	PatchCommitFailed PatchStatus = "commit_failed"
	PatchCommitted    PatchStatus = "committed"
)

type Task struct {
	ID         TaskID
	RepoPath   string
	BaseCommit string
	Prompt     string
	Status     TaskStatus
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type ArtifactProvenance struct {
	TaskID         TaskID
	Role           string
	Harness        string
	HarnessVersion string
	ModelProvider  string
	Model          string
	SessionID      string
	BaseCommit     string
	ConfigRevision string
	CreatedAt      time.Time
}

type Artifact struct {
	Ref        ArtifactRef
	Type       ArtifactType
	ContentRef string
	Provenance ArtifactProvenance
	Parents    []ArtifactRef
	CreatedAt  time.Time
}

type Patch struct {
	Artifact   Artifact
	Number     uint64
	BaseCommit string
	Type       CommitType
	Scope      string
	Subject    string
	Intent     string
	Rationale  string
	Diff       []byte
	Supersedes *uint64
	Status     PatchStatus
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
