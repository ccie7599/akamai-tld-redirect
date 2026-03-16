package beacon

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/bapley/tld-redirect/internal/store"
)

type Sender struct {
	endpoint string
	client   *http.Client
	ch       chan store.RequestLogEntry
	done     chan struct{}
	workers  int
}

func NewSender(endpoint string, bufSize int) (*Sender, chan<- store.RequestLogEntry) {
	ch := make(chan store.RequestLogEntry, bufSize)
	s := &Sender{
		endpoint: endpoint,
		client: &http.Client{
			Timeout: 2 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        20,
				MaxIdleConnsPerHost: 20,
				IdleConnTimeout:    30 * time.Second,
			},
		},
		ch:      ch,
		done:    make(chan struct{}),
		workers: 4,
	}
	return s, ch
}

func (s *Sender) Start() {
	for i := 0; i < s.workers; i++ {
		go s.worker()
	}
	log.Printf("beacon: started %d workers → %s", s.workers, s.endpoint)
}

func (s *Sender) Stop() {
	close(s.done)
}

func (s *Sender) worker() {
	for {
		select {
		case entry, ok := <-s.ch:
			if !ok {
				return
			}
			s.send(entry)
		case <-s.done:
			return
		}
	}
}

func (s *Sender) send(e store.RequestLogEntry) {
	// Encode redirect metrics in the URL path (not query string)
	// because DS2 strips query strings from reqPath.
	// Format: /beacon/{domain}/{status}/{path}/{target}/{clientIP}/{ua}/{referer}/{query}
	// Each segment is URL-path-escaped.
	ua := e.UserAgent
	if len(ua) > 200 {
		ua = ua[:200]
	}

	reqURL := fmt.Sprintf("%s/%s/%d/%s/%s/%s/%s/%s/%s",
		s.endpoint,
		url.PathEscape(e.Domain),
		e.Status,
		url.PathEscape(e.Path),
		url.PathEscape(e.TargetURL),
		url.PathEscape(e.ClientIP),
		url.PathEscape(ua),
		url.PathEscape(e.Referer),
		url.PathEscape(e.Query),
	)

	resp, err := s.client.Get(reqURL)
	if err != nil {
		log.Printf("beacon: send failed: %v", err)
		return
	}
	resp.Body.Close()
}
