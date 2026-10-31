# FAB durable orchestration design

FAB is a durable, harness-agnostic orchestration daemon for software-engineering agents.

## Core invariants

1. Agents do not directly mutate the canonical repository.
2. Every repository mutation originates as an immutable, single-outcome patch.
3. Patch IDs are task-local, monotonically increasing, and never reused.
4. A patch targets a known Git base revision.
5. Deterministic validation precedes review.
6. Only accepted patches may become commits.
7. A commit contains exactly the accepted change and at most one accepted patch.
8. Commit messages use the controlled change-type vocabulary and explain why the change exists.
9. Failed, rejected, and superseded patches remain auditable.
10. Durable artifacts carry immutable provenance observed by the orchestrator.
11. Harness/model choice is execution policy, not domain state.
12. Restarting the daemon must not lose committed history or silently repeat repository mutations.

## Commit types

`feat`, `fix`, `refactor`, `chore`, `perf`, `ci`, `ops`, `build`, `docs`, `style`, `revert`, and `test`.

## Persistence

SQLite is the MVP metadata store. It will run with foreign keys enabled, WAL mode, a busy timeout, and short write transactions. Large artifact content is kept behind a separate blob-store boundary.

## Harness boundary

The durable core owns tasks, artifacts, provenance, patch identity, validation, reviews, repository history, and commit creation. Harness adapters (Fake, Pi, ADK, Claude Code, Antigravity, Codex) may maintain ephemeral execution/session state but are not authoritative for durable workflow state.
