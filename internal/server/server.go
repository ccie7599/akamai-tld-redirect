package server

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/bapley/tld-redirect/internal/api"
	"github.com/bapley/tld-redirect/internal/redirect"
)

type Config struct {
	AdminAddr string
	AdminToken string
}

// NewAdminServer creates the API/UI server on a separate port
func NewAdminServer(cfg Config, apiHandler *api.Handler, engine *redirect.Engine) *http.Server {
	mux := http.NewServeMux()
	apiHandler.Register(mux)

	// Health check
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	// Protected API with token auth
	protected := api.TokenAuth(cfg.AdminToken, api.CORS(mux))
	logged := api.Logging(protected)

	return &http.Server{
		Addr:    cfg.AdminAddr,
		Handler: logged,
	}
}

// NewRedirectServer creates the public-facing redirect server on ports 80/443
func NewRedirectServer(addr string, engine *redirect.Engine) *http.Server {
	return &http.Server{
		Addr:    addr,
		Handler: engine,
	}
}

func Shutdown(ctx context.Context, servers ...*http.Server) {
	for _, s := range servers {
		if err := s.Shutdown(ctx); err != nil {
			log.Printf("server shutdown error: %v", err)
		}
	}
}
