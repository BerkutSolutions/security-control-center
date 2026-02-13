(() => {
  const els = {};
  const state = {
    items: [],
    history: [],
    bound: false,
  };

  async function bindSLA() {
    els.list = document.getElementById('monitoring-sla-list');
    els.history = document.getElementById('monitoring-sla-history');
    els.refresh = document.getElementById('monitoring-sla-refresh');
    els.status = document.getElementById('monitoring-sla-status');
    if (!els.list || !els.history) return;
    if (!MonitoringPage.hasPermission('monitoring.view')) {
      const panel = document.getElementById('monitoring-tab-sla');
      if (panel) panel.hidden = true;
      return;
    }
    if (!state.bound && els.refresh) {
      els.refresh.addEventListener('click', () => {
        loadOverview();
        loadHistory();
      });
    }
    if (!state.bound && els.status) {
      els.status.addEventListener('change', () => loadOverview());
    }
    state.bound = true;
    await loadOverview();
    await loadHistory();
  }

  async function loadOverview() {
    const status = els.status?.value || '';
    const qs = status ? `?status=${encodeURIComponent(status)}` : '';
    try {
      const res = await Api.get(`/api/monitoring/sla/overview${qs}`);
      state.items = Array.isArray(res.items) ? res.items : [];
      renderOverview();
    } catch (err) {
      renderError(els.list, MonitoringPage.sanitizeErrorMessage(err.message || err));
    }
  }

  async function loadHistory() {
    try {
      const res = await Api.get('/api/monitoring/sla/history?limit=25');
      state.history = Array.isArray(res.items) ? res.items : [];
      renderHistory();
    } catch (err) {
      renderError(els.history, MonitoringPage.sanitizeErrorMessage(err.message || err));
    }
  }

  function renderOverview() {
    if (!els.list) return;
    if (!state.items.length) {
      els.list.innerHTML = `<div class="muted">${escapeHtml(MonitoringPage.t('monitoring.sla.empty'))}</div>`;
      return;
    }
    const cards = state.items.map((item) => renderCard(item)).join('');
    els.list.innerHTML = `<div class="monitoring-sla-grid">${cards}</div>`;
    bindPolicyActions();
  }

  function renderCard(item) {
    const target = Number(item.target_pct || 0);
    const w24 = item.window_24h || {};
    const w7 = item.window_7d || {};
    const w30 = item.window_30d || {};
    const period = item.policy?.incident_period || 'day';
    return `
      <section class="monitoring-sla-card" data-monitor-id="${item.monitor_id}">
        <div class="monitoring-sla-row">
          <div class="monitoring-sla-title-wrap">
            <div class="monitoring-sla-title">${escapeHtml(item.name || `#${item.monitor_id}`)}</div>
            <div class="muted">${escapeHtml(item.type || '')}</div>
          </div>
          <label class="checkbox">
            <input type="checkbox" data-role="incident-toggle" ${item.policy?.incident_on_violation ? 'checked' : ''} ${!canManagePolicy() ? 'disabled' : ''}>
            <span>${escapeHtml(MonitoringPage.t('monitoring.sla.incidentToggle'))}</span>
          </label>
          <label class="monitoring-sla-inline-field monitoring-sla-period-field">
            <span>${escapeHtml(MonitoringPage.t('monitoring.sla.incidentPeriod'))}</span>
            <select class="select" data-role="incident-period" ${!canManagePolicy() ? 'disabled' : ''}>
              <option value="day" ${period === 'day' ? 'selected' : ''}>${escapeHtml(MonitoringPage.t('monitoring.sla.period.day'))}</option>
              <option value="week" ${period === 'week' ? 'selected' : ''}>${escapeHtml(MonitoringPage.t('monitoring.sla.period.week'))}</option>
              <option value="month" ${period === 'month' ? 'selected' : ''}>${escapeHtml(MonitoringPage.t('monitoring.sla.period.month'))}</option>
            </select>
          </label>
          <label class="monitoring-sla-inline-field monitoring-sla-target-field">
            <span>${escapeHtml(MonitoringPage.t('monitoring.field.slaTarget'))}</span>
            <input class="input" type="number" min="1" max="100" step="0.1" data-role="target" value="${target.toFixed(1)}" ${!canManagePolicy() ? 'disabled' : ''}>
          </label>
          ${renderStatus(item.window_30d?.status)}
          <button class="btn ghost" data-role="save-policy" ${!canManagePolicy() ? 'disabled' : ''}>${escapeHtml(MonitoringPage.t('common.save'))}</button>
        </div>
        <div class="monitoring-stats monitoring-sla-metrics">
          ${renderMetricCloud(MonitoringPage.t('monitoring.stats.uptime24h'), w24, target)}
          ${renderMetricCloud(MonitoringPage.t('monitoring.range.week'), w7, target)}
          ${renderMetricCloud(MonitoringPage.t('monitoring.range.month'), w30, target)}
        </div>
      </section>
    `;
  }

  function renderMetricCloud(label, window, target) {
    const status = (window?.status || 'unknown').toLowerCase();
    const uptime = Number(window?.uptime_pct || 0);
    const coverage = Number(window?.coverage_pct || 0);
    const cls = metricClass(status, uptime, target);
    const statusText = status === 'unknown'
      ? MonitoringPage.t('monitoring.sla.unknown')
      : `${formatPct(uptime)} / ${formatPct(target)}`;
    return `
      <div class="monitoring-stat monitoring-sla-cloud">
        <div class="label">${escapeHtml(label)}</div>
        <div class="value ${cls}">${escapeHtml(statusText)}</div>
        <div class="muted">${escapeHtml(MonitoringPage.t('monitoring.sla.coverage'))}: ${escapeHtml(formatPct(coverage))}</div>
      </div>
    `;
  }

  function metricClass(status, uptime, target) {
    if (status === 'unknown') return 'sla-unknown';
    return uptime >= target ? 'sla-ok' : 'sla-bad';
  }

  function renderHistory() {
    if (!els.history) return;
    const policyByMonitor = new Map(state.items.map((item) => [item.monitor_id, (item.policy?.incident_period || 'day')]));
    const filtered = state.history.filter((item) => {
      const period = policyByMonitor.get(item.monitor_id);
      return !period || period === item.period_type;
    });
    if (!filtered.length) {
      els.history.innerHTML = `<div class="muted">${escapeHtml(MonitoringPage.t('monitoring.sla.historyEmpty'))}</div>`;
      return;
    }
    const rows = filtered.map((item) => `
      <tr>
        <td>${escapeHtml(item.monitor_name || `#${item.monitor_id}`)}</td>
        <td>${escapeHtml(periodLabel(item.period_type))}</td>
        <td>${escapeHtml(MonitoringPage.formatDate(item.period_start))}</td>
        <td>${escapeHtml(MonitoringPage.formatDate(item.period_end))}</td>
        <td>${formatPct(item.uptime_pct)}</td>
        <td>${formatPct(item.coverage_pct)}</td>
        <td>${formatPct(item.target_pct)}</td>
        <td>${renderStatus(item.status)}</td>
      </tr>
    `).join('');
    els.history.innerHTML = `
      <div class="monitoring-sla-history-table-wrap">
      <table class="monitoring-sla-history-table">
        <thead>
          <tr>
            <th>${escapeHtml(MonitoringPage.t('monitoring.field.name'))}</th>
            <th>${escapeHtml(MonitoringPage.t('monitoring.sla.incidentPeriod'))}</th>
            <th>${escapeHtml(MonitoringPage.t('monitoring.sla.periodStart'))}</th>
            <th>${escapeHtml(MonitoringPage.t('monitoring.sla.periodEnd'))}</th>
            <th>${escapeHtml(MonitoringPage.t('monitoring.sla.uptime'))}</th>
            <th>${escapeHtml(MonitoringPage.t('monitoring.sla.coverage'))}</th>
            <th>${escapeHtml(MonitoringPage.t('monitoring.field.slaTarget'))}</th>
            <th>${escapeHtml(MonitoringPage.t('monitoring.sla.status'))}</th>
          </tr>
        </thead>
        <tbody>${rows}</tbody>
      </table>
      </div>
    `;
  }

  function bindPolicyActions() {
    els.list.querySelectorAll('button[data-role="save-policy"]').forEach((btn) => {
      btn.addEventListener('click', async () => {
        const card = btn.closest('.monitoring-sla-card[data-monitor-id]');
        if (!card) return;
        const monitorId = Number(card.dataset.monitorId || 0);
        if (!monitorId) return;
        const toggle = card.querySelector('input[data-role="incident-toggle"]');
        const period = card.querySelector('select[data-role="incident-period"]');
        const targetInput = card.querySelector('input[data-role="target"]');
        const target = parseFloat(targetInput?.value || '0') || 0;
        if (target <= 0 || target > 100) {
          alert(MonitoringPage.t('monitoring.error.invalidSLA'));
          return;
        }
        try {
          await Api.put(`/api/monitoring/monitors/${monitorId}`, { sla_target_pct: target });
          await Api.put(`/api/monitoring/monitors/${monitorId}/sla-policy`, {
            incident_on_violation: !!toggle?.checked,
            incident_period: period?.value || 'day',
          });
          await loadOverview();
          await loadHistory();
        } catch (err) {
          alert(MonitoringPage.sanitizeErrorMessage(err.message || err));
        }
      });
    });
  }

  function renderStatus(status) {
    const normalized = (status || 'unknown').toLowerCase();
    let key = 'monitoring.sla.unknown';
    if (normalized === 'ok') key = 'monitoring.sla.ok';
    if (normalized === 'violated') key = 'monitoring.sla.violated';
    return `<span class="status-badge ${escapeHtml(normalized)}">${escapeHtml(MonitoringPage.t(key))}</span>`;
  }

  function periodLabel(period) {
    const normalized = (period || '').toLowerCase();
    if (normalized === 'week') return MonitoringPage.t('monitoring.sla.period.week');
    if (normalized === 'month') return MonitoringPage.t('monitoring.sla.period.month');
    return MonitoringPage.t('monitoring.sla.period.day');
  }

  function formatPct(value) {
    if (value === null || value === undefined || Number.isNaN(Number(value))) return '-';
    return `${Number(value).toFixed(2)}%`;
  }

  function renderError(el, text) {
    if (!el) return;
    el.innerHTML = `<div class="alert">${escapeHtml(text || MonitoringPage.t('common.error'))}</div>`;
  }

  function canManagePolicy() {
    return MonitoringPage.hasPermission('monitoring.manage');
  }

  function escapeHtml(str) {
    return (str || '').toString().replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  }

  if (typeof MonitoringPage !== 'undefined') {
    MonitoringPage.bindSLA = bindSLA;
    MonitoringPage.refreshSLA = async () => {
      await loadOverview();
      await loadHistory();
    };
  }
})();
