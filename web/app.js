const state = {
  monitors: [],
  currentSnippetMonitorId: null,
  currentDetailMonitorId: null,
  authRequired: false,
  sse: null,
};

const $ = (sel) => document.querySelector(sel);

function toast(msg) {
  const el = $('#toast');
  el.textContent = msg;
  el.classList.remove('hidden');
  clearTimeout(toast._t);
  toast._t = setTimeout(() => el.classList.add('hidden'), 2600);
}

async function api(path, opts = {}) {
  const res = await fetch(path, {
    headers: { 'Content-Type': 'application/json', ...(opts.headers || {}) },
    ...opts,
  });
  if (res.status === 401 && !path.startsWith('/api/v1/auth/')) {
    showLoginGate();
    throw new Error('Authentication required');
  }
  if (res.status === 204) return null;
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
  return data;
}

function escapeHTML(str = '') {
  return String(str).replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
}

function timeAgo(iso) {
  if (!iso) return 'never';
  const sec = Math.max(0, Math.floor((Date.now() - new Date(iso).getTime()) / 1000));
  if (sec < 60) return `${sec}s ago`;
  if (sec < 3600) return `${Math.floor(sec / 60)}m ago`;
  if (sec < 86400) return `${Math.floor(sec / 3600)}h ago`;
  return `${Math.floor(sec / 86400)}d ago`;
}

// --- Load & Render Monitors ---

async function loadMonitors() {
  try {
    state.monitors = await api('/api/v1/monitors');
    renderSummary();
    renderMonitors();
    if (state.currentDetailMonitorId && !$('#detail-modal').classList.contains('hidden')) {
      openDetail(state.currentDetailMonitorId);
    }
  } catch (err) {
    console.error(err);
  }
}

function renderSummary() {
  const total = state.monitors.length;
  const up = state.monitors.filter((m) => m.status === 'up').length;
  const degraded = state.monitors.filter((m) => m.status === 'degraded').length;
  const down = state.monitors.filter((m) => m.status === 'down').length;
  $('#stat-total').textContent = total;
  $('#stat-up').textContent = up;
  $('#stat-degraded').textContent = degraded;
  $('#stat-down').textContent = down;
}

function renderMonitors() {
  const grid = $('#monitors-grid');
  if (!state.monitors.length) {
    grid.innerHTML = `<div class="empty-state">
      <h3>No monitors yet</h3>
      <p style="margin:0.4rem 0 1rem">Add an Active Ping URL or a Push Heartbeat tracker to start monitoring.</p>
      <button class="btn btn-primary" onclick="openMonitorModal()">+ Add First Monitor</button>
    </div>`;
    return;
  }

  grid.innerHTML = state.monitors.map((m) => {
    const bars = [];
    const recent = (m.recent_bars || []).slice().reverse();
    const pad = Math.max(0, 30 - recent.length);
    for (let i = 0; i < pad; i++) bars.push(`<div class="bar" title="No data"></div>`);
    for (const c of recent) {
      const tip = `${c.status.toUpperCase()} • ${new Date(c.checked_at).toLocaleTimeString()}${c.response_time_ms ? ` • ${c.response_time_ms}ms` : ''}${c.error_message ? ` • ${c.error_message}` : ''}`;
      bars.push(`<div class="bar ${c.status}" title="${escapeHTML(tip)}"></div>`);
    }

    const latest = m.latest_check || {};
    let metricsHTML = '';
    if (m.type === 'push') {
      const pills = [];
      if (latest.cpu_pct != null) {
        const warn = m.cpu_threshold > 0 && latest.cpu_pct >= m.cpu_threshold ? 'warn' : '';
        pills.push(`<span class="metric-pill ${warn}">CPU ${latest.cpu_pct.toFixed(1)}%</span>`);
      }
      if (latest.ram_pct != null) {
        const warn = m.ram_threshold > 0 && latest.ram_pct >= m.ram_threshold ? 'warn' : '';
        pills.push(`<span class="metric-pill ${warn}">RAM ${latest.ram_pct.toFixed(1)}%</span>`);
      }
      if (latest.disk_pct != null) {
        const warn = m.disk_threshold > 0 && latest.disk_pct >= m.disk_threshold ? 'warn' : '';
        pills.push(`<span class="metric-pill ${warn}">Disk ${latest.disk_pct.toFixed(1)}%</span>`);
      }
      if (latest.load_avg != null) pills.push(`<span class="metric-pill">Load ${latest.load_avg.toFixed(2)}</span>`);
      if (latest.app_version) pills.push(`<span class="metric-pill">v${escapeHTML(latest.app_version)}</span>`);
      if (pills.length) metricsHTML = `<div class="metrics-row">${pills.join('')}</div>`;
    } else if (m.avg_latency_ms > 0) {
      metricsHTML = `<div class="metrics-row">
        <span class="metric-pill">Avg ${Math.round(m.avg_latency_ms)} ms</span>
        ${latest.status_code ? `<span class="metric-pill">HTTP ${latest.status_code}</span>` : ''}
      </div>`;
    }

    const sub = m.type === 'ping'
      ? `${m.method} ${m.url}`
      : `Heartbeat every ${m.interval_seconds}s (+${m.grace_seconds}s grace)`;

    return `<article class="monitor-card ${m.is_active ? '' : 'paused'}">
      <div class="card-top">
        <div>
          <div class="card-title">
            <span>${escapeHTML(m.name)}</span>
            <span class="badge badge-type">${m.type}</span>
          </div>
          <div class="card-sub">${escapeHTML(sub)}</div>
        </div>
        <span class="badge badge-${m.status}">${m.is_active ? m.status : 'paused'}</span>
      </div>

      <div class="bars">${bars.join('')}</div>
      ${metricsHTML}
      ${latest.error_message ? `<div class="small" style="color:var(--degraded)">${escapeHTML(latest.error_message)}</div>` : ''}

      <div class="card-meta">
        <span>24h Uptime: <strong>${m.uptime_24h.toFixed(1)}%</strong></span>
        <span>Last: ${timeAgo(m.last_checked_at)}</span>
      </div>

      <div class="card-actions">
        <button class="btn btn-sm btn-secondary" onclick="openDetail('${m.id}')">Graphs & Logs</button>
        ${m.type === 'push'
          ? `<button class="btn btn-sm btn-primary" onclick="openSnippet('${m.id}')">Tracker Script</button>`
          : `<button class="btn btn-sm btn-secondary" onclick="checkNow('${m.id}')">Ping Now</button>`}
        <button class="btn btn-sm btn-secondary" onclick="openMonitorModal('${m.id}')">Edit</button>
        <button class="btn btn-sm btn-danger" onclick="deleteMonitor('${m.id}', '${escapeHTML(m.name)}')">Delete</button>
      </div>
    </article>`;
  }).join('');
}

// --- Add / Edit Monitor ---

function setMonitorType(type) {
  $('#m-type').value = type;
  document.querySelectorAll('.seg-btn').forEach((b) => b.classList.toggle('active', b.dataset.type === type));
  $('#ping-fields').classList.toggle('hidden', type !== 'ping');
  $('#push-grace-field').classList.toggle('hidden', type !== 'push');
  $('#push-thresholds').classList.toggle('hidden', type !== 'push');
}

window.openMonitorModal = (id = null) => {
  $('#monitor-form').reset();
  $('#m-id').value = '';
  const m = id ? state.monitors.find((x) => x.id === id) : null;
  if (m) {
    $('#monitor-modal-title').textContent = 'Edit Monitor';
    $('#m-id').value = m.id;
    setMonitorType(m.type);
    document.querySelectorAll('.seg-btn').forEach((b) => (b.disabled = true));
    $('#m-name').value = m.name;
    $('#m-url').value = m.url || '';
    $('#m-method').value = m.method || 'GET';
    $('#m-keyword').value = m.keyword || '';
    $('#m-interval').value = m.interval_seconds;
    $('#m-timeout').value = m.timeout_seconds;
    $('#m-grace').value = m.grace_seconds;
    $('#m-cpu').value = m.cpu_threshold || 0;
    $('#m-ram').value = m.ram_threshold || 0;
    $('#m-disk').value = m.disk_threshold || 0;
  } else {
    $('#monitor-modal-title').textContent = 'New Monitor';
    document.querySelectorAll('.seg-btn').forEach((b) => (b.disabled = false));
    setMonitorType('ping');
  }
  $('#monitor-modal').classList.remove('hidden');
};

$('#monitor-form').addEventListener('submit', async (e) => {
  e.preventDefault();
  const id = $('#m-id').value;
  const type = $('#m-type').value;
  const payload = {
    name: $('#m-name').value.trim(),
    type,
    url: $('#m-url').value.trim(),
    method: $('#m-method').value,
    keyword: $('#m-keyword').value.trim(),
    interval_seconds: parseInt($('#m-interval').value, 10) || 60,
    timeout_seconds: parseInt($('#m-timeout').value, 10) || 10,
    grace_seconds: parseInt($('#m-grace').value, 10) || 30,
    cpu_threshold: parseFloat($('#m-cpu').value) || 0,
    ram_threshold: parseFloat($('#m-ram').value) || 0,
    disk_threshold: parseFloat($('#m-disk').value) || 0,
    is_active: true,
  };

  try {
    const saved = id
      ? await api(`/api/v1/monitors/${id}`, { method: 'PUT', body: JSON.stringify(payload) })
      : await api('/api/v1/monitors', { method: 'POST', body: JSON.stringify(payload) });
    $('#monitor-modal').classList.add('hidden');
    await loadMonitors();
    toast(id ? 'Monitor updated' : 'Monitor created');
    if (!id && saved.type === 'push') {
      openSnippet(saved.id);
    }
  } catch (err) {
    toast(err.message);
  }
});

window.deleteMonitor = async (id, name) => {
  if (!confirm(`Delete monitor "${name}"?`)) return;
  try {
    await api(`/api/v1/monitors/${id}`, { method: 'DELETE' });
    await loadMonitors();
    toast('Monitor deleted');
  } catch (err) {
    toast(err.message);
  }
};

window.checkNow = async (id) => {
  try {
    await api(`/api/v1/monitors/${id}/check`, { method: 'POST' });
    toast('Check completed');
  } catch (err) {
    toast(err.message);
  }
};

// --- Details & SVG Telemetry Charts ---

function buildSVGChart(seriesList, maxY = null, unit = '', thresholds = []) {
  const W = 720, H = 170, padL = 36, padR = 12, padT = 12, padB = 22;
  const allVals = seriesList.flatMap((s) => s.points.map((p) => p.y)).filter((v) => v != null);
  if (!allVals.length) return `<div class="muted small" style="padding:1rem;text-align:center">Waiting for data points…</div>`;

  const yMax = maxY != null ? maxY : Math.max(10, Math.ceil(Math.max(...allVals) * 1.15));
  const n = Math.max(2, seriesList[0].points.length);
  const xScale = (i) => padL + (i / (n - 1)) * (W - padL - padR);
  const yScale = (v) => padT + (1 - Math.min(v, yMax) / yMax) * (H - padT - padB);

  let grid = '';
  for (let i = 0; i <= 4; i++) {
    const val = Math.round((yMax / 4) * i);
    const y = yScale(val);
    grid += `<line x1="${padL}" y1="${y}" x2="${W - padR}" y2="${y}" stroke="#1f2a40" stroke-width="1"/>`;
    grid += `<text x="${padL - 6}" y="${y + 4}" fill="#64748b" font-size="10" text-anchor="end">${val}${unit}</text>`;
  }

  let threshLines = '';
  for (const t of thresholds) {
    if (t.value > 0 && t.value <= yMax) {
      const y = yScale(t.value);
      threshLines += `<line x1="${padL}" y1="${y}" x2="${W - padR}" y2="${y}" stroke="${t.color}" stroke-dasharray="4 4" stroke-width="1" opacity="0.7"/>`;
    }
  }

  let paths = '';
  for (const s of seriesList) {
    let d = '';
    s.points.forEach((p, idx) => {
      if (p.y == null) return;
      d += (d ? ' L ' : 'M ') + `${xScale(idx).toFixed(1)} ${yScale(p.y).toFixed(1)}`;
    });
    if (d) {
      paths += `<path d="${d}" fill="none" stroke="${s.color}" stroke-width="2" stroke-linejoin="round"/>`;
    }
  }

  return `<svg viewBox="0 0 ${W} ${H}" style="width:100%;height:auto;display:block">${grid}${threshLines}${paths}</svg>`;
}

window.openDetail = async (id) => {
  state.currentDetailMonitorId = id;
  try {
    const data = await api(`/api/v1/monitors/${id}/history?limit=80`);
    const m = data.monitor;
    const checks = (data.checks || []).slice().reverse();

    $('#detail-title').textContent = m.name;
    $('#detail-subtitle').textContent = m.type === 'ping'
      ? `${m.method} ${m.url} • Uptime ${data.uptime_pct.toFixed(2)}% • Avg ${Math.round(data.avg_latency_ms)}ms`
      : `Push Monitor • Uptime ${data.uptime_pct.toFixed(2)}% • ${data.total_checks} heartbeats`;

    let chartHTML = '';
    if (m.type === 'push') {
      const svg = buildSVGChart(
        [
          { name: 'CPU %', color: '#3b82f6', points: checks.map((c) => ({ y: c.cpu_pct })) },
          { name: 'RAM %', color: '#10b981', points: checks.map((c) => ({ y: c.ram_pct })) },
          { name: 'Disk %', color: '#f59e0b', points: checks.map((c) => ({ y: c.disk_pct })) },
        ],
        100,
        '%',
        [
          { value: m.cpu_threshold, color: '#3b82f6' },
          { value: m.ram_threshold, color: '#10b981' },
          { value: m.disk_threshold, color: '#f59e0b' },
        ]
      );
      chartHTML = `<div class="chart-box">
        <div class="chart-legend">
          <span class="legend-item"><span class="legend-swatch" style="background:#3b82f6"></span>CPU %</span>
          <span class="legend-item"><span class="legend-swatch" style="background:#10b981"></span>RAM %</span>
          <span class="legend-item"><span class="legend-swatch" style="background:#f59e0b"></span>Disk %</span>
        </div>
        ${svg}
      </div>`;
    } else {
      const svg = buildSVGChart(
        [{ name: 'Latency', color: '#3b82f6', points: checks.map((c) => ({ y: c.response_time_ms })) }],
        null,
        'ms'
      );
      chartHTML = `<div class="chart-box">
        <div class="chart-legend"><span class="legend-item"><span class="legend-swatch" style="background:#3b82f6"></span>Response Latency (ms)</span></div>
        ${svg}
      </div>`;
    }

    const rows = (data.checks || []).slice(0, 25).map((c) => {
      const detail = m.type === 'push'
        ? [
            c.cpu_pct != null ? `CPU ${c.cpu_pct.toFixed(1)}%` : '',
            c.ram_pct != null ? `RAM ${c.ram_pct.toFixed(1)}%` : '',
            c.disk_pct != null ? `Disk ${c.disk_pct.toFixed(1)}%` : '',
            c.app_version ? `v${c.app_version}` : '',
          ].filter(Boolean).join(' • ') || 'heartbeat'
        : `HTTP ${c.status_code || '-'} • ${c.response_time_ms}ms`;

      return `<tr>
        <td>${new Date(c.checked_at).toLocaleString()}</td>
        <td><span class="badge badge-${c.status}">${c.status}</span></td>
        <td>${escapeHTML(detail)}</td>
        <td class="muted">${escapeHTML(c.error_message || '')}</td>
      </tr>`;
    }).join('');

    $('#detail-body').innerHTML = `
      ${chartHTML}
      <div class="section-subtitle">Recent Checks</div>
      <div style="overflow-x:auto">
        <table class="history-table">
          <thead><tr><th>Time</th><th>Status</th><th>Metrics</th><th>Notes</th></tr></thead>
          <tbody>${rows || '<tr><td colspan="4" class="muted">No checks recorded yet.</td></tr>'}</tbody>
        </table>
      </div>
    `;
    $('#detail-modal').classList.remove('hidden');
  } catch (err) {
    toast(err.message);
  }
};

// --- Tracker Snippet Generator ---

async function loadSnippet(lang) {
  const id = state.currentSnippetMonitorId;
  if (!id) return;
  document.querySelectorAll('#snippet-tabs .tab').forEach((t) => t.classList.toggle('active', t.dataset.lang === lang));
  try {
    const data = await api(`/api/v1/monitors/${id}/snippet?lang=${lang}`);
    $('#snippet-endpoint').textContent = data.endpoint;
    $('#snippet-code').textContent = data.code;
  } catch (err) {
    toast(err.message);
  }
}

window.openSnippet = async (id) => {
  state.currentSnippetMonitorId = id;
  const m = state.monitors.find((x) => x.id === id);
  $('#snippet-title').textContent = `Push Tracker Script — ${m ? m.name : ''}`;
  $('#snippet-modal').classList.remove('hidden');
  await loadSnippet('bash');
};

$('#snippet-tabs').addEventListener('click', (e) => {
  if (e.target.matches('.tab')) loadSnippet(e.target.dataset.lang);
});

$('#btn-copy-code').addEventListener('click', () => {
  navigator.clipboard.writeText($('#snippet-code').textContent);
  toast('Tracker script copied!');
});

$('#btn-copy-endpoint').addEventListener('click', () => {
  navigator.clipboard.writeText($('#snippet-endpoint').textContent);
  toast('Endpoint URL copied!');
});

// --- Smart Alerts ---

async function loadAlerts() {
  const [channels, logs] = await Promise.all([api('/api/v1/alerts'), api('/api/v1/alerts/logs')]);

  const list = $('#alert-channels-list');
  if (!channels.length) {
    list.innerHTML = `<div class="muted small">No alert channels configured yet.</div>`;
  } else {
    list.innerHTML = channels.map((ch) => `
      <div class="alert-row">
        <div>
          <strong>${escapeHTML(ch.name)}</strong>
          <span class="badge badge-type" style="margin-left:0.4rem">${ch.type}</span>
          <div class="muted small">Cooldown: ${ch.cooldown_seconds}s • ${ch.enabled ? 'Enabled' : 'Disabled'}</div>
        </div>
        <div style="display:flex;gap:0.4rem">
          <button class="btn btn-sm btn-secondary" onclick="testAlert('${ch.id}')">Send Test</button>
          <button class="btn btn-sm btn-danger" onclick="deleteAlert('${ch.id}')">Delete</button>
        </div>
      </div>
    `).join('');
  }

  const logsEl = $('#alert-logs-list');
  if (!logs.length) {
    logsEl.innerHTML = `<div class="muted small">No alerts sent yet.</div>`;
  } else {
    logsEl.innerHTML = logs.slice(0, 15).map((l) => `
      <div class="alert-row">
        <div>
          <span class="badge badge-${l.event_type === 'down' ? 'down' : l.event_type === 'threshold' ? 'degraded' : 'up'}">${l.event_type}</span>
          <strong style="margin-left:0.4rem">${escapeHTML(l.monitor_name)}</strong>
          <span class="muted small">→ ${escapeHTML(l.channel_name)}</span>
          <div class="small muted">${escapeHTML(l.message)} ${l.error ? `(${escapeHTML(l.error)})` : ''}</div>
        </div>
        <span class="small muted">${timeAgo(l.sent_at)}</span>
      </div>
    `).join('');
  }
}

$('#a-type').addEventListener('change', (e) => {
  const isTg = e.target.value === 'telegram';
  $('#a-chatid-wrap').classList.toggle('hidden', !isTg);
  $('#a-target-label').textContent = isTg ? 'Telegram Bot Token' : 'Webhook URL';
  $('#a-target').placeholder = isTg ? '123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11' : 'https://...';
});

$('#alert-form').addEventListener('submit', async (e) => {
  e.preventDefault();
  const type = $('#a-type').value;
  const payload = {
    name: $('#a-name').value.trim(),
    type,
    target_url: $('#a-target').value.trim(),
    cooldown_seconds: parseInt($('#a-cooldown').value, 10) || 300,
    config_json: type === 'telegram' ? JSON.stringify({ chat_id: $('#a-chatid').value.trim() }) : '',
  };
  try {
    await api('/api/v1/alerts', { method: 'POST', body: JSON.stringify(payload) });
    $('#alert-form').reset();
    await loadAlerts();
    toast('Alert channel added');
  } catch (err) {
    toast(err.message);
  }
});

window.testAlert = async (id) => {
  try {
    await api(`/api/v1/alerts/${id}/test`, { method: 'POST' });
    toast('Test alert sent!');
  } catch (err) {
    toast(err.message);
  }
};

window.deleteAlert = async (id) => {
  try {
    await api(`/api/v1/alerts/${id}`, { method: 'DELETE' });
    await loadAlerts();
    toast('Alert channel deleted');
  } catch (err) {
    toast(err.message);
  }
};

// --- SSE Live Updates ---

function connectSSE() {
  if (state.sse) state.sse.close();
  const badge = $('#sse-badge');
  const es = new EventSource('/api/v1/events');
  state.sse = es;
  es.onopen = () => {
    badge.textContent = 'live';
    badge.classList.add('live');
  };
  es.onmessage = () => {
    loadMonitors();
  };
  es.onerror = () => {
    badge.textContent = 'reconnecting…';
    badge.classList.remove('live');
  };
}

// --- Auth ---

function showLoginGate() {
  if (state.sse) {
    state.sse.close();
    state.sse = null;
  }
  $('#login-gate').classList.remove('hidden');
  $('#login-password').focus();
}

function hideLoginGate() {
  $('#login-gate').classList.add('hidden');
  $('#login-error').classList.add('hidden');
  $('#login-password').value = '';
}

$('#login-form').addEventListener('submit', async (e) => {
  e.preventDefault();
  const errEl = $('#login-error');
  errEl.classList.add('hidden');
  try {
    await api('/api/v1/auth/login', {
      method: 'POST',
      body: JSON.stringify({ password: $('#login-password').value }),
    });
    hideLoginGate();
    $('#btn-logout').classList.remove('hidden');
    await loadMonitors();
    connectSSE();
  } catch (err) {
    errEl.textContent = err.message;
    errEl.classList.remove('hidden');
  }
});

$('#btn-logout').addEventListener('click', async () => {
  await api('/api/v1/auth/logout', { method: 'POST' }).catch(() => {});
  showLoginGate();
});

// --- Wiring ---

document.querySelectorAll('.seg-btn').forEach((btn) => {
  btn.addEventListener('click', () => setMonitorType(btn.dataset.type));
});

document.querySelectorAll('[data-close]').forEach((el) => {
  el.addEventListener('click', () => {
    $(`#${el.dataset.close}`).classList.add('hidden');
  });
});

$('#btn-new-monitor').addEventListener('click', () => openMonitorModal());
$('#btn-alerts').addEventListener('click', async () => {
  $('#alerts-modal').classList.remove('hidden');
  await loadAlerts();
});

async function init() {
  try {
    const auth = await api('/api/v1/auth/status');
    state.authRequired = !!auth.auth_required;
    if (state.authRequired) {
      $('#btn-logout').classList.remove('hidden');
      if (!auth.authenticated) {
        showLoginGate();
        return;
      }
    }
  } catch (err) {
    console.error(err);
  }
  await loadMonitors();
  connectSSE();
}

init();
