const SoftwarePage = (() => {
  const state = {
    items: [],
    canManage: false,
    initialized: false,
    pendingOpenId: null,
    openProduct: null,
    tagsBound: false,
  };

  function t(key) {
    return (typeof BerkutI18n !== 'undefined' && BerkutI18n.t) ? BerkutI18n.t(key) : key;
  }

  function escapeHTML(str) {
    return (str || '').toString().replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  }

  function localizeError(err) {
    const raw = (err && err.message ? err.message : '').trim();
    const msg = raw ? t(raw) : t('common.error');
    return msg || raw || 'error';
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

  function setSoftwareTags(selected) {
    const select = document.getElementById('software-tags');
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
    const hint = document.querySelector('[data-tag-hint="software-tags"]');
    if (typeof DocUI !== 'undefined' && DocUI.bindTagHint) {
      DocUI.bindTagHint(select, hint);
    }
  }

  async function loadPermissions() {
    const me = await Api.get('/api/auth/me');
    const perms = (me && me.user && Array.isArray(me.user.permissions)) ? me.user.permissions : [];
    state.canManage = perms.includes('software.manage');
  }

  function readFilters() {
    const q = (document.getElementById('software-filter-q')?.value || '').trim();
    const vendor = (document.getElementById('software-filter-vendor')?.value || '').trim();
    const tag = (document.getElementById('software-filter-tag')?.value || '').trim();
    const includeDeletedRaw = document.getElementById('software-filter-include-deleted')?.value || '0';
    const includeDeleted = state.canManage && (includeDeletedRaw === '1' || includeDeletedRaw === 'true');
    return { q, vendor, tag, includeDeleted };
  }

  function buildListURL(limit) {
    const f = readFilters();
    const url = new URL('/api/software', window.location.origin);
    if (f.q) url.searchParams.set('q', f.q);
    if (f.vendor) url.searchParams.set('vendor', f.vendor);
    if (f.tag) url.searchParams.set('tag', f.tag);
    if (f.includeDeleted) url.searchParams.set('include_deleted', '1');
    url.searchParams.set('limit', String(limit || 200));
    return url.pathname + url.search;
  }

  async function refresh() {
    const empty = document.getElementById('software-empty');
    if (empty) empty.hidden = true;
    const tbody = document.querySelector('#software-table tbody');
    if (!tbody) return;
    tbody.innerHTML = '';

    let data;
    try {
      data = await Api.get(buildListURL(200));
    } catch (err) {
      state.items = [];
      if (empty) empty.hidden = false;
      return;
    }
    state.items = (data && Array.isArray(data.items)) ? data.items : [];
    if (state.items.length === 0) {
      if (empty) empty.hidden = false;
      return;
    }

    state.items.forEach(item => {
      const tr = document.createElement('tr');
      const tags = tagsText(item.tags);
      const updated = item.updated_at && window.AppTime?.formatDateTime ? AppTime.formatDateTime(item.updated_at) : (item.updated_at || '');
      tr.innerHTML = `
        <td>${escapeHTML(item.name || '')}${item.deleted_at ? ` <span class="muted">(${escapeHTML(t('common.archived'))})</span>` : ''}</td>
        <td>${escapeHTML(item.vendor || '-')}</td>
        <td>${escapeHTML(tags || '-')}</td>
        <td>${escapeHTML(updated || '-')}</td>
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
    viewBtn.textContent = t('software.actions.view');
    viewBtn.addEventListener('click', () => openEditModal(item.id, true));
    wrap.appendChild(viewBtn);

    if (!state.canManage) return wrap;

    if (item.deleted_at) {
      const restoreBtn = document.createElement('button');
      restoreBtn.className = 'btn ghost btn-xs';
      restoreBtn.textContent = t('software.actions.restore');
      restoreBtn.addEventListener('click', async () => {
        if (!confirm(t('software.confirm.restore'))) return;
        await Api.post(`/api/software/${item.id}/restore`, {});
        await refresh();
      });
      wrap.appendChild(restoreBtn);
      return wrap;
    }

    const editBtn = document.createElement('button');
    editBtn.className = 'btn ghost btn-xs';
    editBtn.textContent = t('software.actions.edit');
    editBtn.addEventListener('click', () => openEditModal(item.id, false));
    wrap.appendChild(editBtn);

    const delBtn = document.createElement('button');
    delBtn.className = 'btn ghost btn-xs danger';
    delBtn.textContent = t('software.actions.archive');
    delBtn.addEventListener('click', async () => {
      if (!confirm(t('software.confirm.archive'))) return;
      await Api.del(`/api/software/${item.id}`);
      await refresh();
    });
    wrap.appendChild(delBtn);
    return wrap;
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

  async function openEditModal(id, readOnly) {
    const modal = document.getElementById('software-modal');
    if (!modal) return;
    const alert = document.getElementById('software-modal-alert');
    setAlert(alert, '');

    const isNew = !id;
    const viewOnly = readOnly || !state.canManage;
    const titleEl = document.getElementById('software-modal-title');
    if (titleEl) titleEl.textContent = isNew ? t('software.modal.createTitle') : (viewOnly ? t('software.modal.viewTitle') : t('software.modal.editTitle'));

    let item = null;
    if (!isNew) {
      try {
        item = await Api.get(`/api/software/${id}`);
      } catch (err) {
        setAlert(alert, localizeError(err));
        return;
      }
    }
    state.openProduct = item;
    fillProductForm(item);
    applyProductFormDisabled(viewOnly);
    configureProductButtons(item, viewOnly);

    const versionsSection = document.getElementById('software-versions-section');
    const assetsSection = document.getElementById('software-assets-section');
    if (versionsSection) versionsSection.hidden = isNew;
    if (assetsSection) assetsSection.hidden = isNew;
    if (!isNew && typeof SoftwareDetail !== 'undefined') {
      SoftwareDetail.loadForProduct(id, { canManage: state.canManage, readOnly: viewOnly });
    }
    openModal('#software-modal');
  }

  function fillProductForm(item) {
    document.getElementById('software-id').value = item ? (item.id || '') : '';
    document.getElementById('software-name').value = item ? (item.name || '') : '';
    document.getElementById('software-vendor').value = item ? (item.vendor || '') : '';
    setSoftwareTags(item ? (item.tags || []) : []);
    document.getElementById('software-description').value = item ? (item.description || '') : '';
    document.getElementById('software-form').dataset.version = String(item ? (item.version || 1) : 0);
  }

  function applyProductFormDisabled(disabled) {
    ['software-name', 'software-vendor', 'software-tags', 'software-description'].forEach(id => {
      const el = document.getElementById(id);
      if (el) el.disabled = !!disabled;
    });
    const saveBtn = document.getElementById('software-save');
    if (saveBtn) saveBtn.hidden = !!disabled;
  }

  function configureProductButtons(item, viewOnly) {
    const archiveBtn = document.getElementById('software-archive');
    if (!archiveBtn) return;
    if (!state.canManage || viewOnly || !item) {
      archiveBtn.hidden = true;
      return;
    }
    archiveBtn.hidden = false;
    if (item.deleted_at) {
      archiveBtn.textContent = t('software.actions.restore');
      archiveBtn.classList.remove('danger');
      archiveBtn.onclick = async () => {
        if (!confirm(t('software.confirm.restore'))) return;
        try {
          await Api.post(`/api/software/${item.id}/restore`, {});
          closeModal('#software-modal');
          await refresh();
        } catch (err) {
          setAlert(document.getElementById('software-modal-alert'), localizeError(err));
        }
      };
    } else {
      archiveBtn.textContent = t('software.actions.archive');
      archiveBtn.classList.add('danger');
      archiveBtn.onclick = async () => {
        if (!confirm(t('software.confirm.archive'))) return;
        try {
          await Api.del(`/api/software/${item.id}`);
          closeModal('#software-modal');
          await refresh();
        } catch (err) {
          setAlert(document.getElementById('software-modal-alert'), localizeError(err));
        }
      };
    }
  }

  async function saveFromModal() {
    if (!state.canManage) return;
    const alert = document.getElementById('software-modal-alert');
    setAlert(alert, '');
    const id = (document.getElementById('software-id')?.value || '').trim();
    const payload = {
      name: (document.getElementById('software-name')?.value || '').trim(),
      vendor: (document.getElementById('software-vendor')?.value || '').trim(),
      description: (document.getElementById('software-description')?.value || '').trim(),
      tags: selectedValues('software-tags'),
      version: parseInt(document.getElementById('software-form')?.dataset.version || '0', 10) || 0,
    };
    if (!payload.name) {
      setAlert(alert, t('software.error.nameRequired'));
      return;
    }
    try {
      const res = id
        ? await Api.put(`/api/software/${id}`, payload)
        : await Api.post('/api/software', payload);
      closeModal('#software-modal');
      await refresh();
      if (res && res.id) {
        state.pendingOpenId = res.id;
      }
    } catch (err) {
      setAlert(alert, localizeError(err));
    }
  }

  async function init() {
    const page = document.getElementById('software-page');
    if (!page || state.initialized) return;
    state.initialized = true;
    state.pendingOpenId = parseInt(new URLSearchParams(window.location.search).get('software') || '', 10) || null;
    await loadPermissions();

    const createBtn = document.getElementById('software-create');
    if (createBtn) createBtn.hidden = !state.canManage;
    if (typeof RegistryReports !== 'undefined' && RegistryReports.bind) {
      RegistryReports.bind('software-create-report', 'software');
    }
    const includeField = document.getElementById('software-include-deleted-field');
    if (includeField) includeField.hidden = !state.canManage;

    document.getElementById('software-apply')?.addEventListener('click', () => refresh());
    document.getElementById('software-create')?.addEventListener('click', () => openEditModal(null, false));
    document.getElementById('software-save')?.addEventListener('click', () => saveFromModal());

    if (!state.tagsBound) {
      state.tagsBound = true;
      document.addEventListener('tags:changed', () => setSoftwareTags(selectedValues('software-tags')));
    }

    if (typeof SoftwareDetail !== 'undefined') {
      SoftwareDetail.bindUI({
        t,
        localizeError,
        canManage: () => state.canManage,
        getOpenProductId: () => parseInt(document.getElementById('software-id')?.value || '', 10) || 0,
      });
    }

    await refresh();
    if (state.pendingOpenId) {
      const id = state.pendingOpenId;
      state.pendingOpenId = null;
      await openEditModal(id, true);
    }
  }

  return { init, openEditModal };
})();
