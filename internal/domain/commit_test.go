package domain

import (
	"strings"
	"testing"
)

func TestCommitMessageRequiresTypeSubjectAndWhy(t *testing.T) {
	for _, m := range []CommitMessage{{Subject: "add store", Why: "needed"}, {Type: CommitFeat, Why: "needed"}, {Type: CommitFeat, Subject: "add store"}} {
		if err := m.Validate(); err == nil {
			t.Fatalf("expected validation failure for %#v", m)
		}
	}
}

func TestCommitMessageRender(t *testing.T) {
	m := CommitMessage{Type: CommitFeat, Scope: "store", Subject: "add transactional patch allocation", Why: "Prevent duplicate patch identifiers during concurrent task execution.", Details: "Persist allocation with task state so numbering survives daemon restarts."}
	got, err := m.Render()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "feat(store): add transactional patch allocation\n\n") {
		t.Fatalf("unexpected message: %q", got)
	}
	if !strings.Contains(got, "Prevent duplicate patch identifiers") {
		t.Fatal("missing rationale")
	}
}
