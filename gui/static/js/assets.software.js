const AssetsSoftware = (() => {
  const state = {
    assetId: 0,
    items: [],
    canManage: false,
    readOnly: true,
    bound: false,
  };

  function t(key) {
    return (typeof BerkutI18n !== 'undefined' && BerkutI18n.t) ? BerkutI18n.t(key) : key;
  }

  function escapeHTML(str) {
    return (str || '').toString().replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  }

  function localizeError(err, fallbackKey) {
    const raw = String((err && err.message) || '').trim();
    if (raw && typeof BerkutI18n !== 'undefined' && typeof BerkutI18n.t === 'function') {
      const translated = BerkutI18n.t(raw);
      if (translated && translated !== raw) return translated;
    }
    if (fallbackKey && typeof BerkutI18n !== 'undefined' && typeof BerkutI18n.t === 'function') {
      return BerkutI18n.t(fallbackKey);
    }
    return raw || fallbackKey || 'Error';
  }

  function fmtDate(raw) {
    if (!raw) return '-';
    if (window.AppTime?.formatDate) return AppTime.formatDate(raw);
    return String(raw).slice(0, 10);
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

  function includeDeleted() {
    if (!state.canManage || state.readOnly) return false;
    const raw = document.getElementById('asset-software-include-deleted')?.value || '0';
    return raw === '1' || raw === 'true';
  }

  async function load() {
    const tbody = document.querySelector('#asset-software-table tbody');
    const empty = document.getElementById('asset-software-empty');
    if (tbody) tbody.innerHTML = '';
    if (empty) empty.hidden = true;
    if (!state.assetId) return;

    let res;
    try {
      res = await Api.get(`/api/assets/${state.assetId}/software?include_deleted=${includeDeleted() ? '1' : '0'}`);
    } catch (err) {
      state.items = [];
      if (empty) empty.hidden = false;
      return;
    }
    state.items = Array.isArray(res?.items) ? res.items : [];
    render();
  }

  function render() {
    const tbody = document.querySelector('#asset-software-table tbody');
    const empty = document.getElementById('asset-software-empty');
    if (!tbody) return;
    tbody.innerHTML = '';
    if (!state.items.length) {
      if (empty) empty.hidden = false;
      return;
    }
    if (empty) empty.hidden = true;
    state.items.forEach(inst => {
      const tr = document.createElement('tr');
      const product = inst.product_name ? `${inst.product_name}${inst.product_vendor ? ` (${inst.product_vendor})` : ''}` : `#${inst.product_id || ''}`;
      tr.innerHTML = `
        <td>${escapeHTML(product)}${inst.deleted_at ? ` <span class="muted">(${escapeHTML(t('common.archived'))})</span>` : ''}</td>
        <td>${escapeHTML(inst.version_label || inst.version_text || '-')}</td>
        <td>${escapeHTML(inst.installed_at ? fmtDate(inst.installed_at) : '-')}</td>
        <td>${escapeHTML(inst.source || '-')}</td>
        <td>${inst.notes ? escapeHTML(inst.notes) : '<span class="meta-empty">-</span>'}</td>
        <td class="actions"></td>
      `;
      const actions = tr.querySelector('td.actions');
      if (actions) actions.appendChild(renderActions(inst));
      tbody.appendChild(tr);
    });
  }

  function renderActions(inst) {
    const wrap = document.createElement('div');
    wrap.className = 'btn-group';

    const viewBtn = document.createElement('button');
    viewBtn.className = 'btn ghost btn-xs';
    viewBtn.textContent = t('assets.software.actions.view');
    viewBtn.onclick = () => openInstallModal(inst, true);
    wrap.appendChild(viewBtn);

    if (!state.canManage || state.readOnly) return wrap;

    if (inst.deleted_at) {
      const restoreBtn = document.createElement('button');
      restoreBtn.className = 'btn ghost btn-xs';
      restoreBtn.textContent = t('assets.software.actions.restore');
      restoreBtn.onclick = async () => {
        if (!confirm(t('assets.software.confirm.restore'))) return;
        try {
          await Api.post(`/api/assets/${state.assetId}/software/${inst.id}/restore`, {});
          await load();
        } catch (err) {
          alert(localizeError(err));
        }
      };
      wrap.appendChild(restoreBtn);
      return wrap;
    }

    const editBtn = document.createElement('button');
    editBtn.className = 'btn ghost btn-xs';
    editBtn.textContent = t('assets.software.actions.edit');
    editBtn.onclick = () => openInstallModal(inst, false);
    wrap.appendChild(editBtn);

    const archiveBtn = document.createElement('button');
    archiveBtn.className = 'btn ghost btn-xs danger';
    archiveBtn.textContent = t('assets.software.actions.archive');
    archiveBtn.onclick = async () => {
      if (!confirm(t('assets.software.confirm.archive'))) return;
      try {
        await Api.del(`/api/assets/${state.assetId}/software/${inst.id}`);
        await load();
      } catch (err) {
        alert(localizeError(err));
      }
    };
    wrap.appendChild(archiveBtn);
    return wrap;
  }

  async function refreshProductOptions(search) {
    const select = document.getElementById('asset-software-product-select');
    if (!select) return;
    select.innerHTML = '';
    const placeholder = document.createElement('option');
    placeholder.value = '';
    placeholder.textContent = t('assets.software.product.placeholder');
    select.appendChild(placeholder);
    let res;
    try {
      res = await Api.get(`/api/software/list?limit=50&q=${encodeURIComponent(search || '')}`);
    } catch (_) {
      return;
    }
    (res.items || []).forEach(p => {
      const opt = document.createElement('option');
      opt.value = String(p.id);
      opt.textContent = p.vendor ? `${p.name} (${p.vendor})` : p.name;
      select.appendChild(opt);
    });
  }

  async function refreshVersionOptions(productID) {
    const select = document.getElementById('asset-software-version-id');
    if (!select) return;
    select.innerHTML = '';
    const other = document.createElement('option');
    other.value = '0';
    other.textContent = t('assets.software.version.other');
    select.appendChild(other);
    if (!productID) return;
    let res;
    try {
      res = await Api.get(`/api/software/${productID}/versions?include_deleted=0`);
    } catch (_) {
      return;
    }
    (res.items || []).forEach(v => {
      if (v.deleted_at) return;
      const opt = document.createElement('option');
      opt.value = String(v.id);
      opt.textContent = v.version || '';
      select.appendChild(opt);
    });
  }

  function syncVersionTextVisibility() {
    const sel = document.getElementById('asset-software-version-id');
    const field = document.getElementById('asset-software-version-text-field');
    const text = document.getElementById('asset-software-version-text');
    if (!sel || !field || !text) return;
    const val = parseInt(sel.value || '0', 10) || 0;
    const useText = val <= 0;
    field.hidden = !useText;
    if (!useText) text.value = '';
  }

  function openInstallModal(inst, readOnly) {
    const alert = document.getElementById('asset-software-modal-alert');
    setAlert(alert, '');
    const title = document.getElementById('asset-software-modal-title');
    const isNew = !inst;
    const viewOnly = !!readOnly || !state.canManage || state.readOnly;
    if (title) title.textContent = isNew ? t('assets.software.modal.createTitle') : (viewOnly ? t('assets.software.modal.viewTitle') : t('assets.software.modal.editTitle'));

    document.getElementById('asset-software-inst-id').value = inst ? (inst.id || '') : '';
    document.getElementById('asset-software-product-id').value = inst ? String(inst.product_id || '') : '';

    const picker = document.getElementById('asset-software-product-picker');
    const label = document.getElementById('asset-software-product-label');
    if (inst) {
      if (picker) picker.hidden = true;
      if (label) label.hidden = false;
      const text = document.getElementById('asset-software-product-text');
      const product = inst.product_name ? `${inst.product_name}${inst.product_vendor ? ` (${inst.product_vendor})` : ''}` : `#${inst.product_id || ''}`;
      if (text) text.textContent = product;
    } else {
      if (picker) picker.hidden = false;
      if (label) label.hidden = true;
      document.getElementById('asset-software-product-q').value = '';
      refreshProductOptions('');
    }

    document.getElementById('asset-software-version-text').value = inst ? (inst.version_text || '') : '';
    document.getElementById('asset-software-installed-at').value = inst && inst.installed_at ? String(inst.installed_at).slice(0, 10) : '';
    document.getElementById('asset-software-source').value = inst ? (inst.source || '') : '';
    document.getElementById('asset-software-notes').value = inst ? (inst.notes || '') : '';

    const productID = inst ? (inst.product_id || 0) : (parseInt(document.getElementById('asset-software-product-id')?.value || '0', 10) || 0);
    refreshVersionOptions(productID).then(() => {
      const vSel = document.getElementById('asset-software-version-id');
      if (!vSel) return;
      vSel.value = inst && inst.version_id ? String(inst.version_id) : '0';
      syncVersionTextVisibility();
    });

    ['asset-software-product-q', 'asset-software-product-select', 'asset-software-version-id', 'asset-software-version-text', 'asset-software-installed-at', 'asset-software-source', 'asset-software-notes'].forEach(id => {
      const el = document.getElementById(id);
      if (!el) return;
      el.disabled = viewOnly;
    });
    const saveBtn = document.getElementById('asset-software-save');
    if (saveBtn) saveBtn.hidden = viewOnly;
    const archiveBtn = document.getElementById('asset-software-archive');
    if (archiveBtn) archiveBtn.hidden = viewOnly || isNew;

    if (archiveBtn && !archiveBtn.hidden && inst) {
      if (inst.deleted_at) {
        archiveBtn.textContent = t('assets.software.actions.restore');
        archiveBtn.classList.remove('danger');
        archiveBtn.onclick = async () => {
          if (!confirm(t('assets.software.confirm.restore'))) return;
          try {
            await Api.post(`/api/assets/${state.assetId}/software/${inst.id}/restore`, {});
            closeModal('#asset-software-modal');
            await load();
          } catch (err) {
            setAlert(alert, localizeError(err));
          }
        };
      } else {
        archiveBtn.textContent = t('assets.software.actions.archive');
        archiveBtn.classList.add('danger');
        archiveBtn.onclick = async () => {
          if (!confirm(t('assets.software.confirm.archive'))) return;
          try {
            await Api.del(`/api/assets/${state.assetId}/software/${inst.id}`);
            closeModal('#asset-software-modal');
            await load();
          } catch (err) {
            setAlert(alert, localizeError(err));
          }
        };
      }
    }
    openModal('#asset-software-modal');
  }

  async function saveFromModal() {
    if (!state.canManage || state.readOnly) return;
    const alert = document.getElementById('asset-software-modal-alert');
    setAlert(alert, '');
    const instID = (document.getElementById('asset-software-inst-id')?.value || '').trim();
    const productID = parseInt(document.getElementById('asset-software-product-id')?.value || '0', 10) || 0;
    const versionIDRaw = parseInt(document.getElementById('asset-software-version-id')?.value || '0', 10) || 0;
    const versionText = (document.getElementById('asset-software-version-text')?.value || '').trim();
    const payload = {
      product_id: productID,
      version_id: versionIDRaw > 0 ? versionIDRaw : null,
      version_text: versionIDRaw > 0 ? '' : versionText,
      installed_at: document.getElementById('asset-software-installed-at')?.value || '',
      source: (document.getElementById('asset-software-source')?.value || '').trim(),
      notes: (document.getElementById('asset-software-notes')?.value || '').trim(),
    };
    if (!instID && payload.product_id <= 0) {
      setAlert(alert, t('software.install.error.productRequired'));
      return;
    }
    if (!payload.version_id && !payload.version_text) {
      setAlert(alert, t('software.install.error.versionRequired'));
      return;
    }
    try {
      if (instID) {
        await Api.put(`/api/assets/${state.assetId}/software/${instID}`, payload);
      } else {
        await Api.post(`/api/assets/${state.assetId}/software`, payload);
      }
      closeModal('#asset-software-modal');
      await load();
    } catch (err) {
      setAlert(alert, localizeError(err));
    }
  }

  function bindUI() {
    if (state.bound) return;
    state.bound = true;
    document.getElementById('asset-software-refresh')?.addEventListener('click', () => load());
    document.getElementById('asset-software-add')?.addEventListener('click', () => openInstallModal(null, false));
    document.getElementById('asset-software-save')?.addEventListener('click', () => saveFromModal());
    document.getElementById('asset-software-product-q')?.addEventListener('input', (e) => refreshProductOptions(e.target.value || ''));
    document.getElementById('asset-software-product-select')?.addEventListener('change', async (e) => {
      const val = parseInt(e.target.value || '0', 10) || 0;
      document.getElementById('asset-software-product-id').value = val > 0 ? String(val) : '';
      await refreshVersionOptions(val);
      const vSel = document.getElementById('asset-software-version-id');
      if (vSel) vSel.value = '0';
      syncVersionTextVisibility();
    });
    document.getElementById('asset-software-version-id')?.addEventListener('change', () => syncVersionTextVisibility());
    document.getElementById('asset-software-include-deleted')?.addEventListener('change', () => load());
  }

  async function onAssetModalOpened({ asset, readOnly, canManage }) {
    bindUI();
    state.assetId = asset ? (asset.id || 0) : 0;
    state.readOnly = !!readOnly;
    state.canManage = !!canManage;

    const section = document.getElementById('asset-software-section');
    if (section) section.hidden = !state.assetId;
    const filters = document.getElementById('asset-software-filters');
    if (filters) filters.hidden = !(state.assetId && state.canManage && !state.readOnly);
    const addBtn = document.getElementById('asset-software-add');
    if (addBtn) addBtn.hidden = !(state.assetId && state.canManage && !state.readOnly);

    if (!state.assetId) return;
    await load();
  }

  return { onAssetModalOpened };
})();

