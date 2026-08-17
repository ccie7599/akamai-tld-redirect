package certs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/caddyserver/certmagic"
)

// S3Storage implements certmagic.Storage over an S3-compatible bucket (Linode
// Object Storage). It is the SHARED, cross-region cert store: every instance in
// every region reads/writes here, so an on-demand ACME challenge solved on one
// instance is servable by all of them, and an issued cert is available fleet-wide.
// This is what makes direct-to-engine auto-TLS work when a domain's A records
// point at both regions. Certs live under a prefix in the same bucket used for
// rules sync (all instances already hold these credentials).
type S3Storage struct {
	client *s3.Client
	bucket string
	prefix string // e.g. "certmagic/"
}

// S3Config configures the shared cert store; mirrors the rules-sync S3 config.
type S3Config struct {
	Endpoint, Region, Bucket, AccessKey, SecretKey string
}

func NewS3Storage(cfg S3Config) *S3Storage {
	return &S3Storage{
		client: s3.New(s3.Options{
			BaseEndpoint: aws.String(cfg.Endpoint),
			Region:       cfg.Region,
			Credentials:  credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
			UsePathStyle: true,
		}),
		bucket: cfg.Bucket,
		prefix: "certmagic/",
	}
}

func (s *S3Storage) full(key string) string { return s.prefix + strings.TrimPrefix(key, "/") }
func (s *S3Storage) rel(fullKey string) string {
	return strings.TrimPrefix(fullKey, s.prefix)
}

func isNotFound(err error) bool {
	var ae smithy.APIError
	if errors.As(err, &ae) {
		switch ae.ErrorCode() {
		case "NoSuchKey", "NotFound", "404":
			return true
		}
	}
	return false
}

func (s *S3Storage) Store(ctx context.Context, key string, value []byte) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.full(key)),
		Body:   bytes.NewReader(value),
	})
	return err
}

func (s *S3Storage) Load(ctx context.Context, key string) ([]byte, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.full(key)),
	})
	if err != nil {
		if isNotFound(err) {
			return nil, fs.ErrNotExist
		}
		return nil, err
	}
	defer out.Body.Close()
	return io.ReadAll(out.Body)
}

func (s *S3Storage) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.full(key)),
	})
	if err != nil && isNotFound(err) {
		return fs.ErrNotExist
	}
	return err
}

func (s *S3Storage) Exists(ctx context.Context, key string) bool {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.full(key)),
	})
	return err == nil
}

func (s *S3Storage) List(ctx context.Context, path string, recursive bool) ([]string, error) {
	prefix := s.full(path)
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	in := &s3.ListObjectsV2Input{Bucket: aws.String(s.bucket), Prefix: aws.String(prefix)}
	if !recursive {
		in.Delimiter = aws.String("/")
	}
	var keys []string
	for {
		out, err := s.client.ListObjectsV2(ctx, in)
		if err != nil {
			return nil, err
		}
		for _, o := range out.Contents {
			keys = append(keys, s.rel(*o.Key))
		}
		for _, cp := range out.CommonPrefixes {
			keys = append(keys, strings.TrimSuffix(s.rel(*cp.Prefix), "/"))
		}
		if out.IsTruncated != nil && *out.IsTruncated {
			in.ContinuationToken = out.NextContinuationToken
			continue
		}
		break
	}
	if len(keys) == 0 {
		return nil, fs.ErrNotExist
	}
	return keys, nil
}

func (s *S3Storage) Stat(ctx context.Context, key string) (certmagic.KeyInfo, error) {
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.full(key)),
	})
	if err != nil {
		if isNotFound(err) {
			return certmagic.KeyInfo{}, fs.ErrNotExist
		}
		return certmagic.KeyInfo{}, err
	}
	ki := certmagic.KeyInfo{Key: key, IsTerminal: true}
	if out.LastModified != nil {
		ki.Modified = *out.LastModified
	}
	if out.ContentLength != nil {
		ki.Size = *out.ContentLength
	}
	return ki, nil
}

// --- Locking (best-effort distributed lock via a lock object) ---
//
// Coordinates issuance so instances don't duplicate ACME orders. Uses a
// conditional PUT (If-None-Match) where the provider supports it; a stale lock
// (older than lockTTL, e.g. a crashed holder) is stolen. Worst case under an
// unsupported conditional-put is a rare duplicate issuance, which Let's Encrypt
// de-duplicates — never corruption.

const lockTTL = 2 * time.Minute
const lockPoll = 2 * time.Second

func (s *S3Storage) lockKey(name string) string {
	h := sha256.Sum256([]byte(name))
	return s.full("locks/" + hex.EncodeToString(h[:16]))
}

func (s *S3Storage) Lock(ctx context.Context, name string) error {
	lk := s.lockKey(name)
	for {
		_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
			Bucket:      aws.String(s.bucket),
			Key:         aws.String(lk),
			Body:        bytes.NewReader([]byte(time.Now().UTC().Format(time.RFC3339))),
			IfNoneMatch: aws.String("*"),
		})
		if err == nil {
			return nil // acquired
		}
		// Someone holds it (or conditional-put unsupported). Check staleness.
		head, herr := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(lk)})
		if herr == nil && head.LastModified != nil && time.Since(*head.LastModified) > lockTTL {
			_, _ = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(lk)})
			continue // retry, likely to acquire
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(lockPoll):
		}
	}
}

func (s *S3Storage) Unlock(ctx context.Context, name string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.lockKey(name)),
	})
	return err
}

func (s *S3Storage) String() string {
	return fmt.Sprintf("S3Storage:%s/%s", s.bucket, s.prefix)
}
