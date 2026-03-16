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
	Path       string
	TargetURL  string
	StatusCode int
	Priority   int
}

type domainEntry struct {
	DefaultURL string
	StatusCode int
	Rules      []rule
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
			DefaultURL: d.DefaultURL,
			StatusCode: d.StatusCode,
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
			entry.Rules = append(entry.Rules, rule{
				Path:       r.Path,
				TargetURL:  r.TargetURL,
				StatusCode: r.StatusCode,
				Priority:   r.Priority,
			})
		}
		sort.Slice(entry.Rules, func(i, j int) bool {
			return entry.Rules[i].Priority > entry.Rules[j].Priority
		})
		newMap[d.Name] = entry
	}

	e.mu.Lock()
	e.domains = newMap
	e.mu.Unlock()
	log.Printf("engine: loaded %d domains", len(newMap))
	return nil
}

func (e *Engine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := stripPort(r.Host)

	e.mu.RLock()
	entry, ok := e.domains[host]
	e.mu.RUnlock()

	if !ok {
		http.NotFound(w, r)
		return
	}

	targetURL := entry.DefaultURL
	statusCode := entry.StatusCode
	path := r.URL.Path

	for _, rule := range entry.Rules {
		if matchPath(rule.Path, path) {
			targetURL = rule.TargetURL
			statusCode = rule.StatusCode
			break
		}
	}

	// Async log
	logEntry := store.RequestLogEntry{
		Domain:    host,
		Path:      path,
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

func (e *Engine) SetBeaconCh(ch chan<- store.RequestLogEntry) {
	e.beaconCh = ch
}

func (e *Engine) HasDomain(host string) bool {
	e.mu.RLock()
	_, ok := e.domains[stripPort(host)]
	e.mu.RUnlock()
	return ok
}

func matchPath(pattern, path string) bool {
	if pattern == path {
		return true
	}
	// Prefix match: pattern "/mortgage" matches "/mortgage/rates"
	if strings.HasSuffix(pattern, "/") {
		return strings.HasPrefix(path, pattern)
	}
	return strings.HasPrefix(path, pattern+"/") || path == pattern
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
