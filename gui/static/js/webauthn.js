(() => {
  if (typeof window === 'undefined') return;
  if (window.BerkutWebAuthn) return;

  const enc = new TextEncoder();

  function supported() {
    return !!(window.PublicKeyCredential && navigator.credentials && typeof navigator.credentials.get === 'function' && typeof navigator.credentials.create === 'function');
  }

  function normalizeB64Url(input) {
    return String(input || '').replace(/-/g, '+').replace(/_/g, '/');
  }

  function base64UrlToBytes(input) {
    const str = normalizeB64Url(input);
    if (!str) return new Uint8Array();
    const pad = str.length % 4 === 0 ? '' : '='.repeat(4 - (str.length % 4));
    const bin = atob(str + pad);
    const out = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
    return out;
  }

  function bytesToBase64Url(bytes) {
    const arr = bytes instanceof ArrayBuffer ? new Uint8Array(bytes) : new Uint8Array(bytes || []);
    let bin = '';
    for (let i = 0; i < arr.length; i++) bin += String.fromCharCode(arr[i]);
    return btoa(bin).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/g, '');
  }

  function toPublicKeyCreationOptions(options) {
    const pk = options && options.publicKey ? options.publicKey : options;
    if (!pk) return null;
    const out = { ...pk };
    if (out.challenge) out.challenge = base64UrlToBytes(out.challenge);
    if (out.user && out.user.id) out.user = { ...out.user, id: base64UrlToBytes(out.user.id) };
    if (Array.isArray(out.excludeCredentials)) {
      out.excludeCredentials = out.excludeCredentials.map((c) => ({ ...c, id: base64UrlToBytes(c.id) }));
    }
    return out;
  }

  function toPublicKeyRequestOptions(options) {
    const pk = options && options.publicKey ? options.publicKey : options;
    if (!pk) return null;
    const out = { ...pk };
    if (out.challenge) out.challenge = base64UrlToBytes(out.challenge);
    if (Array.isArray(out.allowCredentials)) {
      out.allowCredentials = out.allowCredentials.map((c) => ({ ...c, id: base64UrlToBytes(c.id) }));
    }
    return out;
  }

  function credentialToJSON(cred) {
    if (!cred) return null;
    const common = {
      id: cred.id,
      rawId: bytesToBase64Url(cred.rawId),
      type: cred.type,
      clientExtensionResults: cred.getClientExtensionResults ? cred.getClientExtensionResults() : {},
    };
    if (cred.response && cred.response.attestationObject) {
      const att = cred.response;
      const out = {
        ...common,
        response: {
          clientDataJSON: bytesToBase64Url(att.clientDataJSON),
          attestationObject: bytesToBase64Url(att.attestationObject),
        },
      };
      if (typeof att.getTransports === 'function') {
        try {
          out.transports = att.getTransports();
        } catch (_) {
          // ignore
        }
      }
      return out;
    }
    if (cred.response && cred.response.authenticatorData) {
      const asr = cred.response;
      return {
        ...common,
        response: {
          clientDataJSON: bytesToBase64Url(asr.clientDataJSON),
          authenticatorData: bytesToBase64Url(asr.authenticatorData),
          signature: bytesToBase64Url(asr.signature),
          userHandle: asr.userHandle ? bytesToBase64Url(asr.userHandle) : null,
        },
      };
    }
    return common;
  }

  function defaultDeviceName() {
    const plat = (navigator.platform || '').trim();
    const ua = (navigator.userAgent || '').trim();
    const base = plat || ua || 'device';
    return base.length > 48 ? base.slice(0, 48) : base;
  }

  function errorKey(err) {
    if (!err) return 'common.error';
    const name = String(err.name || '').trim();
    const msg = String(err.message || '').trim();
    if (name === 'NotAllowedError') return 'auth.passkeys.notAllowed';
    if (name === 'AbortError') return 'auth.passkeys.aborted';
    if (name === 'NotSupportedError') return 'auth.passkeys.notSupported';
    if (name === 'SecurityError') return 'auth.passkeys.securityError';
    if (name === 'InvalidStateError') return 'auth.passkeys.invalidState';
    if (msg.includes('The operation either timed out or was not allowed')) return 'auth.passkeys.notAllowed';
    if (msg.includes('https://www.w3.org/TR/webauthn-2/')) return 'auth.passkeys.notAllowed';
    return '';
  }

  function createUserHandleHint(username) {
    const raw = String(username || '').trim().toLowerCase();
    return bytesToBase64Url(enc.encode(raw || 'user'));
  }

  window.BerkutWebAuthn = {
    supported,
    toPublicKeyCreationOptions,
    toPublicKeyRequestOptions,
    credentialToJSON,
    bytesToBase64Url,
    base64UrlToBytes,
    defaultDeviceName,
    errorKey,
    createUserHandleHint,
  };
})();
