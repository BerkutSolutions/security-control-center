const AssetsPage = (() => {
  const state = {
    items: [],
    canManage: false,
    initialized: false,
    tagsBound: false,
  };

  function t(key) {
    return (typeof BerkutI18n !== 'undefined' && BerkutI18n.t) ? BerkutI18n.t(key) : key;
  }

  function localizeError(err) {
    const raw = (err && err.message ? err.message : '').trim();
    const msg = raw ? t(raw) : t('common.error');
    return msg || raw || 'error';
  }

  function openModal(selector) {
    const modal = document.querySelector(selector);
    if (modal) modal.hidden = false;
  }

  function closeModal(selector) {
    const modal = document.querySelector(selector);
    if (modal) modal.hidden = true;
  }

  function setAlert(el, msg) {
    if (!el) return;
    el.textContent = msg || '';
    el.hidden = !msg;
  }

  function parseCSV(value) {
    return (value || '')
      .toString()
      .split(',')
      .map(s => s.trim())
      .filter(Boolean);
  }

  function selectedValues(selectId) {
    const el = document.getElementById(selectId);
    return Array.from(el?.selectedOptions || []).map(o => o.value);
  }

  function tagsText(tags) {
    const arr = Array.isArray(tags) ? tags : [];
    if (!arr.length) return '';
    return arr
      .map(t => ((typeof DocUI !== 'undefined' && DocUI.tagLabel) ? DocUI.tagLabel(t) : t))
      .filter(Boolean)
      .join(', ');
  }

  function setAssetTags(selected) {
    const select = document.getElementById('asset-tags');
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
    const hint = document.querySelector('[data-tag-hint="asset-tags"]');
    if (typeof DocUI !== 'undefined' && DocUI.bindTagHint) {
      DocUI.bindTagHint(select, hint);
    }
  }

  function labelType(v) {
    const k = `assets.type.${(v || 'other').toString().toLowerCase()}`;
    const msg = t(k);
    return msg && msg !== k ? msg : (v || 'other');
  }

  function labelCriticality(v) {
    const k = `assets.criticality.${(v || 'medium').toString().toLowerCase()}`;
    const msg = t(k);
    return msg && msg !== k ? msg : (v || 'medium');
  }

  function labelEnv(v) {
    const k = `assets.env.${(v || 'other').toString().toLowerCase()}`;
    const msg = t(k);
    return msg && msg !== k ? msg : (v || 'other');
  }

  function labelStatus(v) {
    const k = `assets.status.${(v || 'active').toString().toLowerCase()}`;
    const msg = t(k);
    return msg && msg !== k ? msg : (v || 'active');
  }

  async function loadPermissions() {
    const me = await Api.get('/api/auth/me');
    const perms = (me && me.user && Array.isArray(me.user.permissions)) ? me.user.permissions : [];
    state.canManage = perms.includes('assets.manage');
  }

  function readFilters() {
    const q = document.getElementById('assets-filter-q')?.value || '';
    const type = document.getElementById('assets-filter-type')?.value || '';
    const criticality = document.getElementById('assets-filter-criticality')?.value || '';
    const env = document.getElementById('assets-filter-env')?.value || '';
    const status = document.getElementById('assets-filter-status')?.value || '';
    const tag = document.getElementById('assets-filter-tag')?.value || '';
    const includeDeletedRaw = document.getElementById('assets-filter-include-deleted')?.value || '0';
    const includeDeleted = state.canManage && (includeDeletedRaw === '1' || includeDeletedRaw === 'true');
    return { q, type, criticality, env, status, tag, includeDeleted };
  }

  function buildListURL() {
    const f = readFilters();
    const url = new URL('/api/assets', window.location.origin);
    if (f.q) url.searchParams.set('q', f.q);
    if (f.type) url.searchParams.set('type', f.type);
    if (f.criticality) url.searchParams.set('criticality', f.criticality);
    if (f.env) url.searchParams.set('env', f.env);
    if (f.status) url.searchParams.set('status', f.status);
    if (f.tag) url.searchParams.set('tag', f.tag);
    if (f.includeDeleted) url.searchParams.set('include_deleted', '1');
    url.searchParams.set('limit', '200');
    return url.pathname + url.search;
  }

  async function refresh() {
    const empty = document.getElementById('assets-empty');
    if (empty) empty.hidden = true;
    const tbody = document.querySelector('#assets-table tbody');
    if (!tbody) return;
    tbody.innerHTML = '';

    const data = await Api.get(buildListURL());
    state.items = (data && Array.isArray(data.items)) ? data.items : [];

    if (state.items.length === 0) {
      if (empty) empty.hidden = false;
      return;
    }
    state.items.forEach(item => {
      const tr = document.createElement('tr');
      const ips = Array.isArray(item.ip_addresses) ? item.ip_addresses.join(', ') : '';
      const tags = tagsText(item.tags);
      tr.innerHTML = `
        <td>${escapeHTML(item.name || '')}</td>
        <td>${escapeHTML(labelType(item.type))}</td>
        <td>${escapeHTML(labelCriticality(item.criticality))}</td>
        <td>${escapeHTML(labelEnv(item.env))}</td>
        <td>${escapeHTML(labelStatus(item.status))}</td>
        <td class="mono">${escapeHTML(ips || '-')}</td>
        <td>${escapeHTML(item.owner || '-')}</td>
        <td>${escapeHTML(item.administrator || '-')}</td>
        <td>${escapeHTML(tags || '-')}</td>
        <td></td>
      `;
      const actionsCell = tr.querySelector('td:last-child');
      if (actionsCell) actionsCell.appendChild(renderRowActions(item));
      tbody.appendChild(tr);
    });
  }

  function renderRowActions(item) {
    const wrap = document.createElement('div');
    wrap.className = 'btn-group';

    const viewBtn = document.createElement('button');
    viewBtn.className = 'btn ghost btn-xs';
    viewBtn.textContent = t('assets.actions.view') || 'View';
    viewBtn.addEventListener('click', () => openEditModal(item, true));
    wrap.appendChild(viewBtn);

    if (!state.canManage) return wrap;

    if (item.deleted_at) {
      const restoreBtn = document.createElement('button');
      restoreBtn.className = 'btn ghost btn-xs';
      restoreBtn.textContent = t('assets.actions.restore') || 'Restore';
      restoreBtn.addEventListener('click', async () => {
        if (!confirm(t('assets.confirm.restore'))) return;
        await Api.post(`/api/assets/${item.id}/restore`, {});
        await refresh();
      });
      wrap.appendChild(restoreBtn);
      return wrap;
    }

    const editBtn = document.createElement('button');
    editBtn.className = 'btn ghost btn-xs';
    editBtn.textContent = t('assets.actions.edit') || 'Edit';
    editBtn.addEventListener('click', () => openEditModal(item, false));
    wrap.appendChild(editBtn);

    const delBtn = document.createElement('button');
    delBtn.className = 'btn ghost btn-xs danger';
    delBtn.textContent = t('assets.actions.archive') || 'Archive';
    delBtn.addEventListener('click', async () => {
      if (!confirm(t('assets.confirm.archive'))) return;
      await Api.del(`/api/assets/${item.id}`);
      await refresh();
    });
    wrap.appendChild(delBtn);
    return wrap;
  }

  function openCreateModal() {
    openEditModal(null, false);
  }

  function openEditModal(item, readOnly) {
    const alert = document.getElementById('asset-modal-alert');
    setAlert(alert, '');

    const title = document.getElementById('asset-modal-title');
    const isNew = !item;
    if (title) {
      title.textContent = isNew ? t('assets.modal.createTitle') : (readOnly ? t('assets.modal.viewTitle') : t('assets.modal.editTitle'));
    }

    setValue('asset-id', item ? item.id : '');
    setValue('asset-name', item ? item.name : '');
    setValue('asset-type', item ? (item.type || 'other') : 'other');
    setValue('asset-criticality', item ? (item.criticality || 'medium') : 'medium');
    setValue('asset-env', item ? (item.env || 'other') : 'other');
    setValue('asset-status', item ? (item.status || 'active') : 'active');
    setValue('asset-description', item ? (item.description || '') : '');
    setValue('asset-owner', item ? (item.owner || '') : '');
    setValue('asset-admin', item ? (item.administrator || '') : '');
    setAssetTags(item ? (item.tags || []) : []);
    setValue('asset-ips', item ? (Array.isArray(item.ip_addresses) ? item.ip_addresses.join(', ') : '') : '');
    setValue('asset-commissioned-at', item && item.commissioned_at ? (item.commissioned_at || '').slice(0, 10) : '');

    const inputs = document.querySelectorAll('#asset-form input, #asset-form select, #asset-form textarea');
    inputs.forEach(el => {
      if (!el) return;
      const isId = el.id === 'asset-id';
      el.disabled = readOnly || (!state.canManage && !isId);
    });
    const saveBtn = document.getElementById('asset-save');
    if (saveBtn) saveBtn.hidden = readOnly || !state.canManage;

    if (typeof AssetsSoftware !== 'undefined' && AssetsSoftware.onAssetModalOpened) {
      AssetsSoftware.onAssetModalOpened({ asset: item, readOnly: !!readOnly, canManage: !!state.canManage });
    }

    openModal('#asset-modal');
  }

  function setValue(id, value) {
    const el = document.getElementById(id);
    if (!el) return;
    el.value = value == null ? '' : value;
  }

  async function saveFromModal() {
    const alert = document.getElementById('asset-modal-alert');
    setAlert(alert, '');

    const idRaw = (document.getElementById('asset-id')?.value || '').toString().trim();
    const payload = {
      name: (document.getElementById('asset-name')?.value || '').toString(),
      type: (document.getElementById('asset-type')?.value || '').toString(),
      description: (document.getElementById('asset-description')?.value || '').toString(),
      commissioned_at: (document.getElementById('asset-commissioned-at')?.value || '').toString(),
      ip_addresses: parseCSV(document.getElementById('asset-ips')?.value || ''),
      criticality: (document.getElementById('asset-criticality')?.value || '').toString(),
      owner: (document.getElementById('asset-owner')?.value || '').toString(),
      administrator: (document.getElementById('asset-admin')?.value || '').toString(),
      env: (document.getElementById('asset-env')?.value || '').toString(),
      status: (document.getElementById('asset-status')?.value || '').toString(),
      tags: selectedValues('asset-tags'),
    };
    try {
      if (idRaw) {
        await Api.put(`/api/assets/${encodeURIComponent(idRaw)}`, payload);
      } else {
        await Api.post('/api/assets', payload);
      }
      closeModal('#asset-modal');
      await refresh();
    } catch (err) {
      setAlert(alert, localizeError(err));
    }
  }

  function escapeHTML(str) {
    const s = (str == null ? '' : String(str));
    return s
      .replaceAll('&', '&amp;')
      .replaceAll('<', '&lt;')
      .replaceAll('>', '&gt;')
      .replaceAll('"', '&quot;')
      .replaceAll("'", '&#39;');
  }

  function wireEvents() {
    document.getElementById('assets-apply')?.addEventListener('click', async (e) => {
      e.preventDefault();
      await refresh();
    });
    document.getElementById('assets-create')?.addEventListener('click', (e) => {
      e.preventDefault();
      openCreateModal();
    });
    document.getElementById('asset-save')?.addEventListener('click', async (e) => {
      e.preventDefault();
      await saveFromModal();
    });
    document.getElementById('asset-form')?.addEventListener('submit', async (e) => {
      e.preventDefault();
      await saveFromModal();
    });
    document.querySelectorAll('[data-close="#asset-modal"]').forEach(btn => {
      btn.addEventListener('click', (e) => {
        e.preventDefault();
        closeModal('#asset-modal');
      });
    });
  }

  async function init() {
    if (state.initialized) {
      await refresh();
      return;
    }
    state.initialized = true;
    try {
      await loadPermissions();
    } catch (_) {
      state.canManage = false;
    }
    const createBtn = document.getElementById('assets-create');
    if (createBtn) createBtn.hidden = !state.canManage;
    if (typeof RegistryReports !== 'undefined' && RegistryReports.bind) {
      RegistryReports.bind('assets-create-report', 'assets');
    }
    const includeDeletedField = document.getElementById('assets-include-deleted-field');
    if (includeDeletedField) includeDeletedField.hidden = !state.canManage;
    wireEvents();
    if (!state.tagsBound) {
      state.tagsBound = true;
      document.addEventListener('tags:changed', () => setAssetTags(selectedValues('asset-tags')));
    }
    await refresh();

    const pending = window.__pendingAssetOpen;
    if (pending) {
      window.__pendingAssetOpen = null;
      await openAsset(pending, 'view');
      return;
    }
    try {
      const url = new URL(window.location.href);
      const assetId = parseInt(url.searchParams.get('asset') || '', 10);
      if (assetId) {
        await openAsset(assetId, 'view');
      }
    } catch (_) {
      // ignore
    }
  }

  async function openAsset(assetId, mode) {
    const id = parseInt(assetId, 10);
    if (!id) return;
    try {
      const item = await Api.get(`/api/assets/${id}`);
      openEditModal(item, mode === 'view');
    } catch (err) {
      alert(localizeError(err));
    }
  }

  return { init, openAsset };
})();
