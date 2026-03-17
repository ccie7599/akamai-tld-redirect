package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/bapley/tld-redirect/internal/redirect"
	"github.com/bapley/tld-redirect/internal/store"
)

type testEnv struct {
	store   *store.Store
	engine  *redirect.Engine
	handler *Handler
	mux     *http.ServeMux
	token   string
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	f, err := os.CreateTemp("", "tld-api-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })

	s, err := store.New(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })

	logCh := make(chan store.RequestLogEntry, 100)
	go func() { for range logCh {} }()

	e := redirect.NewEngine(s, logCh)
	h := NewHandler(s, e)

	mux := http.NewServeMux()
	h.Register(mux)
	wrapped := TokenAuth("test-token", CORS(mux))

	outerMux := http.NewServeMux()
	outerMux.Handle("/api/", wrapped)

	return &testEnv{store: s, engine: e, handler: h, mux: outerMux, token: "test-token"}
}

func (te *testEnv) request(method, path string, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path+"?token="+te.token, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	te.mux.ServeHTTP(w, req)
	return w
}

func (te *testEnv) requestNoAuth(method, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	te.mux.ServeHTTP(w, req)
	return w
}

// --- Auth ---

func TestAuthRequired(t *testing.T) {
	te := newTestEnv(t)
	w := te.requestNoAuth("GET", "/api/v1/domains")
	if w.Code != 401 {
		t.Errorf("got %d, want 401 without token", w.Code)
	}
}

func TestAuthBadToken(t *testing.T) {
	te := newTestEnv(t)
	req := httptest.NewRequest("GET", "/api/v1/domains?token=wrong", nil)
	w := httptest.NewRecorder()
	te.mux.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Errorf("got %d, want 401 with bad token", w.Code)
	}
}

// --- Domain CRUD ---

func TestCreateAndListDomains(t *testing.T) {
	te := newTestEnv(t)

	// Create
	w := te.request("POST", "/api/v1/domains", map[string]any{
		"name": "api-test.example.com", "default_url": "https://target.com",
	})
	if w.Code != 201 {
		t.Fatalf("create: got %d, want 201. Body: %s", w.Code, w.Body.String())
	}
	var created store.Domain
	json.Unmarshal(w.Body.Bytes(), &created)
	if created.Name != "api-test.example.com" {
		t.Errorf("name: got %q", created.Name)
	}
	if created.StatusCode != 301 {
		t.Errorf("status_code should default to 301, got %d", created.StatusCode)
	}

	// List
	w = te.request("GET", "/api/v1/domains", nil)
	if w.Code != 200 {
		t.Fatalf("list: got %d", w.Code)
	}
	var list map[string]any
	json.Unmarshal(w.Body.Bytes(), &list)
	if list["total"].(float64) != 1 {
		t.Errorf("total: got %v, want 1", list["total"])
	}
}

func TestGetDomain(t *testing.T) {
	te := newTestEnv(t)
	te.request("POST", "/api/v1/domains", map[string]any{
		"name": "get-test.example.com", "default_url": "https://target.com",
	})

	w := te.request("GET", "/api/v1/domains/get-test.example.com", nil)
	if w.Code != 200 {
		t.Fatalf("get: got %d", w.Code)
	}
	var d store.DomainWithRules
	json.Unmarshal(w.Body.Bytes(), &d)
	if d.Name != "get-test.example.com" {
		t.Errorf("name: got %q", d.Name)
	}
}

func TestGetDomainNotFound(t *testing.T) {
	te := newTestEnv(t)
	w := te.request("GET", "/api/v1/domains/nonexistent.example.com", nil)
	if w.Code != 404 {
		t.Errorf("got %d, want 404", w.Code)
	}
}

func TestUpdateDomain(t *testing.T) {
	te := newTestEnv(t)
	te.request("POST", "/api/v1/domains", map[string]any{
		"name": "update-test.example.com", "default_url": "https://old.com",
	})

	w := te.request("PUT", "/api/v1/domains/update-test.example.com", map[string]any{
		"default_url": "https://new.com", "status_code": 302, "enabled": false,
	})
	if w.Code != 200 {
		t.Fatalf("update: got %d", w.Code)
	}

	w = te.request("GET", "/api/v1/domains/update-test.example.com", nil)
	var d store.DomainWithRules
	json.Unmarshal(w.Body.Bytes(), &d)
	if d.DefaultURL != "https://new.com" {
		t.Errorf("default_url: got %q", d.DefaultURL)
	}
}

func TestDeleteDomain(t *testing.T) {
	te := newTestEnv(t)
	te.request("POST", "/api/v1/domains", map[string]any{
		"name": "delete-test.example.com", "default_url": "https://target.com",
	})

	w := te.request("DELETE", "/api/v1/domains/delete-test.example.com", nil)
	if w.Code != 200 {
		t.Fatalf("delete: got %d", w.Code)
	}

	w = te.request("GET", "/api/v1/domains/delete-test.example.com", nil)
	if w.Code != 404 {
		t.Errorf("get after delete: got %d, want 404", w.Code)
	}
}

func TestCreateDomainDuplicate(t *testing.T) {
	te := newTestEnv(t)
	te.request("POST", "/api/v1/domains", map[string]any{
		"name": "dup.example.com", "default_url": "https://target.com",
	})

	w := te.request("POST", "/api/v1/domains", map[string]any{
		"name": "dup.example.com", "default_url": "https://other.com",
	})
	if w.Code != 409 {
		t.Errorf("duplicate: got %d, want 409", w.Code)
	}
}

func TestCreateDomainValidation(t *testing.T) {
	te := newTestEnv(t)

	// Missing name
	w := te.request("POST", "/api/v1/domains", map[string]any{
		"default_url": "https://target.com",
	})
	if w.Code != 400 {
		t.Errorf("missing name: got %d, want 400", w.Code)
	}

	// Missing default_url
	w = te.request("POST", "/api/v1/domains", map[string]any{
		"name": "test.example.com",
	})
	if w.Code != 400 {
		t.Errorf("missing default_url: got %d, want 400", w.Code)
	}
}

// --- Rule CRUD ---

func TestCreateAndListRules(t *testing.T) {
	te := newTestEnv(t)
	te.request("POST", "/api/v1/domains", map[string]any{
		"name": "rules.example.com", "default_url": "https://target.com",
	})

	w := te.request("POST", "/api/v1/domains/rules.example.com/rules", map[string]any{
		"path": "/mortgage", "target_url": "https://target.com/mortgage", "priority": 10,
	})
	if w.Code != 201 {
		t.Fatalf("create rule: got %d. Body: %s", w.Code, w.Body.String())
	}

	w = te.request("GET", "/api/v1/domains/rules.example.com/rules", nil)
	if w.Code != 200 {
		t.Fatalf("list rules: got %d", w.Code)
	}
	var rules []store.RedirectRule
	json.Unmarshal(w.Body.Bytes(), &rules)
	if len(rules) != 1 {
		t.Errorf("got %d rules, want 1", len(rules))
	}
}

func TestCreateRuleForNonexistentDomain(t *testing.T) {
	te := newTestEnv(t)
	w := te.request("POST", "/api/v1/domains/nope.example.com/rules", map[string]any{
		"path": "/x", "target_url": "https://target.com/x",
	})
	if w.Code != 404 {
		t.Errorf("got %d, want 404", w.Code)
	}
}

func TestCreateRuleValidation(t *testing.T) {
	te := newTestEnv(t)
	te.request("POST", "/api/v1/domains", map[string]any{
		"name": "rval.example.com", "default_url": "https://target.com",
	})

	w := te.request("POST", "/api/v1/domains/rval.example.com/rules", map[string]any{
		"target_url": "https://target.com/x",
	})
	if w.Code != 400 {
		t.Errorf("missing path: got %d, want 400", w.Code)
	}
}

func TestUpdateRule(t *testing.T) {
	te := newTestEnv(t)
	te.request("POST", "/api/v1/domains", map[string]any{
		"name": "rupdate.example.com", "default_url": "https://target.com",
	})
	w := te.request("POST", "/api/v1/domains/rupdate.example.com/rules", map[string]any{
		"path": "/old", "target_url": "https://target.com/old",
	})
	var rule store.RedirectRule
	json.Unmarshal(w.Body.Bytes(), &rule)

	w = te.request("PUT", "/api/v1/domains/rupdate.example.com/rules/"+itoa(rule.ID), map[string]any{
		"path": "/new", "target_url": "https://target.com/new", "status_code": 302, "priority": 50,
	})
	if w.Code != 200 {
		t.Fatalf("update rule: got %d", w.Code)
	}
}

func TestDeleteRule(t *testing.T) {
	te := newTestEnv(t)
	te.request("POST", "/api/v1/domains", map[string]any{
		"name": "rdelete.example.com", "default_url": "https://target.com",
	})
	w := te.request("POST", "/api/v1/domains/rdelete.example.com/rules", map[string]any{
		"path": "/gone", "target_url": "https://target.com/gone",
	})
	var rule store.RedirectRule
	json.Unmarshal(w.Body.Bytes(), &rule)

	w = te.request("DELETE", "/api/v1/domains/rdelete.example.com/rules/"+itoa(rule.ID), nil)
	if w.Code != 200 {
		t.Fatalf("delete rule: got %d", w.Code)
	}
}

// --- Bulk Operations ---

func TestBulkImport(t *testing.T) {
	te := newTestEnv(t)

	entries := []store.ImportEntry{
		{DomainName: "bulk1.example.com", DefaultURL: "https://target.com", StatusCode: 301},
		{DomainName: "bulk2.example.com", DefaultURL: "https://target.com", StatusCode: 301},
	}
	w := te.request("POST", "/api/v1/import", entries)
	if w.Code != 200 {
		t.Fatalf("import: got %d. Body: %s", w.Code, w.Body.String())
	}
	var result map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["imported"].(float64) != 2 {
		t.Errorf("imported: got %v, want 2", result["imported"])
	}
}

func TestExport(t *testing.T) {
	te := newTestEnv(t)
	te.request("POST", "/api/v1/domains", map[string]any{
		"name": "export.example.com", "default_url": "https://target.com",
	})

	w := te.request("GET", "/api/v1/export", nil)
	if w.Code != 200 {
		t.Fatalf("export: got %d", w.Code)
	}
	var entries []store.ImportEntry
	json.Unmarshal(w.Body.Bytes(), &entries)
	if len(entries) != 1 {
		t.Errorf("exported %d, want 1", len(entries))
	}
}

// --- Analytics ---

func TestAnalyticsSummary(t *testing.T) {
	te := newTestEnv(t)
	w := te.request("GET", "/api/v1/analytics/summary", nil)
	if w.Code != 200 {
		t.Fatalf("analytics summary: got %d", w.Code)
	}
}

func TestAnalyticsDomain(t *testing.T) {
	te := newTestEnv(t)
	w := te.request("GET", "/api/v1/analytics/domains/test.example.com", nil)
	if w.Code != 200 {
		t.Fatalf("analytics domain: got %d", w.Code)
	}
}

func TestAnalyticsPaths(t *testing.T) {
	te := newTestEnv(t)
	w := te.request("GET", "/api/v1/analytics/domains/test.example.com/paths", nil)
	if w.Code != 200 {
		t.Fatalf("analytics paths: got %d", w.Code)
	}
}

func TestAnalyticsInactive(t *testing.T) {
	te := newTestEnv(t)
	w := te.request("GET", "/api/v1/analytics/inactive", nil)
	if w.Code != 200 {
		t.Fatalf("analytics inactive: got %d", w.Code)
	}
}

func TestAnalyticsTrending(t *testing.T) {
	te := newTestEnv(t)
	w := te.request("GET", "/api/v1/analytics/trending", nil)
	if w.Code != 200 {
		t.Fatalf("analytics trending: got %d", w.Code)
	}
}

func TestAnalyticsReferers(t *testing.T) {
	te := newTestEnv(t)
	w := te.request("GET", "/api/v1/analytics/referers", nil)
	if w.Code != 200 {
		t.Fatalf("analytics referers: got %d", w.Code)
	}
}

func TestAnalyticsExportJSON(t *testing.T) {
	te := newTestEnv(t)
	w := te.request("GET", "/api/v1/analytics/export", nil)
	if w.Code != 200 {
		t.Fatalf("analytics export json: got %d", w.Code)
	}
}

func TestAnalyticsExportCSV(t *testing.T) {
	te := newTestEnv(t)

	// Insert a log entry so CSV has data
	te.store.BatchInsertLogs([]store.RequestLogEntry{
		{Domain: "csv.example.com", Path: "/", Status: 301, TargetURL: "https://target.com", ClientIP: "1.2.3.4"},
	})

	req := httptest.NewRequest("GET", "/api/v1/analytics/export?token="+te.token+"&format=csv&since=2020-01-01", nil)
	w := httptest.NewRecorder()
	te.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("analytics export csv: got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/csv" {
		t.Errorf("content-type: got %q, want text/csv", ct)
	}
}

// --- CORS ---

func TestCORSPreflight(t *testing.T) {
	te := newTestEnv(t)
	req := httptest.NewRequest("OPTIONS", "/api/v1/domains?token="+te.token, nil)
	w := httptest.NewRecorder()
	te.mux.ServeHTTP(w, req)

	if w.Code != 204 {
		t.Errorf("CORS preflight: got %d, want 204", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("missing CORS allow-origin header")
	}
}

// --- helpers ---

func itoa(n int64) string {
	return fmt.Sprintf("%d", n)
}
