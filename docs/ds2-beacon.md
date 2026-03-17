# DS2 Beacon Observability Pattern

## Overview

Every redirect served by the data plane fires a non-blocking HTTP beacon to an Akamai edge property backed by DataStream 2 (DS2). This gives us per-request observability at the edge — without adding latency to the redirect response and without requiring log shipping infrastructure.

The pattern: encode redirect metadata in the URL path of a fire-and-forget GET request. DS2 captures the full request path in its `reqPath` field, and the data lands in a downstream webhook (ClickHouse, Splunk, S3, etc.) within 30 seconds.

## Why URL Path Encoding

DS2 strips query strings from `reqPath` in its dataset. To preserve redirect metadata, we encode everything in the URL path itself:

```
GET /beacon/{domain}/{status}/{path}/{target}/{clientIP}/{ua}/{referer}/{query}
```

Each segment is URL-path-escaped. Example beacon for a redirect of `legacy-trust-corp.example.com/mortgage` → `example-target.com/mortgage`:

```
GET /beacon/legacy-trust-corp.example.com/301/%2Fmortgage/https%3A%2F%2Fwww.example-target.com%2Fmortgage/192.168.1.1/Mozilla%2F5.0.../https%3A%2F%2Fgoogle.com/q%3Dlegacy+trust+mortgage
```

## Architecture

<p align="center">
  <a href="diagrams/ds2-beacon.excalidraw">
    <img src="diagrams/ds2-beacon.svg" alt="DS2 beacon pipeline" width="850">
  </a>
</p>

## Beacon Sender Implementation

The beacon sender (`internal/beacon/beacon.go`) runs 4 concurrent workers reading from a buffered channel. The redirect engine drops log entries into the channel non-blocking — if the channel is full, the beacon is silently dropped. This ensures beacon failures never impact redirect latency.

```go
type Sender struct {
    endpoint string           // e.g., https://ds2-beacon.connected-cloud.io/beacon
    client   *http.Client     // 2s timeout, connection pooling
    ch       chan RequestLogEntry
    workers  int              // 4 concurrent senders
}
```

Key design decisions:
- **Fire-and-forget**: response body is discarded, errors are logged but not retried
- **2-second timeout**: prevents slow edge responses from backing up the channel
- **Connection pooling**: 20 idle connections per host, 30s idle timeout
- **User-Agent cap**: truncated to 200 chars to keep URL length manageable
- **Non-blocking channel send**: `select { case ch <- entry: default: }` — drops on backpressure

## Akamai Edge Property Configuration

The beacon property (`ds2-beacon.connected-cloud.io`) is configured to:

1. **Never hit origin** — returns a synthetic 204 response via `constructResponse` behavior
2. **Never cache** — each request is unique, caching would hide data
3. **Enable DS2** — captures the full request path containing encoded redirect metadata

```json
{
  "name": "constructResponse",
  "options": {
    "enabled": true,
    "body": "ok",
    "responseCode": 204
  }
}
```

The property has no origin server. Akamai edge receives the beacon, returns 204 immediately, and DS2 captures the request metadata asynchronously.

## DS2 Dataset Fields

The DataStream 2 stream captures these fields per beacon request:

| Field ID | Name | What It Tells Us |
|----------|------|-----------------|
| 1000 | CP code | Billing/property identifier |
| 1002 | Request host | Always `ds2-beacon.connected-cloud.io` |
| 1005 | Request method | Always GET |
| 1006 | Request path | **The encoded redirect metadata** — domain, status, path, target, client IP, UA, referer, query |
| 1008 | Response status | Always 204 (synthetic) |
| 1016 | User-Agent | Beacon sender UA (identifies data plane instance) |
| 1102 | Turnaround time | Edge processing time (typically <1ms for synthetic response) |
| 2012 | Client IP | Data plane instance IP (not the end user — that's encoded in the path) |
| 2013 | Country | Data plane instance country |
| 3000 | Request ID | Unique Akamai request identifier for correlation |

## Delivery Configuration

DS2 delivers batched JSON payloads every 30 seconds via HTTPS webhook:

```json
{
  "delivery": {
    "format": "JSON",
    "frequency": { "intervalInSecs": 30 }
  },
  "connector": {
    "type": "HTTPS",
    "endpoint": "https://ds2-ingest.example.com/api/ds2/webhook",
    "authentication": "BASIC"
  }
}
```

The webhook receiver can be any HTTP endpoint — ClickHouse ingest, Splunk HEC, a Lambda writing to S3, etc. The JSON payload contains an array of log entries, each with the DS2 fields above.

## Decoding Beacon Data

To extract redirect metadata from the DS2 `reqPath` field, split on `/` and URL-decode each segment:

```python
# Example DS2 reqPath:
# /beacon/legacy-trust-corp.example.com/301/%2Fmortgage/https%3A%2F%2Fwww.example-target.com%2Fmortgage/192.168.1.1/Mozilla%2F5.0.../https%3A%2F%2Fgoogle.com/

import urllib.parse

parts = req_path.split("/")
# parts[0] = "" (leading slash)
# parts[1] = "beacon"
domain    = urllib.parse.unquote(parts[2])   # legacy-trust-corp.example.com
status    = int(parts[3])                    # 301
path      = urllib.parse.unquote(parts[4])   # /mortgage
target    = urllib.parse.unquote(parts[5])   # https://www.example-target.com/mortgage
client_ip = urllib.parse.unquote(parts[6])   # 192.168.1.1
user_agent= urllib.parse.unquote(parts[7])   # Mozilla/5.0...
referer   = urllib.parse.unquote(parts[8])   # https://google.com
query     = urllib.parse.unquote(parts[9]) if len(parts) > 9 else ""
```

## Operational Metrics

With this pattern you get:
- **Per-domain redirect volume** — which legacy domains still receive traffic
- **Path-level granularity** — which specific paths are hit (e.g., `/mortgage` vs `/online-banking`)
- **Client geography** — end-user IP encoded in the beacon (not the data plane IP)
- **Referer tracking** — where traffic originates (search engines, bookmarks, links)
- **User-agent distribution** — browser vs bot vs crawler breakdown
- **Cross-region traffic split** — DS2 client IP reveals which data plane region served the request

All of this without touching the redirect response, without log shipping agents, and with edge-level reliability (Akamai's infrastructure handles the beacon receipt).

## Limitations

- **Best-effort delivery**: DS2 does not guarantee zero data loss. During edge outages or high-volume spikes, some beacon data may be dropped. DS2's SLA is best-effort, not transactional.
- **URL length**: extremely long user agents or referers could exceed URL limits. The sender caps UA at 200 chars; very long referers are a theoretical edge case.
- **Beacon latency**: the 30-second DS2 batch interval means data is not real-time. For alerting, use the local analytics pipeline (5-second flush, 5-minute rollup) rather than DS2.
- **No retry**: dropped beacons (channel full, timeout) are lost. The local analytics pipeline in PG serves as the source of truth; DS2 is the observability layer.
