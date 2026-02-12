(() => {
  const els = {};
  const detailState = {
    metricsRange: '1h',
    eventsRange: '1h',
    pollTimer: null,
    pollInFlight: false,
  };

  function bindDetail() {
    els.detail = document.getElementById('monitor-detail');
    els.empty = document.getElementById('monitor-empty');
    els.title = document.getElementById('monitor-detail-title');
    els.target = document.getElementById('monitor-detail-target');
    els.maintenance = document.getElementById('monitor-maintenance-info');
    els.dot = document.getElementById('monitor-status-dot');
    els.strip = document.getElementById('monitor-status-strip');
    els.stats = document.getElementById('monitor-stats');
    els.chart = document.getElementById('monitor-latency-chart');
    els.latencyRange = document.getElementById('monitor-latency-range');
    els.events = document.getElementById('monitor-events-list');
    els.pause = document.getElementById('monitor-pause-toggle');
    els.edit = document.getElementById('monitor-edit');
    els.clone = document.getElementById('monitor-clone');
    els.remove = document.getElementById('monitor-delete');
    els.checkNow = document.getElementById('monitor-check-now');
    els.eventsRange = document.getElementById('monitor-events-range');
    els.clearStats = document.getElementById('monitor-events-clear');

    if (els.latencyRange) {
      els.latencyRange.value = detailState.metricsRange;
      els.latencyRange.addEventListener('change', () => {
        detailState.metricsRange = els.latencyRange.value;
        const id = MonitoringPage.state.selectedId;
        if (id) loadDetail(id);
      });
    }
    if (els.eventsRange) {
      els.eventsRange.value = detailState.eventsRange;
      els.eventsRange.addEventListener('change', () => {
        detailState.eventsRange = els.eventsRange.value;
        const id = MonitoringPage.state.selectedId;
        if (id) loadDetail(id);
      });
      if (!MonitoringPage.hasPermission('monitoring.events.view')) {
        els.eventsRange.disabled = true;
      }
    }
    if (els.clearStats) {
      const canManage = MonitoringPage.hasPermission('monitoring.manage');
      els.clearStats.disabled = !canManage;
      els.clearStats.addEventListener('change', async () => {
        const action = els.clearStats.value;
        if (!action) return;
        const mon = MonitoringPage.selectedMonitor();
        if (!mon) return;
        const labelKey = action === 'events'
          ? 'monitoring.events.clearConfirmEvents'
          : 'monitoring.events.clearConfirmMetrics';
        const confirmed = window.confirm(MonitoringPage.t(labelKey));
        if (!confirmed) {
          els.clearStats.value = '';
          return;
        }
        try {
          if (action === 'events') {
            await Api.del(`/api/monitoring/monitors/${mon.id}/events`);
          } else {
            await Api.del(`/api/monitoring/monitors/${mon.id}/metrics`);
          }
          await loadDetail(mon.id);
          MonitoringPage.refreshEventsCenter?.();
        } catch (err) {
          console.error('clear stats', err);
        } finally {
          els.clearStats.value = '';
        }
      });
    }
    if (els.pause) els.pause.addEventListener('click', handlePause);
    if (els.edit) els.edit.addEventListener('click', () => MonitoringPage.openMonitorModal?.(MonitoringPage.selectedMonitor()));
    if (els.clone) els.clone.addEventListener('click', handleClone);
    if (els.remove) els.remove.addEventListener('click', handleDelete);
    if (els.checkNow) els.checkNow.addEventListener('click', handleCheckNow);
    document.addEventListener('visibilitychange', () => {
      if (document.hidden) return;
      const id = MonitoringPage.state.selectedId;
      if (id) loadDetail(id);
    });
  }

  async function loadDetail(id) {
    if (!id) return;
    try {
      const canEvents = MonitoringPage.hasPermission('monitoring.events.view');
      const canMaintenance = MonitoringPage.hasPermission('monitoring.maintenance.view');
      const requests = [
        Api.get(`/api/monitoring/monitors/${id}`),
        Api.get(`/api/monitoring/monitors/${id}/state`),
        Api.get(`/api/monitoring/monitors/${id}/metrics?range=${detailState.metricsRange}`),
        canEvents
          ? Api.get(`/api/monitoring/monitors/${id}/events?range=${detailState.eventsRange}`)
          : Promise.resolve({ items: [] }),
        canMaintenance
          ? Api.get(`/api/monitoring/maintenance?active=true&monitor_id=${id}`)
          : Promise.resolve({ items: [] }),
      ];
      const [mon, state, metrics, events, maintenance] = await Promise.all(requests);
      const current = MonitoringPage.state.monitors.find(m => m.id === id);
      if (current) Object.assign(current, mon);
      renderDetail(mon, state, metrics.items || [], events.items || [], maintenance.items || []);
    } catch (err) {
      console.error('monitor detail', err);
      const fallback = MonitoringPage.state.monitors.find(m => m.id === id);
      if (fallback) scheduleDetailRefresh(fallback);
    }
  }

  function renderDetail(mon, state, metrics, events, maintenance) {
    if (!els.detail || !els.empty) return;
    if (!mon) {
      clearDetail();
      return;
    }
    els.empty.hidden = true;
    els.detail.hidden = false;
    if (els.title) els.title.textContent = mon.name || `#${mon.id}`;
    if (els.target) els.target.textContent = mon.type === 'tcp'
      ? `${mon.host}:${mon.port}`
      : (mon.url || mon.host || '-');
    if (els.dot) els.dot.className = `status-dot ${statusClass(state?.status || mon.status)}`;
    renderMaintenanceInfo(mon, state, maintenance);
    renderStatusStrip(metrics);
    renderStats(mon, state);
    renderLatencyChart(metrics);
    renderEvents(events);
    updateActionLabels(mon);
    toggleActionAccess();
    scheduleDetailRefresh(mon);
  }

  function clearDetail() {
    stopDetailRefresh();
    if (els.detail) els.detail.hidden = true;
    if (els.empty) els.empty.hidden = (MonitoringPage.state.monitors || []).length === 0;
  }

  function renderStatusStrip(metrics) {
    if (!els.strip) return;
    els.strip.innerHTML = '';
    const slice = metrics.slice(-50);
    slice.forEach(m => {
      const bar = document.createElement('span');
      bar.className = m.ok ? 'up' : 'down';
      els.strip.appendChild(bar);
    });
    if (!slice.length) {
      const bar = document.createElement('span');
      bar.className = 'paused';
      els.strip.appendChild(bar);
    }
  }

  function renderStats(mon, state) {
    if (!els.stats) return;
    const lastCode = state?.last_status_code ? `${state.last_status_code}` : '-';
    const lastErr = state?.last_error ? MonitoringPage.sanitizeErrorMessage(state.last_error) : '';
    els.stats.innerHTML = '';
    els.stats.appendChild(statCard(MonitoringPage.t('monitoring.stats.current'), lastErr ? lastErr : lastCode));
    els.stats.appendChild(statCard(MonitoringPage.t('monitoring.stats.avg24h'), MonitoringPage.formatLatency(state?.avg_latency_24h)));
    els.stats.appendChild(statCard(MonitoringPage.t('monitoring.stats.uptime24h'), MonitoringPage.formatUptime(state?.uptime_24h)));
    els.stats.appendChild(statCard(MonitoringPage.t('monitoring.stats.uptime30d'), MonitoringPage.formatUptime(state?.uptime_30d)));
    if (mon.sla_target_pct) {
      const slaOk = (state?.uptime_30d || 0) >= mon.sla_target_pct;
      const label = MonitoringPage.t('monitoring.stats.sla');
      const value = slaOk
        ? MonitoringPage.t('monitoring.sla.ok')
        : MonitoringPage.t('monitoring.sla.violated');
      els.stats.appendChild(statCard(label, value));
    }
  }

  function renderLatencyChart(metrics) {
    if (!els.chart) return;
    els.chart.innerHTML = '';
    if (!metrics.length) {
      els.chart.textContent = MonitoringPage.t('monitoring.noMetrics');
      return;
    }
    const values = metrics.map(m => Math.max(0, m.latency_ms || 0));
    const maxVal = Math.max(...values, 0);
    const step = niceStep(Math.max(1, maxVal / 5));
    const maxTick = Math.max(step, Math.ceil(maxVal / step) * step);
    const width = 980;
    const height = 260;
    const pad = { left: 54, right: 14, top: 14, bottom: 28 };
    const chartWidth = width - pad.left - pad.right;
    const chartHeight = height - pad.top - pad.bottom;
    const scaleX = (idx) => pad.left + (idx / Math.max(values.length - 1, 1)) * chartWidth;
    const scaleY = (val) => pad.top + (1 - (val / maxTick)) * chartHeight;
    const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    svg.setAttribute('viewBox', `0 0 ${width} ${height}`);
    svg.setAttribute('preserveAspectRatio', 'xMidYMid meet');

    const ticks = [];
    for (let v = 0; v <= maxTick; v += step) ticks.push(v);
    ticks.forEach(val => {
      const y = scaleY(val);
      const line = document.createElementNS('http://www.w3.org/2000/svg', 'line');
      line.setAttribute('x1', pad.left);
      line.setAttribute('x2', width - pad.right);
      line.setAttribute('y1', y);
      line.setAttribute('y2', y);
      line.setAttribute('stroke', 'rgba(255, 255, 255, 0.08)');
      line.setAttribute('stroke-width', '1');
      svg.appendChild(line);
      const label = document.createElementNS('http://www.w3.org/2000/svg', 'text');
      label.setAttribute('x', `${pad.left - 8}`);
      label.setAttribute('y', `${y + 4}`);
      label.setAttribute('text-anchor', 'end');
      label.setAttribute('font-size', '12');
      label.setAttribute('fill', 'rgba(255, 255, 255, 0.62)');
      label.textContent = `${val}`;
      svg.appendChild(label);
    });

    const points = metrics.map((m, idx) => ({
      x: scaleX(idx),
      y: scaleY(Math.max(0, m.latency_ms || 0)),
      ok: !!m.ok,
      ts: m.timestamp || m.ts,
      latency: m.latency_ms || 0,
    }));

    const segments = [];
    let current = null;
    points.forEach((pt, idx) => {
      if (!current || current.ok !== pt.ok) {
        if (current) segments.push(current);
        current = { ok: pt.ok, points: [] };
        if (idx > 0) {
          current.points.push(points[idx - 1]);
        }
      }
      current.points.push(pt);
    });
    if (current) segments.push(current);

    segments.forEach(seg => {
      const poly = document.createElementNS('http://www.w3.org/2000/svg', 'polyline');
      poly.setAttribute('fill', 'none');
      poly.setAttribute('stroke', seg.ok ? '#2dd27b' : '#ff6b6b');
      poly.setAttribute('stroke-width', '3');
      poly.setAttribute('stroke-linecap', 'round');
      poly.setAttribute('stroke-linejoin', 'round');
      poly.setAttribute('points', seg.points.map(p => `${p.x.toFixed(2)},${p.y.toFixed(2)}`).join(' '));
      svg.appendChild(poly);
    });

    points.forEach((pt, idx) => {
      const circle = document.createElementNS('http://www.w3.org/2000/svg', 'circle');
      circle.setAttribute('cx', pt.x.toFixed(2));
      circle.setAttribute('cy', pt.y.toFixed(2));
      circle.setAttribute('r', (idx === 0 || idx === points.length - 1) ? '4' : '2.5');
      circle.setAttribute('fill', pt.ok ? '#2dd27b' : '#ff6b6b');
      const title = document.createElementNS('http://www.w3.org/2000/svg', 'title');
      title.textContent = `${MonitoringPage.formatDate(pt.ts)} - ${MonitoringPage.formatLatency(pt.latency)}`;
      circle.appendChild(title);
      svg.appendChild(circle);
    });

    els.chart.appendChild(svg);
  }

  function niceStep(raw) {
    const value = Math.max(1, Number(raw) || 1);
    const pow = Math.pow(10, Math.floor(Math.log10(value)));
    const base = value / pow;
    if (base <= 1) return 1 * pow;
    if (base <= 2) return 2 * pow;
    if (base <= 5) return 5 * pow;
    return 10 * pow;
  }

  function stopDetailRefresh() {
    if (detailState.pollTimer) {
      window.clearTimeout(detailState.pollTimer);
      detailState.pollTimer = null;
    }
  }

  function scheduleDetailRefresh(mon) {
    stopDetailRefresh();
    if (!mon || !mon.id || mon.is_paused || !mon.is_active) return;
    const intervalSec = Number(mon.interval_sec) || 30;
    const waitMs = Math.min(Math.max(intervalSec * 1000, 3000), 60000);
    detailState.pollTimer = window.setTimeout(async () => {
      if (document.hidden) {
        scheduleDetailRefresh(mon);
        return;
      }
      if (detailState.pollInFlight) {
        scheduleDetailRefresh(mon);
        return;
      }
      detailState.pollInFlight = true;
      try {
        await loadDetail(mon.id);
      } finally {
        detailState.pollInFlight = false;
      }
    }, waitMs);
  }

  function renderEvents(events) {
    if (!els.events) return;
    els.events.innerHTML = '';
    if (!events.length) {
      const empty = document.createElement('div');
      empty.className = 'muted';
      empty.textContent = MonitoringPage.t('monitoring.noEvents');
      els.events.appendChild(empty);
      return;
    }
    events.forEach(ev => {
      const row = document.createElement('div');
      row.className = `monitoring-event ${statusClass(ev.event_type)}`;
      row.innerHTML = `
        <div>
          <div>${statusLabel(ev.event_type)}</div>
          <div class="event-meta">${MonitoringPage.sanitizeErrorMessage(ev.message || '')}</div>
        </div>
        <div class="event-meta">${MonitoringPage.formatDate(ev.ts)}</div>
      `;
      els.events.appendChild(row);
    });
  }

  function renderMaintenanceInfo(mon, state, items) {
    if (!els.maintenance) return;
    const active = state?.maintenance_active || (state?.status || '').toLowerCase() === 'maintenance';
    if (!active || !items || !items.length) {
      els.maintenance.hidden = true;
      return;
    }
    const tags = mon?.tags || [];
    const applicable = items.filter(item => appliesToMonitor(item, mon?.id, tags));
    if (!applicable.length) {
      els.maintenance.hidden = true;
      return;
    }
    const next = applicable.slice().sort((a, b) => new Date(a.ends_at) - new Date(b.ends_at))[0];
    const until = next?.ends_at ? MonitoringPage.formatDate(next.ends_at) : '-';
    els.maintenance.textContent = `${MonitoringPage.t('monitoring.maintenance.activeUntil')} ${until}`;
    els.maintenance.hidden = false;
  }

  function appliesToMonitor(item, id, tags) {
    if (!item) return false;
    if (item.monitor_id && item.monitor_id !== id) return false;
    const itemTags = item.tags || [];
    if (!itemTags.length) return true;
    return itemTags.some(tag => tags.includes(tag));
  }

  function statCard(label, value) {
    const card = document.createElement('div');
    card.className = 'monitoring-stat';
    card.innerHTML = `<div class="label">${label}</div><div class="value">${value || '-'}</div>`;
    return card;
  }

  function updateActionLabels(mon) {
    if (!els.pause) return;
    const paused = !!mon.is_paused;
    els.pause.textContent = paused
      ? MonitoringPage.t('monitoring.actions.resume')
      : MonitoringPage.t('monitoring.actions.pause');
  }

  function toggleActionAccess() {
    const canManage = MonitoringPage.hasPermission('monitoring.manage');
    [els.pause, els.edit, els.clone, els.remove, els.checkNow].forEach(btn => {
      if (!btn) return;
      btn.disabled = !canManage;
      btn.classList.toggle('disabled', !canManage);
    });
  }

  async function handlePause() {
    const mon = MonitoringPage.selectedMonitor();
    if (!mon) return;
    const paused = !!mon.is_paused;
    const action = paused ? 'resume' : 'pause';
    try {
      await Api.post(`/api/monitoring/monitors/${mon.id}/${action}`);
      await MonitoringPage.loadMonitors?.();
      await loadDetail(mon.id);
    } catch (err) {
      console.error('pause', err);
    }
  }

  async function handleClone() {
    const mon = MonitoringPage.selectedMonitor();
    if (!mon) return;
    try {
      const res = await Api.post(`/api/monitoring/monitors/${mon.id}/clone`, {});
      await MonitoringPage.loadMonitors?.();
      const nextId = res?.id || res?.monitor_id || null;
      if (nextId) {
        MonitoringPage.state.selectedId = nextId;
        await loadDetail(nextId);
      }
    } catch (err) {
      console.error('clone', err);
    }
  }

  async function handleDelete() {
    const mon = MonitoringPage.selectedMonitor();
    if (!mon) return;
    const confirmed = window.confirm(MonitoringPage.t('monitoring.confirmDelete'));
    if (!confirmed) return;
    try {
      await Api.del(`/api/monitoring/monitors/${mon.id}`);
      MonitoringPage.state.selectedId = null;
      await MonitoringPage.loadMonitors?.();
      clearDetail();
    } catch (err) {
      console.error('delete', err);
    }
  }

  async function handleCheckNow() {
    const mon = MonitoringPage.selectedMonitor();
    if (!mon) return;
    try {
      await Api.post(`/api/monitoring/monitors/${mon.id}/check-now`, {});
      setTimeout(() => loadDetail(mon.id), 1200);
    } catch (err) {
      console.error('check now', err);
    }
  }

  function statusClass(status) {
    const val = (status || '').toLowerCase();
    if (val === 'up') return 'up';
    if (val === 'paused') return 'paused';
    if (val === 'maintenance' || val === 'maintenance_start' || val === 'maintenance_end') return 'maintenance';
    return 'down';
  }

  function statusLabel(status) {
    const val = (status || '').toLowerCase();
    if (val === 'maintenance_start') return MonitoringPage.t('monitoring.event.maintenanceStart');
    if (val === 'maintenance_end') return MonitoringPage.t('monitoring.event.maintenanceEnd');
    if (val === 'tls_expiring') return MonitoringPage.t('monitoring.event.tlsExpiring');
    const key = `monitoring.status.${val}`;
    return MonitoringPage.t(key);
  }

  if (typeof MonitoringPage !== 'undefined') {
    MonitoringPage.bindDetail = bindDetail;
    MonitoringPage.loadDetail = loadDetail;
    MonitoringPage.clearDetail = clearDetail;
  }
})();
