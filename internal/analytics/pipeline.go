package analytics

import (
	"log"
	"time"

	"github.com/bapley/tld-redirect/internal/store"
)

type Pipeline struct {
	store    *store.Store
	logCh    chan store.RequestLogEntry
	done     chan struct{}
	batchSize int
	flushInterval time.Duration
	rollupInterval time.Duration
}

func NewPipeline(s *store.Store, bufferSize int) (*Pipeline, chan<- store.RequestLogEntry) {
	ch := make(chan store.RequestLogEntry, bufferSize)
	p := &Pipeline{
		store:          s,
		logCh:          ch,
		done:           make(chan struct{}),
		batchSize:      100,
		flushInterval:  5 * time.Second,
		rollupInterval: 5 * time.Minute,
	}
	return p, ch
}

func (p *Pipeline) Start() {
	go p.batchWriter()
	go p.rollupWorker()
	log.Printf("analytics: pipeline started (batch=%d, flush=%s, rollup=%s)",
		p.batchSize, p.flushInterval, p.rollupInterval)
}

func (p *Pipeline) Stop() {
	close(p.done)
}

func (p *Pipeline) batchWriter() {
	batch := make([]store.RequestLogEntry, 0, p.batchSize)
	ticker := time.NewTicker(p.flushInterval)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := p.store.BatchInsertLogs(batch); err != nil {
			log.Printf("analytics: batch insert failed: %v", err)
		}
		batch = batch[:0]
	}

	for {
		select {
		case entry, ok := <-p.logCh:
			if !ok {
				flush()
				return
			}
			batch = append(batch, entry)
			if len(batch) >= p.batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-p.done:
			// Drain remaining
			for {
				select {
				case entry := <-p.logCh:
					batch = append(batch, entry)
				default:
					flush()
					return
				}
			}
		}
	}
}

func (p *Pipeline) rollupWorker() {
	// Run an initial rollup for the current hour on startup
	p.runRollup()

	ticker := time.NewTicker(p.rollupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.runRollup()
		case <-p.done:
			return
		}
	}
}

func (p *Pipeline) runRollup() {
	now := time.Now().UTC()
	// Roll up current hour and previous hour
	for _, t := range []time.Time{now.Add(-time.Hour), now} {
		if err := p.store.RunRollup(t); err != nil {
			log.Printf("analytics: rollup failed for %s: %v", t.Format(time.RFC3339), err)
		}
	}
}
