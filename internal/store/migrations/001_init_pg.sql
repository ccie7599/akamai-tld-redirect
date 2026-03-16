-- PNC Redirect Engine schema (PostgreSQL)

CREATE TABLE IF NOT EXISTS domains (
    id          SERIAL PRIMARY KEY,
    name        TEXT UNIQUE NOT NULL,
    default_url TEXT NOT NULL,
    status_code INTEGER DEFAULT 301,
    enabled     BOOLEAN DEFAULT TRUE,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS redirect_rules (
    id          SERIAL PRIMARY KEY,
    domain_id   INTEGER NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    path        TEXT NOT NULL,
    target_url  TEXT NOT NULL,
    status_code INTEGER DEFAULT 301,
    priority    INTEGER DEFAULT 0,
    enabled     BOOLEAN DEFAULT TRUE,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(domain_id, path)
);

CREATE TABLE IF NOT EXISTS request_log (
    id          SERIAL PRIMARY KEY,
    domain      TEXT NOT NULL,
    path        TEXT NOT NULL,
    query       TEXT,
    status_code INTEGER NOT NULL,
    target_url  TEXT,
    client_ip   TEXT,
    user_agent  TEXT,
    referer     TEXT,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_request_log_domain_created ON request_log(domain, created_at);
CREATE INDEX IF NOT EXISTS idx_request_log_created ON request_log(created_at);

CREATE TABLE IF NOT EXISTS domain_stats (
    id           SERIAL PRIMARY KEY,
    domain       TEXT NOT NULL,
    bucket       TIMESTAMPTZ NOT NULL,
    hit_count    INTEGER DEFAULT 0,
    unique_ips   INTEGER DEFAULT 0,
    top_paths    TEXT,
    top_referers TEXT,
    UNIQUE(domain, bucket)
);

CREATE INDEX IF NOT EXISTS idx_domain_stats_domain_bucket ON domain_stats(domain, bucket);
CREATE INDEX IF NOT EXISTS idx_domain_stats_bucket ON domain_stats(bucket);

CREATE TABLE IF NOT EXISTS cert_store (
    key        TEXT PRIMARY KEY,
    value      BYTEA NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
