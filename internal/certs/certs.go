package certs

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"log"
	"net/http"

	"github.com/caddyserver/certmagic"
)

type Config struct {
	AdminDomain string
	Email       string
	DataDir     string // File storage (SQLite/dev mode)
	Staging     bool
	CheckDomain func(name string) bool
	PGDB        *sql.DB // If set, use PG storage instead of file storage
}

// Manager handles TLS certificate provisioning and serving.
type Manager struct {
	magic  *certmagic.Config
	issuer *certmagic.ACMEIssuer
	cfg    Config
}

// NewManager creates a cert manager that provisions certs via ACME (control plane).
func NewManager(cfg Config) (*Manager, error) {
	certmagic.DefaultACME.Agreed = true
	certmagic.DefaultACME.Email = cfg.Email

	if cfg.Staging {
		certmagic.DefaultACME.CA = certmagic.LetsEncryptStagingCA
	}

	// Storage backend: PG if available, otherwise file
	if cfg.PGDB != nil {
		certmagic.Default.Storage = NewPGStorage(cfg.PGDB)
	} else if cfg.DataDir != "" {
		certmagic.Default.Storage = &certmagic.FileStorage{Path: cfg.DataDir}
	}

	magic := certmagic.NewDefault()

	magic.OnDemand = &certmagic.OnDemandConfig{
		DecisionFunc: func(ctx context.Context, name string) error {
			if name == cfg.AdminDomain {
				return nil
			}
			if cfg.CheckDomain != nil && cfg.CheckDomain(name) {
				return nil
			}
			return fmt.Errorf("domain %s not managed", name)
		},
	}

	// Get the ACME issuer for HTTP challenge handling
	var issuer *certmagic.ACMEIssuer
	for _, iss := range magic.Issuers {
		if acme, ok := iss.(*certmagic.ACMEIssuer); ok {
			issuer = acme
			break
		}
	}
	if issuer == nil {
		return nil, fmt.Errorf("no ACME issuer configured")
	}

	// Pre-provision the admin domain cert
	ctx := context.Background()
	if err := magic.ManageSync(ctx, []string{cfg.AdminDomain}); err != nil {
		return nil, fmt.Errorf("provision cert for %s: %w", cfg.AdminDomain, err)
	}
	log.Printf("certs: provisioned TLS for %s", cfg.AdminDomain)

	return &Manager{magic: magic, issuer: issuer, cfg: cfg}, nil
}

// NewLoader creates a cert manager that only reads certs from storage (data plane).
// It does NOT provision certs via ACME — it relies on a control plane to do that.
func NewLoader(cfg Config) (*Manager, error) {
	// Storage backend: PG if available, otherwise file
	if cfg.PGDB != nil {
		certmagic.Default.Storage = NewPGStorage(cfg.PGDB)
	} else if cfg.DataDir != "" {
		certmagic.Default.Storage = &certmagic.FileStorage{Path: cfg.DataDir}
	}

	magic := certmagic.NewDefault()

	// On-demand still needs a decision func to validate domains
	magic.OnDemand = &certmagic.OnDemandConfig{
		DecisionFunc: func(ctx context.Context, name string) error {
			if name == cfg.AdminDomain {
				return nil
			}
			if cfg.CheckDomain != nil && cfg.CheckDomain(name) {
				return nil
			}
			return fmt.Errorf("domain %s not managed", name)
		},
	}

	log.Printf("certs: loader mode — reading certs from storage (no ACME provisioning)")
	return &Manager{magic: magic, cfg: cfg}, nil
}

func (m *Manager) TLSConfig() *tls.Config {
	tlsCfg := m.magic.TLSConfig()
	tlsCfg.NextProtos = append([]string{"h2", "http/1.1"}, tlsCfg.NextProtos...)
	return tlsCfg
}

func (m *Manager) HTTPChallengeHandler(fallback http.Handler) http.Handler {
	if m.issuer != nil {
		return m.issuer.HTTPChallengeHandler(fallback)
	}
	return fallback
}
