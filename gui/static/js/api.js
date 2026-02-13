const Api = (() => {
  function csrf() {
    const m = document.cookie.match(/berkut_csrf=([^;]+)/);
    return m ? decodeURIComponent(m[1]) : '';
  }

  async function request(method, url, body) {
    const opts = { method, headers: {}, credentials: 'include' };
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
      throw new Error((text || res.statusText || '').trim());
    }
    const ct = res.headers.get('content-type') || '';
    if (ct.includes('application/json')) return res.json();
    return res.text();
  }

  return {
    get: (url) => request('GET', url),
    post: (url, body) => request('POST', url, body),
    put: (url, body) => request('PUT', url, body),
    del: (url, body) => request('DELETE', url, body),
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
      const ct = res.headers.get('content-type') || '';
      if (ct.includes('application/json')) return res.json();
      return res.text();
    }
  };
})();
