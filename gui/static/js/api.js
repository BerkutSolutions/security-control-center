const Api = (() => {
  let bgBackoffUntil = 0;
  let bgBackoffMessage = 'common.serviceUnavailable';
  let bgBackoffStatus = 0;

  function shouldNotifyDataChanged(method, url) {
    if (!method || String(method).toUpperCase() === 'GET') return false;
    const path = String(url || '');
    if (!path.startsWith('/api/')) return false;
    if (path.startsWith('/api/app/ping')) return false;
    if (path.startsWith('/api/auth/')) return false;
    if (path.startsWith('/api/app/view')) return false;
    return true;
  }

  function dispatchDataChanged(method, url) {
    if (typeof window === 'undefined') return;
    if (!shouldNotifyDataChanged(method, url)) return;
    window.dispatchEvent(new CustomEvent('app:data-changed', {
      detail: { method: String(method || '').toUpperCase(), url: String(url || '') }
    }));
  }

  function csrf() {
    const m = document.cookie.match(/berkut_csrf=([^;]+)/);
    return m ? decodeURIComponent(m[1]) : '';
  }

  function buildHttpError(url, res, text) {
    const status = Number(res && res.status ? res.status : 0);
    const statusText = String((res && res.statusText) || '').trim();
    const raw = String(text || '').trim();
    let msg = raw || statusText || 'common.error';
    const lower = msg.toLowerCase();
    if (lower === 'server error' || lower === 'internal server error') {
      msg = 'common.serverError';
    }
    if (lower === 'invalid credentials') {
      msg = 'auth.invalidCredentials';
    }
    if ((lower === 'forbidden' || lower === 'access denied') && status === 403) {
      msg = 'common.accessDenied';
    }
    const target = String(url || '').trim();
    const err = new Error(msg);
    err.status = status;
    err.path = target;
    err.code = raw || statusText || '';
    return err;
  }

  async function request(method, url, body, options = null) {
    const opts = { method, headers: {}, credentials: 'include' };
    const extraHeaders = options && options.headers && typeof options.headers === 'object' ? options.headers : null;
    const isBackground = extraHeaders && `${extraHeaders['X-Berkut-Background'] || ''}` === '1';
    if (isBackground && Date.now() < bgBackoffUntil) {
      const bgErr = new Error(bgBackoffMessage || 'common.serviceUnavailable');
      bgErr.status = Number(bgBackoffStatus || 0);
      bgErr.path = String(url || '');
      throw bgErr;
    }
    const lang = (localStorage.getItem('berkut_lang') || '').trim();
    if (lang) opts.headers['Accept-Language'] = lang;
    if (extraHeaders) {
      Object.keys(extraHeaders).forEach((key) => {
        if (!key) return;
        opts.headers[key] = extraHeaders[key];
      });
    }
    if (body) {
      opts.body = JSON.stringify(body);
      opts.headers['Content-Type'] = 'application/json';
    }
    if (method !== 'GET') {
      opts.headers['X-CSRF-Token'] = csrf();
    }
    let res;
    try {
      res = await fetch(url, opts);
    } catch (err) {
      const message = (err && err.message ? String(err.message) : 'common.networkError').trim();
      if (isBackground) {
        bgBackoffUntil = Date.now() + 15000;
        bgBackoffMessage = message || 'common.networkError';
        bgBackoffStatus = 0;
      }
      throw new Error(message);
    }
    if (!res.ok) {
      const text = await res.text();
      if (isBackground && (res.status === 401 || res.status === 502 || res.status === 503 || res.status === 504)) {
        bgBackoffUntil = Date.now() + (res.status === 401 ? 45000 : 20000);
        bgBackoffMessage = String(text || '').trim() || (res.status === 401 ? 'unauthorized' : 'common.serviceUnavailable');
        bgBackoffStatus = res.status;
      }
      if (!isBackground) {
        dispatchAuthChallenge(res.status, text);
      }
      throw buildHttpError(url, res, text);
    }
    if (isBackground) {
      bgBackoffUntil = 0;
      bgBackoffMessage = 'common.serviceUnavailable';
      bgBackoffStatus = 0;
    }
    dispatchDataChanged(method, url);
    const ct = res.headers.get('content-type') || '';
    if (ct.includes('application/json')) return res.json();
    return res.text();
  }

  return {
    get: (url, options) => request('GET', url, null, options),
    post: (url, body, options) => request('POST', url, body, options),
    put: (url, body, options) => request('PUT', url, body, options),
    del: (url, body, options) => request('DELETE', url, body, options),
    upload: async (url, formData) => {
      const opts = { method: 'POST', body: formData, credentials: 'include', headers: { 'X-CSRF-Token': csrf() } };
      let res;
      try {
        res = await fetch(url, opts);
      } catch (err) {
        throw new Error((err && err.message ? String(err.message) : 'common.networkError').trim());
      }
      if (!res.ok) {
        const text = await res.text();
        if (`${opts.headers['X-Berkut-Background'] || ''}` !== '1') {
          dispatchAuthChallenge(res.status, text);
        }
        throw buildHttpError(url, res, text);
      }
      dispatchDataChanged('POST', url);
      const ct = res.headers.get('content-type') || '';
      if (ct.includes('application/json')) return res.json();
      return res.text();
    }
  };

  function dispatchAuthChallenge(status, text) {
    if (typeof window === 'undefined') return;
    const code = String(text || '').trim();
    if (code !== 'auth.stepup.required' && code !== 'auth.stepup.locked') return;
    window.dispatchEvent(new CustomEvent('app:auth-challenge', {
      detail: {
        code,
        status: Number(status || 0),
      }
    }));
  }
})();
