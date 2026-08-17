package certs

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"time"

	"github.com/caddyserver/certmagic"
)

// CertInfo is the status of one issued TLS certificate.
type CertInfo struct {
	Domain    string `json:"domain"`
	Issuer    string `json:"issuer"`
	NotBefore string `json:"not_before"`
	NotAfter  string `json:"not_after"`
	DaysLeft  int    `json:"days_left"`
	Expired   bool   `json:"expired"`
}

// ListCerts reads issued certificates from the active CertMagic storage (the
// shared cross-region store in production), so the Certificates view shows every
// cert the fleet has issued regardless of which instance/region issued it.
func ListCerts(ctx context.Context) ([]CertInfo, error) {
	st := certmagic.Default.Storage
	if st == nil {
		return []CertInfo{}, nil
	}
	keys, err := st.List(ctx, "certificates/", true)
	if err != nil {
		// No certs issued yet (empty store) reports as not-exist — treat as empty.
		return []CertInfo{}, nil
	}

	out := []CertInfo{}
	seen := map[string]bool{}
	for _, key := range keys {
		if !strings.HasSuffix(key, ".crt") {
			continue
		}
		parts := strings.Split(key, "/")
		domain := strings.TrimSuffix(parts[len(parts)-1], ".crt")
		if domain == "" || seen[domain] {
			continue
		}
		value, err := st.Load(ctx, key)
		if err != nil {
			continue
		}
		block, _ := pem.Decode(value)
		if block == nil {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}
		seen[domain] = true
		out = append(out, CertInfo{
			Domain:    domain,
			Issuer:    cert.Issuer.CommonName,
			NotBefore: cert.NotBefore.UTC().Format(time.RFC3339),
			NotAfter:  cert.NotAfter.UTC().Format(time.RFC3339),
			DaysLeft:  int(time.Until(cert.NotAfter).Hours() / 24),
			Expired:   time.Now().After(cert.NotAfter),
		})
	}
	return out, nil
}
