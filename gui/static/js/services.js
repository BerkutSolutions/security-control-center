(() => {
  const STORAGE_KEY = 'berkut.services';
  let customServices = [];
  let loaded = false;

  function load() {
    if (loaded) return;
    loaded = true;
    try {
      const raw = localStorage.getItem(STORAGE_KEY);
      if (!raw) return;
      const parsed = JSON.parse(raw);
      if (Array.isArray(parsed)) {
        customServices = parsed.map(normalizeService).filter(Boolean);
      }
    } catch (err) {
      console.warn('[services] failed to load services', err);
      customServices = [];
    }
  }

  function persist() {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(customServices));
    } catch (err) {
      console.warn('[services] failed to persist services', err);
    }
  }

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

  function all() {
    load();
    const seen = new Set();
    const out = [];
    customServices.forEach(item => {
      const norm = normalizeService(item);
      if (!norm) return;
      const key = norm.code.toLowerCase();
      if (seen.has(key)) return;
      seen.add(key);
      out.push({ ...norm, builtIn: false });
    });
    return out;
  }

  function add(label) {
    const norm = normalizeService(label);
    if (!norm) return all();
    const key = norm.code.toLowerCase();
    const existing = customServices.find(s => (s.code || '').toLowerCase() === key);
    if (existing) {
      existing.label = norm.label;
    } else {
      customServices.push(norm);
    }
    persist();
    notifyChange();
    return all();
  }

  function remove(code) {
    const needle = cleanCode(code);
    if (!needle) return all();
    customServices = customServices.filter(s => cleanCode(s.code) !== needle);
    persist();
    notifyChange();
    return all();
  }

  function label(code) {
    if (!code) return '';
    const needle = cleanCode(code);
    const found = all().find(s => cleanCode(s.code) === needle);
    return found?.label || code;
  }

  function codes() {
    return all().map(s => s.code);
  }

  function notifyChange() {
    if (typeof document === 'undefined' || typeof CustomEvent === 'undefined') return;
    document.dispatchEvent(new CustomEvent('services:changed', { detail: { services: all() } }));
  }

  window.ServiceDirectory = {
    all,
    add,
    remove,
    label,
    codes,
  };
})();
