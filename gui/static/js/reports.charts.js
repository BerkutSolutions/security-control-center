(() => {
  const state = ReportsPage.state;
  const CHART_DEFS = [
    { type: 'incidents_severity_bar', section: 'incidents', titleKey: 'reports.charts.incidentsSeverity' },
    { type: 'incidents_status_bar', section: 'incidents', titleKey: 'reports.charts.incidentsStatus' },
    { type: 'incidents_weekly_line', section: 'incidents', titleKey: 'reports.charts.incidentsWeekly', config: { key: 'weeks', labelKey: 'reports.charts.config.weeks', min: 4, max: 16 } },
    { type: 'tasks_status_bar', section: 'tasks', titleKey: 'reports.charts.tasksStatus' },
    { type: 'tasks_weekly_line', section: 'tasks', titleKey: 'reports.charts.tasksWeekly', config: { key: 'weeks', labelKey: 'reports.charts.config.weeks', min: 4, max: 16 } },
    { type: 'docs_approvals_bar', section: 'docs', titleKey: 'reports.charts.docsApprovals' },
    { type: 'docs_weekly_line', section: 'docs', titleKey: 'reports.charts.docsWeekly', config: { key: 'weeks', labelKey: 'reports.charts.config.weeks', min: 4, max: 16 } },
    { type: 'controls_status_bar', section: 'controls', titleKey: 'reports.charts.controlsStatus' },
    { type: 'controls_domains_bar', section: 'controls', titleKey: 'reports.charts.controlsDomains', config: { key: 'top_n', labelKey: 'reports.charts.config.topN', min: 3, max: 12 } },
    { type: 'monitoring_uptime_bar', section: 'monitoring', titleKey: 'reports.charts.monitoringUptime', config: { key: 'top_n', labelKey: 'reports.charts.config.topN', min: 3, max: 12 } },
    { type: 'monitoring_downtime_line', section: 'monitoring', titleKey: 'reports.charts.monitoringDowntime', config: { key: 'days', labelKey: 'reports.charts.config.days', min: 7, max: 31 } },
    { type: 'monitoring_tls_bar', section: 'monitoring', titleKey: 'reports.charts.monitoringTLS' }
  ];

  function bindCharts() {
    const saveBtn = document.getElementById('report-charts-save');
    if (saveBtn) {
      saveBtn.onclick = () => saveCharts();
      saveBtn.disabled = !ReportsPage.hasPermission('reports.edit');
    }
    const execBtn = document.getElementById('report-exec-btn');
    if (execBtn) {
      execBtn.onclick = () => applyExecutivePreset();
      execBtn.disabled = !ReportsPage.hasPermission('reports.edit');
    }
  }

  async function loadCharts(reportId) {
    if (!reportId) return;
    try {
      const res = await Api.get(`/api/reports/${reportId}/charts`);
      state.charts = res.charts || [];
      state.chartsSnapshotId = res.snapshot_id || 0;
      renderCharts();
    } catch (err) {
      ReportsPage.showAlert('report-editor-alert', err.message || BerkutI18n.t('common.error'));
    }
  }

  function renderCharts() {
    const wrap = document.getElementById('report-charts-editor');
    if (!wrap) return;
    const charts = normalizeCharts(state.charts || []);
    wrap.innerHTML = '';
    charts.forEach(ch => {
      const def = CHART_DEFS.find(d => d.type === ch.chart_type);
      const title = BerkutI18n.t(def?.titleKey || 'reports.charts.title');
      const configInput = def?.config ? `
        <div class="form-field">
          <label>${t(def.config.labelKey)}</label>
          <input type="number" class="input" data-config="${def.config.key}" min="${def.config.min}" max="${def.config.max}" value="${escapeAttr(ch.config?.[def.config.key] ?? '')}">
        </div>` : '';
      const card = document.createElement('div');
      card.className = 'report-chart-card';
      card.dataset.type = ch.chart_type;
      card.dataset.id = ch.id || '';
      card.dataset.title = title;
      card.innerHTML = `
        <div class="report-chart-row">
          <label class="checkbox">
            <input type="checkbox" class="report-chart-enabled" ${ch.is_enabled ? 'checked' : ''}>
            <span>${escapeHtml(title)}</span>
          </label>
          <div class="report-chart-actions">
            <button class="btn ghost btn-sm" data-action="up">${t('reports.charts.moveUp')}</button>
            <button class="btn ghost btn-sm" data-action="down">${t('reports.charts.moveDown')}</button>
          </div>
        </div>
        ${configInput}
      `;
      card.querySelectorAll('button[data-action]').forEach(btn => {
        btn.onclick = () => moveChart(card, btn.dataset.action);
      });
      wrap.appendChild(card);
    });
    renderChartsPreview(charts);
  }

  function renderChartsPreview(charts) {
    const preview = document.getElementById('report-charts-preview');
    if (!preview) return;
    const reportId = state.editor.id;
    preview.innerHTML = '';
    if (!state.chartsSnapshotId) {
      preview.hidden = false;
      preview.textContent = t('reports.charts.snapshotRequired');
      return;
    }
    const enabled = charts.filter(c => c.is_enabled && c.id);
    if (!enabled.length) {
      preview.hidden = true;
      return;
    }
    preview.hidden = false;
    enabled.forEach(ch => {
      const def = CHART_DEFS.find(d => d.type === ch.chart_type);
      const title = BerkutI18n.t(def?.titleKey || 'reports.charts.title');
      const card = document.createElement('div');
      card.className = 'report-chart-preview-card';
      card.innerHTML = `
        <div class="muted">${escapeHtml(title)}</div>
        <img src="/api/reports/${reportId}/charts/${ch.id}/render?ts=${Date.now()}" alt="${escapeAttr(title)}">`;
      preview.appendChild(card);
    });
  }

  function normalizeCharts(list) {
    const out = [];
    CHART_DEFS.forEach(def => {
      const existing = list.find(c => c.chart_type === def.type);
      if (existing) {
        out.push(existing);
      }
    });
    return out.length ? out : list;
  }

  function moveChart(card, dir) {
    const parent = card.parentElement;
    if (!parent) return;
    if (dir === 'up' && card.previousElementSibling) {
      parent.insertBefore(card, card.previousElementSibling);
    }
    if (dir === 'down' && card.nextElementSibling) {
      parent.insertBefore(card.nextElementSibling, card);
    }
  }

  async function saveCharts() {
    if (!state.editor.id) return;
    const payload = collectChartsPayload();
    try {
      await Api.put(`/api/reports/${state.editor.id}/charts`, payload);
      ReportsPage.showAlert('report-editor-alert', BerkutI18n.t('common.saved'), true);
      await loadCharts(state.editor.id);
    } catch (err) {
      ReportsPage.showAlert('report-editor-alert', err.message || BerkutI18n.t('common.error'));
    }
  }

  function collectChartsPayload() {
    const charts = [];
    document.querySelectorAll('#report-charts-editor .report-chart-card').forEach(card => {
      const type = card.dataset.type;
      const id = parseInt(card.dataset.id || '0', 10);
      const enabled = card.querySelector('.report-chart-enabled')?.checked || false;
      const config = {};
      card.querySelectorAll('[data-config]').forEach(input => {
        const key = input.dataset.config;
        const val = parseInt(input.value, 10);
        if (key && !Number.isNaN(val)) {
          config[key] = val;
        }
      });
      const def = CHART_DEFS.find(d => d.type === type);
      charts.push({
        id,
        chart_type: type,
        title: card.dataset.title || BerkutI18n.t(def?.titleKey || 'reports.charts.title'),
        section_type: def?.section || '',
        config,
        is_enabled: enabled
      });
    });
    return { charts };
  }

  async function applyExecutivePreset() {
    if (!state.editor.id) return;
    const reason = prompt(t('reports.exec.reasonPrompt'));
    if (!reason) return;
    const template = findExecutiveTemplate();
    try {
      if (template?.template_markdown) {
        const contentEl = document.getElementById('report-editor-content');
        if (contentEl) contentEl.value = template.template_markdown;
        await Api.put(`/api/reports/${state.editor.id}/content`, { content: template.template_markdown, reason });
        await Api.put(`/api/reports/${state.editor.id}`, { template_id: template.id });
      }
      const sectionsPayload = executiveSectionsPayload();
      await Api.put(`/api/reports/${state.editor.id}/sections`, sectionsPayload);
      const chartsPayload = executiveChartsPayload();
      await Api.put(`/api/reports/${state.editor.id}/charts`, chartsPayload);
      ReportsPage.showAlert('report-editor-alert', t('reports.exec.applied'), true);
      await ReportsPage.loadSections(state.editor.id);
      await loadCharts(state.editor.id);
    } catch (err) {
      ReportsPage.showAlert('report-editor-alert', err.message || t('common.error'));
    }
  }

  function executiveSectionsPayload() {
    const pf = document.getElementById('report-sections-period-from')?.value || '';
    const pt = document.getElementById('report-sections-period-to')?.value || '';
    const sections = (state.sections || []).map(sec => {
      const enabled = ['summary', 'incidents', 'tasks', 'controls', 'monitoring'].includes(sec.section_type);
      const cfg = sec.config || {};
      if (sec.section_type === 'summary') {
        cfg.executive = true;
      }
      return { section_type: sec.section_type, title: sec.title, config: cfg, is_enabled: enabled };
    });
    return { period_from: pf, period_to: pt, sections };
  }

  function executiveChartsPayload() {
    const execTypes = new Set([
      'incidents_severity_bar',
      'tasks_status_bar',
      'controls_status_bar',
      'monitoring_downtime_line',
      'monitoring_tls_bar'
    ]);
    const charts = (state.charts || []).map(ch => {
      const def = CHART_DEFS.find(d => d.type === ch.chart_type);
      return {
        id: ch.id,
        chart_type: ch.chart_type,
        title: BerkutI18n.t(def?.titleKey || 'reports.charts.title'),
        section_type: def?.section || ch.section_type || '',
        config: ch.config || {},
        is_enabled: execTypes.has(ch.chart_type)
      };
    });
    return { charts };
  }

  function findExecutiveTemplate() {
    const templates = state.templates || [];
    const candidates = templates.filter(tpl => (tpl.description || '').toUpperCase().includes('EXECUTIVE'));
    if (!candidates.length) return templates[0];
    const lang = BerkutI18n.currentLang();
    const tag = lang === 'ru' ? 'EXECUTIVE_RU' : 'EXECUTIVE_EN';
    return candidates.find(tpl => (tpl.description || '').toUpperCase().includes(tag)) || candidates[0];
  }

  function escapeHtml(str) {
    return (str || '').toString().replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  }

  function escapeAttr(str) {
    return escapeHtml(str).replace(/"/g, '&quot;');
  }

  function t(key) {
    return BerkutI18n.t(key);
  }

  ReportsPage.bindCharts = bindCharts;
  ReportsPage.loadCharts = loadCharts;
})();


