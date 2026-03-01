(() => {
  if (typeof window === 'undefined') return;

  const state = {
    loading: null,
    editor: null,
    scriptUrl: '',
    hostId: '',
    docKey: '',
    ready: false,
    docId: 0,
    mode: 'view',
  };

  function resolveScriptUrl(documentServerUrl) {
    const raw = String(documentServerUrl || '').trim();
    if (!raw) throw new Error('docs.onlyoffice.misconfigured');
    let base = raw;
    try {
      const parsed = new URL(raw, window.location.origin);
      // Prefer same-origin path only when origin matches.
      if (parsed.origin === window.location.origin) {
        base = parsed.pathname || '/';
      } else {
        base = parsed.href;
      }
    } catch (_) {
      base = raw;
    }
    base = String(base).replace(/\/+$/, '');
    return `${base}/web-apps/apps/api/documents/api.js`;
  }

  async function ensureApiScript(documentServerUrl) {
    const scriptUrl = resolveScriptUrl(documentServerUrl);
    if (window.DocsAPI && typeof window.DocsAPI.DocEditor === 'function') {
      state.scriptUrl = scriptUrl;
      return;
    }
    if (state.loading && state.scriptUrl === scriptUrl) {
      return state.loading;
    }
    state.scriptUrl = scriptUrl;
    state.loading = new Promise((resolve, reject) => {
      const existing = document.querySelector(`script[data-onlyoffice-api="1"][src="${scriptUrl}"]`);
      if (existing) {
        existing.addEventListener('load', () => resolve(), { once: true });
        existing.addEventListener('error', () => reject(new Error('docs.onlyoffice.unavailable')), { once: true });
        return;
      }
      const script = document.createElement('script');
      script.src = scriptUrl;
      script.async = true;
      script.defer = true;
      script.dataset.onlyofficeApi = '1';
      script.onload = () => resolve();
      script.onerror = () => {
        const key = scriptUrl.startsWith('/') ? 'docs.onlyoffice.unavailableHint' : 'docs.onlyoffice.unavailable';
        reject(new Error(key));
      };
      document.head.appendChild(script);
    });
    return state.loading;
  }

  function destroy() {
    if (state.editor && typeof state.editor.destroyEditor === 'function') {
      try {
        state.editor.destroyEditor();
      } catch (_) {}
    }
    state.editor = null;
    state.docKey = '';
    state.ready = false;
    state.docId = 0;
    state.mode = 'view';
    if (state.hostId) {
      const host = document.getElementById(state.hostId);
      if (host) host.innerHTML = '';
    }
  }

  async function fetchConfig(docId, mode) {
    return Api.get(`/api/docs/${docId}/office/config?mode=${encodeURIComponent(mode)}`);
  }

  async function refreshFile() {
    if (!state.editor || typeof state.editor.refreshFile !== 'function' || !state.docId) return;
    const resp = await fetchConfig(state.docId, state.mode || 'view');
    const cfg = resp.config || {};
    state.docKey = String((((cfg || {}).document || {}).key) || '').trim();
    state.editor.refreshFile(cfg);
  }

  async function open(docId, hostId, mode) {
    if (!docId || !hostId) throw new Error('docs.onlyoffice.misconfigured');
    const nextMode = mode === 'edit' ? 'edit' : 'view';
    const resp = await fetchConfig(docId, nextMode);
    const documentServerUrl = String(resp.document_server_url || '').trim();
    const cfg = resp.config || {};
    await ensureApiScript(documentServerUrl);
    if (!window.DocsAPI || typeof window.DocsAPI.DocEditor !== 'function') {
      throw new Error('docs.onlyoffice.unavailable');
    }
    const host = document.getElementById(hostId);
    if (!host) throw new Error('docs.onlyoffice.misconfigured');
    state.hostId = hostId;
    state.docId = docId;
    state.mode = nextMode;
    destroy();
    state.hostId = hostId;
    state.docId = docId;
    state.mode = nextMode;
    cfg.type = cfg.type || 'desktop';
    cfg.editorConfig = cfg.editorConfig || {};
    cfg.editorConfig.mode = nextMode;
    if (cfg.events && cfg.events.onOutdatedVersion) {
      delete cfg.events.onOutdatedVersion;
    }
    let markReady;
    const readyPromise = new Promise((resolve) => {
      markReady = () => {
        state.ready = true;
        resolve();
      };
      setTimeout(resolve, 3000);
    });
    cfg.events = Object.assign({}, cfg.events || {}, {
      onAppReady: () => markReady(),
      onDocumentReady: () => markReady(),
      onRequestRefreshFile: async () => {
        try {
          if (state.mode !== nextMode) return;
          await refreshFile();
        } catch (_) {}
      },
      onError: () => {},
    });
    state.docKey = String((((cfg || {}).document || {}).key) || '').trim();
    state.editor = new window.DocsAPI.DocEditor(hostId, cfg);
    await readyPromise;
    return state.editor;
  }

  async function forceSave(docId, reason) {
    if (!docId) throw new Error('docs.onlyoffice.misconfigured');
    const payload = {
      reason: String(reason || '').trim(),
      key: String(state.docKey || '').trim(),
    };
    if (!payload.reason) throw new Error('editor.reasonRequired');
    await Api.post(`/api/docs/${docId}/office/forcesave`, payload);
    return true;
  }

  function isReady() {
    return !!state.ready;
  }

  window.DocsOnlyOffice = { open, destroy, forceSave, isReady };
})();
