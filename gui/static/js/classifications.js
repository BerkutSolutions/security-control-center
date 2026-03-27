(() => {
  const LEGACY_STORAGE_KEY = 'berkut.classifications';
  const API_ENDPOINT = '/api/catalog/classifications';
  const LEVEL_CODES = ['PUBLIC', 'INTERNAL', 'CONFIDENTIAL', 'RESTRICTED', 'SECRET', 'TOP_SECRET', 'SPECIAL_IMPORTANCE'];
  const BASE_CODES = ['CONFIDENTIAL', 'INTERNAL', 'RESTRICTED', 'PUBLIC'];
  const CUSTOM_CODES = ['SECRET', 'TOP_SECRET', 'SPECIAL_IMPORTANCE'];
  const DEFAULT_ORDER = ['CONFIDENTIAL', 'INTERNAL', 'RESTRICTED', 'PUBLIC', 'SECRET', 'TOP_SECRET', 'SPECIAL_IMPORTANCE'];

  let customLabels = { SECRET: '', TOP_SECRET: '', SPECIAL_IMPORTANCE: '' };
  let order = DEFAULT_ORDER.slice();
  let loaded = false;
  let loadingPromise = null;
  let retryTimer = null;

  function cleanLabel(raw) {
    return String(raw || '').trim().replace(/\s+/g, ' ').slice(0, 80);
  }

  function normalizeOrder(input) {
    const seen = new Set();
    const next = [];
    (input || []).forEach((code) => {
      const c = String(code || '').toUpperCase();
      if (!LEVEL_CODES.includes(c) || seen.has(c)) return;
      seen.add(c);
      next.push(c);
    });
    LEVEL_CODES.forEach((code) => {
      if (seen.has(code)) return;
      seen.add(code);
      next.push(code);
    });
    return next;
  }

  function readLegacy() {
    try {
      const raw = localStorage.getItem(LEGACY_STORAGE_KEY);
      if (!raw) return null;
      const parsed = JSON.parse(raw);
      if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
        return {
          labels: {
            SECRET: cleanLabel(parsed?.labels?.SECRET),
            TOP_SECRET: cleanLabel(parsed?.labels?.TOP_SECRET),
            SPECIAL_IMPORTANCE: cleanLabel(parsed?.labels?.SPECIAL_IMPORTANCE),
          },
          order: normalizeOrder(Array.isArray(parsed?.order) ? parsed.order : DEFAULT_ORDER),
        };
      }
      if (Array.isArray(parsed)) {
        return {
          labels: {
            SECRET: cleanLabel(parsed[0]),
            TOP_SECRET: cleanLabel(parsed[1]),
            SPECIAL_IMPORTANCE: cleanLabel(parsed[2]),
          },
          order: DEFAULT_ORDER.slice(),
        };
      }
      return null;
    } catch (_) {
      return null;
    }
  }

  function clearLegacy() {
    try {
      localStorage.removeItem(LEGACY_STORAGE_KEY);
    } catch (_) {
      // ignore
    }
  }

  function notify() {
    if (typeof document === 'undefined' || typeof CustomEvent === 'undefined') return;
    document.dispatchEvent(new CustomEvent('classifications:changed', { detail: { levels: all() } }));
  }

  async function persistRemote() {
    if (!hasApiPut()) throw new Error('common.serviceUnavailable');
    await Api.put(API_ENDPOINT, { labels: customLabels, order });
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
        customLabels = {
          SECRET: cleanLabel(payload?.labels?.SECRET),
          TOP_SECRET: cleanLabel(payload?.labels?.TOP_SECRET),
          SPECIAL_IMPORTANCE: cleanLabel(payload?.labels?.SPECIAL_IMPORTANCE),
        };
        order = normalizeOrder(Array.isArray(payload?.order) ? payload.order : DEFAULT_ORDER);
        const legacy = readLegacy();
        if (legacy) {
          let changed = false;
          CUSTOM_CODES.forEach((code) => {
            if (!customLabels[code] && legacy.labels[code]) {
              customLabels[code] = legacy.labels[code];
              changed = true;
            }
          });
          if (!Array.isArray(payload?.order) || !payload.order.length) {
            order = normalizeOrder(legacy.order);
            changed = true;
          }
          if (changed) {
            await persistRemote();
          }
          clearLegacy();
        }
      } catch (err) {
        console.warn('[classifications] load failed', err);
        customLabels = { SECRET: '', TOP_SECRET: '', SPECIAL_IMPORTANCE: '' };
        order = DEFAULT_ORDER.slice();
      } finally {
        loaded = true;
        loadingPromise = null;
      }
      notify();
      return true;
    })();
    return loadingPromise;
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
    ensureLoaded();
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
    ensureLoaded();
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

  async function add(rawLabel) {
    await ensureLoaded();
    const labelText = cleanLabel(rawLabel);
    if (!labelText) return { ok: false, reason: 'empty' };
    const activeCustoms = CUSTOM_CODES.filter((code) => cleanLabel(customLabels[code]));
    if (activeCustoms.some((code) => cleanLabel(customLabels[code]).toLowerCase() === labelText.toLowerCase())) {
      return { ok: false, reason: 'duplicate' };
    }
    const free = CUSTOM_CODES.find((code) => !cleanLabel(customLabels[code]));
    if (!free) return { ok: false, reason: 'limit' };
    const snapshot = JSON.parse(JSON.stringify({ labels: customLabels, order }));
    customLabels[free] = labelText;
    try {
      await persistRemote();
      notify();
    } catch (err) {
      customLabels = snapshot.labels;
      order = snapshot.order;
      throw err;
    }
    return { ok: true };
  }

  async function remove(code) {
    await ensureLoaded();
    const normalized = String(code || '').toUpperCase();
    if (!CUSTOM_CODES.includes(normalized)) return;
    const snapshot = JSON.parse(JSON.stringify({ labels: customLabels, order }));
    customLabels[normalized] = '';
    try {
      await persistRemote();
      notify();
    } catch (err) {
      customLabels = snapshot.labels;
      order = snapshot.order;
      throw err;
    }
  }

  async function move(code, direction) {
    await ensureLoaded();
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
    const snapshot = JSON.parse(JSON.stringify({ labels: customLabels, order }));
    const swap = order[target];
    order[target] = normalized;
    order[fromIndex] = swap;
    try {
      await persistRemote();
      notify();
    } catch (err) {
      customLabels = snapshot.labels;
      order = snapshot.order;
      throw err;
    }
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
    refresh: ensureLoaded,
  };

  ensureLoaded().catch(() => {});
})();

