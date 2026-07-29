package redirect

import (
	"net/http/httptest"
	"testing"

	"github.com/bapley/tld-redirect/internal/store"
)

// serve issues a request and returns (status, Location).
func serve(e *Engine, host, target string) (int, string) {
	req := httptest.NewRequest("GET", target, nil)
	req.Host = host
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	return w.Code, w.Header().Get("Location")
}

func TestExactMatchType(t *testing.T) {
	e, s := newTestEngine(t)
	d := &store.Domain{Name: "exact.example.com", DefaultURL: "https://t.com/default", StatusCode: 301, Enabled: true, CaseSensitive: true}
	s.CreateDomain(d)
	s.CreateRule(&store.RedirectRule{DomainID: d.ID, Path: "/foo", TargetURL: "https://t.com/foo", StatusCode: 301, MatchType: "exact", Enabled: true})
	e.Reload()

	if _, loc := serve(e, "exact.example.com", "/foo"); loc != "https://t.com/foo" {
		t.Errorf("exact /foo: got %q", loc)
	}
	// Exact must NOT match a sub-path — falls through to the domain default.
	if _, loc := serve(e, "exact.example.com", "/foo/bar"); loc != "https://t.com/default" {
		t.Errorf("exact should not match /foo/bar: got %q, want default", loc)
	}
}

func TestLongestPrefixWins(t *testing.T) {
	e, s := newTestEngine(t)
	d := &store.Domain{Name: "lp.example.com", DefaultURL: "https://t.com", StatusCode: 301, Enabled: true, CaseSensitive: true}
	s.CreateDomain(d)
	// Insert shortest-first to prove ordering isn't insertion order.
	s.CreateRule(&store.RedirectRule{DomainID: d.ID, Path: "/a", TargetURL: "https://t.com/short", StatusCode: 301, MatchType: "prefix", Enabled: true})
	s.CreateRule(&store.RedirectRule{DomainID: d.ID, Path: "/a/b/c", TargetURL: "https://t.com/long", StatusCode: 301, MatchType: "prefix", Enabled: true})
	e.Reload()

	if _, loc := serve(e, "lp.example.com", "/a/b/c/d"); loc != "https://t.com/long" {
		t.Errorf("longest-prefix: got %q, want long", loc)
	}
	if _, loc := serve(e, "lp.example.com", "/a/x"); loc != "https://t.com/short" {
		t.Errorf("shorter-prefix fallback: got %q, want short", loc)
	}
}

func TestPreservePath(t *testing.T) {
	e, s := newTestEngine(t)
	d := &store.Domain{Name: "pp.example.com", DefaultURL: "https://t.com", StatusCode: 301, Enabled: true, CaseSensitive: true}
	s.CreateDomain(d)
	s.CreateRule(&store.RedirectRule{DomainID: d.ID, Path: "/blog", TargetURL: "https://new.com/articles", StatusCode: 301, MatchType: "prefix", PreservePath: true, Enabled: true})
	e.Reload()

	if _, loc := serve(e, "pp.example.com", "/blog/2024/post"); loc != "https://new.com/articles/2024/post" {
		t.Errorf("preserve_path: got %q, want https://new.com/articles/2024/post", loc)
	}
	// Exact hit on the prefix path itself → no suffix appended.
	if _, loc := serve(e, "pp.example.com", "/blog"); loc != "https://new.com/articles" {
		t.Errorf("preserve_path exact: got %q", loc)
	}
}

func TestPreserveQuery(t *testing.T) {
	e, s := newTestEngine(t)
	d := &store.Domain{Name: "pq.example.com", DefaultURL: "https://t.com", StatusCode: 301, Enabled: true, CaseSensitive: true}
	s.CreateDomain(d)
	s.CreateRule(&store.RedirectRule{DomainID: d.ID, Path: "/search", TargetURL: "https://new.com/find", StatusCode: 301, MatchType: "prefix", PreserveQuery: true, Enabled: true})
	e.Reload()

	if _, loc := serve(e, "pq.example.com", "/search?q=hello&n=2"); loc != "https://new.com/find?q=hello&n=2" {
		t.Errorf("preserve_query: got %q", loc)
	}
}

func TestCaseInsensitiveDomain(t *testing.T) {
	e, s := newTestEngine(t)
	d := &store.Domain{Name: "ci.example.com", DefaultURL: "https://t.com", StatusCode: 301, Enabled: true, CaseSensitive: false}
	s.CreateDomain(d)
	s.CreateRule(&store.RedirectRule{DomainID: d.ID, Path: "/Downloads", TargetURL: "https://new.com/dl", StatusCode: 301, MatchType: "prefix", Enabled: true})
	e.Reload()

	if _, loc := serve(e, "ci.example.com", "/DOWNLOADS/file.zip"); loc != "https://new.com/dl" {
		t.Errorf("case-insensitive: got %q, want match", loc)
	}
}

func TestPathNormalization(t *testing.T) {
	e, s := newTestEngine(t)
	d := &store.Domain{Name: "norm.example.com", DefaultURL: "https://t.com/default", StatusCode: 301, Enabled: true, CaseSensitive: true}
	s.CreateDomain(d)
	s.CreateRule(&store.RedirectRule{DomainID: d.ID, Path: "/foo", TargetURL: "https://new.com/foo", StatusCode: 301, MatchType: "exact", Enabled: true})
	e.Reload()

	// Trailing slash normalizes to /foo → exact hit.
	if _, loc := serve(e, "norm.example.com", "/foo/"); loc != "https://new.com/foo" {
		t.Errorf("normalize /foo/: got %q, want https://new.com/foo", loc)
	}
	// Internal doubled slash collapses to /foo.
	if got := normalizePath("/foo//"); got != "/foo" {
		t.Errorf("normalizePath(/foo//): got %q, want /foo", got)
	}
	if got := normalizePath("bar//baz/"); got != "/bar/baz" {
		t.Errorf("normalizePath(bar//baz/): got %q, want /bar/baz", got)
	}
}
