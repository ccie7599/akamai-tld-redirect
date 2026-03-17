(function() {
    'use strict';

    // ─── Chart.js Dark Theme ───
    Chart.defaults.color = '#888';
    Chart.defaults.borderColor = 'rgba(255,255,255,0.06)';
    Chart.defaults.backgroundColor = '#4a9eff';

    // ─── State ───
    let TOKEN = new URLSearchParams(window.location.search).get('token') || sessionStorage.getItem('redirect_token') || '';
    let charts = {};

    // ─── API Client ───
    const api = {
        async get(path) {
            const sep = path.includes('?') ? '&' : '?';
            const res = await fetch(`/api/v1${path}${sep}token=${TOKEN}`);
            if (!res.ok) throw new Error((await res.json()).error || res.statusText);
            return res.json();
        },
        async post(path, body) {
            const sep = path.includes('?') ? '&' : '?';
            const res = await fetch(`/api/v1${path}${sep}token=${TOKEN}`, {
                method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify(body)
            });
            if (!res.ok) throw new Error((await res.json()).error || res.statusText);
            return res.json();
        },
        async put(path, body) {
            const sep = path.includes('?') ? '&' : '?';
            const res = await fetch(`/api/v1${path}${sep}token=${TOKEN}`, {
                method: 'PUT', headers: {'Content-Type': 'application/json'}, body: JSON.stringify(body)
            });
            if (!res.ok) throw new Error((await res.json()).error || res.statusText);
            return res.json();
        },
        async del(path) {
            const sep = path.includes('?') ? '&' : '?';
            const res = await fetch(`/api/v1${path}${sep}token=${TOKEN}`, { method: 'DELETE' });
            if (!res.ok) throw new Error((await res.json()).error || res.statusText);
            return res.json();
        }
    };

    // ─── Toast ───
    function toast(msg, type = 'success') {
        const el = document.createElement('div');
        el.className = `toast toast-${type}`;
        el.textContent = msg;
        document.getElementById('toast-container').appendChild(el);
        setTimeout(() => el.remove(), 3000);
    }

    // ─── DS2 UI Beacon ───
    // Fires a beacon to the DS2 edge property on each page view.
    // Same path-encoding pattern as redirect beacons, prefixed with /ui-visit/.
    // Format: /ui-visit/{admin-host}/{page}/{referrer}
    const DS2_BEACON = (function() {
        const el = document.querySelector('meta[name="ds2-beacon"]');
        return el ? el.content : '';
    })();

    function beaconPageView(page) {
        if (!DS2_BEACON) return;
        const host = encodeURIComponent(window.location.hostname);
        const pg = encodeURIComponent(page || '/');
        const ref = encodeURIComponent(document.referrer || 'direct');
        new Image().src = `${DS2_BEACON}/ui-visit/${host}/${pg}/${ref}/${Date.now()}`;
    }

    // ─── Router ───
    function navigate(hash) {
        window.location.hash = hash;
    }

    function getRoute() {
        const hash = window.location.hash.slice(1) || '/domains';
        return hash;
    }

    async function route() {
        const path = getRoute();
        const app = document.getElementById('app');

        // DS2 beacon for UI page view
        beaconPageView(path);

        // Destroy old charts
        Object.values(charts).forEach(c => c.destroy());
        charts = {};

        // Update nav active state
        document.querySelectorAll('.nav-link').forEach(l => {
            l.classList.toggle('active', path.startsWith('#' + l.getAttribute('data-route')) ||
                path.startsWith('/' + l.getAttribute('data-route')));
        });

        try {
            if (path.match(/^\/domains\/(.+)$/)) {
                const name = decodeURIComponent(path.match(/^\/domains\/(.+)$/)[1]);
                await renderDomainDetail(app, name);
            } else if (path === '/analytics') {
                await renderAnalytics(app);
            } else if (path === '/import') {
                renderImport(app);
            } else if (path === '/docs') {
                renderAPIDocs(app);
            } else {
                await renderDomainList(app);
            }
        } catch (err) {
            app.innerHTML = `<div class="card"><h3>Error</h3><p>${err.message}</p></div>`;
        }
    }

    // ─── Domain List View ───
    async function renderDomainList(app) {
        app.innerHTML = `
            <div class="page-header">
                <h1>Domains</h1>
                <div class="actions">
                    <button class="btn btn-primary" onclick="window._addDomain()">+ Add Domain</button>
                </div>
            </div>
            <div class="card">
                <div class="search-bar" style="margin-bottom: 1rem">
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><path d="m21 21-4.35-4.35"/></svg>
                    <input type="text" id="domain-search" placeholder="Search domains...">
                </div>
                <div class="table-wrap">
                    <table>
                        <thead>
                            <tr>
                                <th>Domain</th>
                                <th>Default Target</th>
                                <th>Code</th>
                                <th>Status</th>
                                <th>Hits (7d)</th>
                                <th>Actions</th>
                            </tr>
                        </thead>
                        <tbody id="domain-tbody"><tr><td colspan="6" style="text-align:center;padding:2rem">Loading...</td></tr></tbody>
                    </table>
                </div>
            </div>`;

        const [domainRes, hitsRes] = await Promise.all([
            api.get('/domains?limit=1000'),
            api.get('/analytics/summary?since=' + sevenDaysAgo()).catch(() => [])
        ]);

        const domains = domainRes.domains || [];
        const hits = Array.isArray(hitsRes) ? hitsRes : [];
        const hitMap = {};
        hits.forEach(h => hitMap[h.domain] = h.hit_count);
        const maxHits = Math.max(1, ...Object.values(hitMap));

        function renderTable(filter) {
            const filtered = filter
                ? domains.filter(d => d.name.includes(filter.toLowerCase()))
                : domains;

            const tbody = document.getElementById('domain-tbody');
            if (filtered.length === 0) {
                tbody.innerHTML = '<tr><td colspan="6" class="empty-state">No domains found</td></tr>';
                return;
            }
            tbody.innerHTML = filtered.map(d => {
                const hc = hitMap[d.name] || 0;
                const pct = Math.round((hc / maxHits) * 100);
                return `<tr>
                    <td><a class="domain-link" href="#/domains/${encodeURIComponent(d.name)}">${esc(d.name)}</a></td>
                    <td style="max-width:300px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap" title="${esc(d.default_url)}">${esc(d.default_url)}</td>
                    <td><span class="badge badge-neutral">${d.status_code}</span></td>
                    <td>${d.enabled ? '<span class="badge badge-success">Active</span>' : '<span class="badge badge-danger">Disabled</span>'}</td>
                    <td><div class="hit-bar"><div class="hit-bar-fill" style="width:${pct}px"></div><span class="hit-bar-label">${hc.toLocaleString()}</span></div></td>
                    <td><button class="btn btn-sm btn-outline" onclick="window._deleteDomain('${esc(d.name)}')">Delete</button></td>
                </tr>`;
            }).join('');
        }

        renderTable('');
        document.getElementById('domain-search').addEventListener('input', e => renderTable(e.target.value));

        // Update sidebar stats
        const inactive = await api.get('/analytics/inactive?days=30').catch(() => ({count: 0}));
        document.getElementById('nav-stats').innerHTML = `
            <span>${domains.length} domains</span>
            <span>${inactive.count || 0} inactive (30d)</span>`;
    }

    window._addDomain = function() {
        showModal('Add Domain', `
            <div class="form-group"><label>Domain Name</label><input class="form-input" id="m-name" placeholder="legacy-bank.com"></div>
            <div class="form-group"><label>Default Redirect URL</label><input class="form-input" id="m-url" placeholder="https://www.example.com"></div>
            <div class="form-row">
                <div class="form-group"><label>Status Code</label>
                    <select class="form-select" id="m-code"><option value="301">301 Permanent</option><option value="302">302 Temporary</option></select>
                </div>
            </div>`,
            async () => {
                const name = document.getElementById('m-name').value.trim();
                const url = document.getElementById('m-url').value.trim();
                const code = parseInt(document.getElementById('m-code').value);
                if (!name || !url) { toast('Name and URL required', 'error'); return false; }
                await api.post('/domains', { name, default_url: url, status_code: code, enabled: true });
                toast('Domain added');
                route();
                return true;
            }
        );
    };

    window._deleteDomain = async function(name) {
        if (!confirm(`Delete ${name} and all its rules?`)) return;
        try {
            await api.del(`/domains/${encodeURIComponent(name)}`);
            toast('Domain deleted');
            route();
        } catch (e) { toast(e.message, 'error'); }
    };

    // ─── Domain Detail View ───
    async function renderDomainDetail(app, name) {
        const [domain, pathData] = await Promise.all([
            api.get(`/domains/${encodeURIComponent(name)}`),
            api.get(`/analytics/domains/${encodeURIComponent(name)}/paths?since=${sevenDaysAgo()}`).catch(() => [])
        ]);

        const trafficData = await api.get(`/analytics/domains/${encodeURIComponent(name)}?since=${thirtyDaysAgo()}&granularity=daily`).catch(() => []);

        app.innerHTML = `
            <div class="breadcrumb">
                <a href="#/domains">Domains</a> <span>/</span> <strong>${esc(name)}</strong>
            </div>

            <div class="page-header">
                <h1>${esc(name)}</h1>
                <div class="actions">
                    <label class="toggle">
                        <input type="checkbox" id="domain-enabled" ${domain.enabled ? 'checked' : ''}>
                        <span class="slider"></span>
                    </label>
                </div>
            </div>

            <div style="display:grid;grid-template-columns:1fr 1fr;gap:1.5rem">
                <div class="card">
                    <div class="card-header"><h3>Configuration</h3><button class="btn btn-sm btn-primary" id="save-domain">Save</button></div>
                    <div class="form-group"><label>Default Redirect URL</label><input class="form-input" id="d-url" value="${esc(domain.default_url)}"></div>
                    <div class="form-row">
                        <div class="form-group"><label>Status Code</label>
                            <select class="form-select" id="d-code">
                                <option value="301" ${domain.status_code===301?'selected':''}>301 Permanent</option>
                                <option value="302" ${domain.status_code===302?'selected':''}>302 Temporary</option>
                            </select>
                        </div>
                    </div>
                </div>

                <div class="card">
                    <div class="card-header"><h3>Traffic (30 days)</h3></div>
                    <div class="chart-container"><canvas id="traffic-chart"></canvas></div>
                </div>
            </div>

            <div class="card">
                <div class="card-header">
                    <h3>Redirect Rules</h3>
                    <button class="btn btn-sm btn-primary" id="add-rule-btn">+ Add Rule</button>
                </div>
                <div class="table-wrap">
                    <table>
                        <thead><tr><th>Path</th><th>Target URL</th><th>Code</th><th>Priority</th><th>Status</th><th>Actions</th></tr></thead>
                        <tbody id="rules-tbody"></tbody>
                    </table>
                </div>
                <div id="rule-form-container"></div>
            </div>

            <div class="card">
                <div class="card-header"><h3>Top Paths (7 days)</h3></div>
                <div class="chart-container" style="height:200px"><canvas id="paths-chart"></canvas></div>
            </div>`;

        // Save domain handler
        document.getElementById('save-domain').onclick = async () => {
            try {
                await api.put(`/domains/${encodeURIComponent(name)}`, {
                    default_url: document.getElementById('d-url').value,
                    status_code: parseInt(document.getElementById('d-code').value),
                    enabled: document.getElementById('domain-enabled').checked
                });
                toast('Domain updated');
            } catch (e) { toast(e.message, 'error'); }
        };

        document.getElementById('domain-enabled').onchange = async function() {
            try {
                await api.put(`/domains/${encodeURIComponent(name)}`, {
                    default_url: document.getElementById('d-url').value,
                    status_code: parseInt(document.getElementById('d-code').value),
                    enabled: this.checked
                });
                toast(this.checked ? 'Domain enabled' : 'Domain disabled');
            } catch (e) { toast(e.message, 'error'); }
        };

        // Render rules
        function renderRules() {
            const tbody = document.getElementById('rules-tbody');
            const rules = domain.rules || [];
            if (rules.length === 0) {
                tbody.innerHTML = '<tr><td colspan="6" style="text-align:center;color:var(--text-secondary);padding:1.5rem">No path-level rules. All requests redirect to the default URL.</td></tr>';
            } else {
                tbody.innerHTML = rules.map(r => `<tr>
                    <td><code>${esc(r.path)}</code></td>
                    <td style="max-width:250px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap" title="${esc(r.target_url)}">${esc(r.target_url)}</td>
                    <td><span class="badge badge-neutral">${r.status_code}</span></td>
                    <td>${r.priority}</td>
                    <td>${r.enabled ? '<span class="badge badge-success">Active</span>' : '<span class="badge badge-danger">Off</span>'}</td>
                    <td><button class="btn btn-sm btn-outline" onclick="window._deleteRule(${r.id}, '${esc(name)}')">Delete</button></td>
                </tr>`).join('');
            }
        }
        renderRules();

        // Add rule form
        document.getElementById('add-rule-btn').onclick = () => {
            document.getElementById('rule-form-container').innerHTML = `
                <div class="rule-form">
                    <input id="rf-path" placeholder="/path" value="/">
                    <input id="rf-url" placeholder="https://target-url.com">
                    <select id="rf-code"><option value="301">301</option><option value="302">302</option></select>
                    <input id="rf-priority" type="number" value="10" placeholder="Priority">
                    <div>
                        <button class="btn btn-sm btn-success" id="rf-save">Add</button>
                        <button class="btn btn-sm btn-outline" onclick="document.getElementById('rule-form-container').innerHTML=''">Cancel</button>
                    </div>
                </div>`;
            document.getElementById('rf-save').onclick = async () => {
                try {
                    const rule = await api.post(`/domains/${encodeURIComponent(name)}/rules`, {
                        path: document.getElementById('rf-path').value,
                        target_url: document.getElementById('rf-url').value,
                        status_code: parseInt(document.getElementById('rf-code').value),
                        priority: parseInt(document.getElementById('rf-priority').value),
                        enabled: true
                    });
                    domain.rules = domain.rules || [];
                    domain.rules.push(rule);
                    renderRules();
                    document.getElementById('rule-form-container').innerHTML = '';
                    toast('Rule added');
                } catch (e) { toast(e.message, 'error'); }
            };
        };

        // Traffic chart
        const labels = (trafficData || []).map(d => d.bucket ? d.bucket.slice(0,10) : '');
        const data = (trafficData || []).map(d => d.hit_count);
        if (labels.length > 0) {
            charts.traffic = new Chart(document.getElementById('traffic-chart'), {
                type: 'line',
                data: {
                    labels,
                    datasets: [{ label: 'Hits', data, borderColor: '#4a9eff', backgroundColor: 'rgba(59,130,246,0.1)', fill: true, tension: 0.3 }]
                },
                options: { responsive: true, maintainAspectRatio: false, plugins: { legend: { display: false } },
                    scales: { y: { beginAtZero: true } } }
            });
        }

        // Paths chart
        const paths = Array.isArray(pathData) ? pathData : [];
        if (paths.length > 0) {
            charts.paths = new Chart(document.getElementById('paths-chart'), {
                type: 'bar',
                data: {
                    labels: paths.map(p => p.path),
                    datasets: [{ label: 'Hits', data: paths.map(p => p.count), backgroundColor: '#4a9eff' }]
                },
                options: { responsive: true, maintainAspectRatio: false, indexAxis: 'y', plugins: { legend: { display: false } } }
            });
        }
    }

    window._deleteRule = async function(id, domainName) {
        if (!confirm('Delete this rule?')) return;
        try {
            await api.del(`/domains/${encodeURIComponent(domainName)}/rules/${id}`);
            toast('Rule deleted');
            route();
        } catch (e) { toast(e.message, 'error'); }
    };

    // ─── Analytics View ───
    async function renderAnalytics(app) {
        app.innerHTML = `
            <div class="page-header"><h1>Analytics</h1></div>
            <div class="stat-grid" id="stat-cards">
                <div class="stat-card"><div class="label">Loading...</div></div>
            </div>
            <div style="display:grid;grid-template-columns:1fr 1fr;gap:1.5rem">
                <div class="card">
                    <div class="card-header"><h3>Top Domains (7 days)</h3></div>
                    <div class="chart-container"><canvas id="top-domains-chart"></canvas></div>
                </div>
                <div class="card">
                    <div class="card-header"><h3>Top Referers</h3></div>
                    <div id="referers-list"></div>
                </div>
            </div>
            <div style="display:grid;grid-template-columns:1fr 1fr;gap:1.5rem">
                <div class="card">
                    <div class="card-header"><h3>Inactive Domains</h3>
                        <select id="inactive-days" class="form-select" style="width:auto">
                            <option value="7">7 days</option><option value="30" selected>30 days</option>
                            <option value="60">60 days</option><option value="90">90 days</option>
                        </select>
                    </div>
                    <div id="inactive-list"></div>
                </div>
                <div class="card">
                    <div class="card-header"><h3>Trending</h3></div>
                    <div id="trending-list"></div>
                </div>
            </div>
            <div class="card">
                <div class="card-header">
                    <h3>Export Analytics</h3>
                    <div class="actions">
                        <a class="btn btn-sm btn-outline" href="/api/v1/analytics/export?token=${TOKEN}&format=csv&since=${thirtyDaysAgo()}" download>CSV</a>
                        <a class="btn btn-sm btn-outline" href="/api/v1/analytics/export?token=${TOKEN}&since=${thirtyDaysAgo()}" download>JSON</a>
                    </div>
                </div>
                <p style="color:var(--text-secondary);font-size:0.9rem">Download raw request logs for offline analysis.</p>
            </div>`;

        const [domainRes, summary, inactive, referers, trending] = await Promise.all([
            api.get('/domains?limit=1000'),
            api.get('/analytics/summary?since=' + sevenDaysAgo()).catch(() => []),
            api.get('/analytics/inactive?days=30').catch(() => ({count: 0, domains: []})),
            api.get('/analytics/referers?since=' + sevenDaysAgo()).catch(() => []),
            api.get('/analytics/trending?since=' + fourteenDaysAgo()).catch(() => [])
        ]);

        const domains = domainRes.domains || [];
        const hits = Array.isArray(summary) ? summary : [];
        const totalHits = hits.reduce((s, h) => s + h.hit_count, 0);
        const activeCount = new Set(hits.map(h => h.domain)).size;

        document.getElementById('stat-cards').innerHTML = `
            <div class="stat-card"><div class="label">Total Domains</div><div class="value">${domains.length}</div></div>
            <div class="stat-card"><div class="label">Total Hits (7d)</div><div class="value">${totalHits.toLocaleString()}</div></div>
            <div class="stat-card success"><div class="label">Active Domains</div><div class="value">${activeCount}</div><div class="sub">received traffic in 7 days</div></div>
            <div class="stat-card ${(inactive.count||0) > 0 ? 'warning' : ''}"><div class="label">Inactive (30d)</div><div class="value">${inactive.count || 0}</div><div class="sub">cleanup candidates</div></div>`;

        // Top domains chart
        if (hits.length > 0) {
            charts.topDomains = new Chart(document.getElementById('top-domains-chart'), {
                type: 'bar',
                data: {
                    labels: hits.slice(0, 15).map(h => h.domain.replace(/\.com$/, '')),
                    datasets: [{ label: 'Hits', data: hits.slice(0, 15).map(h => h.hit_count), backgroundColor: '#4a9eff' }]
                },
                options: { responsive: true, maintainAspectRatio: false, indexAxis: 'y', plugins: { legend: { display: false } } }
            });
        } else {
            document.getElementById('top-domains-chart').parentElement.innerHTML = '<div class="empty-state"><h3>No traffic data yet</h3><p>Send some redirect traffic to see analytics.</p></div>';
        }

        // Referers
        const refs = Array.isArray(referers) ? referers : [];
        document.getElementById('referers-list').innerHTML = refs.length === 0
            ? '<p style="color:var(--text-secondary);padding:1rem">No referer data</p>'
            : '<table>' + refs.map(r => `<tr><td>${esc(r.referer || '(direct)')}</td><td style="text-align:right"><strong>${r.count}</strong></td></tr>`).join('') + '</table>';

        // Inactive domains
        function renderInactive(data) {
            const list = data.domains || [];
            document.getElementById('inactive-list').innerHTML = list.length === 0
                ? '<p style="color:var(--text-secondary);padding:1rem">All domains received traffic</p>'
                : '<table>' + list.map(d => `<tr><td><a class="domain-link" href="#/domains/${encodeURIComponent(d)}">${esc(d)}</a></td></tr>`).join('') + '</table>';
        }
        renderInactive(inactive);

        document.getElementById('inactive-days').onchange = async function() {
            const data = await api.get(`/analytics/inactive?days=${this.value}`);
            renderInactive(data);
        };

        // Trending
        const trends = Array.isArray(trending) ? trending : [];
        document.getElementById('trending-list').innerHTML = trends.length === 0
            ? '<p style="color:var(--text-secondary);padding:1rem">Not enough data for trends</p>'
            : '<table><thead><tr><th>Domain</th><th style="text-align:right">Trend</th></tr></thead><tbody>' +
              trends.map(t => {
                  const arrow = t.hit_count > 0 ? '<span style="color:var(--success)">&#9650;</span>' : t.hit_count < 0 ? '<span style="color:var(--danger)">&#9660;</span>' : '';
                  return `<tr><td><a class="domain-link" href="#/domains/${encodeURIComponent(t.domain)}">${esc(t.domain)}</a></td><td style="text-align:right">${arrow} ${t.hit_count > 0 ? '+' : ''}${t.hit_count}</td></tr>`;
              }).join('') + '</tbody></table>';
    }

    // ─── Import View ───
    function renderImport(app) {
        app.innerHTML = `
            <div class="page-header"><h1>Bulk Import</h1></div>
            <div class="card">
                <div class="card-header"><h3>Import Domains & Rules</h3></div>
                <p style="color:var(--text-secondary);font-size:0.9rem;margin-bottom:1rem">
                    Paste a JSON array of domains with rules. Existing domains will be skipped.
                </p>
                <div class="form-group">
                    <textarea class="form-textarea" id="import-json" rows="14" placeholder='[
  {
    "domain": "example-bank.com",
    "default_url": "https://www.example.com",
    "status_code": 301,
    "rules": [
      {"path": "/mortgage", "target_url": "https://www.example.com/mortgage", "status_code": 301, "priority": 10}
    ]
  }
]'></textarea>
                </div>
                <div class="actions">
                    <button class="btn btn-primary" id="import-btn">Import</button>
                    <button class="btn btn-outline" id="import-sample">Load Sample</button>
                </div>
                <div id="import-result" style="margin-top:1rem"></div>
            </div>
            <div class="card">
                <div class="card-header"><h3>Export All Rules</h3>
                    <a class="btn btn-sm btn-outline" href="/api/v1/export?token=${TOKEN}" download="redirects.json">Download JSON</a>
                </div>
                <p style="color:var(--text-secondary);font-size:0.9rem">Export all domains and redirect rules as JSON for backup or migration.</p>
            </div>`;

        document.getElementById('import-btn').onclick = async () => {
            try {
                const data = JSON.parse(document.getElementById('import-json').value);
                const result = await api.post('/import', data);
                document.getElementById('import-result').innerHTML = `<div class="badge badge-success">Imported ${result.imported} domains</div>`;
                toast(`Imported ${result.imported} domains`);
            } catch (e) {
                toast(e.message, 'error');
                document.getElementById('import-result').innerHTML = `<div class="badge badge-danger">${esc(e.message)}</div>`;
            }
        };

        document.getElementById('import-sample').onclick = () => {
            document.getElementById('import-json').value = JSON.stringify([
                {
                    "domain": "sample-legacy-bank.com",
                    "default_url": "https://www.example.com",
                    "status_code": 301,
                    "rules": [
                        {"path": "/mortgage", "target_url": "https://www.example.com/en/personal-banking/borrowing/home-lending.html", "status_code": 301, "priority": 10},
                        {"path": "/checking", "target_url": "https://www.example.com/en/personal-banking/banking/checking.html", "status_code": 301, "priority": 10}
                    ]
                }
            ], null, 2);
        };
    }

    // ─── API Docs View ───
    function renderAPIDocs(app) {
        app.innerHTML = `
            <div class="page-header"><h1>API Documentation</h1>
                <div class="actions">
                    <a class="btn btn-sm btn-outline" href="/ui/openapi.json" target="_blank" download>Download OpenAPI Spec</a>
                    <a class="btn btn-sm btn-outline" href="/ui/docs/?token=${TOKEN}" target="_blank">Open in New Tab</a>
                </div>
            </div>
            <iframe id="swagger-frame" src="/ui/docs/?token=${TOKEN}" class="docs-frame"></iframe>`;

        // Size the iframe to fill remaining viewport height
        function sizeFrame() {
            const frame = document.getElementById('swagger-frame');
            if (frame) {
                const top = frame.getBoundingClientRect().top;
                frame.style.height = (window.innerHeight - top - 16) + 'px';
            }
        }
        sizeFrame();
        window.addEventListener('resize', sizeFrame);
    }

    // ─── Modal helper ───
    function showModal(title, bodyHTML, onSave) {
        const overlay = document.createElement('div');
        overlay.className = 'modal-overlay';
        overlay.innerHTML = `<div class="modal">
            <h3>${title}</h3>
            <div>${bodyHTML}</div>
            <div class="modal-actions">
                <button class="btn btn-outline modal-cancel">Cancel</button>
                <button class="btn btn-primary modal-save">Save</button>
            </div>
        </div>`;
        document.body.appendChild(overlay);
        overlay.querySelector('.modal-cancel').onclick = () => overlay.remove();
        overlay.querySelector('.modal-save').onclick = async () => {
            try {
                const result = await onSave();
                if (result !== false) overlay.remove();
            } catch (e) { toast(e.message, 'error'); }
        };
        overlay.onclick = e => { if (e.target === overlay) overlay.remove(); };
    }

    // ─── Helpers ───
    function esc(s) { const d = document.createElement('div'); d.textContent = s || ''; return d.innerHTML; }
    function sevenDaysAgo() { return new Date(Date.now() - 7*86400000).toISOString(); }
    function fourteenDaysAgo() { return new Date(Date.now() - 14*86400000).toISOString(); }
    function thirtyDaysAgo() { return new Date(Date.now() - 30*86400000).toISOString(); }

    // ─── Auth Gate ───
    function init() {
        if (TOKEN) {
            sessionStorage.setItem('redirect_token', TOKEN);
            document.getElementById('auth-gate').style.display = 'none';
            document.getElementById('app-shell').style.display = 'block';
            window.addEventListener('hashchange', route);
            route();
        } else {
            document.getElementById('auth-gate').style.display = 'flex';
            document.getElementById('app-shell').style.display = 'none';
            const submit = () => {
                TOKEN = document.getElementById('token-input').value.trim();
                if (TOKEN) {
                    sessionStorage.setItem('redirect_token', TOKEN);
                    document.getElementById('auth-gate').style.display = 'none';
                    document.getElementById('app-shell').style.display = 'block';
                    window.addEventListener('hashchange', route);
                    route();
                }
            };
            document.getElementById('token-submit').onclick = submit;
            document.getElementById('token-input').addEventListener('keydown', e => { if (e.key === 'Enter') submit(); });
        }
    }

    init();
})();
