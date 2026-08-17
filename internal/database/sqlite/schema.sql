CREATE TABLE IF NOT EXISTS notes (
    id TEXT PRIMARY KEY,
    dir TEXT NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    tags TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS note_tags (
    note_id TEXT NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
    tag_name TEXT NOT NULL,
    PRIMARY KEY (note_id, tag_name)
);

CREATE VIRTUAL TABLE IF NOT EXISTS note_search USING fts5(
    id UNINDEXED,
    title,
    content
);
