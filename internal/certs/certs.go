package certs

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"slices"

	"github.com/caddyserver/certmagic"
	"github.com/mholt/acmez/v3/acme"
)

type Config struct {
	AdminDomain string
	Email       string
	DataDir     string // File storage (SQLite/dev mode)
	Staging     bool
	CheckDomain func(name string) bool
	PGDB        *sql.DB           // If set (and Storage is nil), use PG storage
	Storage     certmagic.Storage // Shared cross-region store (preferred); e.g. S3Storage

	// ZeroSSL fallback issuer (data plane). Let's Encrypt refuses some names by
	// policy (e.g. all of pnc.com); with EAB credentials set, ZeroSSL is tried
	// next in the same issuance attempt. ZeroSSLOnly skips Let's Encrypt (QA tests).
	ZeroSSLEAB  *acme.EAB
	ZeroSSLOnly bool

	// AdminAliases are extra names the control plane serves alongside
	// AdminDomain, e.g. a shared origin hostname with A records to every control
	// node so a CDN can verify one name across regions. Issued in the background
	// (never blocks startup); the shared store means one cert serves all nodes.
	AdminAliases []string
}

// setStorage selects the cert storage backend, most-shared first: an explicit
// cross-region Storage (S3), then per-region PG, then local files.
func setStorage(cfg Config) {
	switch {
	case cfg.Storage != nil:
		certmagic.Default.Storage = cfg.Storage
	case cfg.PGDB != nil:
		certmagic.Default.Storage = NewPGStorage(cfg.PGDB)
	case cfg.DataDir != "":
		certmagic.Default.Storage = &certmagic.FileStorage{Path: cfg.DataDir}
	}
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

	setStorage(cfg)

	magic := certmagic.NewDefault()

	issuer, err := configureIssuers(magic, cfg)
	if err != nil {
		return nil, err
	}

	magic.OnDemand = &certmagic.OnDemandConfig{
		DecisionFunc: func(ctx context.Context, name string) error {
			if name == cfg.AdminDomain || slices.Contains(cfg.AdminAliases, name) {
				return nil
			}
			if cfg.CheckDomain != nil && cfg.CheckDomain(name) {
				return nil
			}
			return fmt.Errorf("domain %s not managed", name)
		},
	}

	// Pre-provision the admin domain cert
	ctx := context.Background()
	if err := magic.ManageSync(ctx, []string{cfg.AdminDomain}); err != nil {
		return nil, fmt.Errorf("provision cert for %s: %w", cfg.AdminDomain, err)
	}
	log.Printf("certs: provisioned TLS for %s", cfg.AdminDomain)

	if len(cfg.AdminAliases) > 0 {
		if err := magic.ManageAsync(ctx, cfg.AdminAliases); err != nil {
			log.Printf("certs: managing admin aliases %v: %v", cfg.AdminAliases, err)
		} else {
			log.Printf("certs: managing admin aliases %v (background issuance)", cfg.AdminAliases)
		}
	}

	return &Manager{magic: magic, issuer: issuer, cfg: cfg}, nil
}

// NewLoader creates the data-plane cert manager. It reads existing certs from the
// shared store AND provisions new ones on demand via ACME: the data plane is what
// receives redirect HTTPS traffic (the control plane only provisions its own admin
// cert), so on-demand issuance must run here. The ACME account is configured below
// or Let's Encrypt account registration fails.
func NewLoader(cfg Config) (*Manager, error) {
	setStorage(cfg)

	// Configure the ACME account for on-demand issuance (email + ToS agreement).
	// Without a valid email, LE rejects account registration with invalidContact.
	certmagic.DefaultACME.Agreed = true
	certmagic.DefaultACME.Email = cfg.Email
	if cfg.Staging {
		certmagic.DefaultACME.CA = certmagic.LetsEncryptStagingCA
	}

	magic := certmagic.NewDefault()
	issuer, err := configureIssuers(magic, cfg)
	if err != nil {
		return nil, err
	}

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

	log.Printf("certs: loader + on-demand ACME issuance (account email=%q)", cfg.Email)
	return &Manager{magic: magic, issuer: issuer, cfg: cfg}, nil
}

// configureIssuers sets the issuer chain: Let's Encrypt, then
// ZeroSSL when EAB credentials are configured. It returns the issuer whose HTTP
// challenge handler serves :80 — that handler finds challenge tokens for every
// issuer in the shared store, so any instance can answer an HTTP-01 challenge
// started by any other (ZeroSSL offers only http-01/dns-01, not tls-alpn-01).
func configureIssuers(magic *certmagic.Config, cfg Config) (*certmagic.ACMEIssuer, error) {
	le, ok := magic.Issuers[0].(*certmagic.ACMEIssuer)
	if !ok {
		return nil, fmt.Errorf("no ACME issuer configured")
	}
	if cfg.ZeroSSLEAB == nil {
		if cfg.ZeroSSLOnly {
			return nil, fmt.Errorf("ZeroSSL-only requested but no ZeroSSL EAB credentials set")
		}
		return le, nil
	}
	zs := certmagic.NewACMEIssuer(magic, certmagic.ACMEIssuer{
		CA:              certmagic.ZeroSSLProductionCA,
		Email:           cfg.Email,
		Agreed:          true,
		ExternalAccount: cfg.ZeroSSLEAB,
	})
	if cfg.ZeroSSLOnly {
		magic.Issuers = []certmagic.Issuer{zs}
		log.Printf("certs: issuers = ZeroSSL only (Let's Encrypt disabled)")
	} else {
		magic.Issuers = []certmagic.Issuer{le, zs}
		log.Printf("certs: issuers = Let's Encrypt, then ZeroSSL fallback")
	}
	return zs, nil
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
