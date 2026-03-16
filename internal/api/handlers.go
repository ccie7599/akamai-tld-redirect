package api

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bapley/tld-redirect/internal/redirect"
	"github.com/bapley/tld-redirect/internal/store"
)

// Syncer publishes rules to Object Storage after mutations.
type Syncer interface {
	Publish(ctx context.Context) error
}

type Handler struct {
	store  *store.Store
	engine *redirect.Engine
	syncer Syncer
}

func NewHandler(s *store.Store, e *redirect.Engine) *Handler {
	return &Handler{store: s, engine: e}
}

func (h *Handler) SetSyncer(s Syncer) {
	h.syncer = s
}

// publish calls the syncer if configured (control plane only).
func (h *Handler) publish(r *http.Request) {
	if h.syncer != nil {
		if err := h.syncer.Publish(r.Context()); err != nil {
			log.Printf("api: sync publish failed: %v", err)
		}
	}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/domains", h.listDomains)
	mux.HandleFunc("POST /api/v1/domains", h.createDomain)
	mux.HandleFunc("GET /api/v1/domains/{domain}", h.getDomain)
	mux.HandleFunc("PUT /api/v1/domains/{domain}", h.updateDomain)
	mux.HandleFunc("DELETE /api/v1/domains/{domain}", h.deleteDomain)

	mux.HandleFunc("GET /api/v1/domains/{domain}/rules", h.listRules)
	mux.HandleFunc("POST /api/v1/domains/{domain}/rules", h.createRule)
	mux.HandleFunc("PUT /api/v1/domains/{domain}/rules/{id}", h.updateRule)
	mux.HandleFunc("DELETE /api/v1/domains/{domain}/rules/{id}", h.deleteRule)

	mux.HandleFunc("POST /api/v1/import", h.bulkImport)
	mux.HandleFunc("GET /api/v1/export", h.export)

	mux.HandleFunc("GET /api/v1/analytics/summary", h.analyticsSummary)
	mux.HandleFunc("GET /api/v1/analytics/domains/{domain}", h.analyticsDomain)
	mux.HandleFunc("GET /api/v1/analytics/domains/{domain}/paths", h.analyticsPaths)
	mux.HandleFunc("GET /api/v1/analytics/inactive", h.analyticsInactive)
	mux.HandleFunc("GET /api/v1/analytics/trending", h.analyticsTrending)
	mux.HandleFunc("GET /api/v1/analytics/referers", h.analyticsReferers)
	mux.HandleFunc("GET /api/v1/analytics/export", h.analyticsExport)
}

// --- Domain handlers ---

func (h *Handler) listDomains(w http.ResponseWriter, r *http.Request) {
	search := r.URL.Query().Get("search")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	domains, total, err := h.store.ListDomains(search, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"domains": domains,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

func (h *Handler) createDomain(w http.ResponseWriter, r *http.Request) {
	var d store.Domain
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if d.Name == "" || d.DefaultURL == "" {
		writeError(w, http.StatusBadRequest, "name and default_url are required")
		return
	}
	if d.StatusCode == 0 {
		d.StatusCode = 301
	}
	d.Enabled = true

	if err := h.store.CreateDomain(&d); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			writeError(w, http.StatusConflict, "domain already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.engine.Reload()
	h.publish(r)
	writeJSON(w, http.StatusCreated, d)
}

func (h *Handler) getDomain(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("domain")
	d, err := h.store.GetDomain(name)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "domain not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (h *Handler) updateDomain(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("domain")
	var d store.Domain
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := h.store.UpdateDomain(name, &d); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.engine.Reload()
	h.publish(r)
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *Handler) deleteDomain(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("domain")
	if err := h.store.DeleteDomain(name); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.engine.Reload()
	h.publish(r)
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// --- Rule handlers ---

func (h *Handler) listRules(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("domain")
	d, err := h.store.GetDomain(name)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "domain not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, d.Rules)
}

func (h *Handler) createRule(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("domain")
	d, err := h.store.GetDomain(name)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "domain not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var rule store.RedirectRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if rule.Path == "" || rule.TargetURL == "" {
		writeError(w, http.StatusBadRequest, "path and target_url are required")
		return
	}
	if rule.StatusCode == 0 {
		rule.StatusCode = 301
	}
	rule.DomainID = d.ID
	rule.Enabled = true

	if err := h.store.CreateRule(&rule); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			writeError(w, http.StatusConflict, "rule for this path already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.engine.Reload()
	h.publish(r)
	writeJSON(w, http.StatusCreated, rule)
}

func (h *Handler) updateRule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid rule id")
		return
	}
	var rule store.RedirectRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := h.store.UpdateRule(id, &rule); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.engine.Reload()
	h.publish(r)
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *Handler) deleteRule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid rule id")
		return
	}
	if err := h.store.DeleteRule(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.engine.Reload()
	h.publish(r)
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// --- Bulk operations ---

func (h *Handler) bulkImport(w http.ResponseWriter, r *http.Request) {
	var entries []store.ImportEntry
	if err := json.NewDecoder(r.Body).Decode(&entries); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	count, err := h.store.BulkImport(entries)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.engine.Reload()
	h.publish(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"imported": count,
	})
}

func (h *Handler) export(w http.ResponseWriter, r *http.Request) {
	entries, err := h.store.ExportAll()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

// --- Analytics handlers ---

func (h *Handler) analyticsSummary(w http.ResponseWriter, r *http.Request) {
	since := r.URL.Query().Get("since")
	if since == "" {
		since = time.Now().UTC().AddDate(0, 0, -7).Format(h.store.TimeFmt())
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	result, err := h.store.AnalyticsSummary(since, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) analyticsDomain(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	since := r.URL.Query().Get("since")
	if since == "" {
		since = time.Now().UTC().AddDate(0, 0, -7).Format(h.store.TimeFmt())
	}
	until := r.URL.Query().Get("until")
	if until == "" {
		until = time.Now().UTC().Format(h.store.TimeFmt())
	}
	granularity := r.URL.Query().Get("granularity")
	if granularity == "" {
		granularity = "hourly"
	}

	result, err := h.store.AnalyticsDomainTraffic(domain, since, until, granularity)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) analyticsPaths(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	since := r.URL.Query().Get("since")
	if since == "" {
		since = time.Now().UTC().AddDate(0, 0, -7).Format(h.store.TimeFmt())
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	result, err := h.store.AnalyticsDomainPaths(domain, since, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) analyticsInactive(w http.ResponseWriter, r *http.Request) {
	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	if days <= 0 {
		days = 30
	}
	result, err := h.store.AnalyticsInactive(days)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"days":    days,
		"domains": result,
		"count":   len(result),
	})
}

func (h *Handler) analyticsTrending(w http.ResponseWriter, r *http.Request) {
	since := r.URL.Query().Get("since")
	if since == "" {
		since = time.Now().UTC().AddDate(0, 0, -14).Format(h.store.TimeFmt())
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	result, err := h.store.AnalyticsTrending(since, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) analyticsReferers(w http.ResponseWriter, r *http.Request) {
	since := r.URL.Query().Get("since")
	if since == "" {
		since = time.Now().UTC().AddDate(0, 0, -7).Format(h.store.TimeFmt())
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	result, err := h.store.AnalyticsReferers(since, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) analyticsExport(w http.ResponseWriter, r *http.Request) {
	since := r.URL.Query().Get("since")
	if since == "" {
		since = time.Now().UTC().AddDate(0, 0, -7).Format(h.store.TimeFmt())
	}
	format := r.URL.Query().Get("format")

	result, err := h.store.AnalyticsExport(since, format)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment; filename=analytics-export.csv")
		cw := csv.NewWriter(w)
		headers := []string{"domain", "path", "query", "status_code", "target_url", "client_ip", "user_agent", "referer", "created_at"}
		cw.Write(headers)
		for _, row := range result {
			record := make([]string, len(headers))
			for i, h := range headers {
				record[i] = fmt.Sprintf("%v", row[h])
			}
			cw.Write(record)
		}
		cw.Flush()
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("api: json encode error: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
