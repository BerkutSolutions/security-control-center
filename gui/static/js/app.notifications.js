const AppNotifications = (() => {
  const POLL_MS = 15000;
  const REFRESH_DEBOUNCE_MS = 350;
  const MAX_ITEMS_PER_SOURCE = 8;
  const STORAGE_DISMISSED = 'app.notifications.dismissed.v1';
  const STORAGE_ERRORS = 'app.notifications.errors.v1';
  const MAX_STORED_ERRORS = 80;

  let menuPaths = new Set();
  let timer = null;
  let debounceTimer = null;
  let inFlight = false;
  let queued = false;
  let dismissedKeys = new Set();
  let transientItems = [];

  const counts = {
    docs: 0,
    approvals: 0,
    incidents: 0,
    tasks: 0,
    monitoring: 0,
  };

  let items = [];
  const bgOpts = { headers: { 'X-Berkut-Background': '1' } };

  function init(paths = []) {
    menuPaths = new Set(paths || []);
    loadState();
    bindSignals();
    emitChanged();
    onMenuRendered();
    scheduleRefresh(0);
    if (timer) clearInterval(timer);
    timer = setInterval(() => scheduleRefresh(0), POLL_MS);
  }

  function bindSignals() {
    if (window.__appNotificationsBound) return;
    window.__appNotificationsBound = true;
    window.addEventListener('app:data-changed', () => scheduleRefresh(REFRESH_DEBOUNCE_MS));
    window.addEventListener('focus', () => scheduleRefresh(0));
    document.addEventListener('visibilitychange', () => {
      if (document.visibilityState === 'visible') scheduleRefresh(0);
    });
    window.addEventListener('app:toast-notification', (e) => {
      const item = e && e.detail ? e.detail : null;
      if (!item || !item.key) return;
      const key = String(item.key);
      if (dismissedKeys.has(key)) return;
      transientItems = sortItems([item, ...transientItems]).slice(0, 24);
      persistErrorItems();
      items = dedupeItems(sortItems([...items, item]));
      emitChanged();
    });
  }

  function scheduleRefresh(delayMs) {
    if (debounceTimer) clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => {
      refresh().catch(() => {});
    }, Math.max(0, delayMs || 0));
  }

  async function refresh() {
    if (inFlight) {
      queued = true;
      return;
    }
    inFlight = true;
    try {
      const list = [];
      const jobs = [];
      if (menuPaths.has('docs')) jobs.push(loadDocsCount(list));
      if (menuPaths.has('approvals')) jobs.push(loadApprovalsCount(list));
      if (menuPaths.has('incidents')) jobs.push(loadIncidentsCount(list));
      if (menuPaths.has('tasks')) jobs.push(loadTasksCount(list));
      if (menuPaths.has('monitoring')) jobs.push(loadMonitoringDownCount(list));
      await Promise.all(jobs);
      const merged = dedupeItems([...list, ...transientItems]);
      items = sortItems(merged).filter((it) => !dismissedKeys.has(it.key));
      transientItems = transientItems.filter((it) => !dismissedKeys.has(it.key));
      onMenuRendered();
      emitChanged();
    } finally {
      inFlight = false;
      if (queued) {
        queued = false;
        scheduleRefresh(120);
      }
    }
  }

  async function loadDocsCount(list) {
    try {
      const res = await Api.get('/api/docs?mine=1&status_in=draft,review,returned', bgOpts);
      const docs = Array.isArray(res.items) ? res.items : [];
      const mine = docs.filter((d) => {
        const st = normalizeStatus(d && d.status);
        return st === 'draft' || st === 'review' || st === 'returned';
      });
      counts.docs = mine.length;
      mine.slice(0, MAX_ITEMS_PER_SOURCE).forEach((doc) => {
        const title = String(doc.title || `#${doc.id || ''}`).trim();
        const status = normalizeStatus(doc.status);
        list.push({
          key: `docs:${doc.id}:${status}`,
          section: 'docs',
          path: '/docs',
          target: '/docs',
          title,
          message: statusText(status),
          ts: new Date(doc.updated_at || doc.created_at || 0).getTime() || 0,
        });
      });
    } catch (_) {
      counts.docs = 0;
    }
  }

  async function loadApprovalsCount(list) {
    try {
      const res = await Api.get('/api/approvals?status=review', bgOpts);
      const workflow = Array.isArray(res.items) ? res.items : [];
      const exportItems = Array.isArray(res.export_items) ? res.export_items : [];
      const exportReview = exportItems.filter((x) => normalizeStatus(x && x.status) === 'review');
      counts.approvals = workflow.length + exportReview.length;

      workflow.slice(0, MAX_ITEMS_PER_SOURCE).forEach((a) => {
        list.push({
          key: `approval:w:${a.id}`,
          section: 'approvals',
          path: '/approvals',
          target: '/approvals',
          title: i18n('nav.approvals'),
          message: `#${a.doc_id || ''}`,
          ts: new Date(a.updated_at || a.created_at || 0).getTime() || 0,
        });
      });
      exportReview.slice(0, MAX_ITEMS_PER_SOURCE).forEach((a) => {
        list.push({
          key: `approval:e:${a.id}`,
          section: 'approvals',
          path: '/approvals',
          target: '/approvals',
          title: i18n('docs.export.approvalLabel'),
          message: `#${a.doc_id || ''}`,
          ts: new Date(a.updated_at || a.created_at || 0).getTime() || 0,
        });
      });
    } catch (_) {
      counts.approvals = 0;
    }
  }

  async function loadIncidentsCount(list) {
    try {
      const res = await Api.get('/api/incidents?limit=200', bgOpts);
      const all = Array.isArray(res.items) ? res.items : [];
      const active = all.filter((it) => isActiveIncidentStatus(it && it.status));
      counts.incidents = active.length;
      if (counts.incidents > 0) {
        list.push({
          key: 'incidents:open',
          section: 'incidents',
          path: '/incidents',
          target: '/incidents',
          title: i18n('nav.incidents'),
          message: `${i18n('incidents.status.open')}: ${counts.incidents}`,
          ts: Date.now(),
        });
      }
      active.slice(0, MAX_ITEMS_PER_SOURCE).forEach((it) => {
        const id = Number(it && it.id);
        if (!id) return;
        const st = normalizeStatus(it.status);
        list.push({
          key: `incident:${id}`,
          section: 'incidents',
          path: '/incidents',
          target: `/incidents?incident=${encodeURIComponent(id)}`,
          title: String(it.title || `#${id}`),
          message: i18n(`incidents.status.${st}`) || st,
          ts: new Date(it.updated_at || it.created_at || 0).getTime() || 0,
        });
      });
    } catch (_) {
      counts.incidents = 0;
    }
  }

  function isActiveTaskStatus(status) {
    const s = normalizeStatus(status);
    if (!s) return true;
    const doneSet = new Set(['done', 'completed', 'closed', 'ready', 'готово', 'завершено', 'закрыто']);
    const backlogSet = new Set(['backlog', 'беклог', 'бэклог']);
    return !doneSet.has(s) && !backlogSet.has(s);
  }

  async function loadTasksCount(list) {
    try {
      const res = await Api.get('/api/tasks?mine=1', bgOpts);
      const tasks = Array.isArray(res.items) ? res.items : [];
      const active = tasks.filter((t) => !t.is_archived && !t.closed_at && isActiveTaskStatus(t.status));
      counts.tasks = active.length;
      active.slice(0, MAX_ITEMS_PER_SOURCE).forEach((t) => {
        list.push({
          key: `task:${t.id}`,
          section: 'tasks',
          path: '/tasks',
          target: `/tasks/task/${encodeURIComponent(t.id)}`,
          title: String(t.title || `#${t.id || ''}`),
          message: i18n('tasks.subtitle'),
          ts: new Date(t.updated_at || t.created_at || 0).getTime() || 0,
        });
      });
    } catch (_) {
      counts.tasks = 0;
    }
  }

  async function loadMonitoringDownCount(list) {
    try {
      const res = await Api.get('/api/monitoring/monitors?status=down', bgOpts);
      const down = Array.isArray(res.items) ? res.items : [];
      counts.monitoring = down.length;
      down.slice(0, MAX_ITEMS_PER_SOURCE).forEach((m) => {
        list.push({
          key: `monitor:${m.id}`,
          section: 'monitoring',
          path: '/monitoring',
          target: '/monitoring',
          title: String(m.name || `#${m.id || ''}`),
          message: i18n('monitoring.status.down'),
          ts: new Date(m.updated_at || m.last_check_at || 0).getTime() || 0,
        });
      });
    } catch (_) {
      counts.monitoring = 0;
    }
  }

  function sortItems(list) {
    return (list || []).slice().sort((a, b) => Number(b.ts || 0) - Number(a.ts || 0));
  }

  function dedupeItems(list) {
    const seen = new Set();
    const seenSemantic = new Set();
    return (list || []).filter((it) => {
      if (!it) return false;
      const key = String(it.key || '').trim();
      const semantic = [
        String(it.section || ''),
        String(it.target || ''),
        String(it.message || '').trim().toLowerCase(),
      ].join('|');
      const digest = key || semantic;
      if (!digest || seen.has(digest)) return false;
      if (semantic && seenSemantic.has(semantic)) return false;
      seen.add(digest);
      if (semantic) seenSemantic.add(semantic);
      return true;
    });
  }

  function onMenuRendered() {
    renderBadge('docs', counts.docs);
    renderBadge('approvals', counts.approvals);
    renderBadge('incidents', counts.incidents);
    renderBadge('tasks', counts.tasks);
    renderBadge('monitoring', counts.monitoring);
  }

  function renderBadge(path, value) {
    const link = document.querySelector(`.sidebar-link[data-path="${path}"]`);
    if (!link) return;
    let badge = link.querySelector('.sidebar-notify');
    if (!badge) {
      badge = document.createElement('span');
      badge.className = 'sidebar-notify';
      link.appendChild(badge);
    }
    const count = Number(value || 0);
    if (!count) {
      badge.hidden = true;
      badge.textContent = '';
      return;
    }
    badge.hidden = false;
    badge.textContent = count > 99 ? '99+' : String(count);
  }

  function getItems() {
    return items.slice();
  }

  function getTotal() {
    // Bell badge must match visible center items to avoid phantom counters.
    return items.length;
  }

  function dismissItem(key) {
    const k = String(key || '').trim();
    if (!k) return;
    dismissedKeys.add(k);
    items = items.filter((it) => it.key !== k);
    transientItems = transientItems.filter((it) => it.key !== k);
    saveDismissed();
    persistErrorItems();
    emitChanged();
  }

  function clearAll() {
    items.forEach((it) => dismissedKeys.add(it.key));
    items = [];
    transientItems = [];
    saveDismissed();
    persistErrorItems();
    emitChanged();
  }

  function emitChanged() {
    if (typeof window === 'undefined') return;
    window.dispatchEvent(new CustomEvent('app:notifications-changed', {
      detail: {
        counts: { ...counts },
        total: getTotal(),
        items: getItems(),
      }
    }));
  }

  function normalizeStatus(status) {
    return String(status || '').trim().toLowerCase();
  }

  function isActiveIncidentStatus(status) {
    const s = normalizeStatus(status);
    if (!s) return true;
    return !new Set(['closed', 'resolved']).has(s);
  }

  function statusText(status) {
    if (status === 'draft') return i18n('docs.status.draft');
    if (status === 'review') return i18n('docs.status.review');
    if (status === 'returned') return i18n('docs.status.returned');
    return status || '-';
  }

  function i18n(key) {
    if (typeof BerkutI18n !== 'undefined' && BerkutI18n.t) {
      return BerkutI18n.t(key);
    }
    return key;
  }

  function loadState() {
    dismissedKeys = new Set(readJSON(STORAGE_DISMISSED, []).map((x) => String(x || '').trim()).filter(Boolean));
    const rawErrors = readJSON(STORAGE_ERRORS, []);
    transientItems = (Array.isArray(rawErrors) ? rawErrors : [])
      .map((it) => normalizeStoredError(it))
      .filter(Boolean)
      .filter((it) => !dismissedKeys.has(it.key));
    items = sortItems(transientItems.slice());
  }

  function normalizeStoredError(it) {
    if (!it || typeof it !== 'object') return null;
    const key = String(it.key || '').trim();
    if (!key || !key.startsWith('toast:')) return null;
    return {
      key,
      section: 'errors',
      path: String(it.path || ''),
      target: String(it.target || it.path || ''),
      title: String(it.title || i18n('app.notifications.errorTitle')),
      message: String(it.message || ''),
      ts: Number(it.ts || 0) || Date.now(),
    };
  }

  function activeErrorItems() {
    return transientItems.filter((it) => it && String(it.section || '') === 'errors' && !dismissedKeys.has(String(it.key || '')));
  }

  function persistErrorItems() {
    const errs = activeErrorItems()
      .slice(0, MAX_STORED_ERRORS)
      .map((it) => ({
        key: it.key,
        section: 'errors',
        path: it.path || '',
        target: it.target || '',
        title: it.title || i18n('app.notifications.errorTitle'),
        message: it.message || '',
        ts: Number(it.ts || Date.now()),
      }));
    writeJSON(STORAGE_ERRORS, errs);
  }

  function saveDismissed() {
    const arr = Array.from(dismissedKeys).filter(Boolean).slice(-500);
    writeJSON(STORAGE_DISMISSED, arr);
  }

  function readJSON(key, fallback) {
    if (typeof window === 'undefined' || !window.localStorage) return fallback;
    try {
      const raw = window.localStorage.getItem(key);
      if (!raw) return fallback;
      const parsed = JSON.parse(raw);
      return parsed == null ? fallback : parsed;
    } catch (_) {
      return fallback;
    }
  }

  function writeJSON(key, value) {
    if (typeof window === 'undefined' || !window.localStorage) return;
    try {
      window.localStorage.setItem(key, JSON.stringify(value));
    } catch (_) {
      // ignore storage failures
    }
  }

  return {
    init,
    onMenuRendered,
    refresh: () => scheduleRefresh(0),
    getItems,
    getTotal,
    dismissItem,
    clearAll,
  };
})();

if (typeof window !== 'undefined') {
  window.AppNotifications = AppNotifications;
}
