(() => {
  if (typeof window === 'undefined') return;
  if (window.Settings2FA && window.Settings2FA.bind) return;

  function showAlert(el, msg) {
    if (!el) return;
    el.textContent = msg || '';
    el.hidden = !msg;
  }

  async function loadStatus() {
    return Api.get('/api/auth/2fa/status');
  }

  function renderStatus(data) {
    const enabled = !!data.enabled;
    const remaining = Number.isFinite(parseInt(data.recovery_codes_remaining, 10)) ? parseInt(data.recovery_codes_remaining, 10) : 0;
    const statusEl = document.getElementById('twofa-status-text');
    const remEl = document.getElementById('twofa-recovery-remaining');
    const enableBtn = document.getElementById('twofa-enable-btn');
    const disableBtn = document.getElementById('twofa-disable-btn');
    if (statusEl) {
      statusEl.textContent = enabled ? (BerkutI18n.t('auth.2fa.status.enabled') || 'Enabled') : (BerkutI18n.t('auth.2fa.status.disabled') || 'Disabled');
    }
    if (enableBtn) enableBtn.hidden = enabled;
    if (disableBtn) disableBtn.hidden = !enabled;
    if (remEl) {
      if (enabled) {
        remEl.hidden = false;
        remEl.textContent = (BerkutI18n.t('auth.2fa.recovery.remaining') || 'Recovery codes remaining') + ': ' + remaining;
      } else {
        remEl.hidden = true;
        remEl.textContent = '';
      }
    }
  }

  async function openSetupModal() {
    const alertEl = document.getElementById('twofa-setup-alert');
    showAlert(alertEl, '');
    const step = document.getElementById('twofa-setup-step');
    const recoveryStep = document.getElementById('twofa-recovery-step');
    const codesList = document.getElementById('twofa-recovery-codes');
    const closeBtn = document.getElementById('twofa-setup-close');
    const confirmBtn = document.getElementById('twofa-setup-confirm');
    if (step) step.hidden = false;
    if (recoveryStep) recoveryStep.hidden = true;
    if (codesList) codesList.innerHTML = '';
    if (closeBtn) closeBtn.hidden = true;
    if (confirmBtn) confirmBtn.hidden = false;

    const resp = await Api.post('/api/auth/2fa/setup', {});
    const qr = document.getElementById('twofa-setup-qr');
    const secret = document.getElementById('twofa-setup-secret');
    const code = document.getElementById('twofa-setup-code');
    if (qr) qr.src = resp.qr_png_base64 || '';
    if (secret) secret.value = resp.manual_secret || '';
    if (code) code.value = '';
    const modal = document.getElementById('twofa-setup-modal');
    if (modal) modal.hidden = false;
    if (code) code.focus();
  }

  async function confirmEnable() {
    const alertEl = document.getElementById('twofa-setup-alert');
    const codeEl = document.getElementById('twofa-setup-code');
    const confirmBtn = document.getElementById('twofa-setup-confirm');
    const closeBtn = document.getElementById('twofa-setup-close');
    const cancelBtn = document.getElementById('twofa-setup-cancel');
    showAlert(alertEl, '');
    const code = (codeEl && codeEl.value ? codeEl.value.trim() : '');
    if (!code) {
      showAlert(alertEl, BerkutI18n.t('auth.2fa.codeRequired') || 'Code required');
      return;
    }
    try {
      if (confirmBtn) confirmBtn.disabled = true;
      const resp = await Api.post('/api/auth/2fa/enable', { code });
      const codes = resp.recovery_codes || [];
      const list = document.getElementById('twofa-recovery-codes');
      if (list) {
        list.innerHTML = '';
        codes.forEach(c => {
          const li = document.createElement('li');
          li.textContent = String(c || '').trim();
          list.appendChild(li);
        });
      }
      const setupStep = document.getElementById('twofa-setup-step');
      const recoveryStep = document.getElementById('twofa-recovery-step');
      if (setupStep) setupStep.hidden = true;
      if (recoveryStep) recoveryStep.hidden = false;
      if (confirmBtn) confirmBtn.hidden = true;
      if (closeBtn) closeBtn.hidden = false;
      if (cancelBtn) cancelBtn.hidden = true;
      await refreshStatus();
    } catch (err) {
      const raw = err.message || 'common.error';
      const msg = BerkutI18n.t(raw) || raw;
      showAlert(alertEl, msg);
    } finally {
      if (confirmBtn) confirmBtn.disabled = false;
    }
  }

  async function openDisableModal() {
    const modal = document.getElementById('twofa-disable-modal');
    const alertEl = document.getElementById('twofa-disable-alert');
    const passEl = document.getElementById('twofa-disable-password');
    const recEl = document.getElementById('twofa-disable-recovery');
    showAlert(alertEl, '');
    if (passEl) passEl.value = '';
    if (recEl) recEl.value = '';
    if (modal) modal.hidden = false;
    if (passEl) passEl.focus();
  }

  async function confirmDisable() {
    const alertEl = document.getElementById('twofa-disable-alert');
    const passEl = document.getElementById('twofa-disable-password');
    const recEl = document.getElementById('twofa-disable-recovery');
    const btn = document.getElementById('twofa-disable-confirm');
    showAlert(alertEl, '');
    const password = (passEl && passEl.value ? passEl.value : '');
    const recovery_code = (recEl && recEl.value ? recEl.value.trim() : '');
    if (!password || !recovery_code) {
      showAlert(alertEl, BerkutI18n.t('auth.2fa.disableRequiresRecovery') || 'Password and recovery code required');
      return;
    }
    try {
      if (btn) btn.disabled = true;
      await Api.post('/api/auth/2fa/disable', { password, recovery_code });
      const modal = document.getElementById('twofa-disable-modal');
      if (modal) modal.hidden = true;
      await refreshStatus();
    } catch (err) {
      const raw = err.message || 'common.error';
      const msg = BerkutI18n.t(raw) || raw;
      showAlert(alertEl, msg);
    } finally {
      if (btn) btn.disabled = false;
    }
  }

  async function refreshStatus() {
    try {
      const st = await loadStatus();
      renderStatus(st || {});
    } catch (_) {
      // ignore
    }
  }

  function bind() {
    const page = document.getElementById('settings-page');
    if (!page) return;
    const enableBtn = document.getElementById('twofa-enable-btn');
    const disableBtn = document.getElementById('twofa-disable-btn');
    const confirmBtn = document.getElementById('twofa-setup-confirm');
    const disableConfirm = document.getElementById('twofa-disable-confirm');

    refreshStatus();
    if (enableBtn) enableBtn.addEventListener('click', async (e) => {
      e.preventDefault();
      try {
        await openSetupModal();
      } catch (err) {
        const raw = err.message || 'common.error';
        alert(BerkutI18n.t(raw) || raw);
      }
    });
    if (confirmBtn) confirmBtn.addEventListener('click', async (e) => {
      e.preventDefault();
      await confirmEnable();
    });
    if (disableBtn) disableBtn.addEventListener('click', async (e) => {
      e.preventDefault();
      await openDisableModal();
    });
    if (disableConfirm) disableConfirm.addEventListener('click', async (e) => {
      e.preventDefault();
      await confirmDisable();
    });
  }

  window.Settings2FA = { bind };
})();
