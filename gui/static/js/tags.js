(() => {
  const STORAGE_KEY = 'berkut.tags';
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

  function load() {
    if (loaded) return;
    loaded = true;
    try {
      const raw = localStorage.getItem(STORAGE_KEY);
      if (!raw) return;
      const parsed = JSON.parse(raw);
      if (Array.isArray(parsed)) {
        customTags = parsed
          .map(normalizeTag)
          .filter(Boolean);
      }
    } catch (err) {
      console.warn('[tags] failed to load tags', err);
      customTags = [];
    }
  }

  function persist() {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(customTags));
    } catch (err) {
      console.warn('[tags] failed to persist tags', err);
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

  function normalizeTag(input) {
    if (!input) return null;
    if (typeof input === 'string') {
      const label = input.toString().trim();
      const code = cleanCode(label);
      if (!code) return null;
      return { code, label };
    }
    const code = cleanCode(input.code || input.label || '');
    const label = (input.label || input.code || '').toString().trim();
    if (!code || !label) return null;
    return { code, label };
  }

  function isDefault(code) {
    const needle = cleanCode(code);
    return DEFAULT_TAGS.some(t => cleanCode(t.code) === needle);
  }

  function all() {
    load();
    const seen = new Set();
    const result = [];
    DEFAULT_TAGS.forEach(tag => {
      const norm = normalizeTag(tag);
      if (!norm) return;
      const key = norm.code.toLowerCase();
      if (seen.has(key)) return;
      seen.add(key);
      result.push({ ...norm, builtIn: true });
    });
    customTags.forEach(tag => {
      const norm = normalizeTag(tag);
      if (!norm) return;
      const key = norm.code.toLowerCase();
      if (seen.has(key)) return;
      seen.add(key);
      result.push({ ...norm, builtIn: false });
    });
    return result;
  }

  function add(label) {
    const norm = normalizeTag(label);
    if (!norm) return all();
    if (isDefault(norm.code)) return all();
    const key = norm.code.toLowerCase();
    const existing = customTags.find(t => cleanCode(t.code) === norm.code);
    if (existing) {
      existing.label = norm.label;
    } else {
      customTags.push(norm);
    }
    persist();
    notifyChange();
    return all();
  }

  function remove(code) {
    if (!code || isDefault(code)) return all();
    const needle = cleanCode(code);
    customTags = customTags.filter(t => cleanCode(t.code) !== needle);
    persist();
    notifyChange();
    return all();
  }

  function label(code) {
    if (!code) return '';
    const needle = cleanCode(code);
    const tag = all().find(t => cleanCode(t.code) === needle);
    const i18nKey = `docs.tag.${needle.toLowerCase()}`;
    const localized = (typeof BerkutI18n !== 'undefined' && BerkutI18n.t) ? BerkutI18n.t(i18nKey) : null;
    if (localized && localized !== i18nKey) return localized;
    if (tag && tag.label) return tag.label;
    return code;
  }

  function codes() {
    return all().map(t => t.code);
  }

  function notifyChange() {
    if (typeof document === 'undefined' || typeof CustomEvent === 'undefined') return;
    document.dispatchEvent(new CustomEvent('tags:changed', { detail: { tags: all() } }));
  }

  window.TagDirectory = {
    all,
    add,
    remove,
    label,
    codes,
    isDefault: (code) => isDefault(code),
  };
})();
