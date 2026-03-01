(async () => {
  await BerkutI18n.load(localStorage.getItem('berkut_lang') || 'ru');
  BerkutI18n.apply();

  const form = document.getElementById('login-form');
  const alertBox = document.getElementById('login-alert');
  const username = document.getElementById('username');
  const password = document.getElementById('password');
  const submitBtn = document.getElementById('login-submit-btn');
  const passkeyBtn = document.getElementById('login-passkey-btn');

  const twofaChallengeKey = 'berkut_2fa_challenge_id';
  const loginNextKey = 'berkut_login_next';

  function currentNext() {
    const urlNext = new URLSearchParams(window.location.search).get('next') || '';
    return (urlNext || '/healthcheck').trim();
  }

  function webAuthnSupported() {
    return window.BerkutWebAuthn && window.BerkutWebAuthn.supported && window.BerkutWebAuthn.supported();
  }

  function showError(raw) {
    const key = (raw || '').trim() || 'common.error';
    let msg = BerkutI18n.t(key);
    if (msg === key && key === 'invalid credentials') {
      msg = BerkutI18n.t('auth.invalidCredentials') || key;
    }
    if (!alertBox) return;
    alertBox.textContent = msg || key;
    alertBox.hidden = false;
  }

  function clearError() {
    if (!alertBox) return;
    alertBox.hidden = true;
    alertBox.textContent = '';
  }

  if (form) {
    form.addEventListener('submit', async (e) => {
      e.preventDefault();
      clearError();
      if (submitBtn) submitBtn.disabled = true;
      try {
        const body = { username: username.value.trim(), password: password.value };
        const resp = await Api.post('/api/auth/login', body);
        if (resp && resp.two_factor_required) {
          const challengeId = (resp.challenge_id || '').trim();
          if (!challengeId) throw new Error('auth.2fa.challengeMissing');
          sessionStorage.setItem(twofaChallengeKey, challengeId);
          sessionStorage.setItem(loginNextKey, currentNext());
          window.location.href = `/login/2fa?next=${encodeURIComponent(currentNext())}`;
          return;
        }
        if (resp && resp.user && (resp.user.password_set === false || resp.user.require_password_change)) {
          window.location.href = `/password-change?next=${encodeURIComponent(currentNext())}`;
          return;
        }
        window.location.href = currentNext();
      } catch (err) {
        showError(err && err.message ? String(err.message) : 'common.error');
      } finally {
        if (submitBtn) submitBtn.disabled = false;
      }
    });
  }

  if (passkeyBtn && webAuthnSupported()) {
    passkeyBtn.hidden = false;
    passkeyBtn.addEventListener('click', async () => {
      clearError();
      if (passkeyBtn) passkeyBtn.disabled = true;
      try {
        const begin = await Api.post('/api/auth/passkeys/login/begin', { username: username.value.trim() });
        const pk = begin && begin.options ? begin.options : null;
        const challengeId = (begin && begin.challenge_id ? String(begin.challenge_id) : '').trim();
        if (!pk || !challengeId) throw new Error('common.serverError');
        const publicKey = window.BerkutWebAuthn.toPublicKeyRequestOptions(pk);
        const cred = await navigator.credentials.get({ publicKey });
        const credential = window.BerkutWebAuthn.credentialToJSON(cred);
        const resp = await Api.post('/api/auth/passkeys/login/finish', { challenge_id: challengeId, credential });
        if (resp && resp.user && (resp.user.password_set === false || resp.user.require_password_change)) {
          window.location.href = `/password-change?next=${encodeURIComponent(currentNext())}`;
          return;
        }
        window.location.href = currentNext();
      } catch (err) {
        const key = window.BerkutWebAuthn && BerkutWebAuthn.errorKey ? BerkutWebAuthn.errorKey(err) : '';
        showError(key || (err && err.message ? String(err.message) : 'common.error'));
      } finally {
        if (passkeyBtn) passkeyBtn.disabled = false;
      }
    });
  } else if (passkeyBtn) {
    passkeyBtn.hidden = true;
  }
})();
