package server

import (
	"net/http"
)

// HostMux routes requests based on the Host header.
// Requests to the admin domain go to the admin handler;
// everything else goes to the redirect engine.
type HostMux struct {
	AdminDomain  string
	AdminHandler http.Handler
	RedirectHandler http.Handler
}

func (m *HostMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := stripPort(r.Host)
	if host == m.AdminDomain {
		m.AdminHandler.ServeHTTP(w, r)
		return
	}
	m.RedirectHandler.ServeHTTP(w, r)
}

func stripPort(host string) string {
	for i := len(host) - 1; i >= 0; i-- {
		if host[i] == ':' {
			return host[:i]
		}
		if host[i] < '0' || host[i] > '9' {
			return host
		}
	}
	return host
}
