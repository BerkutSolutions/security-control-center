(() => {
  const LEGACY_STORAGE_KEY = 'berkut.tags';
  const API_ENDPOINT = '/api/catalog/tags';
  const DEFAULT_TAGS = [
    { code: 'COMMERCIAL_SECRET', label: 'Коммерческая тайна' },
    { code: 'PERSONAL_DATA', label: 'ПДн' },
    { code: 'CRITICAL_INFRASTRUCTURE', label: 'КИИ' },
    { code: 'FEDERAL_LAW_152', label: 'ФЗ 152' },
    { code: 'FEDERAL_LAW_149', label: 'ФЗ 149' },
    { code: 'FEDERAL_LAW_187', label: 'ФЗ 187' },
    { code: 'FEDERAL_LAW_63', label: 'ФЗ 63' },
    { code: 'PCI_DSS', label: 'PCI DSS' },
  ];
  let customTags = [];
  let loaded = false;
  let loadingPromise = null;
  let retryTimer = null;

  function cleanCode(raw) {
    const base = (raw || '').toString().trim();
    if (!base) return '';
    const normalized = base.replace(/[^\p{L}\p{N}\s_-]/gu, ' ').replace(/\s+/g, ' ').trim();
    const slug = normalized.replace(/\s+/g, '_').replace(/__+/g, '_');
    return (slug || normalized).toUpperCase();
  }

  function normalizeTag(input) {
    if (!input) return null;
    if (typeof input === 'string') {
      const label = input.toString().trim();
      const code = cleanCode(label);
      if (!code || !label) return null;
      return { code, label };
    }
    const code = cleanCode(input.code || input.label || '');
    const label = (input.label || input.code || '').toString().trim();
    if (!code || !label) return null;
    return { code, label };
  }

  function readLegacyTags() {
    try {
      const raw = localStorage.getItem(LEGACY_STORAGE_KEY);
      if (!raw) return [];
      const parsed = JSON.parse(raw);
      if (!Array.isArray(parsed)) return [];
      return parsed.map(normalizeTag).filter(Boolean);
    } catch (_) {
      return [];
    }
  }

  function clearLegacyTags() {
    try {
      localStorage.removeItem(LEGACY_STORAGE_KEY);
    } catch (_) {
      // ignore
    }
  }

  function mergeTags(base, incoming) {
    const seen = new Set();
    const out = [];
    [...(base || []), ...(incoming || [])].forEach((item) => {
      const norm = normalizeTag(item);
      if (!norm) return;
      const key = norm.code.toLowerCase();
      if (seen.has(key)) return;
      seen.add(key);
      out.push(norm);
    });
    return out;
  }

  function isDefault(code) {
    const needle = cleanCode(code);
    return DEFAULT_TAGS.some((t) => cleanCode(t.code) === needle);
  }

  function notifyChange() {
    if (typeof document === 'undefined' || typeof CustomEvent === 'undefined') return;
    document.dispatchEvent(new CustomEvent('tags:changed', { detail: { tags: all() } }));
  }

  async function persistRemote() {
    if (!hasApiPut()) throw new Error('common.serviceUnavailable');
    await Api.put(API_ENDPOINT, { items: customTags });
  }

  function hasApiGet() {
    return typeof Api !== 'undefined' && typeof Api.get === 'function';
  }

  function hasApiPut() {
    return typeof Api !== 'undefined' && typeof Api.put === 'function';
  }

  function scheduleRetry() {
    if (retryTimer || loaded) return;
    retryTimer = window.setTimeout(() => {
      retryTimer = null;
      ensureLoaded().catch(() => {});
    }, 500);
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
        customTags = (Array.isArray(payload?.items) ? payload.items : []).map(normalizeTag).filter(Boolean);
        const legacy = readLegacyTags();
        if (legacy.length) {
          customTags = mergeTags(customTags, legacy).filter((tag) => !isDefault(tag.code));
          await persistRemote();
          clearLegacyTags();
        }
      } catch (err) {
        console.warn('[tags] failed to load tags', err);
        customTags = [];
      } finally {
        loaded = true;
        loadingPromise = null;
      }
      notifyChange();
      return true;
    })();
    return loadingPromise;
  }

  function all() {
    ensureLoaded();
    const seen = new Set();
    const result = [];
    DEFAULT_TAGS.forEach((tag) => {
      const norm = normalizeTag(tag);
      if (!norm) return;
      const key = norm.code.toLowerCase();
      if (seen.has(key)) return;
      seen.add(key);
      result.push({ ...norm, builtIn: true });
    });
    customTags.forEach((tag) => {
      const norm = normalizeTag(tag);
      if (!norm) return;
      const key = norm.code.toLowerCase();
      if (seen.has(key)) return;
      seen.add(key);
      result.push({ ...norm, builtIn: false });
    });
    return result;
  }

  async function add(label) {
    await ensureLoaded();
    const norm = normalizeTag(label);
    if (!norm || isDefault(norm.code)) return all();
    const snapshot = JSON.parse(JSON.stringify(customTags));
    const key = norm.code.toLowerCase();
    const existing = customTags.find((t) => cleanCode(t.code).toLowerCase() === key);
    if (existing) existing.label = norm.label;
    else customTags.push(norm);
    try {
      await persistRemote();
      notifyChange();
    } catch (err) {
      customTags = snapshot;
      console.warn('[tags] failed to persist tags', err);
      throw err;
    }
    return all();
  }

  async function remove(code) {
    await ensureLoaded();
    if (!code || isDefault(code)) return all();
    const snapshot = JSON.parse(JSON.stringify(customTags));
    const needle = cleanCode(code);
    customTags = customTags.filter((t) => cleanCode(t.code) !== needle);
    try {
      await persistRemote();
      notifyChange();
    } catch (err) {
      customTags = snapshot;
      console.warn('[tags] failed to persist tags', err);
      throw err;
    }
    return all();
  }

  function label(code) {
    if (!code) return '';
    const needle = cleanCode(code);
    const tag = all().find((t) => cleanCode(t.code) === needle);
    const i18nKey = `docs.tag.${needle.toLowerCase()}`;
    const localized = (typeof BerkutI18n !== 'undefined' && BerkutI18n.t) ? BerkutI18n.t(i18nKey) : null;
    if (localized && localized !== i18nKey) return localized;
    if (tag && tag.label) return tag.label;
    return code;
  }

  function codes() {
    return all().map((t) => t.code);
  }

  window.TagDirectory = {
    all,
    add,
    remove,
    label,
    codes,
    isDefault: (code) => isDefault(code),
    refresh: ensureLoaded,
  };

  ensureLoaded().catch(() => {});
})();

