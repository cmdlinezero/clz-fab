PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS tasks (
    id TEXT PRIMARY KEY,
    repo_path TEXT NOT NULL,
    base_commit TEXT NOT NULL,
    prompt TEXT NOT NULL,
    status TEXT NOT NULL,
    next_artifact_number INTEGER NOT NULL DEFAULT 1,
    next_patch_number INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS artifacts (
    task_id TEXT NOT NULL REFERENCES tasks(id),
    id TEXT NOT NULL,
    type TEXT NOT NULL,
    content_ref TEXT NOT NULL DEFAULT '',
    role TEXT NOT NULL DEFAULT '',
    harness TEXT NOT NULL DEFAULT '',
    harness_version TEXT NOT NULL DEFAULT '',
    model_provider TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    session_id TEXT NOT NULL DEFAULT '',
    base_commit TEXT NOT NULL DEFAULT '',
    config_revision TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    PRIMARY KEY (task_id, id)
);

CREATE TABLE IF NOT EXISTS artifact_parents (
    task_id TEXT NOT NULL,
    artifact_id TEXT NOT NULL,
    parent_task_id TEXT NOT NULL,
    parent_artifact_id TEXT NOT NULL,
    ordinal INTEGER NOT NULL,
    PRIMARY KEY (task_id, artifact_id, ordinal),
    FOREIGN KEY (task_id, artifact_id) REFERENCES artifacts(task_id, id),
    FOREIGN KEY (parent_task_id, parent_artifact_id) REFERENCES artifacts(task_id, id)
);

CREATE TABLE IF NOT EXISTS patches (
    task_id TEXT NOT NULL,
    number INTEGER NOT NULL,
    artifact_id TEXT NOT NULL,
    base_commit TEXT NOT NULL,
    commit_type TEXT NOT NULL,
    scope TEXT NOT NULL DEFAULT '',
    subject TEXT NOT NULL,
    intent TEXT NOT NULL,
    rationale TEXT NOT NULL,
    diff BLOB NOT NULL,
    supersedes INTEGER,
    status TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    PRIMARY KEY (task_id, number),
    UNIQUE (task_id, artifact_id),
    FOREIGN KEY (task_id, artifact_id) REFERENCES artifacts(task_id, id),
    FOREIGN KEY (task_id, supersedes) REFERENCES patches(task_id, number)
);

CREATE TABLE IF NOT EXISTS events (
    sequence INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id TEXT NOT NULL REFERENCES tasks(id),
    type TEXT NOT NULL,
    payload TEXT NOT NULL,
    created_at TEXT NOT NULL
);
