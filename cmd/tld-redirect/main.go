package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bapley/tld-redirect/internal/analytics"
	"github.com/bapley/tld-redirect/internal/api"
	"github.com/bapley/tld-redirect/internal/beacon"
	"github.com/bapley/tld-redirect/internal/certs"
	"github.com/bapley/tld-redirect/internal/redirect"
	"github.com/bapley/tld-redirect/internal/server"
	"github.com/bapley/tld-redirect/internal/store"
	rsync "github.com/bapley/tld-redirect/internal/sync"
	"github.com/bapley/tld-redirect/internal/ui"
)

var version = "dev"

func main() {
	var (
		// Mode
		mode = flag.String("mode", "", "Run mode: 'control' or 'data' (empty = legacy single-binary)")

		// Database
		dbPath = flag.String("db", "tld-redirect.db", "SQLite database path (dev/legacy mode)")
		dbURL  = flag.String("db-url", envOr("TLD_DB_URL", ""), "PostgreSQL connection string (production)")

		// Auth
		token = flag.String("token", envOr("TLD_ADMIN_TOKEN", "changeme"), "Admin API token")

		// Seed
		seedFile = flag.String("seed", "", "Seed data file (JSON)")

		// TLS
		useTLS      = flag.Bool("tls", false, "Enable TLS via CertMagic (production mode)")
		adminDomain = flag.String("admin-domain", "redirects.connected-cloud.io", "Admin UI domain")
		acmeEmail   = flag.String("acme-email", "admin@connected-cloud.io", "ACME account email")
		certDir     = flag.String("cert-dir", "", "Certificate storage directory (file-based)")
		staging     = flag.Bool("staging-ca", false, "Use Let's Encrypt staging CA")

		// DS2 beacon
		ds2Endpoint = flag.String("ds2-endpoint", "", "DS2 beacon endpoint URL (empty = disabled)")

		// Dev mode listen addresses
		adminAddr    = flag.String("admin-addr", ":8080", "Admin listen address (dev mode)")
		redirectAddr = flag.String("redirect-addr", ":8081", "Redirect listen address (dev mode)")

		// Object Storage sync
		syncEndpoint  = flag.String("sync-endpoint", envOr("TLD_SYNC_ENDPOINT", ""), "S3-compatible endpoint for rules sync")
		syncBucket    = flag.String("sync-bucket", envOr("TLD_SYNC_BUCKET", "tld-redirect-sync"), "Object Storage bucket for rules sync")
		syncKey       = flag.String("sync-key", envOr("TLD_SYNC_KEY", "rules.json"), "Object key for rules sync")
		syncAccessKey = flag.String("sync-access-key", envOr("TLD_SYNC_ACCESS_KEY", ""), "Object Storage access key")
		syncSecretKey = flag.String("sync-secret-key", envOr("TLD_SYNC_SECRET_KEY", ""), "Object Storage secret key")
		syncRegion    = flag.String("sync-region", envOr("TLD_SYNC_REGION", "us-ord-1"), "Object Storage region")
	)
	flag.Parse()

	log.Printf("tld-redirect %s starting (mode=%s)", version, *mode)

	// Database — PG if db-url set, else SQLite
	var db *store.Store
	var err error
	if *dbURL != "" {
		db, err = store.NewPG(*dbURL)
		if err != nil {
			log.Fatalf("database (pg): %v", err)
		}
		log.Printf("database: connected to PostgreSQL")
	} else {
		db, err = store.New(*dbPath)
		if err != nil {
			log.Fatalf("database (sqlite): %v", err)
		}
		log.Printf("database: using SQLite at %s", *dbPath)
	}
	defer db.Close()

	// Branch on mode
	switch *mode {
	case "control":
		runControl(db, *token, *adminDomain, *acmeEmail, *certDir, *staging, *seedFile, syncCfg(*syncEndpoint, *syncBucket, *syncKey, *syncAccessKey, *syncSecretKey, *syncRegion))
	case "data":
		runData(db, *adminDomain, *certDir, *staging, *ds2Endpoint, syncCfg(*syncEndpoint, *syncBucket, *syncKey, *syncAccessKey, *syncSecretKey, *syncRegion))
	default:
		// Legacy single-binary mode (backwards compatible)
		runLegacy(db, *token, *seedFile, *useTLS, *adminDomain, *acmeEmail, *certDir, *staging, *ds2Endpoint, *adminAddr, *redirectAddr)
	}
}

func syncCfg(endpoint, bucket, key, accessKey, secretKey, region string) rsync.Config {
	return rsync.Config{
		Endpoint:  endpoint,
		Bucket:    bucket,
		Key:       key,
		AccessKey: accessKey,
		SecretKey: secretKey,
		Region:    region,
	}
}

// --- Control Plane ---

func runControl(db *store.Store, token, adminDomain, acmeEmail, certDir string, staging bool, seedFile string, syncCfg rsync.Config) {
	log.Printf("control plane: starting admin API + cert provisioner")

	// Redirect engine (for HasDomain check during cert provisioning)
	// Control plane doesn't serve redirects, but needs the domain list for cert decisions
	dummyLogCh := make(chan store.RequestLogEntry, 1)
	go func() {
		for range dummyLogCh {
		}
	}()
	engine := redirect.NewEngine(db, dummyLogCh)

	// Seed data if requested
	if seedFile != "" {
		if err := seed(db, engine, seedFile); err != nil {
			log.Fatalf("seed: %v", err)
		}
	}

	// API handlers
	apiHandler := api.NewHandler(db, engine)

	// CertMagic — full provisioner (DNS-01 via ACME)
	certCfg := certs.Config{
		AdminDomain: adminDomain,
		Email:       acmeEmail,
		DataDir:     certDir,
		Staging:     staging,
		CheckDomain: engine.HasDomain,
	}
	if db.Dialect() == store.DialectPostgres {
		certCfg.PGDB = db.DB()
	}
	certMgr, err := certs.NewManager(certCfg)
	if err != nil {
		log.Fatalf("certmagic: %v", err)
	}

	// Syncer — publish on mutations, poll for remote changes
	var syncer *rsync.Syncer
	if syncCfg.Endpoint != "" {
		syncer, err = rsync.New(syncCfg, db, engine.Reload)
		if err != nil {
			log.Fatalf("syncer: %v", err)
		}
		syncer.Start()
		defer syncer.Stop()
		apiHandler.SetSyncer(syncer)
	}

	adminRouter := buildAdminRouter(apiHandler, token)

	// HTTPS on :443
	tlsServer := &http.Server{
		Handler:   adminRouter,
		TLSConfig: certMgr.TLSConfig(),
	}
	tlsLn, err := tls.Listen("tcp", ":443", tlsServer.TLSConfig)
	if err != nil {
		log.Fatalf("tls listen: %v", err)
	}

	// HTTP on :80 — ACME challenges + HTTPS upgrade
	httpServer := &http.Server{
		Addr: ":80",
		Handler: certMgr.HTTPChallengeHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			target := "https://" + r.Host + r.URL.RequestURI()
			http.Redirect(w, r, target, http.StatusMovedPermanently)
		})),
	}

	go func() {
		log.Printf("control: HTTPS on :443 (admin: %s)", adminDomain)
		if err := tlsServer.Serve(tlsLn); err != http.ErrServerClosed {
			log.Fatalf("https: %v", err)
		}
	}()
	go func() {
		log.Printf("control: HTTP on :80 (ACME challenges)")
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("http: %v", err)
		}
	}()

	waitForShutdown(tlsServer, httpServer)
}

// --- Data Plane ---

func runData(db *store.Store, adminDomain, certDir string, staging bool, ds2Endpoint string, syncCfg rsync.Config) {
	log.Printf("data plane: starting redirect engine + beacon")

	// Analytics pipeline
	pipeline, logCh := analytics.NewPipeline(db, 10000)
	pipeline.Start()
	defer pipeline.Stop()

	// Redirect engine
	engine := redirect.NewEngine(db, logCh)

	// DS2 beacon
	if ds2Endpoint != "" {
		beaconSender, beaconCh := beacon.NewSender(ds2Endpoint, 10000)
		beaconSender.Start()
		defer beaconSender.Stop()
		engine.SetBeaconCh(beaconCh)
	}

	// CertMagic — loader only (reads certs from PG, no ACME provisioning)
	certCfg := certs.Config{
		AdminDomain: adminDomain,
		Staging:     staging,
		CheckDomain: engine.HasDomain,
	}
	if db.Dialect() == store.DialectPostgres {
		certCfg.PGDB = db.DB()
	} else if certDir != "" {
		certCfg.DataDir = certDir
	}
	certMgr, err := certs.NewLoader(certCfg)
	if err != nil {
		log.Fatalf("cert loader: %v", err)
	}

	// Syncer — poll for rule changes
	if syncCfg.Endpoint != "" {
		syncer, err := rsync.New(syncCfg, db, engine.Reload)
		if err != nil {
			log.Fatalf("syncer: %v", err)
		}
		syncer.Start()
		defer syncer.Stop()
	}

	// HTTPS on :443 — all domains served by redirect engine
	tlsServer := &http.Server{
		Handler:   engine,
		TLSConfig: certMgr.TLSConfig(),
	}
	tlsLn, err := tls.Listen("tcp", ":443", tlsServer.TLSConfig)
	if err != nil {
		log.Fatalf("tls listen: %v", err)
	}

	// HTTP on :80 — redirects (for domains) + HTTPS upgrade
	httpFallback := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := stripHostPort(r.Host)
		if engine.HasDomain(host) {
			engine.ServeHTTP(w, r)
			return
		}
		target := "https://" + r.Host + r.URL.RequestURI()
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	})
	httpServer := &http.Server{
		Addr:    ":80",
		Handler: certMgr.HTTPChallengeHandler(httpFallback),
	}

	go func() {
		log.Printf("data: HTTPS on :443 (redirects)")
		if err := tlsServer.Serve(tlsLn); err != http.ErrServerClosed {
			log.Fatalf("https: %v", err)
		}
	}()
	go func() {
		log.Printf("data: HTTP on :80 (redirects + ACME)")
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("http: %v", err)
		}
	}()

	waitForShutdown(tlsServer, httpServer)
}

// --- Legacy Mode (backwards compatible) ---

func runLegacy(db *store.Store, token, seedFile string, useTLS bool, adminDomain, acmeEmail, certDir string, staging bool, ds2Endpoint, adminAddr, redirectAddr string) {
	// Analytics pipeline
	pipeline, logCh := analytics.NewPipeline(db, 10000)
	pipeline.Start()
	defer pipeline.Stop()

	// Redirect engine
	engine := redirect.NewEngine(db, logCh)

	// DS2 beacon (optional)
	if ds2Endpoint != "" {
		beaconSender, beaconCh := beacon.NewSender(ds2Endpoint, 10000)
		beaconSender.Start()
		defer beaconSender.Stop()
		engine.SetBeaconCh(beaconCh)
	}

	// Seed data if requested
	if seedFile != "" {
		if err := seed(db, engine, seedFile); err != nil {
			log.Fatalf("seed: %v", err)
		}
	}

	// API handlers
	apiHandler := api.NewHandler(db, engine)

	if useTLS {
		runTLS(db, engine, apiHandler, token, adminDomain, acmeEmail, certDir, staging)
	} else {
		runDev(engine, apiHandler, token, adminAddr, redirectAddr)
	}
}

// buildAdminRouter creates the admin HTTP router with API (auth required),
// UI (no auth — JS handles token), and health check.
func buildAdminRouter(apiHandler *api.Handler, token string) http.Handler {
	router := http.NewServeMux()

	// API routes — protected by token auth
	apiMux := http.NewServeMux()
	apiHandler.Register(apiMux)
	router.Handle("/api/", api.TokenAuth(token, api.CORS(apiMux)))

	// UI routes — no auth on static assets, JS reads token from URL
	router.Handle("/ui/", ui.Handler())

	// Health check — no auth
	router.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	// Root → redirect to UI
	router.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ui/", http.StatusFound)
	})

	return api.Logging(router)
}

func runTLS(db *store.Store, engine *redirect.Engine, apiHandler *api.Handler, token, adminDomain, acmeEmail, certDir string, staging bool) {
	adminRouter := buildAdminRouter(apiHandler, token)

	// Host-based mux: admin domain → admin router, everything else → redirects
	hostMux := &server.HostMux{
		AdminDomain:     adminDomain,
		AdminHandler:    adminRouter,
		RedirectHandler: engine,
	}

	// CertMagic
	certCfg := certs.Config{
		AdminDomain: adminDomain,
		Email:       acmeEmail,
		DataDir:     certDir,
		Staging:     staging,
		CheckDomain: engine.HasDomain,
	}
	if db.Dialect() == store.DialectPostgres {
		certCfg.PGDB = db.DB()
	}
	certMgr, err := certs.NewManager(certCfg)
	if err != nil {
		log.Fatalf("certmagic: %v", err)
	}

	// HTTPS server on :443
	tlsServer := &http.Server{
		Handler:   hostMux,
		TLSConfig: certMgr.TLSConfig(),
	}
	tlsLn, err := tls.Listen("tcp", ":443", tlsServer.TLSConfig)
	if err != nil {
		log.Fatalf("tls listen: %v", err)
	}

	// HTTP server on :80 — ACME challenges + domain redirects + HTTPS upgrade for admin
	httpFallback := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := stripHostPort(r.Host)
		if engine.HasDomain(host) {
			engine.ServeHTTP(w, r)
			return
		}
		target := "https://" + r.Host + r.URL.RequestURI()
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	})
	httpServer := &http.Server{
		Addr:    ":80",
		Handler: certMgr.HTTPChallengeHandler(httpFallback),
	}

	go func() {
		log.Printf("HTTPS server listening on :443 (admin: %s)", adminDomain)
		if err := tlsServer.Serve(tlsLn); err != http.ErrServerClosed {
			log.Fatalf("https server: %v", err)
		}
	}()
	go func() {
		log.Printf("HTTP server listening on :80 (ACME + redirect)")
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("http server: %v", err)
		}
	}()

	waitForShutdown(tlsServer, httpServer)
}

func runDev(engine *redirect.Engine, apiHandler *api.Handler, token, adminAddr, redirectAddr string) {
	adminRouter := buildAdminRouter(apiHandler, token)

	adminSrv := &http.Server{
		Addr:    adminAddr,
		Handler: adminRouter,
	}

	redirectSrv := &http.Server{
		Addr:    redirectAddr,
		Handler: engine,
	}

	go func() {
		log.Printf("admin server listening on %s (dev mode)", adminAddr)
		log.Printf("  UI: http://localhost%s/ui/?token=%s", adminAddr, token)
		if err := adminSrv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("admin server: %v", err)
		}
	}()
	go func() {
		log.Printf("redirect server listening on %s (dev mode)", redirectAddr)
		if err := redirectSrv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("redirect server: %v", err)
		}
	}()

	waitForShutdown(adminSrv, redirectSrv)
}

func waitForShutdown(servers ...*http.Server) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, s := range servers {
		s.Shutdown(ctx)
	}
	log.Println("shutdown complete")
}

func seed(db *store.Store, engine *redirect.Engine, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open seed file: %w", err)
	}
	defer f.Close()

	var entries []store.ImportEntry
	if err := json.NewDecoder(f).Decode(&entries); err != nil {
		return fmt.Errorf("parse seed file: %w", err)
	}

	count, err := db.BulkImport(entries)
	if err != nil {
		return fmt.Errorf("import: %w", err)
	}
	log.Printf("seed: imported %d domains from %s", count, path)
	engine.Reload()
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func stripHostPort(host string) string {
	if i := len(host) - 1; i >= 0 {
		for j := i; j >= 0; j-- {
			if host[j] == ':' {
				return host[:j]
			}
			if host[j] < '0' || host[j] > '9' {
				return host
			}
		}
	}
	return host
}
