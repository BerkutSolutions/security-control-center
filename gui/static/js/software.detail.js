const SoftwareDetail = (() => {
  const state = {
    versions: [],
    assets: [],
    bound: false,
    ctx: null,
  };

  function t(key) {
    return state.ctx?.t ? state.ctx.t(key) : key;
  }

  function escapeHTML(str) {
    return (str || '').toString().replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  }

  function localizeError(err, fallbackKey) {
    if (state.ctx?.localizeError) return state.ctx.localizeError(err, fallbackKey);
    return (err && err.message) ? err.message : (fallbackKey || 'Error');
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

  function fmtDate(raw) {
    if (!raw) return '-';
    if (window.AppTime?.formatDate) return AppTime.formatDate(raw);
    return String(raw).slice(0, 10);
  }

  async function loadForProduct(productID, opts = {}) {
    if (!productID) return;
    await Promise.all([
      loadVersions(productID, opts),
      loadAssets(productID, opts),
    ]);
  }

  async function loadVersions(productID, opts) {
    const tbody = document.querySelector('#software-versions-table tbody');
    const empty = document.getElementById('software-versions-empty');
    if (tbody) tbody.innerHTML = '';
    if (empty) empty.hidden = true;

    const includeDeleted = !!(opts.canManage && !opts.readOnly);
    let res;
    try {
      res = await Api.get(`/api/software/${productID}/versions?include_deleted=${includeDeleted ? '1' : '0'}`);
    } catch (err) {
      state.versions = [];
      if (empty) empty.hidden = false;
      return;
    }
    state.versions = Array.isArray(res?.items) ? res.items : [];
    if (!state.versions.length) {
      if (empty) empty.hidden = false;
      return;
    }
    state.versions.forEach(v => {
      const tr = document.createElement('tr');
      tr.innerHTML = `
        <td>${escapeHTML(v.version || '')}${v.deleted_at ? ` <span class="muted">(${escapeHTML(t('common.archived'))})</span>` : ''}</td>
        <td>${escapeHTML(v.release_date ? fmtDate(v.release_date) : '-')}</td>
        <td>${escapeHTML(v.eol_date ? fmtDate(v.eol_date) : '-')}</td>
        <td>${v.notes ? escapeHTML(v.notes) : '<span class="meta-empty">-</span>'}</td>
        <td class="actions"></td>
      `;
      const actions = tr.querySelector('td.actions');
      if (actions) actions.appendChild(renderVersionActions(productID, v, opts));
      tbody?.appendChild(tr);
    });
  }

  function renderVersionActions(productID, v, opts) {
    const wrap = document.createElement('div');
    wrap.className = 'btn-group';

    const viewBtn = document.createElement('button');
    viewBtn.className = 'btn ghost btn-xs';
    viewBtn.textContent = t('software.versions.actions.view');
    viewBtn.onclick = () => openVersionModal({ productID, version: v, readOnly: true });
    wrap.appendChild(viewBtn);

    if (!opts.canManage || opts.readOnly) return wrap;

    if (v.deleted_at) {
      const restoreBtn = document.createElement('button');
      restoreBtn.className = 'btn ghost btn-xs';
      restoreBtn.textContent = t('software.versions.actions.restore');
      restoreBtn.onclick = async () => {
        if (!confirm(t('software.versions.confirm.restore'))) return;
        try {
          await Api.post(`/api/software/${productID}/versions/${v.id}/restore`, {});
          await loadVersions(productID, opts);
        } catch (err) {
          alert(localizeError(err));
        }
      };
      wrap.appendChild(restoreBtn);
      return wrap;
    }

    const editBtn = document.createElement('button');
    editBtn.className = 'btn ghost btn-xs';
    editBtn.textContent = t('software.versions.actions.edit');
    editBtn.onclick = () => openVersionModal({ productID, version: v, readOnly: false });
    wrap.appendChild(editBtn);

    const archiveBtn = document.createElement('button');
    archiveBtn.className = 'btn ghost btn-xs danger';
    archiveBtn.textContent = t('software.versions.actions.archive');
    archiveBtn.onclick = async () => {
      if (!confirm(t('software.versions.confirm.archive'))) return;
      try {
        await Api.del(`/api/software/${productID}/versions/${v.id}`);
        await loadVersions(productID, opts);
      } catch (err) {
        alert(localizeError(err));
      }
    };
    wrap.appendChild(archiveBtn);
    return wrap;
  }

  async function loadAssets(productID, opts) {
    const tbody = document.querySelector('#software-assets-table tbody');
    const empty = document.getElementById('software-assets-empty');
    if (tbody) tbody.innerHTML = '';
    if (empty) empty.hidden = true;

    let res;
    try {
      res = await Api.get(`/api/software/${productID}/assets?include_deleted=${opts.canManage ? '1' : '0'}`);
    } catch (err) {
      state.assets = [];
      if (empty) empty.hidden = false;
      return;
    }
    state.assets = Array.isArray(res?.items) ? res.items : [];
    if (!state.assets.length) {
      if (empty) empty.hidden = false;
      return;
    }
    state.assets.forEach(inst => {
      const tr = document.createElement('tr');
      const installedAt = inst.installed_at ? fmtDate(inst.installed_at) : '-';
      tr.innerHTML = `
        <td>${escapeHTML(inst.asset_name || `#${inst.asset_id || ''}`)}</td>
        <td>${escapeHTML(inst.version_label || inst.version_text || '-')}</td>
        <td>${escapeHTML(installedAt)}</td>
        <td>${escapeHTML(inst.source || '-')}</td>
        <td class="actions"></td>
      `;
      const actions = tr.querySelector('td.actions');
      if (actions) actions.appendChild(renderAssetActions(inst));
      tbody?.appendChild(tr);
    });
  }

  function renderAssetActions(inst) {
    const wrap = document.createElement('div');
    wrap.className = 'btn-group';
    const openBtn = document.createElement('button');
    openBtn.className = 'btn ghost btn-xs';
    openBtn.textContent = t('software.assets.actions.openAsset');
    openBtn.onclick = () => {
      const id = inst.asset_id || '';
      if (!id) return;
      window.history.pushState({}, '', `/assets?asset=${encodeURIComponent(id)}`);
      window.dispatchEvent(new PopStateEvent('popstate'));
    };
    wrap.appendChild(openBtn);
    return wrap;
  }

  function openVersionModal({ productID, version, readOnly }) {
    const modal = document.getElementById('software-version-modal');
    if (!modal) return;
    const alert = document.getElementById('software-version-alert');
    setAlert(alert, '');

    const isNew = !version;
    const titleEl = document.getElementById('software-version-modal-title');
    if (titleEl) titleEl.textContent = isNew ? t('software.versions.modal.createTitle') : (readOnly ? t('software.versions.modal.viewTitle') : t('software.versions.modal.editTitle'));

    document.getElementById('software-version-form').dataset.productId = String(productID || 0);
    document.getElementById('software-version-id').value = version ? (version.id || '') : '';
    document.getElementById('software-version-value').value = version ? (version.version || '') : '';
    document.getElementById('software-version-release').value = version && version.release_date ? String(version.release_date).slice(0, 10) : '';
    document.getElementById('software-version-eol').value = version && version.eol_date ? String(version.eol_date).slice(0, 10) : '';
    document.getElementById('software-version-notes').value = version ? (version.notes || '') : '';

    ['software-version-value', 'software-version-release', 'software-version-eol', 'software-version-notes'].forEach(id => {
      const el = document.getElementById(id);
      if (el) el.disabled = !!readOnly;
    });
    const saveBtn = document.getElementById('software-version-save');
    if (saveBtn) saveBtn.hidden = !!readOnly;
    openModal('#software-version-modal');
  }

  async function saveVersionFromModal() {
    if (!state.ctx?.canManage?.()) return;
    const alert = document.getElementById('software-version-alert');
    setAlert(alert, '');
    const productID = parseInt(document.getElementById('software-version-form')?.dataset.productId || '0', 10) || 0;
    const versionID = (document.getElementById('software-version-id')?.value || '').trim();
    const payload = {
      version: (document.getElementById('software-version-value')?.value || '').trim(),
      release_date: document.getElementById('software-version-release')?.value || '',
      eol_date: document.getElementById('software-version-eol')?.value || '',
      notes: (document.getElementById('software-version-notes')?.value || '').trim(),
    };
    if (!payload.version) {
      setAlert(alert, t('software.version.error.required'));
      return;
    }
    try {
      if (versionID) {
        await Api.put(`/api/software/${productID}/versions/${versionID}`, payload);
      } else {
        await Api.post(`/api/software/${productID}/versions`, payload);
      }
      closeModal('#software-version-modal');
      await loadVersions(productID, { canManage: state.ctx.canManage(), readOnly: false });
    } catch (err) {
      setAlert(alert, localizeError(err));
    }
  }

  function bindUI(ctx) {
    state.ctx = ctx || null;
    if (state.bound) return;
    state.bound = true;
    document.getElementById('software-version-add')?.addEventListener('click', () => {
      if (!state.ctx?.canManage?.()) return;
      const productID = state.ctx.getOpenProductId ? state.ctx.getOpenProductId() : 0;
      if (!productID) return;
      openVersionModal({ productID, version: null, readOnly: false });
    });
    document.getElementById('software-version-save')?.addEventListener('click', () => saveVersionFromModal());
  }

  return { bindUI, loadForProduct, openVersionModal };
})();

