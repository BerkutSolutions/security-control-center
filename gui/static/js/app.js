(async () => {
  let prefs = Preferences.load();
  let inactivityTimer;
  let autoLogoutHandler;
  let pingTimer;
  const MENU_ORDER = ['dashboard', 'tasks', 'monitoring', 'docs', 'approvals', 'incidents', 'registry', 'reports', 'accounts', 'settings', 'backups', 'logs'];
  const lang = prefs.language || localStorage.getItem('berkut_lang') || 'ru';
  await BerkutI18n.load(lang);
  BerkutI18n.apply();
  if (typeof AppToast !== 'undefined' && AppToast.init) {
    AppToast.init();
  }

  let me;
  let pendingDocsTab = null;
  let appMetaTimer;
  try {
    me = await Api.get('/api/auth/me');
  } catch (err) {
    window.location.href = '/login';
    return;
  }
  const user = me.user;
  if (!user.password_set || user.require_password_change) {
    window.location.href = '/password-change';
    return;
  }
  await loadAppMeta();
  bindProfileShortcut();
  bindNotificationsUI();
  bindStepupUI();

  document.getElementById('logout-btn').addEventListener('click', async () => {
    if (typeof IncidentsPage !== 'undefined' && IncidentsPage.clearState) {
      IncidentsPage.clearState();
    }
    await Api.post('/api/auth/logout');
    window.location.href = '/login';
  });

  configureAutoLogout(prefs.autoLogout);
  startSessionPing();
  setupModalDismiss();

  const menuResp = await Api.get('/api/app/menu');
  migrateLegacyHash(menuResp.menu);
  let currentPage = pickInitialPage(menuResp.menu);
  renderMenu(menuResp.menu, currentPage);
  if (typeof AppNotifications !== 'undefined' && AppNotifications.init) {
    AppNotifications.init((menuResp.menu || []).map(i => i.path));
  }
  await navigateTo(currentPage, false);
  if (window.location.pathname === '/' || window.location.pathname === '/app') {
    window.history.replaceState({}, '', `/${currentPage}`);
  }
  if (typeof window !== 'undefined' && window.AppCompat && typeof window.AppCompat.checkAndShowWizard === 'function') {
    window.AppCompat.checkAndShowWizard().catch(() => {});
  }

  window.addEventListener('popstate', async () => {
    const target = pathFromLocation(menuResp.menu) || currentPage;
    if (target !== currentPage) {
      const ok = await navigateTo(target, false);
      if (!ok) return;
    } else {
      setActiveLink(target);
      if (target === 'docs' && typeof DocsPage !== 'undefined' && DocsPage.switchTab) {
        if (pendingDocsTab) {
          const nextTab = pendingDocsTab;
          pendingDocsTab = null;
          DocsPage.switchTab(nextTab);
        }
      }
      if ((target === 'controls' || target === 'registry') && typeof ControlsPage !== 'undefined' && ControlsPage.syncRouteTab) {
        ControlsPage.syncRouteTab();
      }
      if (target === 'monitoring' && typeof MonitoringPage !== 'undefined' && MonitoringPage.syncRouteTab) {
        MonitoringPage.syncRouteTab();
      }
    }
  });

  const switcher = document.getElementById('language-switcher');
  if (switcher) {
    switcher.value = lang;
    switcher.addEventListener('change', async (e) => {
      const nextPrefs = Preferences.save({ ...prefs, language: e.target.value });
      await handlePreferencesChange(nextPrefs, menuResp.menu, currentPage);
    });
  }

  function pickInitialPage(items) {
    const url = new URL(window.location.href);
    if (url.searchParams.get('incident')) {
      const hasIncidents = (items || []).some(i => i.path === 'incidents');
      if (hasIncidents) return 'incidents';
    }
    const fromPath = pathFromLocation(items);
    if (fromPath) return fromPath;
    const ordered = sortMenuItems(items);
    return ordered.length ? ordered[0].path : 'dashboard';
  }

  function pathFromLocation(items) {
    const path = window.location.pathname.replace(/\/+$/, '');
    const parts = path.split('/').filter(Boolean);
    const base = parts[0] || '';
    if (!base || base === 'app') return null;
    if (base === 'profile') return 'profile';
    if (base === 'approvals') return 'approvals';
    if (base === 'docs') return 'docs';
    if (base === 'tasks') return 'tasks';
    if (base === 'incidents') return 'incidents';
    if (base === 'settings') return 'settings';
    if (base === 'accounts') return 'accounts';
    if (base === 'dashboard') return 'dashboard';
    if (base === 'registry') return 'registry';
    if (base === 'controls') return 'registry';
    if (base === 'monitoring') return 'monitoring';
    if (base === 'backups') return 'backups';
    if (base === 'reports') return 'reports';
    if (base === 'assets') return 'assets';
    if (base === 'software') return 'software';
    if (base === 'findings') return 'findings';
    if (base === 'logs') return 'logs';
    return items.find(i => i.path === base)?.path || null;
  }

  function migrateLegacyHash(items) {
    const rawHash = window.location.hash.replace('#', '');
    if (!rawHash) return;
    let next = '';
    if (rawHash.startsWith('tasks/task/')) {
      const [, , id] = rawHash.split('/');
      next = id ? `/tasks/task/${id}` : '/tasks';
    } else if (rawHash.startsWith('tasks/space/')) {
      const [, , id] = rawHash.split('/');
      next = id ? `/tasks/space/${id}` : '/tasks';
    } else if (rawHash === 'tasks') {
      next = '/tasks';
    } else if (rawHash === 'docs') {
      next = '/docs';
    } else if (rawHash === 'approvals') {
      next = '/approvals';
    } else if (rawHash === 'docs/approvals') {
      next = '/docs/approvals';
    } else if (rawHash.startsWith('incident=')) {
      const id = rawHash.split('incident=')[1];
      next = id ? `/incidents/${id}` : '/incidents';
    } else if (rawHash.startsWith('settings/')) {
      const [, tab] = rawHash.split('/');
      next = tab ? `/settings/${tab}` : '/settings';
    } else if (rawHash.startsWith('backups/')) {
      const [, tab] = rawHash.split('/');
      next = tab ? `/backups/${tab}` : '/backups';
    } else if (rawHash === 'backups') {
      next = '/backups';
    } else if (rawHash) {
      const base = rawHash.split('/')[0];
      const known = items.find(i => i.path === base);
      if (known) next = `/${base}`;
    }
    if (next) {
      window.history.replaceState({}, '', next);
      window.location.hash = '';
    }
  }

  function sortMenuItems(items) {
    const originalOrder = new Map();
    (items || []).forEach((item, idx) => originalOrder.set(item, idx));
    return (items || []).slice().sort((a, b) => {
      const aIndex = MENU_ORDER.indexOf(a.path);
      const bIndex = MENU_ORDER.indexOf(b.path);
      const aScore = aIndex === -1 ? Number.MAX_SAFE_INTEGER : aIndex;
      const bScore = bIndex === -1 ? Number.MAX_SAFE_INTEGER : bIndex;
      if (aScore !== bScore) return aScore - bScore;
      return (originalOrder.get(a) ?? 0) - (originalOrder.get(b) ?? 0);
    });
  }

  async function navigateTo(path, updateHash = true) {
    if (currentPage === 'dashboard' && path !== 'dashboard') {
      if (typeof DashboardPage !== 'undefined' && DashboardPage.confirmNavigation) {
        const ok = await DashboardPage.confirmNavigation();
        if (!ok) {
          if (!updateHash) {
            window.history.replaceState({}, '', `/${currentPage}`);
          }
          return false;
        }
      }
    }
    if (currentPage === 'incidents' && path !== 'incidents') {
      clearIncidentQuery();
      if (typeof IncidentsPage !== 'undefined' && IncidentsPage.clearState) {
        IncidentsPage.clearState();
      }
    }
    currentPage = path;
    if (updateHash) {
      const nextPath = `/${path}`;
      if (window.location.pathname !== nextPath) {
        window.history.pushState({}, '', nextPath);
      }
    }
    setActiveLink(path);
    const loaded = await loadPage(path);
    if (loaded && loaded.forbidden) {
      const fallback = firstAllowedPath(path);
      if (fallback) {
        currentPage = fallback;
        window.history.replaceState({}, '', `/${fallback}`);
        setActiveLink(fallback);
        await loadPage(fallback);
      }
      return false;
    }
    return true;
  }

  window.AppNav = {
    navigateTo: (path, updateHash = true) => navigateTo(path, updateHash),
  };

  function renderMenu(items, activePath) {
    const nav = document.getElementById('menu');
    nav.innerHTML = '';
    const activeKey = ['assets', 'software', 'findings'].includes(activePath) ? 'registry' : activePath;
    sortMenuItems(items).forEach(item => {
      const link = document.createElement('a');
      link.className = 'sidebar-link';
      link.href = `/${item.path}`;
      const label = document.createElement('span');
      label.className = 'sidebar-link-label';
      label.textContent = BerkutI18n.t(`nav.${item.name}`) || item.name;
      link.appendChild(label);
      link.dataset.path = item.path;
      if (item.path === activeKey) {
        link.classList.add('active');
      }
      link.addEventListener('click', async (e) => {
        e.preventDefault();
        await navigateTo(item.path);
      });
      nav.appendChild(link);
    });
    if (typeof AppNotifications !== 'undefined' && AppNotifications.onMenuRendered) {
      AppNotifications.onMenuRendered();
    }
  }

  function setActiveLink(path) {
    const effective = ['assets', 'software', 'findings'].includes(path) ? 'registry' : path;
    document.querySelectorAll('.sidebar-link').forEach(link => {
      link.classList.toggle('active', link.dataset.path === effective);
    });
  }

  function firstAllowedPath(exclude) {
    const ordered = sortMenuItems(menuResp?.menu || []);
    const first = ordered.find(i => i && i.path && i.path !== exclude);
    return first ? first.path : null;
  }

  async function loadPage(path) {
    document.body.classList.remove('dashboard-mode');
    const res = await fetch(`/api/page/${path}`, { credentials: 'include' });
    const area = document.getElementById('content-area');
    if (!res.ok) {
      area.textContent = BerkutI18n.t('common.accessDenied');
      return { ok: false, forbidden: res.status === 403 };
    }
    const html = await res.text();
    area.innerHTML = html;
    const titleEl = document.getElementById('page-title');
    const descEl = document.getElementById('page-desc');
    if (path === 'docs') {
      if (titleEl) titleEl.textContent = '';
      if (descEl) descEl.textContent = '';
    } else {
      const titleKey = path === 'registry' ? 'nav.controls' : (path === 'profile' ? 'profile.title' : `nav.${path}`);
      if (titleEl) titleEl.textContent = BerkutI18n.t(titleKey) || path;
      if (descEl) descEl.textContent = descriptionFor(path);
    }
    BerkutI18n.apply();
    if (autoLogoutHandler) {
      autoLogoutHandler();
    }
    if (path === 'accounts' && typeof AccountsPage !== 'undefined') {
      AccountsPage.init();
    }
    if (path === 'docs' && typeof DocsPage !== 'undefined') {
      if (pendingDocsTab) {
        window.__docsPendingTab = pendingDocsTab;
        pendingDocsTab = null;
      }
      DocsPage.init();
    }
    if (path === 'approvals' && typeof ApprovalsPage !== 'undefined') {
      ApprovalsPage.init();
    }
    if (path === 'settings' && typeof SettingsPage !== 'undefined') {
      SettingsPage.init(async (next) => {
        const saved = Preferences.save(next);
        await handlePreferencesChange(saved, menuResp.menu, path);
      });
    }
    if (path === 'profile' && typeof ProfilePage !== 'undefined') {
      ProfilePage.init(async (next) => {
        const saved = Preferences.save(next);
        await handlePreferencesChange(saved, menuResp.menu, path);
      });
    }
    if (path === 'incidents' && typeof IncidentsPage !== 'undefined') {
      IncidentsPage.init();
    }
    if (path === 'tasks' && typeof TasksPage !== 'undefined') {
      TasksPage.init();
    }
    if (path === 'logs' && typeof LogsPage !== 'undefined') {
      LogsPage.init();
    }
    if (path === 'dashboard' && typeof DashboardPage !== 'undefined') {
      DashboardPage.init();
    }
    // legacy "controls" route is mapped to "registry" by pathFromLocation; keep a safety init.
    if (path === 'controls' && typeof ControlsPage !== 'undefined') {
      ControlsPage.init();
    }
    if (path === 'registry' && typeof ControlsPage !== 'undefined') {
      ControlsPage.init();
    }
    if (path === 'assets' && typeof AssetsPage !== 'undefined') {
      AssetsPage.init();
    }
    if (path === 'software' && typeof SoftwarePage !== 'undefined') {
      SoftwarePage.init();
    }
    if (path === 'findings' && typeof FindingsPage !== 'undefined') {
      FindingsPage.init();
    }
    if (path === 'monitoring' && typeof MonitoringPage !== 'undefined') {
      MonitoringPage.init();
    }
    if (path === 'reports' && typeof ReportsPage !== 'undefined') {
      ReportsPage.init();
    }
    if (path === 'backups' && typeof BackupsPage !== 'undefined') {
      BackupsPage.init();
    }
    return { ok: true };
  }

  async function handlePreferencesChange(nextPrefs, menu, currentPath) {
    prefs = nextPrefs;
    await BerkutI18n.load(prefs.language || 'ru');
    BerkutI18n.apply();
    renderMenu(menu, currentPath);
    if (typeof AppNotifications !== 'undefined' && AppNotifications.onMenuRendered) {
      AppNotifications.onMenuRendered();
    }
    setActiveLink(currentPath);
    const switcher = document.getElementById('language-switcher');
    if (switcher) switcher.value = prefs.language || 'ru';
    configureAutoLogout(prefs.autoLogout);
    await loadPage(currentPath);
  }

  function configureAutoLogout(enabled) {
    const events = ['click', 'keydown', 'mousemove', 'scroll'];
    events.forEach(evt => {
      if (autoLogoutHandler) {
        document.removeEventListener(evt, autoLogoutHandler);
      }
    });
    clearTimeout(inactivityTimer);
    if (!enabled) {
      autoLogoutHandler = null;
      return;
    }
    autoLogoutHandler = () => {
      clearTimeout(inactivityTimer);
      inactivityTimer = setTimeout(async () => {
        try {
          await Api.post('/api/auth/logout');
        } catch (err) {
          console.error('auto logout failed', err);
        } finally {
          window.location.href = '/login';
        }
      }, 15 * 60 * 1000);
    };
    events.forEach(evt => document.addEventListener(evt, autoLogoutHandler));
    autoLogoutHandler();
  }

  function startSessionPing() {
    clearInterval(pingTimer);
    const intervalMs = 45000;
    const ping = async () => {
      try {
        await Api.post('/api/app/ping');
      } catch (err) {
        console.debug('ping failed', err);
      }
    };
    pingTimer = setInterval(ping, intervalMs);
    ping();
  }

  async function loadAppMeta() {
    try {
      const meta = await Api.get('/api/app/meta');
      renderAppMeta(meta || {});
      if (meta?.update_checks_enabled) {
        if (!appMetaTimer) {
          appMetaTimer = setInterval(() => {
            loadAppMeta().catch(() => {});
          }, 30 * 60 * 1000);
        }
      } else if (appMetaTimer) {
        clearInterval(appMetaTimer);
        appMetaTimer = null;
      }
    } catch (_) {
      // ignore meta failures for normal navigation
    }
  }

  function renderAppMeta(meta) {
    const versionEl = document.getElementById('app-version');
    const badgeEl = document.getElementById('app-update-badge');
    if (versionEl) {
      const mode = meta?.is_home_mode ? 'HOME' : 'Enterprise';
      versionEl.textContent = `${BerkutI18n.t('settings.version')}: ${meta?.app_version || '-'} | ${mode}`;
    }
    if (!badgeEl) return;
    const update = meta?.update;
    const updatesEnabled = !!meta?.update_checks_enabled;
    if (updatesEnabled && update?.has_update) {
      badgeEl.hidden = false;
      badgeEl.textContent = BerkutI18n.t('settings.updates.available');
      badgeEl.href = update.release_url || meta?.repository_url || '#';
      badgeEl.target = '_blank';
      badgeEl.rel = 'noopener noreferrer';
      const checkedAtRaw = update?.checked_at || '';
      const checkedAt = checkedAtRaw && AppTime?.formatDateTime ? AppTime.formatDateTime(checkedAtRaw) : checkedAtRaw;
      const latest = update.latest_version || '-';
      badgeEl.title = checkedAt ? `${latest} (${checkedAt})` : `${latest}`;
    } else {
      badgeEl.hidden = true;
      badgeEl.textContent = '';
      badgeEl.removeAttribute('href');
      badgeEl.removeAttribute('target');
      badgeEl.removeAttribute('rel');
      badgeEl.removeAttribute('title');
    }
  }

  function setupModalDismiss() {
    const closeModalWithHook = (modal) => {
      if (!modal) return;
      if (modal.id === 'task-modal' && typeof TasksPage !== 'undefined' && typeof TasksPage.closeTaskModal === 'function') {
        TasksPage.closeTaskModal();
        return;
      }
      modal.hidden = true;
    };
    document.addEventListener('keydown', (e) => {
      if (e.key !== 'Escape') return;
      const openModals = Array.from(document.querySelectorAll('.modal')).filter(m => !m.hidden);
      if (!openModals.length) return;
      closeModalWithHook(openModals[openModals.length - 1]);
    });
    document.addEventListener('click', (e) => {
      const closeBtn = e.target.closest('[data-close]');
      if (closeBtn) {
        e.preventDefault();
        e.stopPropagation();
        const sel = closeBtn.getAttribute('data-close');
        if (!sel) return;
        const modal = document.querySelector(sel);
        closeModalWithHook(modal);
        return;
      }
      const backdrop = e.target.closest('.modal-backdrop');
      if (!backdrop) return;
      const modal = backdrop.closest('.modal');
      closeModalWithHook(modal);
    });
  }

  function clearIncidentQuery() {
    const url = new URL(window.location.href);
    if (!url.searchParams.has('incident')) return;
    url.searchParams.delete('incident');
    window.history.replaceState({}, '', url.toString());
  }

  function descriptionFor(path) {
    switch (path) {
      case 'profile':
        return BerkutI18n.t('profile.subtitle');
      case 'dashboard':
        return BerkutI18n.t('dashboard.subtitle');
      case 'accounts':
        return BerkutI18n.t('accounts.subtitle');
      case 'docs':
        return BerkutI18n.t('docs.subtitle');
      case 'approvals':
        return BerkutI18n.t('approvals.subtitle');
      case 'incidents':
        return BerkutI18n.t('incidents.subtitle');
      case 'tasks':
        return BerkutI18n.t('tasks.subtitle');
      case 'controls':
        return BerkutI18n.t('registry.subtitle');
      case 'registry':
        return BerkutI18n.t('registry.subtitle');
      case 'assets':
        return BerkutI18n.t('assets.subtitle');
      case 'software':
        return BerkutI18n.t('software.subtitle');
      case 'findings':
        return BerkutI18n.t('findings.subtitle');
      case 'monitoring':
        return BerkutI18n.t('monitoring.subtitle');
      case 'logs':
        return BerkutI18n.t('logs.subtitle');
      case 'settings':
        return BerkutI18n.t('settings.subtitle');
      case 'reports':
        return BerkutI18n.t('reports.subtitle');
      case 'backups':
        return BerkutI18n.t('backups.subtitle');
      default:
        return BerkutI18n.t('placeholder.subtitle');
    }
  }

  function bindProfileShortcut() {
    const btn = document.getElementById('profile-btn');
    if (!btn) return;
    btn.onclick = async (e) => {
      e.preventDefault();
      await navigateTo('profile');
      if (window.location.pathname !== '/profile') {
        window.history.pushState({}, '', '/profile');
      }
    };
    bindProfileHoverCard(btn);
  }

  function bindProfileHoverCard(btn) {
    if (!btn) return;
    const card = ensureProfileHoverCard();
    const render = () => {
      const lines = [
        `${safe(user.username)}`,
        safe(user.full_name),
        safe(user.department),
        safe(user.position),
        user.session_created_at ? `${BerkutI18n.t('profile.sessionStarted')}: ${formatDateTime(user.session_created_at)}` : '',
        user.session_expires_at ? `${BerkutI18n.t('profile.sessionExpires')}: ${formatDateTime(user.session_expires_at)}` : '',
      ].filter(Boolean);
      card.textContent = '';
      lines.forEach((line) => {
        const row = document.createElement('div');
        row.textContent = line;
        card.appendChild(row);
      });
    };
    const show = () => {
      render();
      const rect = btn.getBoundingClientRect();
      card.hidden = false;
      const margin = 8;
      const cardWidth = card.offsetWidth || 240;
      const cardHeight = card.offsetHeight || 120;
      const maxLeft = Math.max(margin, window.innerWidth - cardWidth - margin);
      const maxTop = Math.max(margin, window.innerHeight - cardHeight - margin);
      const left = Math.min(Math.max(margin, Math.round(rect.right + 10)), maxLeft);
      const top = Math.min(Math.max(margin, Math.round(rect.top)), maxTop);
      card.style.left = `${left}px`;
      card.style.top = `${top}px`;
    };
    const hide = () => { card.hidden = true; };
    btn.addEventListener('mouseenter', show);
    btn.addEventListener('mouseleave', hide);
    btn.addEventListener('focus', show);
    btn.addEventListener('blur', hide);
    window.addEventListener('scroll', hide, { passive: true });
  }

  function ensureProfileHoverCard() {
    let card = document.getElementById('profile-hover-card');
    if (card) return card;
    card = document.createElement('div');
    card.id = 'profile-hover-card';
    card.className = 'profile-hover-card';
    card.hidden = true;
    document.body.appendChild(card);
    return card;
  }

  function formatDateTime(value) {
    if (!value) return '-';
    if (window.AppTime?.formatDateTime) return AppTime.formatDateTime(value);
    return String(value);
  }

  function safe(v) {
    return String(v || '').trim();
  }

  function bindNotificationsUI() {
    const btn = document.getElementById('notifications-btn');
    const drop = document.getElementById('notifications-dropdown');
    const list = document.getElementById('notifications-list');
    const clearBtn = document.getElementById('notifications-clear-all');
    const badge = document.getElementById('notifications-btn-badge');
    if (!btn || !drop || !list || !clearBtn || !badge) return;

    const render = () => {
      const total = (window.AppNotifications && AppNotifications.getTotal) ? AppNotifications.getTotal() : 0;
      if (total > 0) {
        badge.hidden = false;
        badge.textContent = total > 99 ? '99+' : String(total);
      } else {
        badge.hidden = true;
        badge.textContent = '';
      }
      const items = (window.AppNotifications && AppNotifications.getItems) ? AppNotifications.getItems() : [];
      list.innerHTML = '';
      if (!items.length) {
        const empty = document.createElement('div');
        empty.className = 'notifications-empty muted';
        empty.textContent = BerkutI18n.t('app.notifications.empty');
        list.appendChild(empty);
        return;
      }
      items.forEach((item) => {
        const row = document.createElement('div');
        row.className = 'notification-row';
        const head = document.createElement('div');
        head.className = 'notification-head';
        const title = document.createElement('strong');
        title.textContent = item.title || '-';
        const del = document.createElement('button');
        del.type = 'button';
        del.className = 'btn ghost btn-sm';
        del.textContent = BerkutI18n.t('common.delete');
        del.onclick = (e) => {
          e.preventDefault();
          e.stopPropagation();
          if (window.AppNotifications?.dismissItem) {
            AppNotifications.dismissItem(item.key);
          }
          render();
        };
        head.appendChild(title);
        head.appendChild(del);
        const message = document.createElement('div');
        message.className = 'notification-message muted';
        message.textContent = item.message || '';
        row.appendChild(head);
        row.appendChild(message);
        row.onclick = async () => {
          await openNotificationTarget(item);
          drop.hidden = true;
        };
        list.appendChild(row);
      });
    };

    btn.onclick = (e) => {
      e.preventDefault();
      e.stopPropagation();
      drop.hidden = !drop.hidden;
      if (!drop.hidden) render();
    };
    clearBtn.onclick = (e) => {
      e.preventDefault();
      if (window.AppNotifications?.clearAll) {
        AppNotifications.clearAll();
      }
      render();
    };
    document.addEventListener('click', (e) => {
      if (drop.hidden) return;
      if (e.target === btn || btn.contains(e.target)) return;
      if (drop.contains(e.target)) return;
      drop.hidden = true;
    });
    window.addEventListener('app:notifications-changed', () => render());
    render();
  }

  async function openNotificationTarget(item) {
    const raw = String((item && (item.target || item.path)) || '').trim();
    if (!raw) return;
    const absolute = raw.startsWith('/') ? raw : `/${raw}`;
    const url = new URL(absolute, window.location.origin);
    const base = pathFromLocation([]) || url.pathname.replace(/^\/+/, '').split('/')[0] || '';
    if (!base) return;
    if (`/${url.pathname.replace(/^\/+/, '')}` !== window.location.pathname || url.search !== window.location.search) {
      window.history.pushState({}, '', `${url.pathname}${url.search}`);
    }
    await navigateTo(base, false);
  }

  function bindStepupUI() {
    const modal = document.getElementById('stepup-modal');
    const alertEl = document.getElementById('stepup-alert');
    const subtitleEl = document.getElementById('stepup-subtitle');
    const passwordEl = document.getElementById('stepup-password');
    const codeEl = document.getElementById('stepup-code');
    const codeRow = document.getElementById('stepup-code-row');
    const passwordBtn = document.getElementById('stepup-password-btn');
    const totpBtn = document.getElementById('stepup-totp-btn');
    const passkeyBtn = document.getElementById('stepup-passkey-btn');
    const logoutBtn = document.getElementById('stepup-logout-btn');
    if (!modal || !passwordBtn || !totpBtn || !passkeyBtn || !logoutBtn) return;

    const setAlert = (message, ok = false) => {
      if (!alertEl) return;
      const text = String(message || '').trim();
      if (!text) {
        alertEl.hidden = true;
        alertEl.classList.remove('success');
        alertEl.textContent = '';
        return;
      }
      alertEl.hidden = false;
      alertEl.textContent = BerkutI18n.t(text) || text;
      alertEl.classList.toggle('success', !!ok);
    };

    const updateView = (payload) => {
      const methods = payload && payload.methods ? payload.methods : {};
      const passwordVerified = !!payload?.password_verified;
      const locked = !!payload?.locked;
      const lockSec = Number(payload?.lock_seconds || 0);
      const requireSecond = !!methods.totp || !!methods.passkey;

      if (subtitleEl) {
        if (locked) {
          subtitleEl.textContent = (BerkutI18n.t('auth.stepup.lockedFor') || 'Повторите через {sec} сек.').replace('{sec}', `${Math.max(0, lockSec)}`);
        } else if (passwordVerified && requireSecond) {
          subtitleEl.textContent = BerkutI18n.t('auth.stepup.secondFactor');
        } else {
          subtitleEl.textContent = BerkutI18n.t('auth.stepup.subtitle');
        }
      }
      if (codeRow) codeRow.hidden = !(passwordVerified && !!methods.totp);
      if (passwordEl) passwordEl.disabled = locked || passwordVerified;
      if (codeEl) codeEl.disabled = locked || !(passwordVerified && !!methods.totp);
      passwordBtn.disabled = locked || passwordVerified;
      totpBtn.hidden = !(passwordVerified && !!methods.totp);
      passkeyBtn.hidden = !(passwordVerified && !!methods.passkey);
      totpBtn.disabled = locked || !(passwordVerified && !!methods.totp);
      passkeyBtn.disabled = locked || !(passwordVerified && !!methods.passkey);
    };

    const showModal = async () => {
      modal.hidden = false;
      setAlert('');
      try {
        const status = await Api.get('/api/auth/stepup/status');
        updateView(status || {});
      } catch (err) {
        setAlert((err && err.message) || 'common.error');
      }
    };

    const hideModal = () => {
      modal.hidden = true;
      setAlert('');
      if (passwordEl) passwordEl.value = '';
      if (codeEl) codeEl.value = '';
    };

    window.addEventListener('app:auth-challenge', async () => {
      await showModal();
    });

    passwordBtn.addEventListener('click', async () => {
      try {
        const data = await Api.post('/api/auth/stepup/password', { password: passwordEl ? passwordEl.value : '' });
        updateView(data || {});
        setAlert('');
      } catch (err) {
        setAlert((err && err.message) || 'auth.stepup.passwordInvalid');
      }
    });

    totpBtn.addEventListener('click', async () => {
      try {
        const data = await Api.post('/api/auth/stepup/totp', { code: codeEl ? codeEl.value : '' });
        if (data && data.required === false) {
          hideModal();
          if (window.AppToast?.show) {
            AppToast.show(BerkutI18n.t('auth.stepup.success') || 'OK', 'success');
          }
          return;
        }
        updateView(data || {});
      } catch (err) {
        setAlert((err && err.message) || 'auth.2fa.invalidCode');
      }
    });

    passkeyBtn.addEventListener('click', async () => {
      try {
        if (!window.BerkutWebAuthn || !BerkutWebAuthn.supported()) {
          setAlert('auth.passkeys.notSupported');
          return;
        }
        const begin = await Api.post('/api/auth/stepup/passkey/begin', {});
        const options = BerkutWebAuthn.toPublicKeyRequestOptions(begin?.options);
        const cred = await navigator.credentials.get({ publicKey: options });
        const raw = BerkutWebAuthn.credentialToJSON(cred);
        const done = await Api.post('/api/auth/stepup/passkey/finish', {
          challenge_id: begin?.challenge_id || '',
          credential: raw,
        });
        if (done && done.required === false) {
          hideModal();
          if (window.AppToast?.show) {
            AppToast.show(BerkutI18n.t('auth.stepup.success') || 'OK', 'success');
          }
          return;
        }
        updateView(done || {});
      } catch (err) {
        const key = (window.BerkutWebAuthn && BerkutWebAuthn.errorKey) ? BerkutWebAuthn.errorKey(err) : '';
        setAlert(key || (err && err.message) || 'auth.passkeys.failed');
      }
    });

    logoutBtn.addEventListener('click', async () => {
      try {
        await Api.post('/api/auth/logout');
      } catch (_) {
        // ignore
      } finally {
        window.location.href = '/login';
      }
    });
  }
})();
