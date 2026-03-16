package store

import (
	"database/sql"
	"embed"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/mattn/go-sqlite3"
)

const (
	DialectSQLite   = "sqlite"
	DialectPostgres = "postgres"

	SqliteTimeFmt = "2006-01-02 15:04:05"
	PgTimeFmt     = time.RFC3339
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Domain struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	DefaultURL string `json:"default_url"`
	StatusCode int    `json:"status_code"`
	Enabled    bool   `json:"enabled"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

type RedirectRule struct {
	ID         int64  `json:"id"`
	DomainID   int64  `json:"domain_id"`
	Path       string `json:"path"`
	TargetURL  string `json:"target_url"`
	StatusCode int    `json:"status_code"`
	Priority   int    `json:"priority"`
	Enabled    bool   `json:"enabled"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

type RequestLogEntry struct {
	Domain    string
	Path      string
	Query     string
	Status    int
	TargetURL string
	ClientIP  string
	UserAgent string
	Referer   string
}

type DomainWithRules struct {
	Domain
	Rules    []RedirectRule `json:"rules"`
	HitCount int64          `json:"hit_count,omitempty"`
}

type Store struct {
	db      *sql.DB
	dialect string
}

// New creates a Store backed by SQLite (for local dev).
func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	s := &Store{db: db, dialect: DialectSQLite}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

// NewPG creates a Store backed by PostgreSQL.
func NewPG(connStr string) (*Store, error) {
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return nil, fmt.Errorf("open pg: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping pg: %w", err)
	}
	s := &Store{db: db, dialect: DialectPostgres}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *Store) Dialect() string { return s.dialect }
func (s *Store) DB() *sql.DB    { return s.db }
func (s *Store) Close() error   { return s.db.Close() }

func (s *Store) migrate() error {
	var file string
	if s.dialect == DialectPostgres {
		file = "migrations/001_init_pg.sql"
	} else {
		file = "migrations/001_init.sql"
	}
	schema, err := migrationsFS.ReadFile(file)
	if err != nil {
		return fmt.Errorf("read migration: %w", err)
	}
	_, err = s.db.Exec(string(schema))
	return err
}

// q rewrites ? placeholders to $1, $2, ... for PostgreSQL.
func (s *Store) q(query string) string {
	if s.dialect == DialectSQLite {
		return query
	}
	var b strings.Builder
	n := 0
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			n++
			fmt.Fprintf(&b, "$%d", n)
		} else {
			b.WriteByte(query[i])
		}
	}
	return b.String()
}

// now returns the SQL expression for current timestamp.
func (s *Store) now() string {
	if s.dialect == DialectPostgres {
		return "NOW()"
	}
	return "datetime('now')"
}

// TimeFmt returns the time format string for the current dialect.
func (s *Store) TimeFmt() string {
	if s.dialect == DialectPostgres {
		return PgTimeFmt
	}
	return SqliteTimeFmt
}

// boolVal returns the appropriate boolean value for the dialect.
func (s *Store) boolVal(b bool) any {
	if s.dialect == DialectPostgres {
		return b
	}
	if b {
		return 1
	}
	return 0
}

// scanBool scans a boolean from either INTEGER (SQLite) or BOOLEAN (PG).
type scanBool struct {
	b *bool
}

func (sb *scanBool) Scan(src any) error {
	switch v := src.(type) {
	case bool:
		*sb.b = v
	case int64:
		*sb.b = v == 1
	case nil:
		*sb.b = false
	}
	return nil
}

// scanTime scans a timestamp from either TEXT (SQLite) or time.Time (PG) into a string.
type scanTime struct {
	s *string
}

func (st *scanTime) Scan(src any) error {
	switch v := src.(type) {
	case time.Time:
		*st.s = v.UTC().Format(SqliteTimeFmt)
	case string:
		*st.s = v
	case []byte:
		*st.s = string(v)
	case nil:
		*st.s = ""
	}
	return nil
}

// --- Domain CRUD ---

func (s *Store) ListDomains(search string, limit, offset int) ([]Domain, int, error) {
	where := ""
	args := []any{}
	if search != "" {
		where = "WHERE name LIKE ?"
		args = append(args, "%"+search+"%")
	}

	var total int
	countQ := s.q("SELECT COUNT(*) FROM domains " + where)
	if err := s.db.QueryRow(countQ, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	if limit <= 0 {
		limit = 50
	}
	q := s.q(fmt.Sprintf("SELECT id, name, default_url, status_code, enabled, created_at, updated_at FROM domains %s ORDER BY name LIMIT ? OFFSET ?", where))
	args = append(args, limit, offset)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var domains []Domain
	for rows.Next() {
		var d Domain
		if err := rows.Scan(&d.ID, &d.Name, &d.DefaultURL, &d.StatusCode,
			&scanBool{&d.Enabled}, &scanTime{&d.CreatedAt}, &scanTime{&d.UpdatedAt}); err != nil {
			return nil, 0, err
		}
		domains = append(domains, d)
	}
	return domains, total, rows.Err()
}

func (s *Store) GetDomain(name string) (*DomainWithRules, error) {
	d := &DomainWithRules{}
	err := s.db.QueryRow(
		s.q("SELECT id, name, default_url, status_code, enabled, created_at, updated_at FROM domains WHERE name = ?"),
		name,
	).Scan(&d.ID, &d.Name, &d.DefaultURL, &d.StatusCode,
		&scanBool{&d.Enabled}, &scanTime{&d.CreatedAt}, &scanTime{&d.UpdatedAt})
	if err != nil {
		return nil, err
	}

	rules, err := s.ListRules(d.ID)
	if err != nil {
		return nil, err
	}
	d.Rules = rules
	return d, nil
}

func (s *Store) CreateDomain(d *Domain) error {
	if s.dialect == DialectPostgres {
		return s.db.QueryRow(
			"INSERT INTO domains (name, default_url, status_code, enabled) VALUES ($1, $2, $3, $4) RETURNING id, created_at, updated_at",
			d.Name, d.DefaultURL, d.StatusCode, d.Enabled,
		).Scan(&d.ID, &scanTime{&d.CreatedAt}, &scanTime{&d.UpdatedAt})
	}
	res, err := s.db.Exec(
		"INSERT INTO domains (name, default_url, status_code, enabled) VALUES (?, ?, ?, ?)",
		d.Name, d.DefaultURL, d.StatusCode, boolToInt(d.Enabled),
	)
	if err != nil {
		return err
	}
	d.ID, _ = res.LastInsertId()
	return s.db.QueryRow("SELECT created_at, updated_at FROM domains WHERE id = ?", d.ID).Scan(&d.CreatedAt, &d.UpdatedAt)
}

func (s *Store) UpdateDomain(name string, d *Domain) error {
	q := fmt.Sprintf("UPDATE domains SET default_url = ?, status_code = ?, enabled = ?, updated_at = %s WHERE name = ?", s.now())
	_, err := s.db.Exec(s.q(q), d.DefaultURL, d.StatusCode, s.boolVal(d.Enabled), name)
	return err
}

func (s *Store) DeleteDomain(name string) error {
	_, err := s.db.Exec(s.q("DELETE FROM domains WHERE name = ?"), name)
	return err
}

// --- Redirect Rule CRUD ---

func (s *Store) ListRules(domainID int64) ([]RedirectRule, error) {
	rows, err := s.db.Query(
		s.q("SELECT id, domain_id, path, target_url, status_code, priority, enabled, created_at, updated_at FROM redirect_rules WHERE domain_id = ? ORDER BY priority DESC"),
		domainID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []RedirectRule
	for rows.Next() {
		var r RedirectRule
		if err := rows.Scan(&r.ID, &r.DomainID, &r.Path, &r.TargetURL, &r.StatusCode, &r.Priority,
			&scanBool{&r.Enabled}, &scanTime{&r.CreatedAt}, &scanTime{&r.UpdatedAt}); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

func (s *Store) CreateRule(r *RedirectRule) error {
	if s.dialect == DialectPostgres {
		return s.db.QueryRow(
			"INSERT INTO redirect_rules (domain_id, path, target_url, status_code, priority, enabled) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id",
			r.DomainID, r.Path, r.TargetURL, r.StatusCode, r.Priority, r.Enabled,
		).Scan(&r.ID)
	}
	res, err := s.db.Exec(
		"INSERT INTO redirect_rules (domain_id, path, target_url, status_code, priority, enabled) VALUES (?, ?, ?, ?, ?, ?)",
		r.DomainID, r.Path, r.TargetURL, r.StatusCode, r.Priority, boolToInt(r.Enabled),
	)
	if err != nil {
		return err
	}
	r.ID, _ = res.LastInsertId()
	return nil
}

func (s *Store) UpdateRule(id int64, r *RedirectRule) error {
	q := fmt.Sprintf("UPDATE redirect_rules SET path = ?, target_url = ?, status_code = ?, priority = ?, enabled = ?, updated_at = %s WHERE id = ?", s.now())
	_, err := s.db.Exec(s.q(q), r.Path, r.TargetURL, r.StatusCode, r.Priority, s.boolVal(r.Enabled), id)
	return err
}

func (s *Store) DeleteRule(id int64) error {
	_, err := s.db.Exec(s.q("DELETE FROM redirect_rules WHERE id = ?"), id)
	return err
}

func (s *Store) GetRuleDomainID(ruleID int64) (int64, error) {
	var domainID int64
	err := s.db.QueryRow(s.q("SELECT domain_id FROM redirect_rules WHERE id = ?"), ruleID).Scan(&domainID)
	return domainID, err
}

// --- Bulk operations ---

type ImportEntry struct {
	DomainName string       `json:"domain"`
	DefaultURL string       `json:"default_url"`
	StatusCode int          `json:"status_code"`
	Rules      []ImportRule `json:"rules"`
}

type ImportRule struct {
	Path       string `json:"path"`
	TargetURL  string `json:"target_url"`
	StatusCode int    `json:"status_code"`
	Priority   int    `json:"priority"`
}

func (s *Store) BulkImport(entries []ImportEntry) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	count := 0
	for _, e := range entries {
		sc := e.StatusCode
		if sc == 0 {
			sc = 301
		}

		var domainID int64
		if s.dialect == DialectPostgres {
			err := tx.QueryRow(
				"INSERT INTO domains (name, default_url, status_code, enabled) VALUES ($1, $2, $3, TRUE) ON CONFLICT (name) DO NOTHING RETURNING id",
				e.DomainName, e.DefaultURL, sc,
			).Scan(&domainID)
			if err == sql.ErrNoRows {
				if err := tx.QueryRow("SELECT id FROM domains WHERE name = $1", e.DomainName).Scan(&domainID); err != nil {
					return 0, err
				}
			} else if err != nil {
				return 0, fmt.Errorf("insert domain %s: %w", e.DomainName, err)
			}
		} else {
			res, err := tx.Exec(
				"INSERT OR IGNORE INTO domains (name, default_url, status_code, enabled) VALUES (?, ?, ?, 1)",
				e.DomainName, e.DefaultURL, sc,
			)
			if err != nil {
				return 0, fmt.Errorf("insert domain %s: %w", e.DomainName, err)
			}
			if affected, _ := res.RowsAffected(); affected > 0 {
				domainID, _ = res.LastInsertId()
			} else {
				if err := tx.QueryRow("SELECT id FROM domains WHERE name = ?", e.DomainName).Scan(&domainID); err != nil {
					return 0, err
				}
			}
		}

		for _, r := range e.Rules {
			rsc := r.StatusCode
			if rsc == 0 {
				rsc = 301
			}
			if s.dialect == DialectPostgres {
				_, err = tx.Exec(
					"INSERT INTO redirect_rules (domain_id, path, target_url, status_code, priority, enabled) VALUES ($1, $2, $3, $4, $5, TRUE) ON CONFLICT (domain_id, path) DO NOTHING",
					domainID, r.Path, r.TargetURL, rsc, r.Priority,
				)
			} else {
				_, err = tx.Exec(
					"INSERT OR IGNORE INTO redirect_rules (domain_id, path, target_url, status_code, priority, enabled) VALUES (?, ?, ?, ?, ?, 1)",
					domainID, r.Path, r.TargetURL, rsc, r.Priority,
				)
			}
			if err != nil {
				return 0, fmt.Errorf("insert rule %s%s: %w", e.DomainName, r.Path, err)
			}
		}
		count++
	}

	return count, tx.Commit()
}

// BulkImportReplace performs a full sync: UPSERT all entries and delete orphans not in the set.
func (s *Store) BulkImportReplace(entries []ImportEntry) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Track which domain names and (domain_id, path) pairs are in the import set
	importedDomains := make(map[string]bool, len(entries))
	importedRules := make(map[string]bool) // "domainID:path"

	for _, e := range entries {
		sc := e.StatusCode
		if sc == 0 {
			sc = 301
		}
		importedDomains[e.DomainName] = true

		var domainID int64
		if s.dialect == DialectPostgres {
			err := tx.QueryRow(
				`INSERT INTO domains (name, default_url, status_code, enabled)
				 VALUES ($1, $2, $3, TRUE)
				 ON CONFLICT (name) DO UPDATE SET default_url = $2, status_code = $3, enabled = TRUE, updated_at = NOW()
				 RETURNING id`,
				e.DomainName, e.DefaultURL, sc,
			).Scan(&domainID)
			if err != nil {
				return fmt.Errorf("upsert domain %s: %w", e.DomainName, err)
			}
		} else {
			_, err := tx.Exec(
				"INSERT INTO domains (name, default_url, status_code, enabled) VALUES (?, ?, ?, 1) ON CONFLICT(name) DO UPDATE SET default_url = excluded.default_url, status_code = excluded.status_code, enabled = 1, updated_at = datetime('now')",
				e.DomainName, e.DefaultURL, sc,
			)
			if err != nil {
				return fmt.Errorf("upsert domain %s: %w", e.DomainName, err)
			}
			if err := tx.QueryRow("SELECT id FROM domains WHERE name = ?", e.DomainName).Scan(&domainID); err != nil {
				return err
			}
		}

		for _, r := range e.Rules {
			rsc := r.StatusCode
			if rsc == 0 {
				rsc = 301
			}
			importedRules[fmt.Sprintf("%d:%s", domainID, r.Path)] = true

			if s.dialect == DialectPostgres {
				_, err = tx.Exec(
					`INSERT INTO redirect_rules (domain_id, path, target_url, status_code, priority, enabled)
					 VALUES ($1, $2, $3, $4, $5, TRUE)
					 ON CONFLICT (domain_id, path) DO UPDATE SET target_url = $3, status_code = $4, priority = $5, enabled = TRUE, updated_at = NOW()`,
					domainID, r.Path, r.TargetURL, rsc, r.Priority,
				)
			} else {
				_, err = tx.Exec(
					"INSERT INTO redirect_rules (domain_id, path, target_url, status_code, priority, enabled) VALUES (?, ?, ?, ?, ?, 1) ON CONFLICT(domain_id, path) DO UPDATE SET target_url = excluded.target_url, status_code = excluded.status_code, priority = excluded.priority, enabled = 1, updated_at = datetime('now')",
					domainID, r.Path, r.TargetURL, rsc, r.Priority,
				)
			}
			if err != nil {
				return fmt.Errorf("upsert rule %s%s: %w", e.DomainName, r.Path, err)
			}
		}
	}

	// Delete orphan rules: rules whose domain is in the import set but whose path is not
	rows, err := tx.Query(s.q("SELECT r.id, r.domain_id, r.path, d.name FROM redirect_rules r JOIN domains d ON d.id = r.domain_id"))
	if err != nil {
		return fmt.Errorf("list rules for orphan cleanup: %w", err)
	}
	var orphanRuleIDs []int64
	for rows.Next() {
		var rid, did int64
		var path, dname string
		if err := rows.Scan(&rid, &did, &path, &dname); err != nil {
			rows.Close()
			return err
		}
		if importedDomains[dname] && !importedRules[fmt.Sprintf("%d:%s", did, path)] {
			orphanRuleIDs = append(orphanRuleIDs, rid)
		}
	}
	rows.Close()

	for _, rid := range orphanRuleIDs {
		if _, err := tx.Exec(s.q("DELETE FROM redirect_rules WHERE id = ?"), rid); err != nil {
			return fmt.Errorf("delete orphan rule %d: %w", rid, err)
		}
	}

	// Delete orphan domains: domains not in the import set
	dRows, err := tx.Query("SELECT name FROM domains")
	if err != nil {
		return fmt.Errorf("list domains for orphan cleanup: %w", err)
	}
	var orphanDomains []string
	for dRows.Next() {
		var name string
		if err := dRows.Scan(&name); err != nil {
			dRows.Close()
			return err
		}
		if !importedDomains[name] {
			orphanDomains = append(orphanDomains, name)
		}
	}
	dRows.Close()

	for _, name := range orphanDomains {
		if _, err := tx.Exec(s.q("DELETE FROM domains WHERE name = ?"), name); err != nil {
			return fmt.Errorf("delete orphan domain %s: %w", name, err)
		}
	}

	return tx.Commit()
}

func (s *Store) ExportAll() ([]ImportEntry, error) {
	domains, _, err := s.ListDomains("", 10000, 0)
	if err != nil {
		return nil, err
	}

	var entries []ImportEntry
	for _, d := range domains {
		rules, err := s.ListRules(d.ID)
		if err != nil {
			return nil, err
		}
		e := ImportEntry{
			DomainName: d.Name,
			DefaultURL: d.DefaultURL,
			StatusCode: d.StatusCode,
		}
		for _, r := range rules {
			e.Rules = append(e.Rules, ImportRule{
				Path:       r.Path,
				TargetURL:  r.TargetURL,
				StatusCode: r.StatusCode,
				Priority:   r.Priority,
			})
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// --- Request log (batch insert) ---

func (s *Store) BatchInsertLogs(entries []RequestLogEntry) error {
	if len(entries) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(s.q("INSERT INTO request_log (domain, path, query, status_code, target_url, client_ip, user_agent, referer) VALUES (?, ?, ?, ?, ?, ?, ?, ?)"))
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, e := range entries {
		if _, err := stmt.Exec(e.Domain, e.Path, e.Query, e.Status, e.TargetURL, e.ClientIP, e.UserAgent, e.Referer); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// --- Analytics queries ---

type DomainHitCount struct {
	Domain   string `json:"domain"`
	HitCount int64  `json:"hit_count"`
}

type TimeBucket struct {
	Bucket   string `json:"bucket"`
	HitCount int64  `json:"hit_count"`
}

type PathCount struct {
	Path  string `json:"path"`
	Count int64  `json:"count"`
}

type RefererCount struct {
	Referer string `json:"referer"`
	Count   int64  `json:"count"`
}

func (s *Store) AnalyticsSummary(since string, limit int) ([]DomainHitCount, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(
		s.q("SELECT domain, SUM(hit_count) as hits FROM domain_stats WHERE bucket >= ? GROUP BY domain ORDER BY hits DESC LIMIT ?"),
		since, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []DomainHitCount
	for rows.Next() {
		var d DomainHitCount
		if err := rows.Scan(&d.Domain, &d.HitCount); err != nil {
			return nil, err
		}
		result = append(result, d)
	}
	return result, rows.Err()
}

func (s *Store) AnalyticsDomainTraffic(domain, since, until, granularity string) ([]TimeBucket, error) {
	var truncExpr string
	if s.dialect == DialectPostgres {
		switch granularity {
		case "daily":
			truncExpr = "to_char(date_trunc('day', bucket), 'YYYY-MM-DD\"T\"HH24:MI:SS\"Z\"')"
		case "weekly":
			truncExpr = "to_char(date_trunc('week', bucket), 'IYYY-IW')"
		default:
			truncExpr = "to_char(bucket, 'YYYY-MM-DD HH24:MI:SS')"
		}
	} else {
		switch granularity {
		case "daily":
			truncExpr = "strftime('%Y-%m-%dT00:00:00Z', bucket)"
		case "weekly":
			truncExpr = "strftime('%Y-%W', bucket)"
		default:
			truncExpr = "bucket"
		}
	}

	q := fmt.Sprintf(
		"SELECT %s as b, SUM(hit_count) as hits FROM domain_stats WHERE domain = ? AND bucket >= ? AND bucket <= ? GROUP BY b ORDER BY b",
		truncExpr,
	)
	rows, err := s.db.Query(s.q(q), domain, since, until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []TimeBucket
	for rows.Next() {
		var t TimeBucket
		if err := rows.Scan(&t.Bucket, &t.HitCount); err != nil {
			return nil, err
		}
		result = append(result, t)
	}
	return result, rows.Err()
}

func (s *Store) AnalyticsDomainPaths(domain, since string, limit int) ([]PathCount, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(
		s.q("SELECT path, COUNT(*) as cnt FROM request_log WHERE domain = ? AND created_at >= ? GROUP BY path ORDER BY cnt DESC LIMIT ?"),
		domain, since, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []PathCount
	for rows.Next() {
		var p PathCount
		if err := rows.Scan(&p.Path, &p.Count); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

func (s *Store) AnalyticsInactive(days int) ([]string, error) {
	if days <= 0 {
		days = 30
	}
	since := time.Now().UTC().AddDate(0, 0, -days).Format(s.TimeFmt())

	rows, err := s.db.Query(s.q(`
		SELECT d.name FROM domains d
		WHERE d.enabled = ?
		AND d.name NOT IN (
			SELECT DISTINCT domain FROM request_log WHERE created_at >= ?
		)
		ORDER BY d.name`), s.boolVal(true), since,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		result = append(result, name)
	}
	return result, rows.Err()
}

func (s *Store) AnalyticsTrending(since string, limit int) ([]DomainHitCount, error) {
	if limit <= 0 {
		limit = 20
	}
	midpoint := time.Now().UTC().Add(-time.Since(mustParseTime(since)) / 2).Format(s.TimeFmt())

	rows, err := s.db.Query(s.q(`
		SELECT domain,
			COALESCE(SUM(CASE WHEN bucket >= ? THEN hit_count ELSE 0 END), 0) -
			COALESCE(SUM(CASE WHEN bucket < ? THEN hit_count ELSE 0 END), 0) as trend
		FROM domain_stats
		WHERE bucket >= ?
		GROUP BY domain
		ORDER BY trend DESC
		LIMIT ?`), midpoint, midpoint, since, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []DomainHitCount
	for rows.Next() {
		var d DomainHitCount
		if err := rows.Scan(&d.Domain, &d.HitCount); err != nil {
			return nil, err
		}
		result = append(result, d)
	}
	return result, rows.Err()
}

func (s *Store) AnalyticsReferers(since string, limit int) ([]RefererCount, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(
		s.q("SELECT referer, COUNT(*) as cnt FROM request_log WHERE referer != '' AND created_at >= ? GROUP BY referer ORDER BY cnt DESC LIMIT ?"),
		since, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []RefererCount
	for rows.Next() {
		var r RefererCount
		if err := rows.Scan(&r.Referer, &r.Count); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (s *Store) AnalyticsExport(since string, format string) ([]map[string]any, error) {
	rows, err := s.db.Query(
		s.q("SELECT domain, path, query, status_code, target_url, client_ip, user_agent, referer, created_at FROM request_log WHERE created_at >= ? ORDER BY created_at DESC"),
		since,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []map[string]any
	for rows.Next() {
		var domain, path, query, targetURL, clientIP, userAgent, referer, createdAt string
		var statusCode int
		if err := rows.Scan(&domain, &path, &query, &statusCode, &targetURL, &clientIP, &userAgent, &referer, &scanTime{&createdAt}); err != nil {
			return nil, err
		}
		result = append(result, map[string]any{
			"domain":      domain,
			"path":        path,
			"query":       query,
			"status_code": statusCode,
			"target_url":  targetURL,
			"client_ip":   clientIP,
			"user_agent":  userAgent,
			"referer":     referer,
			"created_at":  createdAt,
		})
	}
	return result, rows.Err()
}

// --- Rollup aggregation ---

func (s *Store) RunRollup(bucketTime time.Time) error {
	bucket := bucketTime.UTC().Truncate(time.Hour).Format(s.TimeFmt())
	nextBucket := bucketTime.UTC().Truncate(time.Hour).Add(time.Hour).Format(s.TimeFmt())

	rows, err := s.db.Query(s.q(`
		SELECT domain, COUNT(*) as hits, COUNT(DISTINCT client_ip) as ips
		FROM request_log
		WHERE created_at >= ? AND created_at < ?
		GROUP BY domain`), bucket, nextBucket,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for rows.Next() {
		var domain string
		var hits, ips int
		if err := rows.Scan(&domain, &hits, &ips); err != nil {
			return err
		}

		topPaths, _ := s.getTopPaths(domain, bucket, nextBucket)
		topReferers, _ := s.getTopReferers(domain, bucket, nextBucket)

		if s.dialect == DialectPostgres {
			_, err = tx.Exec(
				`INSERT INTO domain_stats (domain, bucket, hit_count, unique_ips, top_paths, top_referers)
				 VALUES ($1, $2, $3, $4, $5, $6)
				 ON CONFLICT(domain, bucket) DO UPDATE SET
				 	hit_count = EXCLUDED.hit_count,
				 	unique_ips = EXCLUDED.unique_ips,
				 	top_paths = EXCLUDED.top_paths,
				 	top_referers = EXCLUDED.top_referers`,
				domain, bucket, hits, ips, topPaths, topReferers,
			)
		} else {
			_, err = tx.Exec(
				`INSERT INTO domain_stats (domain, bucket, hit_count, unique_ips, top_paths, top_referers)
				 VALUES (?, ?, ?, ?, ?, ?)
				 ON CONFLICT(domain, bucket) DO UPDATE SET
				 	hit_count = excluded.hit_count,
				 	unique_ips = excluded.unique_ips,
				 	top_paths = excluded.top_paths,
				 	top_referers = excluded.top_referers`,
				domain, bucket, hits, ips, topPaths, topReferers,
			)
		}
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) getTopPaths(domain, since, until string) (string, error) {
	rows, err := s.db.Query(
		s.q("SELECT path, COUNT(*) as cnt FROM request_log WHERE domain = ? AND created_at >= ? AND created_at < ? GROUP BY path ORDER BY cnt DESC LIMIT 10"),
		domain, since, until,
	)
	if err != nil {
		return "[]", err
	}
	defer rows.Close()

	var parts []string
	for rows.Next() {
		var path string
		var count int
		if err := rows.Scan(&path, &count); err != nil {
			continue
		}
		parts = append(parts, fmt.Sprintf(`{"path":%q,"count":%d}`, path, count))
	}
	return "[" + strings.Join(parts, ",") + "]", nil
}

func (s *Store) getTopReferers(domain, since, until string) (string, error) {
	rows, err := s.db.Query(
		s.q("SELECT referer, COUNT(*) as cnt FROM request_log WHERE domain = ? AND referer != '' AND created_at >= ? AND created_at < ? GROUP BY referer ORDER BY cnt DESC LIMIT 10"),
		domain, since, until,
	)
	if err != nil {
		return "[]", err
	}
	defer rows.Close()

	var parts []string
	for rows.Next() {
		var referer string
		var count int
		if err := rows.Scan(&referer, &count); err != nil {
			continue
		}
		parts = append(parts, fmt.Sprintf(`{"referer":%q,"count":%d}`, referer, count))
	}
	return "[" + strings.Join(parts, ",") + "]", nil
}

func (s *Store) PruneOldLogs(olderThan time.Time) (int64, error) {
	res, err := s.db.Exec(s.q("DELETE FROM request_log WHERE created_at < ?"), olderThan.Format(s.TimeFmt()))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// --- Helpers ---

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func mustParseTime(s string) time.Time {
	t, err := time.Parse(SqliteTimeFmt, s)
	if err != nil {
		t, err = time.Parse(time.RFC3339, s)
	}
	if err != nil {
		return time.Now().UTC().AddDate(0, 0, -7)
	}
	return t
}
