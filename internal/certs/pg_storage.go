package certs

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/caddyserver/certmagic"
)

// PGStorage implements certmagic.Storage backed by PostgreSQL.
// Uses the cert_store table (key TEXT PK, value BYTEA, updated_at TIMESTAMPTZ).
// Locking uses PG advisory locks keyed on a hash of the lock name.
type PGStorage struct {
	db *sql.DB
}

var _ certmagic.Storage = (*PGStorage)(nil)

func NewPGStorage(db *sql.DB) *PGStorage {
	return &PGStorage{db: db}
}

func (s *PGStorage) Lock(ctx context.Context, name string) error {
	_, err := s.db.ExecContext(ctx, "SELECT pg_advisory_lock($1)", s.lockKey(name))
	return err
}

func (s *PGStorage) Unlock(ctx context.Context, name string) error {
	_, err := s.db.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", s.lockKey(name))
	return err
}

func (s *PGStorage) Store(ctx context.Context, key string, value []byte) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO cert_store (key, value, updated_at) VALUES ($1, $2, NOW())
		 ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = NOW()`,
		key, value,
	)
	return err
}

func (s *PGStorage) Load(ctx context.Context, key string) ([]byte, error) {
	var value []byte
	err := s.db.QueryRowContext(ctx, "SELECT value FROM cert_store WHERE key = $1", key).Scan(&value)
	if err == sql.ErrNoRows {
		return nil, fs.ErrNotExist
	}
	return value, err
}

func (s *PGStorage) Delete(ctx context.Context, key string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM cert_store WHERE key = $1", key)
	return err
}

func (s *PGStorage) Exists(ctx context.Context, key string) bool {
	var n int
	err := s.db.QueryRowContext(ctx, "SELECT 1 FROM cert_store WHERE key = $1", key).Scan(&n)
	return err == nil
}

func (s *PGStorage) List(ctx context.Context, prefix string, recursive bool) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT key FROM cert_store WHERE key LIKE $1 ORDER BY key", prefix+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		if !recursive {
			// Only include keys at the immediate level (no additional path separators beyond prefix)
			suffix := strings.TrimPrefix(key, prefix)
			if i := strings.Index(suffix, "/"); i >= 0 {
				// Include the directory name but deduplicate
				dir := prefix + suffix[:i+1]
				if len(keys) == 0 || keys[len(keys)-1] != dir {
					keys = append(keys, dir)
				}
				continue
			}
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

func (s *PGStorage) Stat(ctx context.Context, key string) (certmagic.KeyInfo, error) {
	var updatedAt time.Time
	var value []byte
	err := s.db.QueryRowContext(ctx, "SELECT value, updated_at FROM cert_store WHERE key = $1", key).Scan(&value, &updatedAt)
	if err == sql.ErrNoRows {
		return certmagic.KeyInfo{}, fs.ErrNotExist
	}
	if err != nil {
		return certmagic.KeyInfo{}, err
	}
	return certmagic.KeyInfo{
		Key:        key,
		Modified:   updatedAt,
		Size:       int64(len(value)),
		IsTerminal: !strings.HasSuffix(key, "/"),
	}, nil
}

// lockKey generates a stable int64 hash for PG advisory locks.
func (s *PGStorage) lockKey(name string) int64 {
	var h int64
	for _, c := range name {
		h = h*31 + int64(c)
	}
	if h < 0 {
		h = -h
	}
	return h
}

func init() {
	// Verify interface compliance at compile time
	var _ fmt.Stringer = (*PGStorage)(nil)
}

func (s *PGStorage) String() string {
	return "PGStorage"
}
