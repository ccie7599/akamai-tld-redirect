package store

import (
	"crypto/x509"
	"encoding/pem"
	"strings"
	"time"
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

// ListCerts reads issued certificates from the CertMagic cert_store table and
// returns per-domain status (issuer, validity window, days remaining). Returns
// an empty list (not an error) when no certs have been provisioned yet.
func (s *Store) ListCerts() ([]CertInfo, error) {
	rows, err := s.db.Query(s.q("SELECT key, value FROM cert_store WHERE key LIKE ? ORDER BY key"), "%.crt")
	if err != nil {
		// cert_store may not exist until the first cert is provisioned.
		return []CertInfo{}, nil
	}
	defer rows.Close()

	out := []CertInfo{}
	seen := map[string]bool{}
	for rows.Next() {
		var key string
		var value []byte
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		// certmagic key: certificates/<issuer-dir>/<domain>/<domain>.crt
		parts := strings.Split(key, "/")
		domain := strings.TrimSuffix(parts[len(parts)-1], ".crt")
		if domain == "" || seen[domain] {
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
	return out, rows.Err()
}
