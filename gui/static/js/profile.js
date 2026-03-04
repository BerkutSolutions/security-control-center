const ProfilePage = (() => {
  let me = null;

  async function init(onPrefsChange) {
    const page = document.getElementById('profile-page');
    if (!page) return;
    const alertBox = document.getElementById('profile-alert');
    bindPasswordChange(alertBox);
    bindPreferences(alertBox, onPrefsChange);
    me = await loadMe();
    renderProfileInfo(me);
    renderPasswordMeta(me);
    if (window.Settings2FA?.bind) {
      window.Settings2FA.bind(alertBox);
    }
    if (window.SettingsPasskeys?.bind) {
      window.SettingsPasskeys.bind(alertBox);
    }
  }

  function bindPasswordChange(alertBox) {
    const form = document.getElementById('password-change-form');
    if (!form) return;
    form.onsubmit = async (e) => {
      e.preventDefault();
      hideAlert(alertBox);
      const data = Object.fromEntries(new FormData(form).entries());
      if ((data.password || '') !== (data.password_confirm || '')) {
        showAlert(alertBox, BerkutI18n.t('accounts.passwordMismatch'));
        return;
      }
      try {
        await Api.post('/api/auth/change-password', {
          current_password: data.current_password,
          password: data.password,
        });
        showAlert(alertBox, BerkutI18n.t('accounts.passwordChangeDone') || BerkutI18n.t('common.saved'), true);
        if (window.AppToast?.show) window.AppToast.show(BerkutI18n.t('accounts.passwordChangeDone') || BerkutI18n.t('common.saved'), 'success');
        form.reset();
      } catch (err) {
        showAlert(alertBox, err.message || BerkutI18n.t('common.error'));
      }
    };
  }

  function bindPreferences(alertBox, onPrefsChange) {
    const prefs = Preferences.load();
    const langSelect = document.getElementById('settings-language');
    const tzSelect = document.getElementById('settings-timezone');
    const autoLogoutToggle = document.getElementById('settings-auto-logout');
    const autoSaveToggle = document.getElementById('settings-autosave-enabled');
    const autoSavePeriod = document.getElementById('settings-autosave-period');
    const form = document.getElementById('settings-form');
    if (!form || !langSelect || !tzSelect || !autoLogoutToggle) return;

    langSelect.value = prefs.language || 'ru';
    autoLogoutToggle.checked = !!prefs.autoLogout;
    if (autoSaveToggle) autoSaveToggle.checked = !!prefs.incidentAutoSaveEnabled;
    if (autoSavePeriod) {
      const period = parseInt(prefs.incidentAutoSavePeriod, 10);
      autoSavePeriod.value = Number.isFinite(period) && period > 0 ? `${period}` : '5';
    }

    if (AppTime?.timeZones) {
      const zones = AppTime.timeZones();
      tzSelect.innerHTML = '';
      zones.forEach((zone) => {
        const opt = document.createElement('option');
        opt.value = zone;
        opt.textContent = zone;
        tzSelect.appendChild(opt);
      });
      tzSelect.value = prefs.timeZone || AppTime.getTimeZone();
    }

    const syncAutoSave = () => {
      if (autoSaveToggle && autoSavePeriod) autoSavePeriod.disabled = !autoSaveToggle.checked;
    };
    syncAutoSave();
    autoSaveToggle?.addEventListener('change', syncAutoSave);

    form.onsubmit = async (e) => {
      e.preventDefault();
      hideAlert(alertBox);
      const nextPrefs = Preferences.save({
        language: langSelect.value || 'ru',
        timeZone: tzSelect.value || AppTime.getTimeZone(),
        autoLogout: !!autoLogoutToggle.checked,
        incidentAutoSaveEnabled: !!(autoSaveToggle && autoSaveToggle.checked),
        incidentAutoSavePeriod: autoSavePeriod ? parseInt(autoSavePeriod.value, 10) || 0 : 0,
      });
      if (onPrefsChange) {
        await onPrefsChange(nextPrefs);
      }
      showAlert(alertBox, BerkutI18n.t('settings.saved'), true);
      if (window.AppToast?.show) window.AppToast.show(BerkutI18n.t('settings.saved'), 'success');
    };
  }

  async function loadMe() {
    try {
      const res = await Api.get('/api/auth/me');
      return res?.user || null;
    } catch (_) {
      return null;
    }
  }

  function renderProfileInfo(user) {
    setText('profile-username', user?.username || '-');
    setText('profile-fullname', user?.full_name || '-');
    setText('profile-department', user?.department || '-');
    setText('profile-position', user?.position || '-');
    setText('profile-session-start', formatDateTime(user?.session_created_at));
    setText('profile-session-expire', formatDateTime(user?.session_expires_at));
    setText('profile-last-login-ip', user?.last_login_ip || '-');
    setText('profile-trusted-ip', user?.frequent_login_ip || '-');
  }

  function renderPasswordMeta(user) {
    const el = document.getElementById('password-last-changed');
    if (!el) return;
    if (!user?.password_changed_at) {
      el.textContent = '';
      return;
    }
    el.textContent = `${BerkutI18n.t('accounts.passwordLastChanged')}: ${formatDateTime(user.password_changed_at)}`;
  }

  function formatDateTime(value) {
    if (!value) return '-';
    if (window.AppTime?.formatDateTime) return AppTime.formatDateTime(value);
    return String(value);
  }

  function setText(id, value) {
    const el = document.getElementById(id);
    if (el) el.textContent = value;
  }

  function hideAlert(alertBox) {
    if (!alertBox) return;
    alertBox.hidden = true;
    alertBox.classList.remove('success');
  }

  function showAlert(alertBox, message, success) {
    if (!alertBox) return;
    alertBox.textContent = message;
    alertBox.hidden = false;
    alertBox.classList.toggle('success', !!success);
  }

  return { init };
})();

if (typeof window !== 'undefined') {
  window.ProfilePage = ProfilePage;
}
