package redirect

import (
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/bapley/tld-redirect/internal/store"
)

type rule struct {
	Path          string
	TargetURL     string
	StatusCode    int
	Priority      int
	PreservePath  bool
	PreserveQuery bool
}

type domainEntry struct {
	DefaultURL    string
	StatusCode    int
	CaseSensitive bool
	exact         map[string]rule // exact rules keyed by (normalized) path
	prefixes      []rule          // prefix rules, sorted longest-path first
}

type Engine struct {
	mu       sync.RWMutex
	domains  map[string]*domainEntry
	store    *store.Store
	logCh    chan<- store.RequestLogEntry
	beaconCh chan<- store.RequestLogEntry
}

func NewEngine(s *store.Store, logCh chan<- store.RequestLogEntry) *Engine {
	e := &Engine{
		domains: make(map[string]*domainEntry),
		store:   s,
		logCh:   logCh,
	}
	if err := e.Reload(); err != nil {
		log.Printf("engine: initial load failed: %v", err)
	}
	return e
}

func (e *Engine) Reload() error {
	domains, _, err := e.store.ListDomains("", 100000, 0)
	if err != nil {
		return err
	}

	newMap := make(map[string]*domainEntry, len(domains))
	for _, d := range domains {
		if !d.Enabled {
			continue
		}
		entry := &domainEntry{
			DefaultURL:    d.DefaultURL,
			StatusCode:    d.StatusCode,
			CaseSensitive: d.CaseSensitive,
			exact:         make(map[string]rule),
		}
		rules, err := e.store.ListRules(d.ID)
		if err != nil {
			log.Printf("engine: load rules for %s: %v", d.Name, err)
			continue
		}
		for _, r := range rules {
			if !r.Enabled {
				continue
			}
			p := normalizePath(r.Path)
			if !d.CaseSensitive {
				p = strings.ToLower(p)
			}
			cr := rule{
				Path:          p,
				TargetURL:     r.TargetURL,
				StatusCode:    r.StatusCode,
				Priority:      r.Priority,
				PreservePath:  r.PreservePath,
				PreserveQuery: r.PreserveQuery,
			}
			if r.MatchType == "exact" {
				entry.exact[p] = cr
			} else {
				entry.prefixes = append(entry.prefixes, cr)
			}
		}
		// Longest path first; priority breaks ties between equal-length prefixes.
		sort.Slice(entry.prefixes, func(i, j int) bool {
			if len(entry.prefixes[i].Path) != len(entry.prefixes[j].Path) {
				return len(entry.prefixes[i].Path) > len(entry.prefixes[j].Path)
			}
			return entry.prefixes[i].Priority > entry.prefixes[j].Priority
		})
		newMap[strings.ToLower(d.Name)] = entry
	}

	e.mu.Lock()
	e.domains = newMap
	e.mu.Unlock()
	log.Printf("engine: loaded %d domains", len(newMap))
	return nil
}

func (e *Engine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := strings.ToLower(stripPort(r.Host))

	e.mu.RLock()
	entry, ok := e.domains[host]
	e.mu.RUnlock()

	if !ok {
		http.NotFound(w, r)
		return
	}

	reqPath := normalizePath(r.URL.Path)
	lookup := reqPath
	if !entry.CaseSensitive {
		lookup = strings.ToLower(reqPath)
	}

	// Resolve: exact match first, then longest-prefix, else domain default.
	matched, ok := entry.match(lookup)
	var targetURL string
	var statusCode int
	if ok {
		statusCode = matched.StatusCode
		targetURL = buildLocation(matched, reqPath, r.URL.RawQuery)
	} else {
		targetURL = entry.DefaultURL
		statusCode = entry.StatusCode
	}

	// Async log
	logEntry := store.RequestLogEntry{
		Domain:    host,
		Path:      reqPath,
		Query:     r.URL.RawQuery,
		Status:    statusCode,
		TargetURL: targetURL,
		ClientIP:  clientIP(r),
		UserAgent: r.UserAgent(),
		Referer:   r.Referer(),
	}
	select {
	case e.logCh <- logEntry:
	default:
	}
	if e.beaconCh != nil {
		select {
		case e.beaconCh <- logEntry:
		default:
		}
	}

	http.Redirect(w, r, targetURL, statusCode)
}

// match returns the winning rule for an (already case-folded) lookup path:
// exact match wins, then the longest matching prefix.
func (d *domainEntry) match(lookup string) (rule, bool) {
	if r, ok := d.exact[lookup]; ok {
		return r, true
	}
	for _, r := range d.prefixes { // pre-sorted longest-first
		if lookup == r.Path || strings.HasPrefix(lookup, r.Path+"/") {
			return r, true
		}
	}
	return rule{}, false
}

// buildLocation constructs the redirect target, applying preserve_path (append
// the unmatched suffix of the request path) and preserve_query.
func buildLocation(r rule, reqPath, rawQuery string) string {
	loc := r.TargetURL
	if r.PreservePath && len(reqPath) > len(r.Path) {
		suffix := reqPath[len(r.Path):]
		loc = strings.TrimSuffix(loc, "/") + suffix
	}
	if r.PreserveQuery && rawQuery != "" {
		if strings.Contains(loc, "?") {
			loc += "&" + rawQuery
		} else {
			loc += "?" + rawQuery
		}
	}
	return loc
}

func (e *Engine) SetBeaconCh(ch chan<- store.RequestLogEntry) {
	e.beaconCh = ch
}

func (e *Engine) HasDomain(host string) bool {
	e.mu.RLock()
	_, ok := e.domains[strings.ToLower(stripPort(host))]
	e.mu.RUnlock()
	return ok
}

// normalizePath ensures a leading slash, collapses repeated slashes, and strips
// a trailing slash (except for root). Mirrors the redirect-functions normalizer.
func normalizePath(raw string) string {
	p := raw
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	for strings.Contains(p, "//") {
		p = strings.ReplaceAll(p, "//", "/")
	}
	if len(p) > 1 && strings.HasSuffix(p, "/") {
		p = strings.TrimSuffix(p, "/")
	}
	return p
}

func stripPort(host string) string {
	if i := strings.LastIndex(host, ":"); i != -1 {
		return host[:i]
	}
	return host
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.Index(xff, ","); i != -1 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if xri := r.Header.Get("X-Real-Ip"); xri != "" {
		return xri
	}
	if i := strings.LastIndex(r.RemoteAddr, ":"); i != -1 {
		return r.RemoteAddr[:i]
	}
	return r.RemoteAddr
}
