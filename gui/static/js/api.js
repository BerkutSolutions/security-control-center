const Api = (() => {
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
    const target = String(url || '').trim();
    if (status > 0 && target) {
      msg = `${msg} (HTTP ${status}, ${target})`;
    } else if (status > 0) {
      msg = `${msg} (HTTP ${status})`;
    }
    const err = new Error(msg);
    err.status = status;
    err.path = target;
    err.code = raw || statusText || '';
    return err;
  }

  async function request(method, url, body, options = null) {
    const opts = { method, headers: {}, credentials: 'include' };
    const extraHeaders = options && options.headers && typeof options.headers === 'object' ? options.headers : null;
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
      throw new Error((err && err.message ? String(err.message) : 'common.networkError').trim());
    }
    if (!res.ok) {
      const text = await res.text();
      dispatchAuthChallenge(res.status, text);
      throw buildHttpError(url, res, text);
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
        throw new Error((text || res.statusText || '').trim());
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
