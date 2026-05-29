CREATE TABLE IF NOT EXISTS sources (
    id INTEGER PRIMARY KEY,
    kind TEXT NOT NULL,
    root_path TEXT NOT NULL,
    display_name TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(kind, root_path)
);

CREATE TABLE IF NOT EXISTS projects (
    id INTEGER PRIMARY KEY,
    canonical_path TEXT,
    slug TEXT NOT NULL,
    name TEXT,
    git_root TEXT,
    remote_url TEXT,
    first_seen_at TEXT,
    last_seen_at TEXT,
    UNIQUE(slug)
);

CREATE TABLE IF NOT EXISTS source_files (
    id INTEGER PRIMARY KEY,
    source_id INTEGER NOT NULL REFERENCES sources(id),
    project_id INTEGER REFERENCES projects(id),
    path TEXT NOT NULL,
    kind TEXT NOT NULL,
    size_bytes INTEGER,
    mtime_unix INTEGER,
    sha256 TEXT,
    indexed_at TEXT,
    parser_version TEXT NOT NULL,
    UNIQUE(source_id, path)
);

CREATE TABLE IF NOT EXISTS conversations (
    id INTEGER PRIMARY KEY,
    source_id INTEGER NOT NULL REFERENCES sources(id),
    project_id INTEGER REFERENCES projects(id),
    source_file_id INTEGER REFERENCES source_files(id),
    external_id TEXT NOT NULL,
    parent_external_id TEXT,
    title TEXT,
    started_at TEXT,
    ended_at TEXT,
    message_count INTEGER NOT NULL DEFAULT 0,
    is_subagent INTEGER NOT NULL DEFAULT 0,
    raw_path TEXT NOT NULL,
    UNIQUE(source_id, external_id)
);

CREATE TABLE IF NOT EXISTS messages (
    id INTEGER PRIMARY KEY,
    conversation_id INTEGER NOT NULL REFERENCES conversations(id),
    source_file_id INTEGER REFERENCES source_files(id),
    role TEXT NOT NULL,
    seq INTEGER NOT NULL,
    created_at TEXT,
    text TEXT,
    raw_json TEXT NOT NULL,
    raw_line INTEGER,
    content_hash TEXT NOT NULL,
    UNIQUE(conversation_id, seq),
    UNIQUE(source_file_id, raw_line)
);

CREATE TABLE IF NOT EXISTS message_blocks (
    id INTEGER PRIMARY KEY,
    message_id INTEGER NOT NULL REFERENCES messages(id),
    seq INTEGER NOT NULL,
    type TEXT NOT NULL,
    text TEXT,
    raw_json TEXT,
    UNIQUE(message_id, seq)
);

CREATE TABLE IF NOT EXISTS tool_calls (
    id INTEGER PRIMARY KEY,
    message_id INTEGER NOT NULL REFERENCES messages(id),
    block_id INTEGER REFERENCES message_blocks(id),
    tool_name TEXT NOT NULL,
    arguments_json TEXT,
    working_directory TEXT,
    status TEXT,
    created_at TEXT
);

CREATE TABLE IF NOT EXISTS files (
    id INTEGER PRIMARY KEY,
    project_id INTEGER REFERENCES projects(id),
    path TEXT NOT NULL,
    normalized_path TEXT,
    kind TEXT,
    basename TEXT,
    UNIQUE(project_id, path)
);

CREATE TABLE IF NOT EXISTS file_mentions (
    id INTEGER PRIMARY KEY,
    file_id INTEGER NOT NULL REFERENCES files(id),
    message_id INTEGER REFERENCES messages(id),
    tool_call_id INTEGER REFERENCES tool_calls(id),
    mention_kind TEXT NOT NULL,
    line_start INTEGER,
    line_end INTEGER,
    snippet TEXT
);

CREATE TABLE IF NOT EXISTS parse_errors (
    id INTEGER PRIMARY KEY,
    source_file_id INTEGER REFERENCES source_files(id),
    raw_line INTEGER,
    parser_version TEXT NOT NULL,
    error TEXT NOT NULL,
    raw_hash TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE VIRTUAL TABLE IF NOT EXISTS message_fts USING fts5(
    text,
    content='messages',
    content_rowid='id',
    tokenize='unicode61'
);

CREATE INDEX IF NOT EXISTS idx_conversations_project_time ON conversations(project_id, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_messages_conversation_seq ON messages(conversation_id, seq);
CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at);
CREATE INDEX IF NOT EXISTS idx_file_mentions_file ON file_mentions(file_id);
CREATE INDEX IF NOT EXISTS idx_files_project_basename ON files(project_id, basename);

CREATE TRIGGER IF NOT EXISTS messages_ai AFTER INSERT ON messages BEGIN
    INSERT INTO message_fts(rowid, text) VALUES (new.id, coalesce(new.text, ''));
END;

CREATE TRIGGER IF NOT EXISTS messages_ad AFTER DELETE ON messages BEGIN
    INSERT INTO message_fts(message_fts, rowid, text) VALUES('delete', old.id, coalesce(old.text, ''));
END;

CREATE TRIGGER IF NOT EXISTS messages_au AFTER UPDATE ON messages BEGIN
    INSERT INTO message_fts(message_fts, rowid, text) VALUES('delete', old.id, coalesce(old.text, ''));
    INSERT INTO message_fts(rowid, text) VALUES (new.id, coalesce(new.text, ''));
END;
