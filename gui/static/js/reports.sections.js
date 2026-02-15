(() => {
  const state = ReportsPage.state;
  const SECTION_DEFS = [
    { type: 'summary', titleKey: 'reports.sections.summary' },
    { type: 'incidents', titleKey: 'reports.sections.incidents' },
    { type: 'tasks', titleKey: 'reports.sections.tasks' },
    { type: 'docs', titleKey: 'reports.sections.docs' },
    { type: 'controls', titleKey: 'reports.sections.controls' },
    { type: 'monitoring', titleKey: 'reports.sections.monitoring' },
    { type: 'sla_summary', titleKey: 'reports.sections.slaSummary' },
    { type: 'audit', titleKey: 'reports.sections.audit' },
    { type: 'custom_md', titleKey: 'reports.sections.custom' }
  ];

  function bindSections() {
    const saveBtn = document.getElementById('report-sections-save');
    if (saveBtn) {
      saveBtn.onclick = () => saveSections();
      saveBtn.disabled = !ReportsPage.hasPermission('reports.edit');
    }
    const buildBtn = document.getElementById('report-sections-build');
    if (buildBtn) {
      buildBtn.onclick = () => buildReport('markers');
      buildBtn.disabled = !ReportsPage.hasPermission('reports.edit');
    }
    const rebuildBtn = document.getElementById('report-sections-rebuild');
    if (rebuildBtn) {
      rebuildBtn.onclick = () => buildReport('replace');
      rebuildBtn.disabled = !ReportsPage.hasPermission('reports.edit');
    }
    const snapBtn = document.getElementById('report-snapshots-view');
    if (snapBtn) snapBtn.onclick = () => toggleSnapshots();
  }

  async function loadSections(reportId) {
    if (!reportId) return;
    try {
      const res = await Api.get(`/api/reports/${reportId}/sections`);
      state.sections = res.sections || [];
      state.sectionsMeta = res.meta || {};
      renderSections();
      applySectionPeriod(state.sectionsMeta);
    } catch (err) {
      ReportsPage.showAlert('report-editor-alert', err.message || BerkutI18n.t('common.error'));
    }
  }

  function applySectionPeriod(meta) {
    const pf = document.getElementById('report-sections-period-from');
    const pt = document.getElementById('report-sections-period-to');
    if (pf) pf.value = ReportsPage.formatDateInput(meta?.period_from);
    if (pt) pt.value = ReportsPage.formatDateInput(meta?.period_to);
  }

  function renderSections() {
    const wrap = document.getElementById('report-sections-editor');
    if (!wrap) return;
    const sections = normalizeSections(state.sections || []);
    wrap.innerHTML = '';
    sections.forEach(sec => {
      const def = SECTION_DEFS.find(d => d.type === sec.section_type);
      const title = sec.title || BerkutI18n.t(def?.titleKey || 'reports.sections.custom');
      const container = document.createElement('div');
      container.className = 'report-section-card';
      container.dataset.type = sec.section_type;
      container.dataset.title = title;
      container.innerHTML = `
        <div class="report-section-header">
          <label class="checkbox">
            <input type="checkbox" class="report-section-enabled" ${sec.is_enabled ? 'checked' : ''}>
            <span>${escapeHtml(title)}</span>
          </label>
        </div>
        <div class="report-section-body"></div>
      `;
      const body = container.querySelector('.report-section-body');
      if (body) body.innerHTML = renderSectionFields(sec);
      wrap.appendChild(container);
    });
  }

  function renderSectionFields(sec) {
    const cfg = sec.config || {};
    switch (sec.section_type) {
      case 'incidents':
        return `
          <div class="form-field"><label>${t('reports.sections.filters.severity')}</label>
            <input class="input" data-field="severity" value="${escapeAttr(cfg.severity || '')}">
          </div>
          <div class="form-field"><label>${t('reports.sections.filters.status')}</label>
            <input class="input" data-field="status" value="${escapeAttr(cfg.status || '')}">
          </div>
          <div class="form-field"><label>${t('reports.sections.filters.type')}</label>
            <input class="input" data-field="type" value="${escapeAttr(cfg.type || '')}">
          </div>
          <div class="form-field"><label>${t('reports.sections.filters.owner')}</label>
            <select class="select" data-field="owner">${userOptions(cfg.owner)}</select>
          </div>
          <div class="form-field"><label>${t('reports.sections.filters.tags')}</label>
            <input class="input" data-field="tags" value="${escapeAttr((cfg.tags || []).join(', '))}">
          </div>
          <div class="form-field"><label>${t('reports.sections.filters.limit')}</label>
            <input type="number" class="input" data-field="limit" value="${cfg.limit || 20}">
          </div>`;
      case 'tasks':
        return `
          <div class="form-field"><label>${t('reports.sections.filters.status')}</label>
            <input class="input" data-field="status" value="${escapeAttr(cfg.status || '')}">
          </div>
          <div class="form-field"><label>${t('reports.sections.filters.assignee')}</label>
            <select class="select" data-field="assignee">${userOptions(cfg.assignee)}</select>
          </div>
          <div class="form-field"><label>${t('reports.sections.filters.board')}</label>
            <input type="number" class="input" data-field="board_id" value="${cfg.board_id || ''}">
          </div>
          <div class="form-field"><label>${t('reports.sections.filters.space')}</label>
            <input type="number" class="input" data-field="space_id" value="${cfg.space_id || ''}">
          </div>
          <div class="form-field"><label>${t('reports.sections.filters.tags')}</label>
            <input class="input" data-field="tags" value="${escapeAttr((cfg.tags || []).join(', '))}">
          </div>
          <div class="form-field"><label>${t('reports.sections.filters.limit')}</label>
            <input type="number" class="input" data-field="limit" value="${cfg.limit || 20}">
          </div>`;
      case 'docs':
        return `
          <div class="form-field"><label>${t('reports.sections.filters.status')}</label>
            <input class="input" data-field="status" value="${escapeAttr(cfg.status || '')}">
          </div>
          <div class="form-field"><label>${t('reports.sections.filters.classification')}</label>
            <input class="input" data-field="classification" value="${escapeAttr(cfg.classification || '')}">
          </div>
          <div class="form-field"><label>${t('reports.sections.filters.tags')}</label>
            <input class="input" data-field="tags" value="${escapeAttr((cfg.tags || []).join(', '))}">
          </div>
          <div class="form-field"><label>${t('reports.sections.filters.limit')}</label>
            <input type="number" class="input" data-field="limit" value="${cfg.limit || 20}">
          </div>`;
      case 'controls':
        return `
          <div class="form-field"><label>${t('reports.sections.filters.status')}</label>
            <input class="input" data-field="status" value="${escapeAttr(cfg.status || '')}">
          </div>
          <div class="form-field"><label>${t('reports.sections.filters.risk')}</label>
            <input class="input" data-field="risk" value="${escapeAttr(cfg.risk || '')}">
          </div>
          <div class="form-field"><label>${t('reports.sections.filters.domain')}</label>
            <input class="input" data-field="domain" value="${escapeAttr(cfg.domain || '')}">
          </div>
          <div class="form-field"><label>${t('reports.sections.filters.limit')}</label>
            <input type="number" class="input" data-field="limit" value="${cfg.limit || 20}">
          </div>`;
      case 'monitoring':
        return `
          <div class="form-field">
            <label class="checkbox"><input type="checkbox" data-field="only_critical" ${cfg.only_critical ? 'checked' : ''}>
            <span>${t('reports.sections.filters.onlyCritical')}</span></label>
          </div>
          <div class="form-field">
            <label class="checkbox"><input type="checkbox" data-field="only_down" ${cfg.only_down ? 'checked' : ''}>
            <span>${t('reports.sections.filters.onlyDown')}</span></label>
          </div>
          <div class="form-field"><label>${t('reports.sections.filters.tlsDays')}</label>
            <input type="number" class="input" data-field="tls_expiring_days" value="${cfg.tls_expiring_days || ''}">
          </div>
          <div class="form-field"><label>${t('reports.sections.filters.eventsLimit')}</label>
            <input type="number" class="input" data-field="events_limit" value="${cfg.events_limit || 20}">
          </div>
          <div class="form-field"><label>${t('reports.sections.filters.limit')}</label>
            <input type="number" class="input" data-field="limit" value="${cfg.limit || 20}">
          </div>`;
      case 'audit':
        return `
          <div class="form-field">
            <label class="checkbox"><input type="checkbox" data-field="important_only" ${cfg.important_only ? 'checked' : ''}>
            <span>${t('reports.sections.filters.importantOnly')}</span></label>
          </div>
          <div class="form-field"><label>${t('reports.sections.filters.limit')}</label>
            <input type="number" class="input" data-field="limit" value="${cfg.limit || 50}">
          </div>`;
      case 'sla_summary':
        return `
          <div class="form-field"><label>${t('reports.sections.filters.slaPeriod')}</label>
            <select class="select" data-field="period_type">
              <option value="day" ${(cfg.period_type || 'month') === 'day' ? 'selected' : ''}>${t('reports.sections.filters.periodDay')}</option>
              <option value="week" ${(cfg.period_type || 'month') === 'week' ? 'selected' : ''}>${t('reports.sections.filters.periodWeek')}</option>
              <option value="month" ${(cfg.period_type || 'month') === 'month' ? 'selected' : ''}>${t('reports.sections.filters.periodMonth')}</option>
            </select>
          </div>
          <div class="form-field">
            <label class="checkbox"><input type="checkbox" data-field="only_violations" ${cfg.only_violations ? 'checked' : ''}>
            <span>${t('reports.sections.filters.onlyViolations')}</span></label>
          </div>
          <div class="form-field">
            <label class="checkbox"><input type="checkbox" data-field="include_current" ${cfg.include_current !== false ? 'checked' : ''}>
            <span>${t('reports.sections.filters.includeCurrent')}</span></label>
          </div>
          <div class="form-field"><label>${t('reports.sections.filters.limit')}</label>
            <input type="number" class="input" data-field="limit" value="${cfg.limit || 50}">
          </div>`;
      case 'custom_md':
        return `
          <div class="form-field"><label>${t('reports.sections.filters.customKey')}</label>
            <input class="input" data-field="key" value="${escapeAttr(cfg.key || '')}">
          </div>
          <div class="form-field"><label>${t('reports.sections.filters.customMarkdown')}</label>
            <textarea class="textarea" data-field="markdown" rows="6">${escapeHtml(cfg.markdown || '')}</textarea>
          </div>`;
      default:
        return '';
    }
  }

  async function saveSections() {
    if (!state.editor.id) return;
    const payload = collectSectionsPayload();
    try {
      await Api.put(`/api/reports/${state.editor.id}/sections`, payload);
      ReportsPage.showAlert('report-editor-alert', BerkutI18n.t('common.saved'), true);
      await loadSections(state.editor.id);
    } catch (err) {
      ReportsPage.showAlert('report-editor-alert', err.message || BerkutI18n.t('common.error'));
    }
  }

  async function buildReport(mode) {
    if (!state.editor.id) return;
    const reason = prompt(BerkutI18n.t('reports.sections.buildReason'));
    if (!reason) return;
    const payload = collectSectionsPayload();
    payload.reason = reason;
    payload.mode = mode || 'markers';
    try {
      await Api.post(`/api/reports/${state.editor.id}/build`, payload);
      ReportsPage.showAlert('report-editor-alert', BerkutI18n.t('reports.sections.buildSuccess'), true);
      await ReportsPage.openEditor(state.editor.id);
    } catch (err) {
      ReportsPage.showAlert('report-editor-alert', err.message || BerkutI18n.t('common.error'));
    }
  }

  async function toggleSnapshots() {
    const panel = document.getElementById('report-snapshots');
    if (!panel || !state.editor.id) return;
    if (!panel.hidden) {
      panel.hidden = true;
      return;
    }
    panel.hidden = false;
    panel.innerHTML = '';
    try {
      const res = await Api.get(`/api/reports/${state.editor.id}/snapshots`);
      const items = res.items || [];
      if (!items.length) {
        panel.textContent = t('reports.sections.snapshotsEmpty');
        return;
      }
      const table = document.createElement('table');
      table.className = 'data-table compact';
      table.innerHTML = `
        <thead><tr>
          <th>${t('reports.sections.snapshotTime')}</th>
          <th>${t('reports.sections.snapshotReason')}</th>
          <th>${t('reports.sections.snapshotActions')}</th>
        </tr></thead>
        <tbody></tbody>`;
      const tbody = table.querySelector('tbody');
      items.forEach(s => {
        const tr = document.createElement('tr');
        const when = s.created_at ? ReportsPage.formatDate(s.created_at) : '-';
        tr.innerHTML = `
          <td>${escapeHtml(when)}</td>
          <td>${escapeHtml(s.reason || '')}</td>
          <td><button class="btn ghost btn-sm" data-id="${s.id}">${t('reports.sections.snapshotOpen')}</button></td>`;
        tbody.appendChild(tr);
      });
      panel.appendChild(table);
      panel.querySelectorAll('button[data-id]').forEach(btn => {
        btn.onclick = () => openSnapshot(btn.dataset.id);
      });
    } catch (err) {
      panel.textContent = err.message || t('common.error');
    }
  }

  async function openSnapshot(snapshotId) {
    if (!snapshotId || !state.editor.id) return;
    try {
      const res = await Api.get(`/api/reports/${state.editor.id}/snapshots/${snapshotId}`);
      const snap = res.snapshot || {};
      const items = res.items || [];
      const panel = document.getElementById('report-snapshots');
      if (!panel) return;
      panel.innerHTML = `
        <div class="snapshot-meta">
          <div>${t('reports.sections.snapshotTime')}: ${escapeHtml(snap.created_at ? ReportsPage.formatDate(snap.created_at) : '')}</div>
          <div>${t('reports.sections.snapshotReason')}: ${escapeHtml(snap.reason || '')}</div>
          <div>${t('reports.sections.snapshotHash')}: ${escapeHtml(snap.sha256 || '')}</div>
        </div>
        <pre class="snapshot-json">${escapeHtml(JSON.stringify(snap.snapshot || {}, null, 2))}</pre>
        <div class="snapshot-items">${escapeHtml(JSON.stringify(items.map(i => i.entity || {}), null, 2))}</div>`;
    } catch (err) {
      ReportsPage.showAlert('report-editor-alert', err.message || BerkutI18n.t('common.error'));
    }
  }

  function collectSectionsPayload() {
    const pf = ReportsPage.toISODateInput(document.getElementById('report-sections-period-from')?.value || '');
    const pt = ReportsPage.toISODateInput(document.getElementById('report-sections-period-to')?.value || '');
    const sections = [];
    document.querySelectorAll('#report-sections-editor .report-section-card').forEach(card => {
      const type = card.dataset.type;
      const enabled = card.querySelector('.report-section-enabled')?.checked || false;
      const config = {};
      card.querySelectorAll('[data-field]').forEach(el => {
        const key = el.dataset.field;
        if (!key) return;
        if (el.type === 'checkbox') {
          config[key] = el.checked;
        } else if (el.tagName === 'SELECT') {
          const val = el.value;
          if (val) config[key] = val;
        } else {
          const val = el.value;
          if (val) {
            if (key === 'tags') {
              config[key] = val.split(',').map(t => t.trim()).filter(Boolean);
            } else {
              config[key] = val;
            }
          }
        }
      });
      sections.push({
        section_type: type,
        title: card.dataset.title || titleForType(type),
        config,
        is_enabled: enabled
      });
    });
    return { period_from: pf, period_to: pt, sections };
  }

  function titleForType(type) {
    const def = SECTION_DEFS.find(d => d.type === type);
    return BerkutI18n.t(def?.titleKey || 'reports.sections.custom') || type;
  }

  function normalizeSections(list) {
    const out = [];
    SECTION_DEFS.forEach(def => {
      const existing = list.find(s => s.section_type === def.type);
      if (existing) {
        out.push(existing);
      } else if (def.type !== 'custom_md') {
        out.push({ section_type: def.type, title: '', config: {}, is_enabled: true });
      }
    });
    list.filter(s => s.section_type === 'custom_md').forEach(s => out.push(s));
    return out;
  }

  function userOptions(selected) {
    const users = UserDirectory?.all ? UserDirectory.all() : [];
    let html = `<option value="">${t('common.all')}</option>`;
    users.forEach(u => {
      const label = u.full_name || u.username;
      html += `<option value="${u.id}" ${String(selected) === String(u.id) ? 'selected' : ''}>${escapeHtml(label)}</option>`;
    });
    return html;
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

  ReportsPage.bindSections = bindSections;
  ReportsPage.loadSections = loadSections;
})();
