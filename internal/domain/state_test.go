package domain

import "testing"

func TestPatchTransitions(t *testing.T) {
	valid := [][2]PatchStatus{{PatchProposed, PatchValidating}, {PatchValidating, PatchValidated}, {PatchValidated, PatchReviewing}, {PatchReviewing, PatchAccepted}, {PatchAccepted, PatchCommitting}, {PatchCommitting, PatchCommitted}}
	for _, tt := range valid {
		if err := ValidatePatchTransition(tt[0], tt[1]); err != nil {
			t.Errorf("expected %s -> %s: %v", tt[0], tt[1], err)
		}
	}
	if err := ValidatePatchTransition(PatchProposed, PatchCommitted); err == nil {
		t.Fatal("expected direct proposed -> committed transition to fail")
	}
	if err := ValidatePatchTransition(PatchRejected, PatchValidating); err == nil {
		t.Fatal("expected rejected state to be terminal")
	}
}
