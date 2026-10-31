package domain

import "fmt"

var patchTransitions = map[PatchStatus]map[PatchStatus]bool{
	PatchProposed:   {PatchValidating: true},
	PatchValidating: {PatchInvalid: true, PatchValidated: true},
	PatchValidated:  {PatchReviewing: true},
	PatchReviewing:  {PatchRejected: true, PatchAccepted: true},
	PatchAccepted:   {PatchCommitting: true},
	PatchCommitting: {PatchCommitFailed: true, PatchCommitted: true},
}

func ValidatePatchTransition(from, to PatchStatus) error {
	if patchTransitions[from][to] {
		return nil
	}
	return fmt.Errorf("invalid patch transition %q -> %q", from, to)
}

func (t CommitType) Valid() bool {
	switch t {
	case CommitFeat, CommitFix, CommitRefactor, CommitChore, CommitPerf, CommitCI, CommitOps, CommitBuild, CommitDocs, CommitStyle, CommitRevert, CommitTest:
		return true
	default:
		return false
	}
}
