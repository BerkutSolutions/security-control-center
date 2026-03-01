(() => {
  if (typeof window === 'undefined') return;
  if (window.SettingsPasskeys && window.SettingsPasskeys.bind) return;

  let lastRenameID = 0;
  let lastDeleteID = 0;

  function showAlert(el, msg, success) {
    if (!el) return;
    el.textContent = msg || '';
    el.hidden = !msg;
    if (success) el.classList.add('success'); else el.classList.remove('success');
  }

  function webAuthnSupported() {
    return window.BerkutWebAuthn && window.BerkutWebAuthn.supported && window.BerkutWebAuthn.supported();
  }

  function fmtDateTime(value) {
    if (!value) return '—';
    if (typeof AppTime !== 'undefined' && AppTime.formatDateTime) return AppTime.formatDateTime(value);
    const d = new Date(value);
    if (Number.isNaN(d.getTime())) return '—';
    return d.toISOString();
  }

  async function loadList() {
    return Api.get('/api/auth/passkeys');
  }

  function renderList(data) {
    const statusEl = document.getElementById('passkeys-status-text');
    const wrap = document.getElementById('passkeys-table-wrap');
    const body = document.getElementById('passkeys-table-body');
    const addBtn = document.getElementById('passkeys-add-btn');
    if (!body || !wrap || !statusEl) return;

    const items = data && Array.isArray(data.items) ? data.items : [];
    body.innerHTML = '';
    if (!items.length) {
      statusEl.hidden = false;
      statusEl.textContent = BerkutI18n.t('auth.passkeys.status.empty') || 'No passkeys yet';
      wrap.hidden = true;
    } else {
      statusEl.hidden = true;
      wrap.hidden = false;
      items.forEach((it) => {
        const tr = document.createElement('tr');
        const name = String(it.name || '').trim() || (BerkutI18n.t('auth.passkeys.unnamed') || 'Passkey');
        const created = fmtDateTime(it.created_at);
        const lastUsed = it.last_used_at ? fmtDateTime(it.last_used_at) : (BerkutI18n.t('auth.passkeys.neverUsed') || 'Never');
        tr.innerHTML = `
          <td>${escapeHtml(name)}</td>
          <td class="muted">${escapeHtml(created)}</td>
          <td class="muted">${escapeHtml(lastUsed)}</td>
          <td class="table-actions">
            <button class="btn ghost btn-sm" data-action="rename" data-id="${escapeHtml(it.id)}">${escapeHtml(BerkutI18n.t('common.rename') || 'Rename')}</button>
            <button class="btn ghost danger btn-sm" data-action="delete" data-id="${escapeHtml(it.id)}">${escapeHtml(BerkutI18n.t('common.delete') || 'Delete')}</button>
          </td>
        `;
        body.appendChild(tr);

        const renameBtn = tr.querySelector('[data-action="rename"]');
        const deleteBtn = tr.querySelector('[data-action="delete"]');
        if (renameBtn) renameBtn.addEventListener('click', () => openRenameModal(it.id, name));
        if (deleteBtn) deleteBtn.addEventListener('click', () => openDeleteModal(it.id, name));
      });
    }
    if (addBtn) addBtn.disabled = !webAuthnSupported();
  }

  function escapeHtml(str) {
    return String(str || '')
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;');
  }

  async function refresh() {
    const statusEl = document.getElementById('passkeys-status-text');
    const wrap = document.getElementById('passkeys-table-wrap');
    if (statusEl) {
      statusEl.hidden = false;
      statusEl.textContent = BerkutI18n.t('auth.passkeys.status.loading') || 'Loading…';
    }
    if (wrap) wrap.hidden = true;
    try {
      const resp = await loadList();
      renderList(resp || {});
    } catch (err) {
      const raw = err && err.message ? String(err.message) : 'common.error';
      const msg = BerkutI18n.t(raw) || raw;
      if (statusEl) {
        statusEl.hidden = false;
        statusEl.textContent = msg;
      }
    }
  }

  function openRegisterModal() {
    const modal = document.getElementById('passkeys-register-modal');
    const alertEl = document.getElementById('passkeys-register-alert');
    const nameEl = document.getElementById('passkeys-register-name');
    const btn = document.getElementById('passkeys-register-confirm');
    showAlert(alertEl, '');
    if (nameEl) nameEl.value = (window.BerkutWebAuthn && BerkutWebAuthn.defaultDeviceName ? BerkutWebAuthn.defaultDeviceName() : '') || '';
    if (btn) btn.disabled = false;
    if (modal) modal.hidden = false;
    if (nameEl) nameEl.focus();
  }

  async function confirmRegister() {
    const alertEl = document.getElementById('passkeys-register-alert');
    const nameEl = document.getElementById('passkeys-register-name');
    const btn = document.getElementById('passkeys-register-confirm');
    showAlert(alertEl, '');
    if (!webAuthnSupported()) {
      showAlert(alertEl, BerkutI18n.t('auth.passkeys.unsupported') || 'Not supported');
      return;
    }
    const name = (nameEl && nameEl.value ? nameEl.value.trim() : '');
    if (!name) {
      showAlert(alertEl, BerkutI18n.t('auth.passkeys.nameRequired') || 'Name required');
      return;
    }
    try {
      if (btn) btn.disabled = true;
      const begin = await Api.post('/api/auth/passkeys/register/begin', { name });
      const pk = begin && begin.options ? begin.options : null;
      const challengeId = (begin && begin.challenge_id ? String(begin.challenge_id) : '').trim();
      if (!pk || !challengeId) throw new Error('common.serverError');
      const publicKey = window.BerkutWebAuthn.toPublicKeyCreationOptions(pk);
      const cred = await navigator.credentials.create({ publicKey });
      const credential = window.BerkutWebAuthn.credentialToJSON(cred);
      await Api.post('/api/auth/passkeys/register/finish', { challenge_id: challengeId, name, credential });
      const modal = document.getElementById('passkeys-register-modal');
      if (modal) modal.hidden = true;
      await refresh();
    } catch (err) {
      const key = window.BerkutWebAuthn && BerkutWebAuthn.errorKey ? BerkutWebAuthn.errorKey(err) : '';
      const raw = key || (err && err.message ? String(err.message) : 'common.error');
      const msg = BerkutI18n.t(raw) || raw;
      showAlert(alertEl, msg);
    } finally {
      if (btn) btn.disabled = false;
    }
  }

  function openRenameModal(id, name) {
    lastRenameID = Number(id || 0);
    const modal = document.getElementById('passkeys-rename-modal');
    const alertEl = document.getElementById('passkeys-rename-alert');
    const nameEl = document.getElementById('passkeys-rename-name');
    showAlert(alertEl, '');
    if (nameEl) nameEl.value = String(name || '').trim();
    const btn = document.getElementById('passkeys-rename-confirm');
    if (btn) btn.disabled = false;
    if (modal) modal.hidden = false;
    if (nameEl) nameEl.focus();
  }

  async function confirmRename() {
    const alertEl = document.getElementById('passkeys-rename-alert');
    const nameEl = document.getElementById('passkeys-rename-name');
    const btn = document.getElementById('passkeys-rename-confirm');
    showAlert(alertEl, '');
    const id = Number(lastRenameID || 0);
    const name = (nameEl && nameEl.value ? nameEl.value.trim() : '');
    if (!id || !name) {
      showAlert(alertEl, BerkutI18n.t('common.badRequest') || 'Bad request');
      return;
    }
    try {
      if (btn) btn.disabled = true;
      await Api.put(`/api/auth/passkeys/${id}/rename`, { name });
      const modal = document.getElementById('passkeys-rename-modal');
      if (modal) modal.hidden = true;
      await refresh();
    } catch (err) {
      const raw = err && err.message ? String(err.message) : 'common.error';
      const msg = BerkutI18n.t(raw) || raw;
      showAlert(alertEl, msg);
    } finally {
      if (btn) btn.disabled = false;
    }
  }

  function openDeleteModal(id, name) {
    lastDeleteID = Number(id || 0);
    const modal = document.getElementById('passkeys-delete-modal');
    const alertEl = document.getElementById('passkeys-delete-alert');
    const msgEl = document.getElementById('passkeys-delete-message');
    showAlert(alertEl, '');
    if (msgEl) {
      const tpl = BerkutI18n.t('auth.passkeys.delete.confirm') || 'Delete passkey "{name}"?';
      msgEl.textContent = String(tpl).replace('{name}', String(name || '').trim());
    }
    const btn = document.getElementById('passkeys-delete-confirm');
    if (btn) btn.disabled = false;
    if (modal) modal.hidden = false;
  }

  async function confirmDelete() {
    const alertEl = document.getElementById('passkeys-delete-alert');
    const btn = document.getElementById('passkeys-delete-confirm');
    showAlert(alertEl, '');
    const id = Number(lastDeleteID || 0);
    if (!id) {
      showAlert(alertEl, BerkutI18n.t('common.badRequest') || 'Bad request');
      return;
    }
    try {
      if (btn) btn.disabled = true;
      await Api.del(`/api/auth/passkeys/${id}`, {});
      const modal = document.getElementById('passkeys-delete-modal');
      if (modal) modal.hidden = true;
      await refresh();
    } catch (err) {
      const raw = err && err.message ? String(err.message) : 'common.error';
      const msg = BerkutI18n.t(raw) || raw;
      showAlert(alertEl, msg);
    } finally {
      if (btn) btn.disabled = false;
    }
  }

  function bind(alertBox) {
    const page = document.getElementById('settings-page');
    if (!page) return;

    const unsupported = document.getElementById('passkeys-unsupported');
    if (unsupported) unsupported.hidden = webAuthnSupported();
    const addBtn = document.getElementById('passkeys-add-btn');
    if (addBtn) {
      addBtn.disabled = !webAuthnSupported();
      addBtn.addEventListener('click', async (e) => {
        e.preventDefault();
        if (!webAuthnSupported()) {
          if (alertBox) showAlert(alertBox, BerkutI18n.t('auth.passkeys.unsupported') || 'Not supported');
          return;
        }
        openRegisterModal();
      });
    }

    const registerConfirm = document.getElementById('passkeys-register-confirm');
    if (registerConfirm) registerConfirm.addEventListener('click', async (e) => {
      e.preventDefault();
      await confirmRegister();
    });

    const renameConfirm = document.getElementById('passkeys-rename-confirm');
    if (renameConfirm) renameConfirm.addEventListener('click', async (e) => {
      e.preventDefault();
      await confirmRename();
    });

    const deleteConfirm = document.getElementById('passkeys-delete-confirm');
    if (deleteConfirm) deleteConfirm.addEventListener('click', async (e) => {
      e.preventDefault();
      await confirmDelete();
    });

    refresh().catch(() => {});
  }

  window.SettingsPasskeys = { bind };
})();
