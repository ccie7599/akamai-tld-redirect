package sync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bapley/tld-redirect/internal/store"
)

type Config struct {
	Endpoint  string // S3-compatible endpoint (e.g., us-ord-1.linodeobjects.com)
	Bucket    string // Bucket name (e.g., tld-redirect-sync)
	Key       string // Object key (e.g., rules.json)
	AccessKey string
	SecretKey string
	Region    string // e.g., us-ord-1
}

type Syncer struct {
	client    *s3.Client
	bucket    string
	key       string
	store     *store.Store
	onReload  func() error
	lastETag  string
	mu        sync.Mutex
	pollEvery time.Duration
	done      chan struct{}
}

func New(cfg Config, st *store.Store, onReload func() error) (*Syncer, error) {
	client := s3.New(s3.Options{
		BaseEndpoint: aws.String(cfg.Endpoint),
		Region:       cfg.Region,
		Credentials:  credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		UsePathStyle: true,
	})

	return &Syncer{
		client:    client,
		bucket:    cfg.Bucket,
		key:       cfg.Key,
		store:     st,
		onReload:  onReload,
		pollEvery: 5 * time.Second,
		done:      make(chan struct{}),
	}, nil
}

// Publish exports all rules from the store and uploads to Object Storage.
func (s *Syncer) Publish(ctx context.Context) error {
	entries, err := s.store.ExportAll()
	if err != nil {
		return fmt.Errorf("export rules: %w", err)
	}

	data, err := json.Marshal(entries)
	if err != nil {
		return fmt.Errorf("marshal rules: %w", err)
	}

	out, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(s.key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String("application/json"),
	})
	if err != nil {
		return fmt.Errorf("put object: %w", err)
	}

	s.mu.Lock()
	if out.ETag != nil {
		s.lastETag = *out.ETag
	}
	s.mu.Unlock()

	log.Printf("sync: published %d domains (%d bytes)", len(entries), len(data))
	return nil
}

// Start begins the background polling loop.
func (s *Syncer) Start() {
	go s.pollLoop()
	log.Printf("sync: polling %s/%s every %s", s.bucket, s.key, s.pollEvery)
}

// Stop ends the polling loop.
func (s *Syncer) Stop() {
	close(s.done)
}

func (s *Syncer) pollLoop() {
	ticker := time.NewTicker(s.pollEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := s.poll(context.Background()); err != nil {
				log.Printf("sync: poll error: %v", err)
			}
		case <-s.done:
			return
		}
	}
}

func (s *Syncer) poll(ctx context.Context) error {
	head, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.key),
	})
	if err != nil {
		return fmt.Errorf("head object: %w", err)
	}

	etag := ""
	if head.ETag != nil {
		etag = *head.ETag
	}

	s.mu.Lock()
	changed := etag != s.lastETag
	s.mu.Unlock()

	if !changed {
		return nil
	}

	// ETag changed — fetch and import
	obj, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.key),
	})
	if err != nil {
		return fmt.Errorf("get object: %w", err)
	}
	defer obj.Body.Close()

	data, err := io.ReadAll(obj.Body)
	if err != nil {
		return fmt.Errorf("read object: %w", err)
	}

	var entries []store.ImportEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("unmarshal rules: %w", err)
	}

	if err := s.store.BulkImportReplace(entries); err != nil {
		return fmt.Errorf("import replace: %w", err)
	}

	if s.onReload != nil {
		if err := s.onReload(); err != nil {
			return fmt.Errorf("reload: %w", err)
		}
	}

	s.mu.Lock()
	s.lastETag = etag
	s.mu.Unlock()

	log.Printf("sync: imported %d domains from remote (etag=%s)", len(entries), etag)
	return nil
}
