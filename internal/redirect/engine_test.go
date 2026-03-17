package redirect

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/bapley/tld-redirect/internal/store"
)

func newTestEngine(t *testing.T) (*Engine, *store.Store) {
	t.Helper()
	f, err := os.CreateTemp("", "tld-engine-test-*.db")
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

	e := NewEngine(s, logCh)
	return e, s
}

func TestRedirectDefaultURL(t *testing.T) {
	e, s := newTestEngine(t)

	s.CreateDomain(&store.Domain{Name: "test.example.com", DefaultURL: "https://target.com", StatusCode: 301, Enabled: true})
	e.Reload()

	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "test.example.com"
	w := httptest.NewRecorder()

	e.ServeHTTP(w, req)

	if w.Code != 301 {
		t.Errorf("got status %d, want 301", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "https://target.com" {
		t.Errorf("got location %q, want https://target.com", loc)
	}
}

func TestRedirectWithRule(t *testing.T) {
	e, s := newTestEngine(t)

	d := &store.Domain{Name: "rules.example.com", DefaultURL: "https://target.com", StatusCode: 301, Enabled: true}
	s.CreateDomain(d)
	s.CreateRule(&store.RedirectRule{DomainID: d.ID, Path: "/mortgage", TargetURL: "https://target.com/mortgage", StatusCode: 302, Priority: 10, Enabled: true})
	e.Reload()

	req := httptest.NewRequest("GET", "/mortgage", nil)
	req.Host = "rules.example.com"
	w := httptest.NewRecorder()

	e.ServeHTTP(w, req)

	if w.Code != 302 {
		t.Errorf("got status %d, want 302", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "https://target.com/mortgage" {
		t.Errorf("got location %q, want https://target.com/mortgage", loc)
	}
}

func TestRedirectPriorityOrdering(t *testing.T) {
	e, s := newTestEngine(t)

	d := &store.Domain{Name: "priority.example.com", DefaultURL: "https://target.com/default", StatusCode: 301, Enabled: true}
	s.CreateDomain(d)
	// Both paths match /docs/page — /docs (low priority) and /docs/page (high priority, exact match)
	s.CreateRule(&store.RedirectRule{DomainID: d.ID, Path: "/docs", TargetURL: "https://target.com/low", StatusCode: 301, Priority: 1, Enabled: true})
	s.CreateRule(&store.RedirectRule{DomainID: d.ID, Path: "/docs/page", TargetURL: "https://target.com/high", StatusCode: 301, Priority: 100, Enabled: true})
	e.Reload()

	// /docs/page should match the high-priority exact rule first
	req := httptest.NewRequest("GET", "/docs/page", nil)
	req.Host = "priority.example.com"
	w := httptest.NewRecorder()

	e.ServeHTTP(w, req)

	if loc := w.Header().Get("Location"); loc != "https://target.com/high" {
		t.Errorf("expected high-priority rule, got %q", loc)
	}
}

func TestRedirectPrefixMatch(t *testing.T) {
	e, s := newTestEngine(t)

	d := &store.Domain{Name: "prefix.example.com", DefaultURL: "https://target.com", StatusCode: 301, Enabled: true}
	s.CreateDomain(d)
	s.CreateRule(&store.RedirectRule{DomainID: d.ID, Path: "/docs", TargetURL: "https://target.com/docs", StatusCode: 301, Priority: 10, Enabled: true})
	e.Reload()

	// /docs/page should match the /docs prefix rule
	req := httptest.NewRequest("GET", "/docs/page", nil)
	req.Host = "prefix.example.com"
	w := httptest.NewRecorder()

	e.ServeHTTP(w, req)

	if loc := w.Header().Get("Location"); loc != "https://target.com/docs" {
		t.Errorf("prefix match: got %q, want https://target.com/docs", loc)
	}
}

func TestRedirectUnknownDomain(t *testing.T) {
	e, _ := newTestEngine(t)

	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "unknown.example.com"
	w := httptest.NewRecorder()

	e.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Errorf("got status %d, want 404 for unknown domain", w.Code)
	}
}

func TestRedirectDisabledDomain(t *testing.T) {
	e, s := newTestEngine(t)

	s.CreateDomain(&store.Domain{Name: "disabled.example.com", DefaultURL: "https://target.com", StatusCode: 301, Enabled: false})
	e.Reload()

	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "disabled.example.com"
	w := httptest.NewRecorder()

	e.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Errorf("got status %d, want 404 for disabled domain", w.Code)
	}
}

func TestRedirectDisabledRule(t *testing.T) {
	e, s := newTestEngine(t)

	d := &store.Domain{Name: "drule.example.com", DefaultURL: "https://target.com/default", StatusCode: 301, Enabled: true}
	s.CreateDomain(d)
	s.CreateRule(&store.RedirectRule{DomainID: d.ID, Path: "/disabled", TargetURL: "https://target.com/nope", StatusCode: 301, Priority: 10, Enabled: false})
	e.Reload()

	req := httptest.NewRequest("GET", "/disabled", nil)
	req.Host = "drule.example.com"
	w := httptest.NewRecorder()

	e.ServeHTTP(w, req)

	// Disabled rule should not match — falls through to default
	if loc := w.Header().Get("Location"); loc != "https://target.com/default" {
		t.Errorf("got %q, want default URL (disabled rule should not match)", loc)
	}
}

func TestHasDomain(t *testing.T) {
	e, s := newTestEngine(t)

	s.CreateDomain(&store.Domain{Name: "exists.example.com", DefaultURL: "https://target.com", StatusCode: 301, Enabled: true})
	e.Reload()

	if !e.HasDomain("exists.example.com") {
		t.Error("expected HasDomain=true for exists.example.com")
	}
	if e.HasDomain("nope.example.com") {
		t.Error("expected HasDomain=false for nope.example.com")
	}
	if !e.HasDomain("exists.example.com:443") {
		t.Error("HasDomain should strip port")
	}
}

func TestReloadUpdatesMap(t *testing.T) {
	e, s := newTestEngine(t)

	s.CreateDomain(&store.Domain{Name: "reload.example.com", DefaultURL: "https://old.com", StatusCode: 301, Enabled: true})
	e.Reload()

	// Update and reload
	s.UpdateDomain("reload.example.com", &store.Domain{DefaultURL: "https://new.com", StatusCode: 302, Enabled: true})
	e.Reload()

	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "reload.example.com"
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)

	if w.Code != 302 {
		t.Errorf("got status %d after reload, want 302", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "https://new.com" {
		t.Errorf("got %q after reload, want https://new.com", loc)
	}
}

func TestBeaconChannel(t *testing.T) {
	e, s := newTestEngine(t)

	s.CreateDomain(&store.Domain{Name: "beacon.example.com", DefaultURL: "https://target.com", StatusCode: 301, Enabled: true})
	e.Reload()

	beaconCh := make(chan store.RequestLogEntry, 10)
	e.SetBeaconCh(beaconCh)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Host = "beacon.example.com"
	req.Header.Set("User-Agent", "TestAgent")
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)

	select {
	case entry := <-beaconCh:
		if entry.Domain != "beacon.example.com" {
			t.Errorf("beacon domain: got %q", entry.Domain)
		}
		if entry.Path != "/test" {
			t.Errorf("beacon path: got %q", entry.Path)
		}
		if entry.UserAgent != "TestAgent" {
			t.Errorf("beacon UA: got %q", entry.UserAgent)
		}
	default:
		t.Error("expected beacon entry on channel")
	}
}

func TestConcurrentRequests(t *testing.T) {
	e, s := newTestEngine(t)

	// Load 100 domains
	for i := 0; i < 100; i++ {
		s.CreateDomain(&store.Domain{
			Name:       "concurrent-" + http.StatusText(i+200) + ".example.com",
			DefaultURL: "https://target.com",
			StatusCode: 301,
			Enabled:    true,
		})
	}
	e.Reload()

	// Fire 100 concurrent requests
	done := make(chan struct{}, 100)
	for i := 0; i < 100; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			req := httptest.NewRequest("GET", "/", nil)
			req.Host = "concurrent-OK.example.com"
			w := httptest.NewRecorder()
			e.ServeHTTP(w, req)
		}()
	}
	for i := 0; i < 100; i++ {
		<-done
	}
}
