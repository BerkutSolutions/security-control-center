(() => {
  const STORAGE_KEY = 'berkut.classifications';
  const LEVEL_CODES = ['PUBLIC', 'INTERNAL', 'CONFIDENTIAL', 'RESTRICTED', 'SECRET', 'TOP_SECRET', 'SPECIAL_IMPORTANCE'];
  const BASE_CODES = ['CONFIDENTIAL', 'INTERNAL', 'RESTRICTED', 'PUBLIC'];
  const CUSTOM_CODES = ['SECRET', 'TOP_SECRET', 'SPECIAL_IMPORTANCE'];
  const DEFAULT_ORDER = ['CONFIDENTIAL', 'INTERNAL', 'RESTRICTED', 'PUBLIC', 'SECRET', 'TOP_SECRET', 'SPECIAL_IMPORTANCE'];

  let customLabels = { SECRET: '', TOP_SECRET: '', SPECIAL_IMPORTANCE: '' };
  let order = DEFAULT_ORDER.slice();
  let loaded = false;

  function load() {
    if (loaded) return;
    loaded = true;
    try {
      const raw = localStorage.getItem(STORAGE_KEY);
      if (!raw) return;
      const parsed = JSON.parse(raw);
      if (parsed && typeof parsed === 'object') {
        const labels = parsed.labels || {};
        customLabels = {
          SECRET: cleanLabel(labels.SECRET),
          TOP_SECRET: cleanLabel(labels.TOP_SECRET),
          SPECIAL_IMPORTANCE: cleanLabel(labels.SPECIAL_IMPORTANCE),
        };
        const candidate = Array.isArray(parsed.order) ? parsed.order.map((c) => String(c || '').toUpperCase()) : [];
        if (candidate.length) {
          order = normalizeOrder(candidate);
        }
      } else if (Array.isArray(parsed)) {
        // Compatibility with previous storage format.
        customLabels = {
          SECRET: cleanLabel(parsed[0]),
          TOP_SECRET: cleanLabel(parsed[1]),
          SPECIAL_IMPORTANCE: cleanLabel(parsed[2]),
        };
      }
    } catch (err) {
      console.warn('[classifications] load failed', err);
      customLabels = { SECRET: '', TOP_SECRET: '', SPECIAL_IMPORTANCE: '' };
      order = DEFAULT_ORDER.slice();
    }
  }

  function normalizeOrder(input) {
    const seen = new Set();
    const next = [];
    input.forEach((code) => {
      if (!LEVEL_CODES.includes(code) || seen.has(code)) return;
      seen.add(code);
      next.push(code);
    });
    LEVEL_CODES.forEach((code) => {
      if (seen.has(code)) return;
      seen.add(code);
      next.push(code);
    });
    return next;
  }

  function persist() {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify({ labels: customLabels, order }));
    } catch (err) {
      console.warn('[classifications] persist failed', err);
    }
  }

  function notify() {
    if (typeof document === 'undefined' || typeof CustomEvent === 'undefined') return;
    document.dispatchEvent(new CustomEvent('classifications:changed', { detail: { levels: all() } }));
  }

  function cleanLabel(raw) {
    return String(raw || '').trim().replace(/\s+/g, ' ').slice(0, 80);
  }

  function defaultLabel(code, fallback) {
    const key = `docs.classification.${String(code || '').toLowerCase()}`;
    const localized = (typeof BerkutI18n !== 'undefined' && BerkutI18n.t) ? BerkutI18n.t(key) : '';
    if (localized && localized !== key) return localized;
    return fallback || code;
  }

  function genericLevelLabel(level) {
    const lang = (typeof BerkutI18n !== 'undefined' && BerkutI18n.currentLang) ? BerkutI18n.currentLang() : 'en';
    return lang === 'ru' ? `Уровень ${level}` : `Level ${level}`;
  }

  function all() {
    load();
    const items = [];
    order.forEach((code) => {
      if (BASE_CODES.includes(code)) {
        items.push({
          code,
          level: items.length,
          label: defaultLabel(code, code),
          builtIn: true,
        });
        return;
      }
      const custom = cleanLabel(customLabels[code]);
      if (!custom) return;
      items.push({
        code,
        level: items.length,
        label: custom,
        builtIn: false,
      });
    });
    return items;
  }

  function codes() {
    return all().map((item) => item.code);
  }

  function label(code) {
    load();
    const normalized = String(code || '').toUpperCase();
    if (BASE_CODES.includes(normalized)) return defaultLabel(normalized, normalized);
    if (CUSTOM_CODES.includes(normalized)) {
      const custom = cleanLabel(customLabels[normalized]);
      if (custom) return custom;
      return genericLevelLabel(levelByCode(normalized));
    }
    return defaultLabel(normalized, normalized);
  }

  function levelByCode(code) {
    const idx = LEVEL_CODES.indexOf(String(code || '').toUpperCase());
    return idx >= 0 ? idx : 0;
  }

  function labelByLevel(level) {
    const idx = Number(level);
    if (!Number.isFinite(idx) || idx < 0 || idx >= LEVEL_CODES.length) return '';
    return label(LEVEL_CODES[idx]);
  }

  function add(rawLabel) {
    load();
    const labelText = cleanLabel(rawLabel);
    if (!labelText) return { ok: false, reason: 'empty' };
    const activeCustoms = CUSTOM_CODES.filter((code) => cleanLabel(customLabels[code]));
    if (activeCustoms.some((code) => cleanLabel(customLabels[code]).toLowerCase() === labelText.toLowerCase())) {
      return { ok: false, reason: 'duplicate' };
    }
    const free = CUSTOM_CODES.find((code) => !cleanLabel(customLabels[code]));
    if (!free) return { ok: false, reason: 'limit' };
    customLabels[free] = labelText;
    persist();
    notify();
    return { ok: true };
  }

  function remove(code) {
    load();
    const normalized = String(code || '').toUpperCase();
    if (!CUSTOM_CODES.includes(normalized)) return;
    customLabels[normalized] = '';
    persist();
    notify();
  }

  function move(code, direction) {
    load();
    const normalized = String(code || '').toUpperCase();
    if (!CUSTOM_CODES.includes(normalized) || !cleanLabel(customLabels[normalized])) return;
    const step = direction === 'down' ? 1 : -1;
    const isVisible = (c) => BASE_CODES.includes(c) || !!cleanLabel(customLabels[c]);
    const fromIndex = order.indexOf(normalized);
    if (fromIndex < 0) return;
    let target = fromIndex + step;
    while (target >= 0 && target < order.length && !isVisible(order[target])) {
      target += step;
    }
    if (target < 0 || target >= order.length) return;
    const swap = order[target];
    order[target] = normalized;
    order[fromIndex] = swap;
    persist();
    notify();
  }

  window.ClassificationDirectory = {
    all,
    codes,
    label,
    labelByLevel,
    levelByCode,
    add,
    remove,
    move,
    maxCustom: CUSTOM_CODES.length,
  };
})();
