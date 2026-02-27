const FindingsPage = (() => {
  const state = {
    items: [],
    links: [],
    linkOptions: { assets: [], software: [], controls: [], incidents: [], tasks: [], docs: [] },
    linkOptionsLoaded: false,
    permissions: [],
    pendingOpenId: null,
    tagsBound: false
  };

  function t(key) {
    const val = BerkutI18n.t(key);
    return val === key ? key : val;
  }

  function hasPerm(perm) {
    if (!perm) return true;
    const perms = Array.isArray(state.permissions) ? state.permissions : [];
    if (!perms.length) return true;
    return perms.includes(perm);
  }

  async function init() {
    const page = document.getElementById('findings-page');
    if (!page) return;
    state.pendingOpenId = parseInt(new URLSearchParams(window.location.search).get('finding') || '', 10) || null;
    await loadCurrentUser();
    bindUI();
    if (typeof RegistryReports !== 'undefined' && RegistryReports.bind) {
      RegistryReports.bind('findings-create-report', 'findings');
    }
    applyAccessControls();
    bindTagDirectory();
    await load();
    if (state.pendingOpenId) {
      const id = state.pendingOpenId;
      state.pendingOpenId = null;
      openFinding(id, 'view');
    }
  }

  async function loadCurrentUser() {
    try {
      const res = await Api.get('/api/auth/me');
      const me = res.user;
      state.permissions = Array.isArray(me?.permissions) ? me.permissions : [];
    } catch (_) {
      state.permissions = [];
    }
  }

  function bindUI() {
    document.getElementById('findings-apply')?.addEventListener('click', () => load());
    document.getElementById('findings-create')?.addEventListener('click', () => openFinding(null, 'create'));
    document.getElementById('finding-save')?.addEventListener('click', () => saveFinding());
    document.getElementById('finding-archive')?.addEventListener('click', () => archiveOrRestore());
    document.getElementById('finding-link-add')?.addEventListener('click', () => addLink());
    document.getElementById('finding-link-target-type')?.addEventListener('change', () => refreshLinkTargets());
    document.getElementById('finding-link-search')?.addEventListener('input', () => refreshLinkTargets());
  }

  function applyAccessControls() {
    const createBtn = document.getElementById('findings-create');
    if (createBtn) createBtn.hidden = !hasPerm('findings.manage');
    const includeField = document.getElementById('findings-include-deleted-field');
    if (includeField) includeField.hidden = !hasPerm('findings.manage');
  }

  function bindTagDirectory() {
    setFindingTags(selectedValues('finding-tags'));
    if (state.tagsBound) return;
    state.tagsBound = true;
    document.addEventListener('tags:changed', () => setFindingTags(selectedValues('finding-tags')));
  }

  function selectedValues(selectId) {
    const el = document.getElementById(selectId);
    return Array.from(el?.selectedOptions || []).map(o => o.value);
  }

  function setFindingTags(selected) {
    const select = document.getElementById('finding-tags');
    if (!select) return;
    const available = (typeof DocUI !== 'undefined' && DocUI.availableTags) ? DocUI.availableTags() : [];
    const selectedSet = new Set((selected || []).map(v => (v || '').toString().trim().toUpperCase()).filter(Boolean));
    select.innerHTML = '';

    const addOpt = (code) => {
      const normalized = (code || '').toString().trim().toUpperCase();
      if (!normalized) return;
      const opt = document.createElement('option');
      opt.value = normalized;
      const label = (typeof DocUI !== 'undefined' && DocUI.tagLabel) ? DocUI.tagLabel(normalized) : normalized;
      opt.textContent = label;
      opt.dataset.label = label;
      if (selectedSet.has(normalized)) opt.selected = true;
      select.appendChild(opt);
    };

    available.forEach(tag => addOpt(tag.code || tag));
    // keep tags that are already selected but not present in directory
    Array.from(selectedSet.values()).forEach(code => {
      if (!Array.from(select.options).some(o => o.value === code)) addOpt(code);
    });

    if (typeof DocsPage !== 'undefined' && DocsPage.enhanceMultiSelects) {
      DocsPage.enhanceMultiSelects([select.id]);
    } else {
      select.multiple = true;
      select.setAttribute('multiple', 'multiple');
      if (!select.size || select.size < 2) select.size = 6;
    }
    const hint = document.querySelector('[data-tag-hint="finding-tags"]');
    if (typeof DocUI !== 'undefined' && DocUI.bindTagHint) {
      DocUI.bindTagHint(select, hint);
    }
  }

  function buildFilterQuery() {
    const q = new URLSearchParams();
    const val = (id) => (document.getElementById(id)?.value || '').trim();
    if (val('findings-filter-q')) q.set('q', val('findings-filter-q'));
    if (val('findings-filter-status')) q.set('status', val('findings-filter-status'));
    if (val('findings-filter-severity')) q.set('severity', val('findings-filter-severity'));
    if (val('findings-filter-type')) q.set('type', val('findings-filter-type'));
    if (val('findings-filter-tag')) q.set('tag', val('findings-filter-tag'));
    const includeDeleted = val('findings-filter-include-deleted');
    if (includeDeleted) q.set('include_deleted', includeDeleted);
    q.set('limit', '200');
    return q.toString();
  }

  async function load() {
    try {
      const res = await Api.get(`/api/findings?${buildFilterQuery()}`);
      state.items = res.items || [];
    } catch (_) {
      state.items = [];
    }
    renderTable();
  }

  function renderTable() {
    const table = document.getElementById('findings-table');
    const tbody = table?.querySelector('tbody');
    const empty = document.getElementById('findings-empty');
    if (!tbody) return;
    tbody.innerHTML = '';
    if (!state.items.length) {
      if (empty) empty.hidden = false;
      return;
    }
    if (empty) empty.hidden = true;
    state.items.forEach((item) => {
      const tr = document.createElement('tr');
      const tags = Array.isArray(item.tags)
        ? item.tags.map(t => ((typeof DocUI !== 'undefined' && DocUI.tagLabel) ? DocUI.tagLabel(t) : t)).filter(Boolean).join(', ')
        : '';
      tr.innerHTML = `
        <td>${escapeHtml(item.title || '')}${item.deleted_at ? ` <span class="muted">(${escapeHtml(t('common.archived'))})</span>` : ''}</td>
        <td>${escapeHtml(statusLabel(item.status))}</td>
        <td>${escapeHtml(severityLabel(item.severity))}</td>
        <td>${escapeHtml(typeLabel(item.finding_type))}</td>
        <td>${escapeHtml(item.owner || '-')}</td>
        <td>${escapeHtml(item.due_at ? formatDate(item.due_at) : '-')}</td>
        <td>${escapeHtml(tags || '-')}</td>
        <td>${escapeHtml(item.updated_at ? formatDateTime(item.updated_at) : '-')}</td>
        <td class="actions"></td>
      `;
      const actions = tr.querySelector('.actions');
      const openBtn = document.createElement('button');
      openBtn.type = 'button';
      openBtn.className = 'btn ghost btn-xs';
      openBtn.textContent = t('common.open');
      openBtn.addEventListener('click', () => openFinding(item.id, 'view'));
      actions.appendChild(openBtn);
      tbody.appendChild(tr);
    });
  }

  async function openFinding(id, mode) {
    const modal = document.getElementById('finding-modal');
    const alert = document.getElementById('finding-modal-alert');
    if (alert) alert.hidden = true;
    if (!modal) return;
    const canManage = hasPerm('findings.manage');
    const isCreate = !id;
    const viewOnly = !canManage || mode === 'view';
    const titleEl = document.getElementById('finding-modal-title');
    if (titleEl) titleEl.textContent = isCreate ? t('findings.modal.createTitle') : t(viewOnly ? 'findings.modal.viewTitle' : 'findings.modal.editTitle');
    const archiveBtn = document.getElementById('finding-archive');
    if (archiveBtn) archiveBtn.hidden = isCreate || !canManage;

    resetForm();
    if (!isCreate) {
      try {
        const item = await Api.get(`/api/findings/${id}`);
        fillForm(item);
        await loadLinks(id);
      } catch (err) {
        if (alert) {
          alert.textContent = localizeError(err);
          alert.hidden = false;
        }
        return;
      }
    } else {
      state.links = [];
      renderLinks();
    }
    setFormDisabled(viewOnly);
    modal.hidden = false;
  }

  function closeFindingModal() {
    const modal = document.getElementById('finding-modal');
    if (modal) modal.hidden = true;
  }

  function resetForm() {
    setVal('finding-id', '');
    setVal('finding-title', '');
    setVal('finding-description', '');
    setVal('finding-status', 'open');
    setVal('finding-severity', 'medium');
    setVal('finding-type', 'other');
    setVal('finding-owner', '');
    setVal('finding-due-at', '');
    setFindingTags([]);
  }

  function fillForm(item) {
    setVal('finding-id', item.id || '');
    setVal('finding-title', item.title || '');
    setVal('finding-description', item.description_md || '');
    setVal('finding-status', (item.status || 'open').toLowerCase());
    setVal('finding-severity', (item.severity || 'medium').toLowerCase());
    setVal('finding-type', (item.finding_type || 'other').toLowerCase());
    setVal('finding-owner', item.owner || '');
    setVal('finding-due-at', item.due_at ? formatDateInput(item.due_at) : '');
    setFindingTags(Array.isArray(item.tags) ? item.tags : []);
    document.getElementById('finding-form').dataset.version = String(item.version || 1);
    const archiveBtn = document.getElementById('finding-archive');
    if (archiveBtn) {
      archiveBtn.textContent = item.deleted_at ? t('findings.actions.restore') : t('findings.actions.archive');
      archiveBtn.dataset.archived = item.deleted_at ? '1' : '0';
    }
  }

  function setFormDisabled(disabled) {
    ['finding-title', 'finding-description', 'finding-status', 'finding-severity', 'finding-type', 'finding-owner', 'finding-due-at', 'finding-tags'].forEach((id) => {
      const el = document.getElementById(id);
      if (el) el.disabled = !!disabled;
    });
    const saveBtn = document.getElementById('finding-save');
    if (saveBtn) saveBtn.hidden = !!disabled;
    const linkSection = document.getElementById('finding-links-section');
    if (linkSection) linkSection.hidden = !document.getElementById('finding-id')?.value;
    const linkAdd = document.getElementById('finding-link-add');
    const linkType = document.getElementById('finding-link-target-type');
    const linkTarget = document.getElementById('finding-link-target-id');
    const linkSearch = document.getElementById('finding-link-search');
    if (linkAdd) linkAdd.disabled = !!disabled;
    if (linkType) linkType.disabled = !!disabled;
    if (linkTarget) linkTarget.disabled = !!disabled;
    if (linkSearch) linkSearch.disabled = !!disabled;
  }

  async function saveFinding() {
    if (!hasPerm('findings.manage')) return;
    const alert = document.getElementById('finding-modal-alert');
    if (alert) alert.hidden = true;
    const id = (document.getElementById('finding-id')?.value || '').trim();
    const payload = {
      title: (document.getElementById('finding-title')?.value || '').trim(),
      description_md: (document.getElementById('finding-description')?.value || '').trim(),
      status: document.getElementById('finding-status')?.value || 'open',
      severity: document.getElementById('finding-severity')?.value || 'medium',
      finding_type: document.getElementById('finding-type')?.value || 'other',
      owner: (document.getElementById('finding-owner')?.value || '').trim(),
      due_at: dateToISO(document.getElementById('finding-due-at')?.value || ''),
      tags: selectedValues('finding-tags'),
      version: parseInt(document.getElementById('finding-form')?.dataset.version || '1', 10) || 1
    };
    if (!payload.title) {
      showAlert(alert, t('findings.titleRequired'));
      return;
    }
    try {
      const res = id
        ? await Api.put(`/api/findings/${id}`, payload)
        : await Api.post('/api/findings', payload);
      await load();
      closeFindingModal();
    } catch (err) {
      showAlert(alert, localizeError(err));
    }
  }

  async function archiveOrRestore() {
    if (!hasPerm('findings.manage')) return;
    const id = (document.getElementById('finding-id')?.value || '').trim();
    if (!id) return;
    const btn = document.getElementById('finding-archive');
    const archived = btn?.dataset.archived === '1';
    const confirmed = window.confirm(t('common.confirm'));
    if (!confirmed) return;
    try {
      if (archived) {
        await Api.post(`/api/findings/${id}/restore`, {});
      } else {
        await Api.del(`/api/findings/${id}`);
      }
      await load();
      openFinding(parseInt(id, 10), 'edit');
    } catch (err) {
      const alert = document.getElementById('finding-modal-alert');
      showAlert(alert, localizeError(err));
    }
  }

  async function loadLinks(id) {
    state.links = [];
    try {
      const res = await Api.get(`/api/findings/${id}/links`);
      state.links = res.items || [];
    } catch (_) {
      state.links = [];
    }
    renderLinks();
    refreshLinkTargets();
  }

  function renderLinks() {
    const list = document.getElementById('finding-links-list');
    const empty = document.getElementById('finding-links-empty');
    if (!list) return;
    list.innerHTML = '';
    if (!state.links.length) {
      if (empty) empty.hidden = false;
      return;
    }
    if (empty) empty.hidden = true;
    state.links.forEach((l) => {
      const row = document.createElement('div');
      row.className = 'link-item';
      const label = `${linkTypeLabel(l.target_type)} #${l.target_id}`;
      const title = l.target_title ? ` - ${l.target_title}` : '';
      row.innerHTML = `<span>${escapeHtml(label)}${title ? ` <span class="muted">${escapeHtml(title)}</span>` : ''}</span><span class="muted">${escapeHtml(relationLabel(l.relation_type))}</span>`;
      const actions = document.createElement('div');
      actions.className = 'table-actions';
      const openHref = linkHref(l.target_type, l.target_id);
      if (openHref) {
        const open = document.createElement('a');
        open.className = 'btn ghost btn-xs';
        open.href = openHref;
        open.textContent = t('controls.links.open');
        actions.appendChild(open);
      }
      if (hasPerm('findings.manage')) {
        const del = document.createElement('button');
        del.type = 'button';
        del.className = 'btn ghost btn-xs';
        del.textContent = t('findings.actions.removeLink');
        del.addEventListener('click', () => deleteLink(l.id));
        actions.appendChild(del);
      }
      row.appendChild(actions);
      list.appendChild(row);
    });
  }

  async function ensureLinkOptions() {
    if (state.linkOptionsLoaded) return;
    try {
      const [assetsRes, softwareRes, controlsRes, incRes, tasksRes, docsRes] = await Promise.all([
        Api.get('/api/assets/list?limit=200').catch(() => ({ items: [] })),
        Api.get('/api/software/list?limit=200').catch(() => ({ items: [] })),
        Api.get('/api/controls?limit=200').catch(() => ({ items: [] })),
        Api.get('/api/incidents/list?limit=200').catch(() => ({ items: [] })),
        Api.get('/api/tasks?limit=200&include_archived=1').catch(() => ({ items: [] })),
        Api.get('/api/docs/list?limit=200').catch(() => ({ items: [] })),
      ]);
      state.linkOptions = {
        assets: assetsRes.items || [],
        software: softwareRes.items || [],
        controls: controlsRes.items || [],
        incidents: incRes.items || [],
        tasks: tasksRes.items || [],
        docs: docsRes.items || []
      };
    } finally {
      state.linkOptionsLoaded = true;
    }
  }

  function refreshLinkTargets() {
    const type = document.getElementById('finding-link-target-type')?.value || '';
    const select = document.getElementById('finding-link-target-id');
    const search = (document.getElementById('finding-link-search')?.value || '').toLowerCase().trim();
    if (!select) return;
    select.innerHTML = '';
    const placeholder = document.createElement('option');
    placeholder.value = '';
    placeholder.textContent = t('findings.links.targetPlaceholder');
    select.appendChild(placeholder);
    if (!type) return;
    ensureLinkOptions().then(() => {
      let items = [];
      if (type === 'asset') items = state.linkOptions.assets || [];
      if (type === 'software') items = state.linkOptions.software || [];
      if (type === 'control') items = state.linkOptions.controls || [];
      if (type === 'incident') items = state.linkOptions.incidents || [];
      if (type === 'task') items = state.linkOptions.tasks || [];
      if (type === 'doc') items = state.linkOptions.docs || [];
      items
        .filter(item => linkOptionLabel(type, item).toLowerCase().includes(search))
        .forEach(item => {
          const opt = document.createElement('option');
          opt.value = linkOptionValue(type, item);
          opt.textContent = linkOptionLabel(type, item);
          select.appendChild(opt);
        });
    });
  }

  function linkOptionValue(type, item) {
    if (!item) return '';
    if (type === 'incident') return item.reg_no || item.regNo || item.id || '';
    return item.id || '';
  }

  function linkOptionLabel(type, item) {
    if (!item) return '';
    if (type === 'asset') {
      const typeLabel = item.type ? (t(`assets.type.${item.type}`) || item.type) : '';
      const suffix = typeLabel ? ` (${typeLabel})` : '';
      return `#${item.id} ${item.name || ''}${suffix}`.trim();
    }
    if (type === 'software') {
      const vendor = item.vendor ? ` (${item.vendor})` : '';
      return `#${item.id} ${item.name || ''}${vendor}`.trim();
    }
    if (type === 'control') {
      const code = item.code ? `${item.code} - ` : '';
      return `${code}${item.title || ''}`.trim();
    }
    if (type === 'incident') {
      const reg = item.reg_no || item.regNo || `#${item.id}`;
      return `${reg} - ${item.title || ''}`.trim();
    }
    if (type === 'task') return `#${item.id} ${item.title || ''}`.trim();
    const reg = item.reg_no ? `${item.reg_no} ` : '';
    return `${reg}${item.title || ''}`.trim();
  }

  async function addLink() {
    const findingId = document.getElementById('finding-id')?.value || '';
    if (!findingId || !hasPerm('findings.manage')) return;
    const targetType = document.getElementById('finding-link-target-type')?.value || '';
    const targetId = document.getElementById('finding-link-target-id')?.value || '';
    const relationType = document.getElementById('finding-link-relation')?.value || '';
    const alert = document.getElementById('finding-modal-alert');
    if (!targetType || !targetId) {
      showAlert(alert, t('findings.links.required'));
      return;
    }
    try {
      await Api.post(`/api/findings/${findingId}/links`, {
        target_type: targetType,
        target_id: targetId.trim(),
        relation_type: relationType
      });
      document.getElementById('finding-link-target-id').value = '';
      document.getElementById('finding-link-search').value = '';
      await loadLinks(findingId);
    } catch (err) {
      showAlert(alert, localizeError(err));
    }
  }

  async function deleteLink(linkId) {
    const findingId = document.getElementById('finding-id')?.value || '';
    if (!findingId || !linkId) return;
    if (!window.confirm(t('common.confirm'))) return;
    try {
      await Api.del(`/api/findings/${findingId}/links/${linkId}`);
      await loadLinks(findingId);
    } catch (err) {
      const alert = document.getElementById('finding-modal-alert');
      showAlert(alert, localizeError(err));
    }
  }

  function linkHref(type, id) {
    if (!type || !id) return '';
    const tt = String(type).toLowerCase();
    if (tt === 'asset') return `/assets?asset=${encodeURIComponent(id)}`;
    if (tt === 'software') return `/software?software=${encodeURIComponent(id)}`;
    if (tt === 'control') return `/registry/controls?control=${encodeURIComponent(id)}`;
    if (tt === 'incident') return `/incidents?incident=${encodeURIComponent(id)}`;
    if (tt === 'task') return `/tasks/task/${encodeURIComponent(id)}`;
    if (tt === 'doc') return `/docs/${encodeURIComponent(id)}`;
    return '';
  }

  function linkTypeLabel(val) {
    const map = {
      asset: t('findings.links.type.asset'),
      software: t('findings.links.type.software'),
      control: t('findings.links.type.control'),
      incident: t('findings.links.type.incident'),
      task: t('findings.links.type.task'),
      doc: t('findings.links.type.doc')
    };
    return map[val] || val || '-';
  }

  function relationLabel(val) {
    const map = {
      related: t('findings.links.relation.related'),
      affects: t('findings.links.relation.affects'),
      evidence: t('findings.links.relation.evidence')
    };
    return map[val] || val || '-';
  }

  function statusLabel(val) {
    const map = {
      open: t('findings.status.open'),
      in_progress: t('findings.status.in_progress'),
      resolved: t('findings.status.resolved'),
      accepted_risk: t('findings.status.accepted_risk'),
      false_positive: t('findings.status.false_positive')
    };
    return map[val] || val || '-';
  }

  function severityLabel(val) {
    const map = {
      low: t('findings.severity.low'),
      medium: t('findings.severity.medium'),
      high: t('findings.severity.high'),
      critical: t('findings.severity.critical')
    };
    return map[val] || val || '-';
  }

  function typeLabel(val) {
    const map = {
      technical: t('findings.type.technical'),
      config: t('findings.type.config'),
      process: t('findings.type.process'),
      compliance: t('findings.type.compliance'),
      other: t('findings.type.other')
    };
    return map[val] || val || '-';
  }

  function localizeError(err) {
    const raw = (err && err.message ? err.message : '').trim();
    const msg = raw ? t(raw) : t('common.error');
    return msg || raw || 'error';
  }

  function showAlert(el, msg) {
    if (!el) return;
    el.textContent = msg || '';
    el.hidden = !msg;
  }

  function escapeHtml(str) {
    return (str || '').toString().replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  }

  function setVal(id, val) {
    const el = document.getElementById(id);
    if (el) el.value = val;
  }

  function splitList(raw) {
    return (raw || '').split(',').map(x => x.trim()).filter(Boolean).map(x => x.toUpperCase());
  }

  function formatDateInput(raw) {
    try {
      const dt = new Date(raw);
      const pad = (n) => String(n).padStart(2, '0');
      return `${dt.getFullYear()}-${pad(dt.getMonth() + 1)}-${pad(dt.getDate())}`;
    } catch (_) {
      return '';
    }
  }

  function dateToISO(val) {
    const s = String(val || '').trim();
    if (!s) return null;
    try {
      const dt = new Date(`${s}T00:00:00Z`);
      return dt.toISOString();
    } catch (_) {
      return null;
    }
  }

  function formatDate(raw) {
    if (!raw) return '-';
    try {
      if (typeof AppTime !== 'undefined' && AppTime.formatDate) return AppTime.formatDate(raw);
      const dt = new Date(raw);
      const pad = (n) => String(n).padStart(2, '0');
      return `${pad(dt.getDate())}.${pad(dt.getMonth() + 1)}.${dt.getFullYear()}`;
    } catch (_) {
      return raw;
    }
  }

  function formatDateTime(raw) {
    if (!raw) return '-';
    try {
      if (typeof AppTime !== 'undefined' && AppTime.formatDateTime) return AppTime.formatDateTime(raw);
      const dt = new Date(raw);
      const pad = (n) => String(n).padStart(2, '0');
      return `${pad(dt.getDate())}.${pad(dt.getMonth() + 1)}.${dt.getFullYear()} ${pad(dt.getHours())}:${pad(dt.getMinutes())}`;
    } catch (_) {
      return raw;
    }
  }

  return { init, openFinding };
})();
