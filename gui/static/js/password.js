(async () => {
  await BerkutI18n.load(localStorage.getItem('berkut_lang') || 'ru');
  BerkutI18n.apply();
  const alertBox = document.getElementById('password-alert');
  const form = document.getElementById('password-form');
  const params = new URLSearchParams(window.location.search || '');
  const next = params.get('next') || '/healthcheck';

  let me;
  try {
    me = await Api.get('/api/auth/me');
  } catch (err) {
    window.location.href = '/login';
    return;
  }
  if (me.user && me.user.password_set) {
    window.location.href = '/dashboard';
    return;
  }

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    alertBox.hidden = true;
    const pwd = document.getElementById('new-password').value.trim();
    const confirm = document.getElementById('new-password-confirm').value.trim();
    if (!pwd || pwd.length < 6) {
      alertBox.textContent = BerkutI18n.t('accounts.newPasswordPrompt');
      alertBox.hidden = false;
      return;
    }
    if (pwd !== confirm) {
      alertBox.textContent = BerkutI18n.t('accounts.passwordMismatch');
      alertBox.hidden = false;
      return;
    }
    try {
      await Api.post('/api/auth/change-password', { password: pwd });
      window.location.href = next;
    } catch (err) {
      alertBox.textContent = err.message || BerkutI18n.t('common.error');
      alertBox.hidden = false;
    }
  });
})();
