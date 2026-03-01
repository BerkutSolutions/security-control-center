(async () => {
  const MIN_STEP_MS = 180;
  const PROBE_TIMEOUT_MS = 12_000;

  const escapeHtml = (str) =>
    String(str || '')
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;');

  const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

  const lang = (localStorage.getItem('berkut_lang') || 'ru').trim() || 'ru';
  if (window.BerkutI18n && typeof BerkutI18n.load === 'function') {
    try {
      await BerkutI18n.load(lang);
      BerkutI18n.apply();
    } catch (_) {
      // ignore
    }
  }
  const t = (key) => (window.BerkutI18n && BerkutI18n.t ? BerkutI18n.t(key) : key);

  const state = {
    menuKeys: new Set(),
    steps: new Map(), // id -> { spec, status, desc }
    stepEls: new Map(), // id -> HTMLElement
    compatItems: [],
    preflight: { loaded: false, loading: false, checks: [] },
  };

  function setError(msg) {
    const errorEl = document.getElementById('healthcheck-error');
    if (!errorEl) return;
    const trimmed = msg == null ? '' : String(msg).trim();
    if (!trimmed) {
      errorEl.textContent = '';
      errorEl.hidden = true;
      return;
    }
    errorEl.textContent = trimmed;
    errorEl.hidden = false;
  }

  function setToggleCount(id, text) {
    const el = document.getElementById(id);
    if (!el) return;
    el.textContent = String(text || '').trim();
  }

  function setToggleStatus(panelID, status) {
    const btn = document.querySelector(`[data-hc-toggle="${panelID}"]`);
    if (!btn) return;
    const icon = btn.querySelector('.hc-toggle-icon');
    if (!icon) return;
    icon.dataset.status = iconStatusFor(status);
  }

  function updateChecksCount() {
    const total = state.steps.size;
    let done = 0;
    state.steps.forEach((s) => {
      if (!s) return;
      const st = String(s.status || '').trim();
      if (st === 'done' || st === 'failed' || st === 'skipped') done++;
    });
    setToggleCount('hc-checks-count', t('healthcheck.count.checked').replace('{done}', String(done)).replace('{total}', String(total)));

    // Header status: running while anything pending/running; failed if any failed; else done.
    let hasRunning = false;
    let hasPending = false;
    let hasFailed = false;
    state.steps.forEach((s) => {
      const st = String(s && s.status ? s.status : '').trim();
      if (st === 'running') hasRunning = true;
      if (st === 'pending') hasPending = true;
      if (st === 'failed') hasFailed = true;
    });
    if (hasRunning || hasPending) setToggleStatus('hc-checks', 'running');
    else if (hasFailed) setToggleStatus('hc-checks', 'failed');
    else setToggleStatus('hc-checks', 'done');
  }

  function updateCompatCount() {
    const total = Array.isArray(state.compatItems) ? state.compatItems.length : 0;
    setToggleCount('hc-compat-count', t('healthcheck.count.checked').replace('{done}', String(total)).replace('{total}', String(total)));
  }

  function updatePreflightCount(checks) {
    const items = Array.isArray(checks) ? checks : [];
    const total = items.length;
    setToggleCount('hc-preflight-count', t('healthcheck.count.checked').replace('{done}', String(total)).replace('{total}', String(total)));

    if (!state.preflight.loaded) {
      setToggleStatus('hc-preflight', 'pending');
      return;
    }
    let hasFailed = false;
    let hasNeeds = false;
    items.forEach((c) => {
      const st = String(c && c.status ? c.status : '').trim();
      if (st === 'failed') hasFailed = true;
      if (st === 'needs_attention') hasNeeds = true;
    });
    if (hasFailed) setToggleStatus('hc-preflight', 'failed');
    else if (hasNeeds) setToggleStatus('hc-preflight', 'skipped');
    else setToggleStatus('hc-preflight', 'done');
  }

  function iconStatusFor(status) {
    const s = String(status || '').trim();
    if (s === 'done' || s === 'ok') return 'done';
    if (s === 'running') return 'running';
    if (s === 'skipped') return 'skipped';
    if (s === 'failed' || s === 'broken' || s === 'needs_reinit') return 'failed';
    return 'pending';
  }

  function addRow(parent, id, title, subtitle, status) {
    const li = document.createElement('li');
    li.className = 'hc-item';
    li.dataset.itemId = id;
    li.innerHTML = `
      <span class="healthcheck-step-icon" aria-hidden="true" data-status="${escapeHtml(iconStatusFor(status))}"></span>
      <div class="hc-item-body">
        <div class="hc-item-title">${escapeHtml(title)}</div>
        <div class="muted hc-item-sub">${escapeHtml(subtitle || '')}</div>
      </div>
    `;
    parent.appendChild(li);
    return li;
  }

  function updateRow(el, title, subtitle, status) {
    if (!el) return;
    const icon = el.querySelector('.healthcheck-step-icon');
    const tEl = el.querySelector('.hc-item-title');
    const sEl = el.querySelector('.hc-item-sub');
    if (icon) icon.dataset.status = iconStatusFor(status);
    if (tEl && title != null) tEl.textContent = String(title);
    if (sEl) sEl.textContent = String(subtitle || '');
  }

  async function probe(url, opts = {}) {
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), PROBE_TIMEOUT_MS);
    try {
      const res = await fetch(url, {
        method: opts.method || 'GET',
        credentials: 'include',
        signal: controller.signal,
        headers: {
          ...(lang ? { 'Accept-Language': lang } : {}),
          ...(opts.headers || {}),
        },
      });
      const text = await res.text().catch(() => '');
      return { ok: res.ok, status: res.status, text, url };
    } catch (err) {
      const msg = err && err.name === 'AbortError' ? 'timeout' : (err && err.message ? String(err.message) : 'network error');
      return { ok: false, status: 0, text: msg, url };
    } finally {
      clearTimeout(timeout);
    }
  }

  async function probeJSON(url) {
    const res = await probe(url, { headers: { Accept: 'application/json' } });
    if (!res.ok) return { ...res, json: null };
    try {
      return { ...res, json: JSON.parse(res.text || '{}') };
    } catch (_) {
      return { ...res, ok: false, text: 'invalid json', json: null };
    }
  }

  function normalizeProbeResult(res) {
    if (!res) return { status: 'failed', desc: t('common.serverError') };
    if (res.ok) return { status: 'done', desc: t('healthcheck.status.done') };
    if (res.status === 401) return { status: 'failed', desc: t('healthcheck.desc.unauthorized') };
    if (res.status === 403) return { status: 'skipped', desc: t('healthcheck.desc.forbidden') };
    if (res.status === 404) return { status: 'failed', desc: t('healthcheck.desc.notFound') };
    if (res.status === 0 && String(res.text || '').toLowerCase().includes('timeout')) return { status: 'failed', desc: t('healthcheck.desc.timeout') };
    const details = (res.text || '').trim();
    return { status: 'failed', desc: details || t('common.serverError') };
  }

  function menuGate(spec) {
    if (!spec.menuKey) return null;
    const key = String(spec.menuKey || '').trim();
    if (!state.menuKeys.has(key)) return { status: 'skipped', desc: t('healthcheck.desc.notInMenu') };
    return null;
  }

  function buildChecks() {
    const page = (name) => async () => normalizeProbeResult(await probe(`/api/page/${name}`));
    const apiGet = (url) => async () => normalizeProbeResult(await probe(url));

    return [
      {
        id: 'session',
        labelKey: 'healthcheck.step.session',
        run: async () => {
          const res = await probeJSON('/api/auth/me');
          if (!res.ok) return normalizeProbeResult(res);
          const user = res.json && res.json.user ? res.json.user : null;
          if (!user || !user.username) return { status: 'failed', desc: t('healthcheck.session.failed') };
          return { status: 'done', desc: `${t('healthcheck.session.user')}: ${user.username}` };
        },
      },
      {
        id: 'menu',
        labelKey: 'healthcheck.step.menu',
        run: async () => {
          const res = await probeJSON('/api/app/menu');
          if (!res.ok) return normalizeProbeResult(res);
          const menu = res.json && Array.isArray(res.json.menu) ? res.json.menu : [];
          state.menuKeys = new Set(menu.map((it) => String(it && (it.path || it.name) || '').trim()).filter(Boolean));
          return { status: 'done', desc: t('healthcheck.desc.menuLoaded').replace('{count}', String(state.menuKeys.size)) };
        },
      },

      { id: 'tab.dashboard', labelKey: 'healthcheck.step.tab.dashboard', menuKey: 'dashboard', run: page('dashboard') },
      { id: 'tab.tasks', labelKey: 'healthcheck.step.tab.tasks', menuKey: 'tasks', run: page('tasks') },
      { id: 'tab.monitoring', labelKey: 'healthcheck.step.tab.monitoring', menuKey: 'monitoring', run: page('monitoring') },
      { id: 'tab.docs', labelKey: 'healthcheck.step.tab.docs', menuKey: 'docs', run: page('docs') },
      { id: 'tab.approvals', labelKey: 'healthcheck.step.tab.approvals', menuKey: 'approvals', run: page('approvals') },
      { id: 'tab.incidents', labelKey: 'healthcheck.step.tab.incidents', menuKey: 'incidents', run: page('incidents') },
      { id: 'tab.registry', labelKey: 'healthcheck.step.tab.registry', menuKey: 'registry', run: page('registry') },
      { id: 'tab.reports', labelKey: 'healthcheck.step.tab.reports', menuKey: 'reports', run: page('reports') },
      { id: 'tab.accounts', labelKey: 'healthcheck.step.tab.accounts', menuKey: 'accounts', run: page('accounts') },
      { id: 'tab.settings', labelKey: 'healthcheck.step.tab.settings', menuKey: 'settings', run: page('settings') },
      { id: 'tab.backups', labelKey: 'healthcheck.step.tab.backups', menuKey: 'backups', run: page('backups') },
      { id: 'tab.logs', labelKey: 'healthcheck.step.tab.logs', menuKey: 'logs', run: page('logs') },

      { id: 'monitoring.monitors', labelKey: 'healthcheck.step.monitoring.monitors', menuKey: 'monitoring', run: apiGet('/api/monitoring/monitors') },
      { id: 'monitoring.events', labelKey: 'healthcheck.step.monitoring.events', menuKey: 'monitoring', run: apiGet('/api/monitoring/events') },
      { id: 'monitoring.engine', labelKey: 'healthcheck.step.monitoring.engine', menuKey: 'monitoring', run: apiGet('/api/monitoring/engine/stats') },
      { id: 'monitoring.sla', labelKey: 'healthcheck.step.monitoring.sla', menuKey: 'monitoring', run: apiGet('/api/monitoring/sla/history?limit=1') },
      { id: 'monitoring.maintenance', labelKey: 'healthcheck.step.monitoring.maintenance', menuKey: 'monitoring', run: apiGet('/api/monitoring/maintenance') },
      { id: 'monitoring.notifications', labelKey: 'healthcheck.step.monitoring.notifications', menuKey: 'monitoring', run: apiGet('/api/monitoring/notifications') },
      { id: 'monitoring.deliveries', labelKey: 'healthcheck.step.monitoring.deliveries', menuKey: 'monitoring', run: apiGet('/api/monitoring/notifications/deliveries') },
      { id: 'monitoring.certs', labelKey: 'healthcheck.step.monitoring.certs', menuKey: 'monitoring', run: apiGet('/api/monitoring/certs') },
      { id: 'monitoring.settings', labelKey: 'healthcheck.step.monitoring.settings', menuKey: 'monitoring', run: apiGet('/api/monitoring/settings') },

      { id: 'registry.overview', labelKey: 'healthcheck.step.registry.overview', menuKey: 'registry', run: apiGet('/api/controls/types') },
      { id: 'registry.controls', labelKey: 'healthcheck.step.registry.controls', menuKey: 'registry', run: apiGet('/api/controls') },
      { id: 'registry.checks', labelKey: 'healthcheck.step.registry.checks', menuKey: 'registry', run: apiGet('/api/checks') },
      { id: 'registry.violations', labelKey: 'healthcheck.step.registry.violations', menuKey: 'registry', run: apiGet('/api/violations') },
      { id: 'registry.frameworks', labelKey: 'healthcheck.step.registry.frameworks', menuKey: 'registry', run: apiGet('/api/frameworks') },
      { id: 'registry.assets', labelKey: 'healthcheck.step.registry.assets', menuKey: 'registry', run: apiGet('/api/assets/list') },
      { id: 'registry.software', labelKey: 'healthcheck.step.registry.software', menuKey: 'registry', run: apiGet('/api/software/list') },
      { id: 'registry.findings', labelKey: 'healthcheck.step.registry.findings', menuKey: 'registry', run: apiGet('/api/findings/list') },

      { id: 'backups.list', labelKey: 'healthcheck.step.backups.list', menuKey: 'backups', run: apiGet('/api/backups') },
      { id: 'backups.plan', labelKey: 'healthcheck.step.backups.plan', menuKey: 'backups', run: apiGet('/api/backups/plan') },
      { id: 'backups.integrity', labelKey: 'healthcheck.step.backups.integrity', menuKey: 'backups', run: apiGet('/api/backups/integrity') },
    ];
  }

  function renderCompat(items) {
    const listEl = document.getElementById('healthcheck-compat');
    const summaryEl = document.getElementById('hc-compat-summary');
    if (!listEl) return;
    listEl.innerHTML = '';
    state.compatItems = Array.isArray(items) ? items : [];
    updateCompatCount();

    const counts = { ok: 0, needs_attention: 0, needs_reinit: 0, broken: 0, other: 0 };
    state.compatItems.forEach((it) => {
      const st = String(it && it.status ? it.status : '').trim();
      if (counts[st] != null) counts[st]++; else counts.other++;
    });
    if (summaryEl) {
      summaryEl.textContent = t('healthcheck.summary')
        .replace('{ok}', String(counts.ok))
        .replace('{attention}', String(counts.needs_attention))
        .replace('{reinit}', String(counts.needs_reinit))
        .replace('{broken}', String(counts.broken));
    }

    const versionsTpl = t('healthcheck.compat.versions') || 'schema {applied_schema}/{expected_schema}, behavior {applied_behavior}/{expected_behavior}';
    state.compatItems.forEach((it) => {
      const title = t(it.title_i18n_key) || it.module_id || '-';
      const versions =
        typeof it.expected_schema_version === 'number'
          ? String(versionsTpl)
              .replace('{applied_schema}', String(Number(it.applied_schema_version || 0)))
              .replace('{expected_schema}', String(Number(it.expected_schema_version || 0)))
              .replace('{applied_behavior}', String(Number(it.applied_behavior_version || 0)))
              .replace('{expected_behavior}', String(Number(it.expected_behavior_version || 0)))
          : '';
      const sub = `${t(`compat.status.${it.status || 'unknown'}`)}${versions ? ` • ${versions}` : ''}`;
      addRow(listEl, String(it.module_id || title), title, sub, it.status);
    });
  }

  function renderPreflightReport(report) {
    const listEl = document.getElementById('healthcheck-preflight');
    const summaryEl = document.getElementById('hc-preflight-summary');
    if (!listEl) return;
    listEl.innerHTML = '';

    const checks = report && Array.isArray(report.checks) ? report.checks : [];
    state.preflight.checks = checks;
    updatePreflightCount(checks);

    const counts = { ok: 0, needs_attention: 0, failed: 0 };
    checks.forEach((c) => {
      const st = String(c && c.status ? c.status : '').trim() || 'ok';
      if (counts[st] != null) counts[st]++;
    });
    if (summaryEl) {
      const ver = report && report.version ? String(report.version) : '';
      const mode = report && report.run_mode ? String(report.run_mode) : '';
      const head = [];
      if (ver) head.push(`v${ver}`);
      if (mode) head.push(t('healthcheck.preflight.runMode').replace('{mode}', mode));
      const headText = head.length ? `${head.join(' ')} — ` : '';
      summaryEl.textContent = t('healthcheck.preflight.summary')
        .replace('{head}', headText)
        .replace('{ok}', String(counts.ok))
        .replace('{needs_attention}', String(counts.needs_attention))
        .replace('{failed}', String(counts.failed));
    }

    checks.forEach((c) => {
      const id = String(c && c.id ? c.id : '').trim() || 'check';
      const key = String(c && c.i18n_key ? c.i18n_key : '').trim();
      const title = key ? (BerkutI18n.t(key) || key) : id;
      const details = c && c.details ? JSON.stringify(c.details) : '';
      addRow(listEl, `preflight.${id}`, title, details, c && c.status ? c.status : '');
    });
  }

  async function loadPreflightIfNeeded() {
    if (state.preflight.loading || state.preflight.loaded) return;
    state.preflight.loading = true;
    const listEl = document.getElementById('healthcheck-preflight');
    if (listEl) {
      listEl.innerHTML = '';
      addRow(listEl, 'preflight.loading', t('healthcheck.preflight.loading'), t('healthcheck.desc.running'), 'running');
    }

    const res = await probeJSON('/api/app/preflight');
    state.preflight.loading = false;
    state.preflight.loaded = true;

    if (!res.ok) {
      if (listEl) {
        listEl.innerHTML = '';
        const msg = res.status === 403 ? t('healthcheck.preflight.noAccess') : (res.text || `HTTP ${res.status}`);
        addRow(listEl, 'preflight.error', t('healthcheck.preflight.title'), msg, res.status === 403 ? 'skipped' : 'failed');
      }
      updatePreflightCount([]);
      return;
    }
    renderPreflightReport(res.json || {});
  }

  function bindAccordion() {
    document.querySelectorAll('[data-hc-toggle]').forEach((btn) => {
      btn.addEventListener('click', () => {
        const id = btn.getAttribute('data-hc-toggle');
        const panel = id ? document.getElementById(id) : null;
        if (!panel) return;
        const open = !panel.hidden;
        panel.hidden = open;
        btn.dataset.open = open ? '0' : '1';
        if (!open && id === 'hc-preflight') {
          loadPreflightIfNeeded();
        }
      });
    });
  }

  async function runAll() {
    await BerkutI18n.load(lang);
    BerkutI18n.apply();

    const continueBtn = document.getElementById('healthcheck-continue');
    if (continueBtn) {
      continueBtn.addEventListener('click', () => {
        window.location.href = '/dashboard';
      });
    }

    bindAccordion();
    // Open checks by default, keep tabs closed until compat is loaded.
    const checksPanel = document.getElementById('hc-checks');
    if (checksPanel) checksPanel.hidden = false;
    const compatPanel = document.getElementById('hc-compat');
    if (compatPanel) compatPanel.hidden = true;
    const preflightPanel = document.getElementById('hc-preflight');
    if (preflightPanel) preflightPanel.hidden = true;
    document.querySelectorAll('[data-hc-toggle]').forEach((btn) => {
      const id = btn.getAttribute('data-hc-toggle');
      const panel = id ? document.getElementById(id) : null;
      if (!panel) return;
      btn.dataset.open = panel.hidden ? '0' : '1';
    });

    setError(null);

    const checksListEl = document.getElementById('healthcheck-steps');
    if (!checksListEl) return;
    checksListEl.innerHTML = '';

    const checks = buildChecks();
    state.steps.clear();
    state.stepEls.clear();
    checks.forEach((spec) => {
      state.steps.set(spec.id, { spec, status: 'pending', desc: t('healthcheck.status.pending') });
      const el = addRow(checksListEl, spec.id, t(spec.labelKey), t('healthcheck.status.pending'), 'pending');
      state.stepEls.set(spec.id, el);
    });
    setToggleStatus('hc-compat', 'pending');
    setToggleStatus('hc-preflight', 'pending');
    updatePreflightCount([]);
    updateChecksCount();

    for (const spec of checks) {
      const row = state.stepEls.get(spec.id) || null;
      const gated = menuGate(spec);
      if (gated) {
        state.steps.set(spec.id, { spec, status: gated.status, desc: gated.desc });
        updateRow(row, null, gated.desc, gated.status);
        updateChecksCount();
        continue;
      }

      const startedAt = Date.now();
      state.steps.set(spec.id, { spec, status: 'running', desc: t('healthcheck.status.running') });
      updateRow(row, null, t('healthcheck.status.running'), 'running');
      updateChecksCount();

      let result;
      try {
        result = await spec.run();
      } catch (err) {
        const msg = err && err.message ? String(err.message) : t('common.serverError');
        result = { status: 'failed', desc: msg };
      }
      const elapsed = Date.now() - startedAt;
      if (elapsed < MIN_STEP_MS) await sleep(MIN_STEP_MS - elapsed);

      state.steps.set(spec.id, { spec, status: result.status, desc: result.desc });
      updateRow(row, null, result.desc, result.status);
      updateChecksCount();

      if (spec.id === 'session' && result.status !== 'done') {
        setError(result.desc || t('common.serverError'));
        return;
      }
    }

    // Compat report is loaded after checks: shows tab/module states.
    setToggleStatus('hc-compat', 'running');
    const compatRes = await probeJSON('/api/app/compat');
    if (compatRes.ok && compatRes.json) {
      const items = Array.isArray(compatRes.json.items) ? compatRes.json.items : [];
      renderCompat(items);
      // Keep tabs section collapsed by default (user can expand).

      const allOK = items.length > 0 && items.every((it) => String(it && it.status ? it.status : '').trim() === 'ok');
      setToggleStatus('hc-compat', allOK ? 'done' : (items.length ? 'failed' : 'pending'));
    } else {
      setError((compatRes && compatRes.text ? compatRes.text : '').trim() || t('healthcheck.compat.failed'));
      setToggleStatus('hc-compat', 'failed');
    }
  }

  runAll().catch((err) => {
    setError(err && err.message ? String(err.message) : t('common.serverError'));
  });
})();
