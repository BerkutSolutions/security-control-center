(() => {
  const API_ENDPOINT = '/api/services';
  const LEGACY_STORAGE_KEY = 'berkut.services';
  let customServices = [];
  let loaded = false;
  let loadingPromise = null;
  let retryTimer = null;

  function cleanCode(raw) {
    const base = (raw || '').toString().trim();
    if (!base) return '';
    const normalized = base
      .replace(/[^\p{L}\p{N}\s_-]/gu, ' ')
      .replace(/\s+/g, ' ')
      .trim();
    const slug = normalized.replace(/\s+/g, '_').replace(/__+/g, '_');
    return (slug || normalized).toUpperCase();
  }

  function normalizeService(input) {
    if (!input) return null;
    if (typeof input === 'string') {
      const label = input.toString().trim();
      const code = cleanCode(label);
      if (!label || !code) return null;
      return { code, label };
    }
    const code = cleanCode(input.code || input.label || '');
    const label = (input.label || input.code || '').toString().trim();
    if (!code || !label) return null;
    return { code, label };
  }

  function notifyChange() {
    if (typeof document === 'undefined' || typeof CustomEvent === 'undefined') return;
    document.dispatchEvent(new CustomEvent('services:changed', { detail: { services: all() } }));
  }

  function scheduleRetry() {
    if (retryTimer || loaded) return;
    retryTimer = window.setTimeout(() => {
      retryTimer = null;
      ensureLoaded().catch(() => {});
    }, 500);
  }

  function hasApiGet() {
    return typeof Api !== 'undefined' && typeof Api.get === 'function';
  }

  function hasApiPut() {
    return typeof Api !== 'undefined' && typeof Api.put === 'function';
  }

  async function ensureLoaded() {
    if (loaded) return true;
    if (loadingPromise) return loadingPromise;
    if (!hasApiGet()) {
      scheduleRetry();
      return false;
    }
    loadingPromise = (async () => {
      try {
        const payload = await Api.get(API_ENDPOINT);
        const items = Array.isArray(payload?.items) ? payload.items : [];
        customServices = items.map(normalizeService).filter(Boolean);
        const legacy = readLegacyServices();
        if (legacy.length) {
          customServices = mergeServices(customServices, legacy);
          await persistRemote();
          clearLegacyServices();
        }
      } catch (err) {
        console.warn('[services] failed to load services', err);
        customServices = [];
      } finally {
        loaded = true;
        loadingPromise = null;
      }
      notifyChange();
      return true;
    })();
    return loadingPromise;
  }

  async function persistRemote() {
    if (!hasApiPut()) throw new Error('common.serviceUnavailable');
    await Api.put(API_ENDPOINT, { items: customServices });
  }

  function readLegacyServices() {
    try {
      const raw = localStorage.getItem(LEGACY_STORAGE_KEY);
      if (!raw) return [];
      const parsed = JSON.parse(raw);
      if (!Array.isArray(parsed)) return [];
      return parsed.map(normalizeService).filter(Boolean);
    } catch (_) {
      return [];
    }
  }

  function clearLegacyServices() {
    try {
      localStorage.removeItem(LEGACY_STORAGE_KEY);
    } catch (_) {
      // ignore
    }
  }

  function mergeServices(base, incoming) {
    const out = [];
    const seen = new Set();
    [...(base || []), ...(incoming || [])].forEach((item) => {
      const norm = normalizeService(item);
      if (!norm) return;
      const key = norm.code.toLowerCase();
      if (seen.has(key)) return;
      seen.add(key);
      out.push(norm);
    });
    return out;
  }

  function all() {
    ensureLoaded();
    const seen = new Set();
    const out = [];
    customServices.forEach((item) => {
      const norm = normalizeService(item);
      if (!norm) return;
      const key = norm.code.toLowerCase();
      if (seen.has(key)) return;
      seen.add(key);
      out.push({ ...norm, builtIn: false });
    });
    return out;
  }

  async function add(label) {
    await ensureLoaded();
    const norm = normalizeService(label);
    if (!norm) return all();
    const snapshot = JSON.parse(JSON.stringify(customServices));
    const key = norm.code.toLowerCase();
    const existing = customServices.find((s) => (s.code || '').toLowerCase() === key);
    if (existing) existing.label = norm.label;
    else customServices.push(norm);
    try {
      await persistRemote();
      notifyChange();
    } catch (err) {
      customServices = snapshot;
      console.warn('[services] failed to persist services', err);
      throw err;
    }
    return all();
  }

  async function remove(code) {
    await ensureLoaded();
    const needle = cleanCode(code);
    if (!needle) return all();
    const snapshot = JSON.parse(JSON.stringify(customServices));
    customServices = customServices.filter((s) => cleanCode(s.code) !== needle);
    try {
      await persistRemote();
      notifyChange();
    } catch (err) {
      customServices = snapshot;
      console.warn('[services] failed to persist services', err);
      throw err;
    }
    return all();
  }

  function label(code) {
    if (!code) return '';
    const needle = cleanCode(code);
    const found = all().find((s) => cleanCode(s.code) === needle);
    return found?.label || code;
  }

  function codes() {
    return all().map((s) => s.code);
  }

  window.ServiceDirectory = {
    all,
    add,
    remove,
    label,
    codes,
    refresh: ensureLoaded,
  };

  ensureLoaded().catch(() => {});
})();
