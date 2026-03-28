// godevtool Dashboard - Frontend Application

const API = window.location.origin;
let ws = null;
let reconnectTimer = null;

// --- Tab Navigation ---
document.getElementById('nav').addEventListener('click', (e) => {
  const btn = e.target.closest('button');
  if (!btn) return;
  const tab = btn.dataset.tab;

  document.querySelectorAll('.nav button').forEach(b => b.classList.remove('active'));
  btn.classList.add('active');

  document.querySelectorAll('.panel').forEach(p => p.classList.remove('active'));
  document.getElementById('panel-' + tab).classList.add('active');

  // refresh data when switching tabs
  switch (tab) {
    case 'overview': refreshOverview(); break;
    case 'logs': refreshLogs(); break;
    case 'requests': refreshRequests(); break;
    case 'goroutines': refreshGoroutines(); break;
    case 'memory': refreshMemStats(); break;
    case 'timers': refreshTimers(); break;
    case 'queries': refreshQueries(); break;
    case 'timeline': refreshTimeline(); break;
    case 'config': refreshConfig(); break;
  }
});

// --- WebSocket ---
function connectWS() {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  ws = new WebSocket(proto + '//' + window.location.host + '/ws');

  ws.onopen = () => {
    document.getElementById('ws-status').classList.add('connected');
    document.getElementById('ws-label').textContent = 'Connected';
    if (reconnectTimer) { clearTimeout(reconnectTimer); reconnectTimer = null; }
  };

  ws.onclose = () => {
    document.getElementById('ws-status').classList.remove('connected');
    document.getElementById('ws-label').textContent = 'Disconnected';
    reconnectTimer = setTimeout(connectWS, 2000);
  };

  ws.onerror = () => ws.close();

  ws.onmessage = (e) => {
    try {
      const evt = JSON.parse(e.data);
      handleEvent(evt);
    } catch (err) {
      console.error('ws parse error:', err);
    }
  };
}

function handleEvent(evt) {
  switch (evt.type) {
    case 'log':
      prependLogRow(evt.data);
      updateBadge('log-count', 1);
      break;
    case 'request':
      prependRequestRow(evt.data);
      updateBadge('req-count', 1);
      break;
    case 'goroutine':
      document.getElementById('gr-count').textContent = evt.data.Count || 0;
      break;
    case 'memstats':
      renderMemStats(evt.data);
      break;
    case 'timeline':
      prependTimelineEvent(evt.data);
      updateBadge('tl-count', 1);
      break;
    case 'query':
      prependQueryRow(evt.data);
      updateBadge('query-count', 1);
      break;
  }
}

function updateBadge(id, increment) {
  const el = document.getElementById(id);
  el.textContent = parseInt(el.textContent || '0') + increment;
}

// --- API Fetchers ---
async function fetchJSON(path) {
  const res = await fetch(API + path);
  return res.json();
}

async function refreshOverview() {
  const data = await fetchJSON('/api/overview');
  const cards = document.getElementById('overview-cards');
  cards.innerHTML = `
    <div class="card">
      <div class="card-label">Log Entries</div>
      <div class="card-value accent">${data.log_count || 0}</div>
    </div>
    <div class="card">
      <div class="card-label">HTTP Requests</div>
      <div class="card-value green">${data.request_count || 0}</div>
    </div>
    <div class="card">
      <div class="card-label">Goroutines</div>
      <div class="card-value yellow">${data.goroutine_count || 0}</div>
    </div>
    <div class="card">
      <div class="card-label">Heap Alloc</div>
      <div class="card-value cyan">${data.heap_alloc || '—'}</div>
    </div>
    <div class="card">
      <div class="card-label">Active Timers</div>
      <div class="card-value purple">${data.timer_count || 0}</div>
    </div>
    <div class="card">
      <div class="card-label">DB Queries</div>
      <div class="card-value accent">${data.query_count || 0}</div>
    </div>
    <div class="card">
      <div class="card-label">Timeline Events</div>
      <div class="card-value green">${data.timeline_count || 0}</div>
    </div>
    <div class="card">
      <div class="card-label">Config Sections</div>
      <div class="card-value purple">${data.config_count || 0}</div>
    </div>
    <div class="card">
      <div class="card-label">WS Clients</div>
      <div class="card-value accent">${data.ws_clients || 0}</div>
    </div>
  `;
}

async function refreshLogs() {
  const logs = await fetchJSON('/api/logs');
  const tbody = document.getElementById('log-table');
  tbody.innerHTML = '';
  document.getElementById('log-count').textContent = logs.length;
  // show newest first
  for (let i = logs.length - 1; i >= 0; i--) {
    appendLogRow(tbody, logs[i]);
  }
}

async function refreshRequests() {
  const reqs = await fetchJSON('/api/requests');
  const tbody = document.getElementById('req-table');
  tbody.innerHTML = '';
  document.getElementById('req-count').textContent = reqs.length;
  for (let i = reqs.length - 1; i >= 0; i--) {
    appendRequestRow(tbody, reqs[i]);
  }
}

async function refreshGoroutines() {
  const snap = await fetchJSON('/api/goroutines');
  const el = document.getElementById('goroutine-list');
  document.getElementById('gr-count').textContent = snap.Count || 0;

  if (!snap.Goroutines || snap.Goroutines.length === 0) {
    el.innerHTML = '<div class="empty">No goroutine data available. Start the goroutine monitor.</div>';
    return;
  }

  let html = '';
  for (const g of snap.Goroutines) {
    html += `<div class="goroutine-item">
      <span class="id">#${g.ID}</span>
      <span class="func">${escapeHtml(g.Function)}</span>
      <span class="state">[${escapeHtml(g.State)}]</span>
    </div>`;
  }
  el.innerHTML = html;
}

async function refreshMemStats() {
  const snap = await fetchJSON('/api/memstats');
  renderMemStats(snap);
}

function renderMemStats(snap) {
  const grid = document.getElementById('mem-grid');
  if (!snap || !snap.HeapAlloc) {
    grid.innerHTML = '<div class="empty">No memory data available. Start the memstats collector.</div>';
    return;
  }

  grid.innerHTML = `
    ${memCard('Heap Alloc', formatBytes(snap.HeapAlloc))}
    ${memCard('Heap Sys', formatBytes(snap.HeapSys))}
    ${memCard('Heap In-Use', formatBytes(snap.HeapInuse))}
    ${memCard('Heap Idle', formatBytes(snap.HeapIdle))}
    ${memCard('Heap Objects', snap.HeapObjects?.toLocaleString() || '0')}
    ${memCard('Stack In-Use', formatBytes(snap.StackInuse))}
    ${memCard('Total Sys', formatBytes(snap.Sys))}
    ${memCard('Total Alloc', formatBytes(snap.TotalAlloc))}
    ${memCard('Mallocs', snap.Mallocs?.toLocaleString() || '0')}
    ${memCard('Frees', snap.Frees?.toLocaleString() || '0')}
    ${memCard('GC Cycles', snap.NumGC || 0)}
    ${memCard('Goroutines', snap.Goroutines || 0)}
  `;
}

function memCard(label, value) {
  return `<div class="mem-stat">
    <div class="mem-stat-label">${label}</div>
    <div class="mem-stat-value">${value}</div>
  </div>`;
}

async function refreshTimers() {
  const timers = await fetchJSON('/api/timers');
  const tbody = document.getElementById('timer-table');
  tbody.innerHTML = '';

  if (!timers || timers.length === 0) {
    tbody.innerHTML = '<tr><td colspan="7" class="empty">No timer data recorded yet.</td></tr>';
    return;
  }

  for (const t of timers) {
    const tr = document.createElement('tr');
    tr.innerHTML = `
      <td>${escapeHtml(t.Label)}</td>
      <td>${t.Count}</td>
      <td>${formatDuration(t.Total)}</td>
      <td>${formatDuration(t.Avg)}</td>
      <td>${formatDuration(t.Min)}</td>
      <td>${formatDuration(t.Max)}</td>
      <td>${formatDuration(t.Last)}</td>
    `;
    tbody.appendChild(tr);
  }
}

// --- Row Renderers ---
function appendLogRow(tbody, log) {
  const tr = document.createElement('tr');
  const time = formatTime(log.time);
  const level = log.level || 'INFO';
  const fields = renderFields(log.fields);

  tr.innerHTML = `
    <td>${time}</td>
    <td><span class="level-${level}">${level}</span></td>
    <td>${escapeHtml(log.message)}</td>
    <td class="log-fields">${fields}</td>
  `;
  tbody.appendChild(tr);
}

function prependLogRow(log) {
  const tbody = document.getElementById('log-table');
  const tr = document.createElement('tr');
  const time = formatTime(log.time);
  const level = log.level || 'INFO';
  const fields = renderFields(log.fields);

  tr.innerHTML = `
    <td>${time}</td>
    <td><span class="level-${level}">${level}</span></td>
    <td>${escapeHtml(log.message)}</td>
    <td class="log-fields">${fields}</td>
  `;
  tbody.insertBefore(tr, tbody.firstChild);

  // cap at 500 rows
  while (tbody.children.length > 500) {
    tbody.removeChild(tbody.lastChild);
  }
}

function appendRequestRow(tbody, req) {
  const tr = document.createElement('tr');
  const time = formatTime(req.Timestamp);
  const status = req.StatusCode;
  const statusClass = status < 300 ? 'status-2xx' : status < 400 ? 'status-3xx' : status < 500 ? 'status-4xx' : 'status-5xx';
  const dur = formatNanoDuration(req.Duration);
  const durClass = req.Duration < 100000000 ? 'fast' : req.Duration < 500000000 ? 'medium' : 'slow';
  const path = req.Query ? `${req.Path}?${req.Query}` : req.Path;

  tr.innerHTML = `
    <td>${time}</td>
    <td><span class="method method-${req.Method}">${req.Method}</span></td>
    <td>${escapeHtml(path)}</td>
    <td><span class="${statusClass}">${status}</span></td>
    <td><span class="duration ${durClass}">${dur}</span></td>
  `;
  tbody.appendChild(tr);
}

function prependRequestRow(req) {
  const tbody = document.getElementById('req-table');
  const tr = document.createElement('tr');
  const time = formatTime(req.Timestamp);
  const status = req.StatusCode;
  const statusClass = status < 300 ? 'status-2xx' : status < 400 ? 'status-3xx' : status < 500 ? 'status-4xx' : 'status-5xx';
  const dur = formatNanoDuration(req.Duration);
  const durClass = req.Duration < 100000000 ? 'fast' : req.Duration < 500000000 ? 'medium' : 'slow';
  const path = req.Query ? `${req.Path}?${req.Query}` : req.Path;

  tr.innerHTML = `
    <td>${time}</td>
    <td><span class="method method-${req.Method}">${req.Method}</span></td>
    <td>${escapeHtml(path)}</td>
    <td><span class="${statusClass}">${status}</span></td>
    <td><span class="duration ${durClass}">${dur}</span></td>
  `;
  tbody.insertBefore(tr, tbody.firstChild);

  while (tbody.children.length > 500) {
    tbody.removeChild(tbody.lastChild);
  }
}

// --- Queries ---
async function refreshQueries() {
  const queries = await fetchJSON('/api/queries');
  const tbody = document.getElementById('query-table');
  tbody.innerHTML = '';
  document.getElementById('query-count').textContent = queries ? queries.length : 0;

  if (!queries || queries.length === 0) {
    tbody.innerHTML = '<tr><td colspan="6" class="empty">No database queries recorded yet.</td></tr>';
    return;
  }

  for (let i = queries.length - 1; i >= 0; i--) {
    appendQueryRow(tbody, queries[i]);
  }
}

function appendQueryRow(tbody, q) {
  const tr = document.createElement('tr');
  const time = formatTime(q.timestamp);
  const dur = formatDuration(q.duration);
  const durClass = q.duration < 50000000 ? 'fast' : q.duration < 200000000 ? 'medium' : 'slow';
  const opClass = 'method method-' + (q.operation === 'SELECT' ? 'GET' : q.operation === 'INSERT' ? 'POST' : q.operation === 'UPDATE' ? 'PUT' : q.operation === 'DELETE' ? 'DELETE' : 'GET');

  tr.innerHTML = `
    <td>${time}</td>
    <td><span class="${opClass}">${escapeHtml(q.operation)}</span></td>
    <td style="max-width:400px;overflow:hidden;text-overflow:ellipsis" title="${escapeHtml(q.query)}">${escapeHtml(q.query)}</td>
    <td><span class="duration ${durClass}">${dur}</span></td>
    <td>${q.rows >= 0 ? q.rows : '—'}</td>
    <td>${q.error ? '<span class="level-ERROR">' + escapeHtml(q.error) + '</span>' : '—'}</td>
  `;
  tbody.appendChild(tr);
}

function prependQueryRow(q) {
  const tbody = document.getElementById('query-table');
  const tr = document.createElement('tr');
  const time = formatTime(q.timestamp);
  const dur = formatDuration(q.duration);
  const durClass = q.duration < 50000000 ? 'fast' : q.duration < 200000000 ? 'medium' : 'slow';

  tr.innerHTML = `
    <td>${time}</td>
    <td><span class="method method-GET">${escapeHtml(q.operation)}</span></td>
    <td style="max-width:400px;overflow:hidden;text-overflow:ellipsis">${escapeHtml(q.query)}</td>
    <td><span class="duration ${durClass}">${dur}</span></td>
    <td>${q.rows >= 0 ? q.rows : '—'}</td>
    <td>${q.error ? '<span class="level-ERROR">' + escapeHtml(q.error) + '</span>' : '—'}</td>
  `;
  tbody.insertBefore(tr, tbody.firstChild);
  while (tbody.children.length > 500) tbody.removeChild(tbody.lastChild);
}

// --- Timeline ---
async function refreshTimeline() {
  const events = await fetchJSON('/api/timeline');
  const el = document.getElementById('timeline-list');
  document.getElementById('tl-count').textContent = events ? events.length : 0;

  if (!events || events.length === 0) {
    el.innerHTML = '<div class="empty">No timeline events recorded yet.</div>';
    return;
  }

  let html = '';
  for (let i = events.length - 1; i >= 0; i--) {
    html += renderTimelineEvent(events[i]);
  }
  el.innerHTML = html;
}

function renderTimelineEvent(evt) {
  const time = formatTime(evt.timestamp);
  const dur = evt.is_span && evt.duration ? formatDuration(evt.duration) : '';
  const catColor = {http:'green',db:'cyan',custom:'accent',gc:'yellow',goroutine:'yellow',timer:'purple',log:''}[evt.category] || 'accent';
  const dataStr = evt.data ? Object.entries(evt.data).map(([k,v]) => `<span class="key">${escapeHtml(k)}</span>=${escapeHtml(String(v))}`).join(' ') : '';

  return `<div class="goroutine-item">
    <span style="color:var(--text-muted)">${time}</span>
    <span class="method method-GET" style="margin:0 6px">${escapeHtml(evt.category)}</span>
    <span class="func">${escapeHtml(evt.label)}</span>
    ${dur ? `<span class="duration fast" style="margin-left:8px">${dur}</span>` : ''}
    ${dataStr ? `<span class="log-fields" style="margin-left:8px">${dataStr}</span>` : ''}
  </div>`;
}

function prependTimelineEvent(evt) {
  const el = document.getElementById('timeline-list');
  const div = document.createElement('div');
  div.innerHTML = renderTimelineEvent(evt);
  el.insertBefore(div.firstElementChild, el.firstChild);
  while (el.children.length > 500) el.removeChild(el.lastChild);
}

// --- Config ---
async function refreshConfig() {
  const configs = await fetchJSON('/api/config');
  const el = document.getElementById('config-list');

  if (!configs || configs.length === 0) {
    el.innerHTML = '<div class="empty">No configuration registered. Use dt.RegisterConfig() to add configs.</div>';
    return;
  }

  let html = '';
  for (const section of configs) {
    html += `<div class="card" style="margin-bottom:12px;padding:16px">
      <div class="panel-title" style="margin-bottom:8px;color:var(--accent)">${escapeHtml(section.name)}</div>
      <div class="table-wrap"><table><thead>
        <tr><th>Key</th><th>Value</th><th>Type</th><th>Source</th></tr>
      </thead><tbody>`;

    for (const entry of (section.entries || [])) {
      const val = entry.redacted ? '<span class="level-ERROR">********</span>' : escapeHtml(entry.value);
      html += `<tr>
        <td>${escapeHtml(entry.key)}</td>
        <td>${val}</td>
        <td style="color:var(--text-muted)">${escapeHtml(entry.type)}</td>
        <td style="color:var(--text-muted)">${escapeHtml(entry.source || '')}</td>
      </tr>`;
    }

    html += '</tbody></table></div></div>';
  }
  el.innerHTML = html;
}

// --- Helpers ---
function formatTime(isoStr) {
  if (!isoStr) return '—';
  const d = new Date(isoStr);
  return d.toLocaleTimeString('en-US', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit', fractionalSecondDigits: 3 });
}

function formatDuration(nanos) {
  if (!nanos) return '—';
  if (nanos < 1000) return nanos + 'ns';
  if (nanos < 1000000) return (nanos / 1000).toFixed(1) + 'us';
  if (nanos < 1000000000) return (nanos / 1000000).toFixed(2) + 'ms';
  return (nanos / 1000000000).toFixed(3) + 's';
}

function formatNanoDuration(nanos) {
  return formatDuration(nanos);
}

function formatBytes(bytes) {
  if (!bytes) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB'];
  let i = 0;
  let val = bytes;
  while (val >= 1024 && i < units.length - 1) {
    val /= 1024;
    i++;
  }
  return val.toFixed(i === 0 ? 0 : 2) + ' ' + units[i];
}

function renderFields(fields) {
  if (!fields) return '';
  return Object.entries(fields)
    .map(([k, v]) => `<span class="key">${escapeHtml(k)}</span>=${escapeHtml(String(v))}`)
    .join(' ');
}

function escapeHtml(str) {
  if (!str) return '';
  return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

// --- Auto-refresh ---
let refreshInterval = null;

function startAutoRefresh() {
  refreshInterval = setInterval(() => {
    const active = document.querySelector('.nav button.active');
    if (!active) return;
    switch (active.dataset.tab) {
      case 'overview': refreshOverview(); break;
      case 'goroutines': refreshGoroutines(); break;
      case 'memory': refreshMemStats(); break;
    }
  }, 3000);
}

// --- Init ---
refreshOverview();
connectWS();
startAutoRefresh();
