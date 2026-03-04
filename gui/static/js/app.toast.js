const AppToast = (() => {
  const HIDE_MS = 5000;
  let container = null;
  let observer = null;
  let lastDigest = '';

  function init() {
    ensureContainer();
    patchWindowAlert();
    bindAlertObserver();
    flushExistingAlerts();
  }

  function show(message, type = 'error', ttl = HIDE_MS, options = {}) {
    const text = String(message || '').trim();
    if (!text) return;
    const toast = document.createElement('div');
    toast.className = `app-toast ${type === 'success' ? 'success' : 'error'}`;
    const content = document.createElement('div');
    content.className = 'app-toast-text';
    content.textContent = normalizeMessage(text);
    const close = document.createElement('button');
    close.type = 'button';
    close.className = 'app-toast-close';
    close.setAttribute('aria-label', i18n('common.close'));
    close.textContent = 'x';
    close.onclick = () => hideToast(toast);
    toast.appendChild(content);
    toast.appendChild(close);
    ensureContainer().appendChild(toast);
    requestAnimationFrame(() => toast.classList.add('show'));
    const timeout = setTimeout(() => hideToast(toast), Math.max(1200, Number(ttl || HIDE_MS)));
    toast.dataset.timeoutId = String(timeout);
    emitToastNotification(content.textContent || text, type, options);
  }

  function ensureContainer() {
    if (container) return container;
    container = document.getElementById('app-toast-container');
    if (!container) {
      container = document.createElement('div');
      container.id = 'app-toast-container';
      document.body.appendChild(container);
    }
    return container;
  }

  function patchWindowAlert() {
    if (window.__appToastAlertPatched) return;
    window.__appToastAlertPatched = true;
    const nativeAlert = window.alert;
    window.alert = function patchedAlert(msg) {
      show(msg, 'error');
      return nativeAlert && typeof nativeAlert === 'function' ? undefined : undefined;
    };
  }

  function bindAlertObserver() {
    if (observer || !document.body) return;
    observer = new MutationObserver(handleMutations);
    observer.observe(document.body, {
      attributes: true,
      subtree: true,
      childList: true,
      characterData: true,
      attributeFilter: ['hidden', 'class']
    });
  }

  function flushExistingAlerts() {
    document.querySelectorAll('.alert').forEach((el) => maybeToastFromAlert(el));
  }

  function handleMutations(mutations) {
    for (const m of mutations) {
      if (m.type === 'childList') {
        m.addedNodes.forEach((node) => {
          if (!(node instanceof HTMLElement)) return;
          if (node.classList.contains('alert')) maybeToastFromAlert(node);
          node.querySelectorAll?.('.alert').forEach((el) => maybeToastFromAlert(el));
        });
      } else if (m.type === 'attributes') {
        const el = m.target;
        if (el instanceof HTMLElement && el.classList.contains('alert')) {
          maybeToastFromAlert(el);
        }
      } else if (m.type === 'characterData') {
        const parent = m.target && m.target.parentElement ? m.target.parentElement.closest('.alert') : null;
        if (parent instanceof HTMLElement) {
          maybeToastFromAlert(parent);
        }
      }
    }
  }

  function maybeToastFromAlert(el) {
    if (!el || el.hidden || el.dataset.keepInlineToast === '1') return;
    const text = String(el.textContent || '').trim();
    if (!text) {
      el.hidden = true;
      return;
    }
    const digest = `${el.id || ''}|${el.className}|${text}`;
    if (digest === lastDigest) return;
    lastDigest = digest;
    const isSuccess = el.classList.contains('success');
    show(text, isSuccess ? 'success' : 'error', HIDE_MS, {
      source: 'inline-alert',
    });
    el.hidden = true;
  }

  function hideToast(toast) {
    if (!toast) return;
    const timeoutId = Number(toast.dataset.timeoutId || 0);
    if (timeoutId) clearTimeout(timeoutId);
    toast.classList.remove('show');
    setTimeout(() => toast.remove(), 240);
  }

  function emitToastNotification(message, type, options) {
    if (typeof window === 'undefined') return;
    const level = String(type || '').toLowerCase();
    if (level === 'success') return;
    const path = options && typeof options.path === 'string' ? options.path : '';
    const title = options && options.title ? String(options.title) : i18n('app.notifications.errorTitle');
    const source = options && options.source ? String(options.source) : 'toast';
    window.dispatchEvent(new CustomEvent('app:toast-notification', {
      detail: {
        key: `toast:${source}:${digestMessage(message, path)}`,
        section: 'errors',
        path,
        title,
        message: normalizeMessage(message),
        ts: Date.now(),
      }
    }));
  }

  function digestMessage(message, path) {
    return `${String(message || '').trim()}|${String(path || '').trim()}`.toLowerCase();
  }

  function normalizeMessage(message) {
    const raw = String(message || '').trim();
    const detailsMatch = raw.match(/^([a-z0-9_.-]+)\s*(\(.+\))$/i);
    if (detailsMatch && typeof BerkutI18n !== 'undefined' && BerkutI18n.t) {
      const base = BerkutI18n.t(detailsMatch[1]);
      if (base && base !== detailsMatch[1]) return `${base} ${detailsMatch[2]}`.trim();
    }
    if (typeof BerkutI18n !== 'undefined' && BerkutI18n.t) {
      const translated = BerkutI18n.t(raw);
      if (translated && translated !== raw) return translated;
    }
    return raw;
  }

  function i18n(key) {
    if (typeof BerkutI18n !== 'undefined' && BerkutI18n.t) {
      return BerkutI18n.t(key);
    }
    return key;
  }

  return { init, show };
})();

if (typeof window !== 'undefined') {
  window.AppToast = AppToast;
}
