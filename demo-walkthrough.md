# PNC Demo Walkthrough — Thursday 2026-03-20, 10:00 AM (45 min)

**Attendees:** Eric Landgraf, Steve Skalley (PNC), Rob Yates (SE), Brian Apley

## Agenda (45 min)

| Time | Segment | Duration |
|------|---------|----------|
| 0:00 | Context recap + what we built | 3 min |
| 0:03 | Live demo — Admin UI walkthrough | 12 min |
| 0:15 | Architecture deep-dive | 8 min |
| 0:23 | DS2 observability — the "which domains still get traffic" answer | 7 min |
| 0:30 | Security + production readiness path | 5 min |
| 0:35 | Q&A + next steps | 10 min |

---

## Segment 1 — Context Recap (3 min)

**Key points to hit:**
- "Last week you described 2000+ legacy domains on F5 appliances — single DC, single point of failure, no visibility into which domains still get traffic"
- "We built the production-grade version of what we discussed — two regions, six nodes, automatic TLS, full API, and per-redirect analytics"
- "Everything I'm about to show you is live right now"

---

## Segment 2 — Live Demo: Admin UI (12 min)

**Open:** https://tld-control-ord.connected-cloud.io/ui/?token=62aef5f15078b36544317ca5d497839f

### 2a. Domain list (2 min)
- Show the 10 sample domains loaded
- Point out enabled/disabled toggle, status codes, search
- "This is what your 2000 domains would look like — searchable, sortable, with traffic counts"

### 2b. Create a domain live (3 min)
- Create `demo-acquisition.example.com` → `https://www.pnc.com`
- Add a path rule: `/mortgage` → `https://www.pnc.com/mortgage` (priority 10)
- Add another: `/about` → `https://www.pnc.com/about` (priority 5)
- "That domain is now live and serving redirects"

### 2c. Cross-region sync (2 min)
- Open IAD UI in second tab: https://tld-control-iad.connected-cloud.io/ui/?token=62aef5f15078b36544317ca5d497839f
- Show the domain appeared in IAD within seconds
- "No manual replication — Object Storage syncs rules across regions every 5 seconds"

### 2d. Test the redirect (2 min)
- In terminal: `curl -sI -H "Host: demo-acquisition.example.com" http://172.238.165.172/`
- Show 301 → pnc.com
- `curl -sI -H "Host: demo-acquisition.example.com" http://172.238.165.172/mortgage`
- Show 301 → pnc.com/mortgage
- "That went through the Chicago NodeBalancer. Same thing works through Washington DC"
- `curl -sI -H "Host: demo-acquisition.example.com" http://139.144.195.77/`

### 2e. Disable a domain (1 min)
- Toggle `demo-acquisition.example.com` to disabled in ORD UI
- Show it's disabled in IAD within seconds
- "When you're ready to decommission a domain, disable it first — if nobody complains, delete it"

### 2f. Bulk import (2 min)
- "For your 2000 domains, you wouldn't click through one by one"
- Show the Import tab
- "This accepts a JSON array — we have a bulk import API that handles thousands in a single call"
- Reference the OpenAPI spec in the API Docs tab

---

## Segment 3 — Architecture Deep-Dive (8 min)

**Open:** https://github.com/ccie7599/akamai-tld-redirect (README)

### 3a. Architecture diagram (3 min)
- Walk through the diagram: client → DNS → NodeBalancer → data plane → 301
- "Two regions, two data nodes per region — if a node goes down, the NB routes to the other"
- "If an entire region goes down, DNS round-robin sends clients to the other region"
- "Control plane is separate — if admin goes down, redirects keep serving"

### 3b. What's managed vs what you manage (3 min)
- Reference the Infrastructure Components table
- "PostgreSQL, NodeBalancers, Object Storage — all managed PaaS. Auto-failover, auto-backup, no patching"
- "The only thing you manage is the Go binary on 6 instances — one binary, one systemd service, one env file"

### 3c. Performance numbers (2 min)
- "We load tested this with 1,600 domains and 500 concurrent users"
- "23,000 requests per second, median latency under 3 milliseconds, zero failures"
- "That's through a single region — both regions together would double that"
- Pause and let Steve react — this is where the F5 comparison hits home

---

## Segment 4 — DS2 Observability (7 min)

*This is the segment Steve cares about most — the "which domains still get traffic" question from last week.*

### 4a. The problem (1 min)
- "Steve, you mentioned last week that you have no visibility into F5 redirect traffic"
- "Every redirect this engine serves fires a beacon to Akamai DataStream 2"

### 4b. What gets captured (2 min)
- Domain, path, status code, client IP, user agent, referer, query string
- "So you can see: national-city-bank.com got 47 hits last month, 30 from Google organic, 12 from direct bookmarks, 5 from bing"
- "You can see which paths are hit — /mortgage gets traffic, /careers doesn't"

### 4c. Analytics dashboard (2 min)
- Show the Analytics tab in the UI
- "This is built-in analytics from the local PG database — top domains, traffic trends, inactive domains"
- "The DS2 data feeds into whatever you already use — ClickHouse, Splunk, S3, any webhook endpoint"

### 4d. The cleanup use case (2 min)
- Show the inactive domains API: `GET /api/v1/analytics/inactive?days=30`
- "This tells you which domains haven't received a single hit in 30 days — those are your decommission candidates"
- "Between the analytics dashboard and DS2 data, you can build the decision tree you mentioned for domain cleanup"

---

## Segment 5 — Security + Production Path (5 min)

### 5a. What's in place (2 min)
- Cloud Firewalls on every component — NodeBalancers, data plane, control plane
- Production Let's Encrypt TLS on control planes
- Data plane only reachable through NodeBalancer (no direct access)
- Token auth on all admin APIs

### 5b. What we'd add for production (3 min)
- **Certificate storage in Vault** — "Right now certs are in PG. For production we'd move private keys into your existing Vault instance. The architecture supports it — just swap the storage backend."
- **App & API Protector** — "We can front this with Akamai WAF if you need bot protection or DDoS mitigation. Documented but not deployed — easy to add."
- **DNS-01 cert provisioning** — "Currently using HTTP-01 challenges. For production with 2000 domains, we'd switch to DNS-01 so certs provision without pointing DNS to the control plane first."
- **Per-user auth** — "The current shared token works for POC. Production would integrate with your identity provider."

---

## Segment 6 — Q&A + Next Steps (10 min)

**Close with:**
- "The repo is public — full source, Terraform, documentation, test results"
- "What would you need to see to move forward with a pilot?"

---

## Expected Questions & Responses

### "How do we handle the DNS cutover for 2000 domains?"

You don't have to do them all at once. The engine handles domains as they arrive — add them to the system, point DNS when ready. You can migrate 10 domains this week, 50 next week, 200 the week after. Each domain is independent. The bulk import API handles the initial load, and DNS changes can be rolled out in batches. Works with your existing DNS provider — no requirement to move to Akamai Edge DNS.

### "What happens to traffic during the DNS cutover?"

DNS TTLs. Set the TTL low (60-300s) before the cutover. Once the A record points to the NodeBalancer IP, traffic starts flowing to the new engine immediately. The old F5 and new engine can run in parallel — there's no hard cutover moment.

### "What about HTTPS for the redirect domains? We need certs for 2000 domains."

CertMagic provisions certs automatically via Let's Encrypt. For production we'd use DNS-01 challenges (proving domain ownership via a DNS TXT record) so certs can be provisioned before traffic arrives. The engine handles renewal automatically — Let's Encrypt certs are 90-day, CertMagic renews at 30 days remaining. For production key management, private keys would live in Vault, not PG.

### "What's the ongoing operational cost?" (Eric will ask this)

The managed components (PG, NodeBalancers, Object Storage) are ~$100/mo and require zero maintenance. The compute (6 instances) is the bulk. Total infrastructure is in the ~$500/mo range for two-region, six-node deployment. Compare that to the F5 appliance licensing, data center costs, and the operational hours spent managing it.

### "Can we get an SLA on this?" (Steve will ask this)

The projected availability is 99.9996% — both regions would need to fail simultaneously. That's based on the published component SLAs. For a contractual SLA with financial backing, that would be part of an enterprise agreement discussion. The architecture is designed for it — the question is the commercial wrapper.

### "What about security? Our security team will want to review this." (Steve raised this last week)

The architecture is designed for their review: Cloud Firewalls on every component, TLS everywhere, no direct access to data plane, separate control/data planes so admin compromise doesn't affect redirect serving. For production: private keys move to Vault, admin auth integrates with your IdP, and we can add App & API Protector for WAF/bot/DDoS. The codebase is open — happy to do a security walkthrough with your team.

### "Can we use this for non-redirect use cases? What about rewrites?"

The engine serves HTTP redirects (301/302). It doesn't do URL rewrites (same-origin path mapping). If you need rewrites, that's an Akamai edge use case — Property Manager rules or EdgeWorkers. This engine is purpose-built for the TLD redirect consolidation problem.

### "What if we want to manage this ourselves vs. having Akamai manage it?"

Both models work. The repo has everything — source code, Terraform, deploy scripts, documentation. Your team can run it. Or we can propose a managed engagement where Akamai/delivery team operates it. The HANDOFF.md is designed for exactly this transition. The operational surface is small — one binary, systemd, env file. No Kubernetes, no containers, no orchestration complexity.

### "What about DNSSEC?" (Steve mentioned this last week)

Not implemented in this POC. DNSSEC is a DNS-layer concern — the redirect engine doesn't interact with DNS. If your DNS provider supports DNSSEC and you enable it, the redirect engine works exactly the same. The only consideration is that DNS-01 cert provisioning needs to create TXT records, which requires DNSSEC-compatible tooling if the zone is signed.

---

## Demo Prep Checklist

- [ ] Both admin UIs accessible (test from home IP before call)
- [ ] Both NodeBalancers returning 301s
- [ ] Clean up the load test domains from the UI (the 1,600 seeded domains) — leave only the 10 sample domains for a clean demo
- [ ] Have terminal ready with curl commands pre-typed
- [ ] Have the GitHub repo open in a browser tab
- [ ] Have the request-flow diagram open for the architecture segment
