# Tests

## Component Tests (Go)

53 tests across 3 packages covering the full API surface, redirect engine, and data store.

```bash
# Run all
make test

# Run a specific package
go test github.com/bapley/tld-redirect/internal/store -v
go test github.com/bapley/tld-redirect/internal/redirect -v
go test github.com/bapley/tld-redirect/internal/api -v
```

### Store (`internal/store/store_test.go`) — 17 tests

| Test | What it validates |
|------|-------------------|
| `TestCreateAndGetDomain` | Create domain, verify ID/timestamps populated, read back by name |
| `TestCreateDomainDuplicate` | UNIQUE constraint on domain name returns error |
| `TestUpdateDomain` | Update default_url, status_code, enabled flag |
| `TestDeleteDomain` | Delete by name, verify not found after |
| `TestListDomains` | List all, search filter, limit/offset pagination |
| `TestCreateAndListRules` | Create 2 rules, verify priority DESC ordering |
| `TestUpdateRule` | Update path, target_url, status_code, priority |
| `TestDeleteRule` | Delete rule, verify empty list after |
| `TestDeleteDomainCascadesRules` | FK cascade — deleting domain deletes its rules |
| `TestBulkImport` | Import 2 domains with rules, verify default status_code (301), idempotent re-import |
| `TestBulkImportReplace` | UPSERT + orphan cleanup — updated domains keep, orphans deleted, new domains added |
| `TestExportAll` | Export all domains/rules as ImportEntry array |
| `TestBatchInsertLogs` | Batch insert request logs, verify via analytics export |
| `TestAnalyticsSummary` | Insert logs, run rollup, verify top domain by hit count |
| `TestAnalyticsInactive` | Active domain (has logs) excluded, inactive domain included |
| `TestPruneOldLogs` | Delete logs older than threshold, verify count |
| `TestGetRuleDomainID` | Look up which domain owns a rule |

### Redirect Engine (`internal/redirect/engine_test.go`) — 11 tests

| Test | What it validates |
|------|-------------------|
| `TestRedirectDefaultURL` | Domain with no matching rule redirects to default_url |
| `TestRedirectWithRule` | Path-specific rule overrides default, uses rule's status code |
| `TestRedirectPriorityOrdering` | Higher-priority rule matched before lower-priority for overlapping paths |
| `TestRedirectPrefixMatch` | `/docs` rule matches `/docs/page` via prefix |
| `TestRedirectUnknownDomain` | Unknown domain returns 404 |
| `TestRedirectDisabledDomain` | Disabled domain returns 404 (not loaded into engine) |
| `TestRedirectDisabledRule` | Disabled rule skipped, falls through to default URL |
| `TestHasDomain` | HasDomain returns true/false, strips port from host |
| `TestReloadUpdatesMap` | After DB update + Reload(), engine serves new values |
| `TestBeaconChannel` | Redirect writes log entry to beacon channel with correct fields |
| `TestConcurrentRequests` | 100 goroutines hit engine simultaneously without race/panic |

### API Handlers (`internal/api/handlers_test.go`) — 25 tests

| Test | What it validates |
|------|-------------------|
| `TestAuthRequired` | Request without token returns 401 |
| `TestAuthBadToken` | Request with wrong token returns 401 |
| `TestCreateAndListDomains` | POST /domains → 201, GET /domains → list with total |
| `TestGetDomain` | GET /domains/{name} → 200 with domain + rules |
| `TestGetDomainNotFound` | GET /domains/{name} for nonexistent → 404 |
| `TestUpdateDomain` | PUT /domains/{name} → 200, verify updated fields |
| `TestDeleteDomain` | DELETE /domains/{name} → 200, GET after → 404 |
| `TestCreateDomainDuplicate` | POST duplicate name → 409 Conflict |
| `TestCreateDomainValidation` | Missing name → 400, missing default_url → 400 |
| `TestCreateAndListRules` | POST /domains/{name}/rules → 201, GET → list |
| `TestCreateRuleForNonexistentDomain` | POST rule for missing domain → 404 |
| `TestCreateRuleValidation` | Missing path → 400 |
| `TestUpdateRule` | PUT /domains/{name}/rules/{id} → 200 |
| `TestDeleteRule` | DELETE /domains/{name}/rules/{id} → 200 |
| `TestBulkImport` | POST /import with 2 entries → 200, imported=2 |
| `TestExport` | GET /export → 200, array of ImportEntry |
| `TestAnalyticsSummary` | GET /analytics/summary → 200 |
| `TestAnalyticsDomain` | GET /analytics/domains/{name} → 200 |
| `TestAnalyticsPaths` | GET /analytics/domains/{name}/paths → 200 |
| `TestAnalyticsInactive` | GET /analytics/inactive → 200 |
| `TestAnalyticsTrending` | GET /analytics/trending → 200 |
| `TestAnalyticsReferers` | GET /analytics/referers → 200 |
| `TestAnalyticsExportJSON` | GET /analytics/export → 200 JSON |
| `TestAnalyticsExportCSV` | GET /analytics/export?format=csv → 200, Content-Type: text/csv |
| `TestCORSPreflight` | OPTIONS → 204, Access-Control-Allow-Origin: * |

## Load Test (k6)

See [load/README.md](load/README.md) for the test of record — 23,779 req/sec, p50 2.78ms, 100% success rate at 500 concurrent VUs with 1,610 domains.
