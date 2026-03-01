(async () => {
  await BerkutI18n.load(localStorage.getItem('berkut_lang') || 'ru');
  BerkutI18n.apply();
  const form = document.getElementById('login-form');
  const alertBox = document.getElementById('login-alert');
  const username = document.getElementById('username');
  const password = document.getElementById('password');

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    alertBox.hidden = true;
    try {
      const body = { username: username.value.trim(), password: password.value };
      const resp = await Api.post('/api/auth/login', body);
      if (resp.user && (resp.user.password_set === false || resp.user.require_password_change)) {
        window.location.href = '/password-change?next=/healthcheck';
        return;
      }
      window.location.href = '/healthcheck';
    } catch (err) {
      const raw = err.message || 'common.error';
      let msg = BerkutI18n.t(raw);
      if (msg === raw && raw === 'invalid credentials') {
        msg = BerkutI18n.t('auth.invalidCredentials') || raw;
      }
      alertBox.textContent = msg || 'Ошибка';
      alertBox.hidden = false;
    }
  });
})();
