package store

import (
	"os"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	f, err := os.CreateTemp("", "tld-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })

	s, err := New(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestCreateAndGetDomain(t *testing.T) {
	s := newTestStore(t)

	d := &Domain{Name: "test.example.com", DefaultURL: "https://target.com", StatusCode: 301, Enabled: true}
	if err := s.CreateDomain(d); err != nil {
		t.Fatalf("CreateDomain: %v", err)
	}
	if d.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	if d.CreatedAt == "" {
		t.Fatal("expected CreatedAt to be set")
	}

	got, err := s.GetDomain("test.example.com")
	if err != nil {
		t.Fatalf("GetDomain: %v", err)
	}
	if got.Name != "test.example.com" {
		t.Errorf("got name %q, want test.example.com", got.Name)
	}
	if got.DefaultURL != "https://target.com" {
		t.Errorf("got default_url %q, want https://target.com", got.DefaultURL)
	}
	if got.StatusCode != 301 {
		t.Errorf("got status %d, want 301", got.StatusCode)
	}
	if !got.Enabled {
		t.Error("expected enabled=true")
	}
}

func TestCreateDomainDuplicate(t *testing.T) {
	s := newTestStore(t)

	d := &Domain{Name: "dup.example.com", DefaultURL: "https://target.com", StatusCode: 301, Enabled: true}
	if err := s.CreateDomain(d); err != nil {
		t.Fatal(err)
	}
	d2 := &Domain{Name: "dup.example.com", DefaultURL: "https://other.com", StatusCode: 302, Enabled: true}
	if err := s.CreateDomain(d2); err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestUpdateDomain(t *testing.T) {
	s := newTestStore(t)

	d := &Domain{Name: "update.example.com", DefaultURL: "https://old.com", StatusCode: 301, Enabled: true}
	s.CreateDomain(d)

	updated := &Domain{DefaultURL: "https://new.com", StatusCode: 302, Enabled: false}
	if err := s.UpdateDomain("update.example.com", updated); err != nil {
		t.Fatalf("UpdateDomain: %v", err)
	}

	got, _ := s.GetDomain("update.example.com")
	if got.DefaultURL != "https://new.com" {
		t.Errorf("got default_url %q, want https://new.com", got.DefaultURL)
	}
	if got.StatusCode != 302 {
		t.Errorf("got status %d, want 302", got.StatusCode)
	}
	if got.Enabled {
		t.Error("expected enabled=false")
	}
}

func TestDeleteDomain(t *testing.T) {
	s := newTestStore(t)

	d := &Domain{Name: "delete.example.com", DefaultURL: "https://target.com", StatusCode: 301, Enabled: true}
	s.CreateDomain(d)

	if err := s.DeleteDomain("delete.example.com"); err != nil {
		t.Fatalf("DeleteDomain: %v", err)
	}

	_, err := s.GetDomain("delete.example.com")
	if err == nil {
		t.Fatal("expected not found after delete")
	}
}

func TestListDomains(t *testing.T) {
	s := newTestStore(t)

	for _, name := range []string{"alpha.example.com", "beta.example.com", "gamma.example.com"} {
		s.CreateDomain(&Domain{Name: name, DefaultURL: "https://target.com", StatusCode: 301, Enabled: true})
	}

	domains, total, err := s.ListDomains("", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 {
		t.Errorf("got total %d, want 3", total)
	}
	if len(domains) != 3 {
		t.Errorf("got %d domains, want 3", len(domains))
	}

	// Test search
	domains, total, _ = s.ListDomains("beta", 10, 0)
	if total != 1 {
		t.Errorf("search: got total %d, want 1", total)
	}

	// Test pagination
	domains, _, _ = s.ListDomains("", 2, 0)
	if len(domains) != 2 {
		t.Errorf("limit: got %d, want 2", len(domains))
	}
	domains, _, _ = s.ListDomains("", 2, 2)
	if len(domains) != 1 {
		t.Errorf("offset: got %d, want 1", len(domains))
	}
}

func TestCreateAndListRules(t *testing.T) {
	s := newTestStore(t)

	d := &Domain{Name: "rules.example.com", DefaultURL: "https://target.com", StatusCode: 301, Enabled: true}
	s.CreateDomain(d)

	r1 := &RedirectRule{DomainID: d.ID, Path: "/mortgage", TargetURL: "https://target.com/mortgage", StatusCode: 301, Priority: 10, Enabled: true}
	r2 := &RedirectRule{DomainID: d.ID, Path: "/about", TargetURL: "https://target.com/about", StatusCode: 301, Priority: 5, Enabled: true}
	if err := s.CreateRule(r1); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateRule(r2); err != nil {
		t.Fatal(err)
	}

	rules, err := s.ListRules(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 2 {
		t.Fatalf("got %d rules, want 2", len(rules))
	}
	// Should be sorted by priority DESC
	if rules[0].Priority != 10 {
		t.Errorf("first rule priority %d, want 10", rules[0].Priority)
	}
	if rules[1].Priority != 5 {
		t.Errorf("second rule priority %d, want 5", rules[1].Priority)
	}
}

func TestUpdateRule(t *testing.T) {
	s := newTestStore(t)

	d := &Domain{Name: "ruleupdate.example.com", DefaultURL: "https://target.com", StatusCode: 301, Enabled: true}
	s.CreateDomain(d)
	r := &RedirectRule{DomainID: d.ID, Path: "/old", TargetURL: "https://target.com/old", StatusCode: 301, Priority: 1, Enabled: true}
	s.CreateRule(r)

	updated := &RedirectRule{Path: "/new", TargetURL: "https://target.com/new", StatusCode: 302, Priority: 20, Enabled: false}
	if err := s.UpdateRule(r.ID, updated); err != nil {
		t.Fatal(err)
	}

	rules, _ := s.ListRules(d.ID)
	if rules[0].Path != "/new" {
		t.Errorf("got path %q, want /new", rules[0].Path)
	}
	if rules[0].StatusCode != 302 {
		t.Errorf("got status %d, want 302", rules[0].StatusCode)
	}
}

func TestDeleteRule(t *testing.T) {
	s := newTestStore(t)

	d := &Domain{Name: "ruledelete.example.com", DefaultURL: "https://target.com", StatusCode: 301, Enabled: true}
	s.CreateDomain(d)
	r := &RedirectRule{DomainID: d.ID, Path: "/gone", TargetURL: "https://target.com/gone", StatusCode: 301, Priority: 1, Enabled: true}
	s.CreateRule(r)

	s.DeleteRule(r.ID)
	rules, _ := s.ListRules(d.ID)
	if len(rules) != 0 {
		t.Errorf("got %d rules after delete, want 0", len(rules))
	}
}

func TestDeleteDomainCascadesRules(t *testing.T) {
	s := newTestStore(t)

	d := &Domain{Name: "cascade.example.com", DefaultURL: "https://target.com", StatusCode: 301, Enabled: true}
	s.CreateDomain(d)
	s.CreateRule(&RedirectRule{DomainID: d.ID, Path: "/a", TargetURL: "https://target.com/a", StatusCode: 301, Priority: 1, Enabled: true})
	s.CreateRule(&RedirectRule{DomainID: d.ID, Path: "/b", TargetURL: "https://target.com/b", StatusCode: 301, Priority: 1, Enabled: true})

	s.DeleteDomain("cascade.example.com")
	rules, _ := s.ListRules(d.ID)
	if len(rules) != 0 {
		t.Errorf("expected rules to be cascade deleted, got %d", len(rules))
	}
}

func TestBulkImport(t *testing.T) {
	s := newTestStore(t)

	entries := []ImportEntry{
		{DomainName: "bulk1.example.com", DefaultURL: "https://target.com", StatusCode: 301, Rules: []ImportRule{
			{Path: "/a", TargetURL: "https://target.com/a", StatusCode: 301, Priority: 10},
		}},
		{DomainName: "bulk2.example.com", DefaultURL: "https://target.com", StatusCode: 0}, // should default to 301
	}

	count, err := s.BulkImport(entries)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("imported %d, want 2", count)
	}

	d, _ := s.GetDomain("bulk2.example.com")
	if d.StatusCode != 301 {
		t.Errorf("default status %d, want 301", d.StatusCode)
	}

	// Import again — should be idempotent (INSERT OR IGNORE)
	count2, _ := s.BulkImport(entries)
	if count2 != 2 {
		t.Errorf("re-import count %d, want 2", count2)
	}
}

func TestBulkImportReplace(t *testing.T) {
	s := newTestStore(t)

	// Initial import
	s.BulkImport([]ImportEntry{
		{DomainName: "keep.example.com", DefaultURL: "https://old.com", StatusCode: 301, Rules: []ImportRule{
			{Path: "/old-path", TargetURL: "https://old.com/old", StatusCode: 301, Priority: 1},
		}},
		{DomainName: "orphan.example.com", DefaultURL: "https://orphan.com", StatusCode: 301},
	})

	// Replace with new set — keep.example.com updated, orphan.example.com deleted
	err := s.BulkImportReplace([]ImportEntry{
		{DomainName: "keep.example.com", DefaultURL: "https://new.com", StatusCode: 302, Rules: []ImportRule{
			{Path: "/new-path", TargetURL: "https://new.com/new", StatusCode: 302, Priority: 5},
		}},
		{DomainName: "added.example.com", DefaultURL: "https://added.com", StatusCode: 301},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify keep.example.com was updated
	d, err := s.GetDomain("keep.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if d.DefaultURL != "https://new.com" {
		t.Errorf("got default_url %q, want https://new.com", d.DefaultURL)
	}
	if len(d.Rules) != 1 || d.Rules[0].Path != "/new-path" {
		t.Errorf("expected updated rule /new-path, got %v", d.Rules)
	}

	// Verify orphan was deleted
	_, err = s.GetDomain("orphan.example.com")
	if err == nil {
		t.Fatal("expected orphan.example.com to be deleted")
	}

	// Verify added.example.com exists
	_, err = s.GetDomain("added.example.com")
	if err != nil {
		t.Fatalf("expected added.example.com to exist: %v", err)
	}
}

func TestExportAll(t *testing.T) {
	s := newTestStore(t)

	s.BulkImport([]ImportEntry{
		{DomainName: "export1.example.com", DefaultURL: "https://target.com", StatusCode: 301, Rules: []ImportRule{
			{Path: "/x", TargetURL: "https://target.com/x", StatusCode: 301, Priority: 1},
		}},
		{DomainName: "export2.example.com", DefaultURL: "https://target.com", StatusCode: 302},
	})

	entries, err := s.ExportAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Errorf("exported %d entries, want 2", len(entries))
	}
}

func TestBatchInsertLogs(t *testing.T) {
	s := newTestStore(t)

	logs := []RequestLogEntry{
		{Domain: "test.example.com", Path: "/", Status: 301, TargetURL: "https://target.com", ClientIP: "1.2.3.4", UserAgent: "test"},
		{Domain: "test.example.com", Path: "/about", Status: 301, TargetURL: "https://target.com/about", ClientIP: "5.6.7.8", UserAgent: "test"},
	}
	if err := s.BatchInsertLogs(logs); err != nil {
		t.Fatal(err)
	}

	// Verify via analytics export
	result, err := s.AnalyticsExport("2020-01-01", "json")
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 {
		t.Errorf("got %d log entries, want 2", len(result))
	}
}

func TestAnalyticsSummary(t *testing.T) {
	s := newTestStore(t)

	// Insert logs and run rollup
	for i := 0; i < 5; i++ {
		s.BatchInsertLogs([]RequestLogEntry{
			{Domain: "popular.example.com", Path: "/", Status: 301, TargetURL: "https://target.com", ClientIP: "1.2.3.4"},
		})
	}
	s.BatchInsertLogs([]RequestLogEntry{
		{Domain: "quiet.example.com", Path: "/", Status: 301, TargetURL: "https://target.com", ClientIP: "1.2.3.4"},
	})

	s.RunRollup(time.Now())

	summary, err := s.AnalyticsSummary("2020-01-01", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(summary) < 1 {
		t.Fatal("expected at least 1 domain in summary")
	}
	if summary[0].Domain != "popular.example.com" {
		t.Errorf("top domain: got %q, want popular.example.com", summary[0].Domain)
	}
}

func TestAnalyticsInactive(t *testing.T) {
	s := newTestStore(t)

	s.CreateDomain(&Domain{Name: "active.example.com", DefaultURL: "https://target.com", StatusCode: 301, Enabled: true})
	s.CreateDomain(&Domain{Name: "inactive.example.com", DefaultURL: "https://target.com", StatusCode: 301, Enabled: true})

	s.BatchInsertLogs([]RequestLogEntry{
		{Domain: "active.example.com", Path: "/", Status: 301, TargetURL: "https://target.com", ClientIP: "1.2.3.4"},
	})

	inactive, err := s.AnalyticsInactive(30)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, name := range inactive {
		if name == "inactive.example.com" {
			found = true
		}
		if name == "active.example.com" {
			t.Error("active.example.com should not be in inactive list")
		}
	}
	if !found {
		t.Error("inactive.example.com should be in inactive list")
	}
}

func TestPruneOldLogs(t *testing.T) {
	s := newTestStore(t)

	s.BatchInsertLogs([]RequestLogEntry{
		{Domain: "test.example.com", Path: "/", Status: 301, TargetURL: "https://target.com", ClientIP: "1.2.3.4"},
	})

	// Prune with future date — should delete everything
	deleted, err := s.PruneOldLogs(time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Errorf("pruned %d, want 1", deleted)
	}
}

func TestGetRuleDomainID(t *testing.T) {
	s := newTestStore(t)

	d := &Domain{Name: "ruleowner.example.com", DefaultURL: "https://target.com", StatusCode: 301, Enabled: true}
	s.CreateDomain(d)
	r := &RedirectRule{DomainID: d.ID, Path: "/x", TargetURL: "https://target.com/x", StatusCode: 301, Priority: 1, Enabled: true}
	s.CreateRule(r)

	domainID, err := s.GetRuleDomainID(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if domainID != d.ID {
		t.Errorf("got domain_id %d, want %d", domainID, d.ID)
	}
}
