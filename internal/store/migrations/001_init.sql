-- PNC Redirect Engine schema

CREATE TABLE IF NOT EXISTS domains (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT UNIQUE NOT NULL,
    default_url TEXT NOT NULL,
    status_code INTEGER DEFAULT 301,
    enabled     INTEGER DEFAULT 1,
    created_at  TEXT DEFAULT (datetime('now')),
    updated_at  TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS redirect_rules (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    domain_id   INTEGER NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    path        TEXT NOT NULL,
    target_url  TEXT NOT NULL,
    status_code INTEGER DEFAULT 301,
    priority    INTEGER DEFAULT 0,
    enabled     INTEGER DEFAULT 1,
    created_at  TEXT DEFAULT (datetime('now')),
    updated_at  TEXT DEFAULT (datetime('now')),
    UNIQUE(domain_id, path)
);

CREATE TABLE IF NOT EXISTS request_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    domain      TEXT NOT NULL,
    path        TEXT NOT NULL,
    query       TEXT,
    status_code INTEGER NOT NULL,
    target_url  TEXT,
    client_ip   TEXT,
    user_agent  TEXT,
    referer     TEXT,
    created_at  TEXT DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_request_log_domain_created ON request_log(domain, created_at);
CREATE INDEX IF NOT EXISTS idx_request_log_created ON request_log(created_at);

CREATE TABLE IF NOT EXISTS domain_stats (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    domain      TEXT NOT NULL,
    bucket      TEXT NOT NULL,
    hit_count   INTEGER DEFAULT 0,
    unique_ips  INTEGER DEFAULT 0,
    top_paths   TEXT,
    top_referers TEXT,
    UNIQUE(domain, bucket)
);

CREATE INDEX IF NOT EXISTS idx_domain_stats_domain_bucket ON domain_stats(domain, bucket);
CREATE INDEX IF NOT EXISTS idx_domain_stats_bucket ON domain_stats(bucket);
