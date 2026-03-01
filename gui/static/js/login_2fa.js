(async () => {
  await BerkutI18n.load(localStorage.getItem('berkut_lang') || 'ru');
  BerkutI18n.apply();

  const alertBox = document.getElementById('login2fa-alert');
  const form = document.getElementById('login2fa-form');
  const codeEl = document.getElementById('twofa-code');
  const useRecoveryEl = document.getElementById('twofa-use-recovery');
  const backBtn = document.getElementById('login2fa-back');
  const passkeyBtn = document.getElementById('login2fa-passkey-btn');

  const storageKey = 'berkut_2fa_challenge_id';
  const nextKey = 'berkut_login_next';

  function showError(raw) {
    const key = (raw || '').trim() || 'common.error';
    const msg = BerkutI18n.t(key) || key;
    if (!alertBox) return;
    alertBox.textContent = msg;
    alertBox.hidden = false;
  }

  function clearError() {
    if (!alertBox) return;
    alertBox.hidden = true;
    alertBox.textContent = '';
  }

  function getNext() {
    const urlNext = new URLSearchParams(window.location.search).get('next') || '';
    const stored = (sessionStorage.getItem(nextKey) || '').trim();
    return (urlNext || stored || '/healthcheck').trim();
  }

  function getChallengeID() {
    return (sessionStorage.getItem(storageKey) || '').trim();
  }

  function clearChallenge() {
    sessionStorage.removeItem(storageKey);
    sessionStorage.removeItem(nextKey);
  }

  function goBack() {
    clearChallenge();
    window.location.href = '/login';
  }

  function webAuthnSupported() {
    return window.BerkutWebAuthn && window.BerkutWebAuthn.supported && window.BerkutWebAuthn.supported();
  }

  async function confirmWithCode() {
    const challengeId = getChallengeID();
    if (!challengeId) {
      showError('auth.2fa.challengeMissing');
      return;
    }
    const useRecovery = !!(useRecoveryEl && useRecoveryEl.checked);
    const value = (codeEl && codeEl.value ? codeEl.value.trim() : '');
    if (!value) {
      showError('auth.2fa.codeRequired');
      return;
    }
    const resp = await Api.post('/api/auth/login/2fa', {
      challenge_id: challengeId,
      code: useRecovery ? '' : value,
      recovery_code: useRecovery ? value : '',
    });
    if (resp && resp.user && (resp.user.password_set === false || resp.user.require_password_change)) {
      clearChallenge();
      window.location.href = `/password-change?next=${encodeURIComponent(getNext())}`;
      return;
    }
    clearChallenge();
    window.location.href = getNext();
  }

  async function confirmWithPasskey() {
    const challengeId = getChallengeID();
    if (!challengeId) {
      showError('auth.2fa.challengeMissing');
      return;
    }
    const begin = await Api.post('/api/auth/login/2fa/passkey/begin', { challenge_id: challengeId });
    const pk = begin && begin.options ? begin.options : null;
    const webauthnChallengeID = (begin && begin.webauthn_challenge_id ? String(begin.webauthn_challenge_id) : '').trim();
    if (!pk || !webauthnChallengeID) {
      showError('common.serverError');
      return;
    }
    const publicKey = window.BerkutWebAuthn.toPublicKeyRequestOptions(pk);
    const cred = await navigator.credentials.get({ publicKey });
    const credential = window.BerkutWebAuthn.credentialToJSON(cred);
    const resp = await Api.post('/api/auth/login/2fa/passkey/finish', {
      challenge_id: challengeId,
      webauthn_challenge_id: webauthnChallengeID,
      credential,
    });
    if (resp && resp.user && (resp.user.password_set === false || resp.user.require_password_change)) {
      clearChallenge();
      window.location.href = `/password-change?next=${encodeURIComponent(getNext())}`;
      return;
    }
    clearChallenge();
    window.location.href = getNext();
  }

  if (backBtn) backBtn.addEventListener('click', goBack);

  if (form) {
    form.addEventListener('submit', async (e) => {
      e.preventDefault();
      clearError();
      try {
        await confirmWithCode();
      } catch (err) {
        showError(err && err.message ? String(err.message) : 'common.error');
      }
    });
  }

  if (passkeyBtn) {
    if (webAuthnSupported()) {
      passkeyBtn.hidden = false;
      passkeyBtn.addEventListener('click', async () => {
        clearError();
        try {
          await confirmWithPasskey();
        } catch (err) {
          const key = window.BerkutWebAuthn && BerkutWebAuthn.errorKey ? BerkutWebAuthn.errorKey(err) : '';
          showError(key || (err && err.message ? String(err.message) : 'common.error'));
        }
      });
    } else {
      passkeyBtn.hidden = true;
    }
  }

  if (!getChallengeID()) {
    showError('auth.2fa.challengeMissing');
    if (codeEl) codeEl.disabled = true;
  } else {
    if (codeEl) codeEl.focus();
  }
})();
