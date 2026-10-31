package domain

import (
	"fmt"
	"strings"
)

type CommitMessage struct {
	Type    CommitType
	Scope   string
	Subject string
	Why     string
	Details string
}

func (m CommitMessage) Validate() error {
	if !m.Type.Valid() {
		return fmt.Errorf("invalid commit type %q", m.Type)
	}
	if strings.TrimSpace(m.Subject) == "" {
		return fmt.Errorf("commit subject is required")
	}
	if strings.TrimSpace(m.Why) == "" {
		return fmt.Errorf("commit rationale is required")
	}
	if strings.Contains(m.Subject, "\n") {
		return fmt.Errorf("commit subject must be one line")
	}
	return nil
}

func (m CommitMessage) Render() (string, error) {
	if err := m.Validate(); err != nil {
		return "", err
	}
	head := string(m.Type)
	if m.Scope != "" {
		head += "(" + m.Scope + ")"
	}
	head += ": " + strings.TrimSpace(m.Subject)
	parts := []string{head, strings.TrimSpace(m.Why)}
	if d := strings.TrimSpace(m.Details); d != "" {
		parts = append(parts, d)
	}
	return strings.Join(parts, "\n\n") + "\n", nil
}
