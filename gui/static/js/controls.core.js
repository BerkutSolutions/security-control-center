const ControlsPage = (() => {
  const state = {
    controls: [],
    checks: [],
    violations: [],
    controlLinks: [],
    controlTypes: [],
    frameworks: [],
    frameworkItems: [],
    frameworkMap: [],
    linkOptions: { docs: [], tasks: [], incidents: [] },
    linkOptionsLoaded: false,
    selectedFramework: null,
    currentUser: null,
    permissions: [],
    customDomains: []
  };
  const DEFAULT_CONTROL_TYPES = ['organizational', 'technical', 'procedural'];
  const DEFAULT_DOMAINS = ['pdn', 'kii', 'infra', 'dev', 'other'];
  const CUSTOM_OPTIONS_KEY = 'controls.customOptions';
  let customLoaded = false;
  let activeTab = 'controls-tab-overview';
  let overviewFilters = { lastCheck: '', noOwner: false, noChecks: false, status: '' };
  let pendingControlId = '';
  const TAB_MAP = {
    overview: 'controls-tab-overview',
    controls: 'controls-tab-controls',
    checks: 'controls-tab-checks',
    violations: 'controls-tab-violations',
    frameworks: 'controls-tab-frameworks'
  };

  function init() {
    const page = document.getElementById('controls-page');
    if (!page) return;
    loadCustomOptions();
    pendingControlId = new URLSearchParams(window.location.search).get('control') || '';
    const initialTab = resolveTabFromUrl();
    if (initialTab) activeTab = initialTab;
    bindTabs();
    bindOverview();
    bindControlsTab();
    bindChecksTab();
    bindViolationsTab();
    bindFrameworksTab();
    bindModalCloseButtons();
    applyDateInputLocale();
    switchTab(activeTab);
    if (typeof UserDirectory !== 'undefined' && UserDirectory.load) {
      UserDirectory.load().then(() => {
        populateUserSelect('controls-check-filter-owner', true);
      });
    }
    loadCurrentUser().then(async () => {
      applyAccessControls();
      await loadControlTypes();
      refreshControlTypeSelects();
      loadControls();
      loadOverview();
      loadFrameworks();
    });
    document.addEventListener('controls:optionsChanged', () => {
      refreshDomainSelects();
    });
    document.addEventListener('controls:typesUpdated', () => {
      loadControlTypes().then(() => {
        refreshControlTypeSelects();
        loadControls();
      });
    });
  }

  function t(key) {
    const val = BerkutI18n.t(key);
    return val === key ? key : val;
  }

  function hasPerm(perm) {
    if (!perm) return true;
    const perms = Array.isArray(state.permissions) ? state.permissions : [];
    if (!perms.length) return true;
    return perms.includes(perm);
  }

  function currentLocale() {
    const lang = (window.BerkutI18n && BerkutI18n.currentLang && BerkutI18n.currentLang()) || 'ru';
    return lang === 'ru' ? 'ru-RU' : 'en-US';
  }

  function applyDateInputLocale() {
    const page = document.getElementById('controls-page');
    if (!page) return;
    page.querySelectorAll('input[type="date"]').forEach(input => {
      input.lang = 'ru';
    });
    page.querySelectorAll('input[data-i18n-placeholder="common.datetimePlaceholder"]').forEach(input => {
      input.inputMode = 'numeric';
    });
  }

  function shouldIgnoreRowClick(target) {
    if (!target) return false;
    return Boolean(target.closest('button, a, input, select, textarea, label'));
  }

  function resolveTabFromUrl() {
    const params = new URLSearchParams(window.location.search);
    const raw = params.get('tab');
    if (!raw) return '';
    const key = raw.toLowerCase();
    return TAB_MAP[key] || '';
  }

  function updateTabInUrl(tabId) {
    const reverse = Object.entries(TAB_MAP).find(([, val]) => val === tabId);
    const key = reverse ? reverse[0] : '';
    const url = new URL(window.location.href);
    if (key) {
      url.searchParams.set('tab', key);
    } else {
      url.searchParams.delete('tab');
    }
    window.history.replaceState({}, '', url.toString());
  }

  async function loadCurrentUser() {
    try {
      const res = await Api.get('/api/auth/me');
      state.currentUser = res.user;
      state.permissions = Array.isArray(res.user?.permissions) ? res.user.permissions : [];
    } catch (_) {
      state.currentUser = null;
      state.permissions = [];
    }
  }

  function applyAccessControls() {
    const createBtn = document.getElementById('controls-create-btn');
    if (createBtn) createBtn.hidden = !hasPerm('controls.manage');
    const checkBtn = document.getElementById('controls-check-create');
    if (checkBtn) checkBtn.hidden = !hasPerm('controls.checks.manage');
    const violationBtn = document.getElementById('controls-violation-create');
    if (violationBtn) violationBtn.hidden = !hasPerm('controls.violations.manage');
    const frameworkBtn = document.getElementById('framework-create-btn');
    if (frameworkBtn) frameworkBtn.hidden = !hasPerm('controls.frameworks.manage');
    const frameworkItemBtn = document.getElementById('framework-item-create');
    if (frameworkItemBtn) frameworkItemBtn.hidden = !hasPerm('controls.frameworks.manage');
  }

  function bindTabs() {
    const tabs = document.querySelectorAll('#controls-tabs .tab-btn');
    tabs.forEach(btn => {
      btn.addEventListener('click', () => {
        if (btn.disabled || btn.hidden) return;
        const target = btn.dataset.tab;
        if (target) switchTab(target);
      });
    });
  }

  function switchTab(tabId) {
    activeTab = tabId || activeTab;
    document.querySelectorAll('#controls-tabs .tab-btn').forEach(btn => {
      btn.classList.toggle('active', btn.dataset.tab === activeTab);
    });
    document.querySelectorAll('.controls-panels .tab-panel').forEach(panel => {
      panel.hidden = panel.dataset.tab !== activeTab;
    });
    updateTabInUrl(activeTab);
    if (activeTab === 'controls-tab-controls') loadControls();
    if (activeTab === 'controls-tab-checks') loadChecks();
    if (activeTab === 'controls-tab-violations') loadViolations();
    if (activeTab === 'controls-tab-frameworks') loadFrameworks();
  }

  function bindOverview() {
    const refreshBtn = document.getElementById('controls-overview-refresh');
    if (refreshBtn) {
      refreshBtn.addEventListener('click', () => loadOverview());
    }
    const metrics = document.getElementById('controls-overview-metrics');
    if (metrics) {
      metrics.addEventListener('click', (e) => {
        const card = e.target.closest('.metric-tile');
        if (!card || !card.dataset.filter) return;
        applyOverviewFilter(card.dataset.filter);
      });
    }
  }

  function applyOverviewFilter(filterKey) {
    overviewFilters = { lastCheck: '', noOwner: false, noChecks: false, status: '' };
    switch (filterKey) {
      case 'last_check.pass':
        overviewFilters.lastCheck = 'pass';
        break;
      case 'last_check.partial':
        overviewFilters.lastCheck = 'partial';
        break;
      case 'last_check.fail':
        overviewFilters.lastCheck = 'fail';
        break;
      case 'status.not_implemented':
        overviewFilters.status = 'not_implemented';
        break;
      case 'risk.high':
        document.getElementById('controls-filter-risk').value = 'high';
        break;
      case 'risk.critical':
        document.getElementById('controls-filter-risk').value = 'critical';
        break;
      case 'no_owner':
        overviewFilters.noOwner = true;
        break;
      case 'no_checks':
        overviewFilters.noChecks = true;
        break;
      default:
        break;
    }
    switchTab('controls-tab-controls');
    loadControls();
  }

  function bindControlsTab() {
    const applyBtn = document.getElementById('controls-apply-filters');
    if (applyBtn) applyBtn.addEventListener('click', () => loadControls());
    const createBtn = document.getElementById('controls-create-btn');
    if (createBtn) {
      createBtn.addEventListener('click', () => openControlModal());
    }
    const controlForm = document.getElementById('control-form');
    if (controlForm) {
      controlForm.addEventListener('submit', (e) => {
        e.preventDefault();
        saveControl();
      });
    }
    const deleteBtn = document.getElementById('control-delete-btn');
    if (deleteBtn) {
      deleteBtn.addEventListener('click', () => deleteControl());
    }
    const linkAdd = document.getElementById('control-link-add');
    if (linkAdd) {
      linkAdd.addEventListener('click', () => addControlLink());
    }
    const linkType = document.getElementById('control-link-target-type');
    if (linkType) {
      linkType.addEventListener('change', () => refreshLinkTargets());
    }
    const linkSearch = document.getElementById('control-link-search');
    if (linkSearch) {
      linkSearch.addEventListener('input', () => refreshLinkTargets());
    }
    document.getElementById('controls-table')?.addEventListener('click', (e) => {
      const editBtn = e.target.closest('[data-control-edit]');
      if (editBtn) {
        openControlModal(editBtn.dataset.controlEdit);
        return;
      }
      const checkBtn = e.target.closest('[data-control-check]');
      if (checkBtn) {
        openCheckModal(checkBtn.dataset.controlCheck);
        return;
      }
      const row = e.target.closest('tr');
      if (row && row.dataset.controlId && !shouldIgnoreRowClick(e.target)) {
        openControlDetail(row.dataset.controlId);
      }
    });
    populateControlStaticSelects();
  }

  function bindChecksTab() {
    const applyBtn = document.getElementById('controls-check-apply');
    if (applyBtn) applyBtn.addEventListener('click', () => loadChecks());
    const createBtn = document.getElementById('controls-check-create');
    if (createBtn) createBtn.addEventListener('click', () => openCheckModal());
    const checkForm = document.getElementById('check-form');
    if (checkForm) checkForm.addEventListener('submit', (e) => {
      e.preventDefault();
      saveCheck();
    });
    document.getElementById('controls-checks-table')?.addEventListener('click', (e) => {
      const delBtn = e.target.closest('[data-check-delete]');
      if (delBtn) {
        deleteCheck(delBtn.dataset.checkDelete);
        return;
      }
      const row = e.target.closest('tr');
      if (row && row.dataset.checkId && !shouldIgnoreRowClick(e.target)) {
        openCheckDetail(row.dataset.checkId);
      }
    });
  }

  function bindViolationsTab() {
    const applyBtn = document.getElementById('controls-violation-apply');
    if (applyBtn) applyBtn.addEventListener('click', () => loadViolations());
    const createBtn = document.getElementById('controls-violation-create');
    if (createBtn) createBtn.addEventListener('click', () => openViolationModal());
    const form = document.getElementById('violation-form');
    if (form) form.addEventListener('submit', (e) => {
      e.preventDefault();
      saveViolation();
    });
    const incidentSearch = document.getElementById('violation-incident-search');
    if (incidentSearch) {
      incidentSearch.addEventListener('input', () => refreshIncidentOptions());
    }
    document.getElementById('controls-violations-table')?.addEventListener('click', (e) => {
      const delBtn = e.target.closest('[data-violation-delete]');
      if (delBtn) {
        deleteViolation(delBtn.dataset.violationDelete);
        return;
      }
      const row = e.target.closest('tr');
      if (row && row.dataset.violationId && !shouldIgnoreRowClick(e.target)) {
        openViolationDetail(row.dataset.violationId);
      }
    });
  }

  function bindFrameworksTab() {
    const createBtn = document.getElementById('framework-create-btn');
    if (createBtn) createBtn.addEventListener('click', () => openFrameworkModal());
    const form = document.getElementById('framework-form');
    if (form) form.addEventListener('submit', (e) => {
      e.preventDefault();
      saveFramework();
    });
    const itemBtn = document.getElementById('framework-item-create');
    if (itemBtn) itemBtn.addEventListener('click', () => openFrameworkItemModal());
    const itemForm = document.getElementById('framework-item-form');
    if (itemForm) itemForm.addEventListener('submit', (e) => {
      e.preventDefault();
      saveFrameworkItem();
    });
    const mapForm = document.getElementById('framework-map-form');
    if (mapForm) mapForm.addEventListener('submit', (e) => {
      e.preventDefault();
      saveFrameworkMap();
    });
    document.getElementById('framework-list')?.addEventListener('click', (e) => {
      const btn = e.target.closest('[data-framework-id]');
      if (btn) openFrameworkDetails(btn.dataset.frameworkId);
    });
    document.getElementById('framework-items-table')?.addEventListener('click', (e) => {
      const mapBtn = e.target.closest('[data-framework-map]');
      if (mapBtn) openFrameworkMapModal(mapBtn.dataset.frameworkMap);
    });
  }

  function bindModalCloseButtons() {
    document.querySelectorAll('#controls-page [data-close]').forEach(btn => {
      btn.addEventListener('click', () => {
        const target = btn.getAttribute('data-close');
        if (target) closeModal(target);
      });
    });
  }

  function populateControlStaticSelects() {
    populateSelect('control-type', controlsTypes());
    populateSelect('control-frequency', controlsFrequencies());
    populateSelect('control-status', controlsStatuses());
    populateSelect('control-risk', controlsRisks());
    populateSelect('check-result', controlsCheckResults());
    populateSelect('violation-severity', controlsRisks());
    populateSelect('controls-filter-status', controlsStatuses(true));
    populateSelect('controls-filter-risk', controlsRisks(true));
    populateSelect('controls-check-filter-result', controlsCheckResults(true));
    populateSelect('controls-violation-filter-severity', controlsRisks(true));
  }

  function controlsTypes() {
    const items = state.controlTypes && state.controlTypes.length
      ? state.controlTypes.slice()
      : DEFAULT_CONTROL_TYPES.map(name => ({ name, is_builtin: true }));
    items.sort((a, b) => (a.name || '').localeCompare(b.name || ''));
    return items.filter(item => item && item.name).map(item => ({
      value: item.name,
      label: labelForType(item.name, item.is_builtin)
    }));
  }

  function controlsStatuses(includeAll) {
    const items = [
      { value: 'implemented', label: t('controls.status.implemented') },
      { value: 'partial', label: t('controls.status.partial') },
      { value: 'not_implemented', label: t('controls.status.not_implemented') },
      { value: 'not_applicable', label: t('controls.status.not_applicable') }
    ];
    return includeAll ? [{ value: '', label: t('controls.filter.all') }, ...items] : items;
  }

  function controlsRisks(includeAll) {
    const items = [
      { value: 'low', label: t('controls.risk.low') },
      { value: 'medium', label: t('controls.risk.medium') },
      { value: 'high', label: t('controls.risk.high') },
      { value: 'critical', label: t('controls.risk.critical') }
    ];
    return includeAll ? [{ value: '', label: t('controls.filter.all') }, ...items] : items;
  }

  function controlsFrequencies() {
    return [
      { value: 'manual', label: t('controls.frequency.manual') },
      { value: 'daily', label: t('controls.frequency.daily') },
      { value: 'weekly', label: t('controls.frequency.weekly') },
      { value: 'monthly', label: t('controls.frequency.monthly') },
      { value: 'quarterly', label: t('controls.frequency.quarterly') },
      { value: 'semiannual', label: t('controls.frequency.semiannual') },
      { value: 'annual', label: t('controls.frequency.annual') }
    ];
  }

  function controlsCheckResults(includeAll) {
    const items = [
      { value: 'pass', label: t('controls.check.pass') },
      { value: 'partial', label: t('controls.check.partial') },
      { value: 'fail', label: t('controls.check.fail') },
      { value: 'not_applicable', label: t('controls.check.not_applicable') }
    ];
    return includeAll ? [{ value: '', label: t('controls.filter.all') }, ...items] : items;
  }

  function populateSelect(id, items) {
    const select = document.getElementById(id);
    if (!select) return;
    select.innerHTML = '';
    items.forEach(item => {
      const opt = document.createElement('option');
      opt.value = item.value;
      opt.textContent = item.label;
      select.appendChild(opt);
    });
  }

  function refreshControlTypeSelects() {
    populateSelect('control-type', controlsTypes());
  }

  function refreshDomainSelects() {
    populateDomainSelect('controls-filter-domain', true);
    populateDomainSelect('control-domain', false);
  }

  function populateDomainSelect(id, includeAll) {
    const select = document.getElementById(id);
    if (!select) return;
    const domains = getDomains();
    select.innerHTML = '';
    if (includeAll) {
      const opt = document.createElement('option');
      opt.value = '';
      opt.textContent = t('controls.filter.all');
      select.appendChild(opt);
    }
    domains.forEach(domain => {
      const opt = document.createElement('option');
      opt.value = domain;
      opt.textContent = domainLabel(domain);
      select.appendChild(opt);
    });
  }

  function domainLabel(code) {
    const key = `controls.domain.${(code || '').toLowerCase()}`;
    const label = t(key);
    if (label !== key) return label;
    return code;
  }

  async function loadControlTypes() {
    try {
      const res = await Api.get('/api/controls/types');
      state.controlTypes = res.items || [];
    } catch (_) {
      state.controlTypes = [];
    }
  }
  async function loadControls() {
    const params = new URLSearchParams();
    const q = document.getElementById('controls-filter-q')?.value || '';
    const status = document.getElementById('controls-filter-status')?.value || '';
    const risk = document.getElementById('controls-filter-risk')?.value || '';
    const domain = document.getElementById('controls-filter-domain')?.value || '';
    const owner = document.getElementById('controls-filter-owner')?.value || '';
    const tag = document.getElementById('controls-filter-tag')?.value || '';
    const active = document.getElementById('controls-filter-active')?.value || '';
    if (q) params.set('q', q);
    if (status) params.set('status', status);
    if (risk) params.set('risk', risk);
    if (domain) params.set('domain', domain);
    if (owner) params.set('owner', owner);
    if (tag) params.set('tag', tag);
    if (active && active !== 'all') {
      params.set('active', active);
    }
    try {
      const res = await Api.get(`/api/controls${params.toString() ? `?${params}` : ''}`);
      state.controls = res.items || [];
      renderControlsTable();
      populateControlOptions();
      refreshDomainSelects();
      if (pendingControlId) {
        const target = pendingControlId;
        pendingControlId = '';
        openControlModal(target);
        const url = new URL(window.location.href);
        url.searchParams.delete('control');
        window.history.replaceState({}, '', url.toString());
      }
    } catch (err) {
      console.error('controls load', err);
    }
  }

  async function loadOverview() {
    try {
      const res = await Api.get('/api/controls');
      const items = res.items || [];
      renderOverview(items);
    } catch (err) {
      console.error('controls overview', err);
    }
  }

  function renderOverview(items) {
    const metrics = document.getElementById('controls-overview-metrics');
    if (!metrics) return;
    const total = items.length;
    const lastPass = items.filter(c => c.last_check_result === 'pass').length;
    const lastPartial = items.filter(c => c.last_check_result === 'partial').length;
    const lastFail = items.filter(c => c.last_check_result === 'fail').length;
    const notImplemented = items.filter(c => c.status === 'not_implemented').length;
    const highRisk = items.filter(c => c.risk_level === 'high' || c.risk_level === 'critical').length;
    const noOwner = items.filter(c => !c.owner_user_id).length;
    const noChecks = items.filter(c => !c.last_check_at).length;
    const cards = [
      { label: t('controls.overview.total'), value: total, filter: '' },
      { label: t('controls.overview.pass'), value: lastPass, filter: 'last_check.pass' },
      { label: t('controls.overview.partial'), value: lastPartial, filter: 'last_check.partial' },
      { label: t('controls.overview.fail'), value: lastFail, filter: 'last_check.fail' },
      { label: t('controls.overview.notImplemented'), value: notImplemented, filter: 'status.not_implemented' },
      { label: t('controls.overview.highRisk'), value: highRisk, filter: 'risk.high' },
      { label: t('controls.overview.noOwner'), value: noOwner, filter: 'no_owner' },
      { label: t('controls.overview.noChecks'), value: noChecks, filter: 'no_checks' }
    ];
    metrics.innerHTML = '';
    cards.forEach(card => {
      const div = document.createElement('div');
      div.className = 'metric-tile clickable';
      div.dataset.filter = card.filter;
      div.innerHTML = `<div class="metric-value">${card.value}</div><div class="metric-label">${escapeHtml(card.label)}</div>`;
      metrics.appendChild(div);
    });
  }

  function renderControlsTable() {
    const tbody = document.querySelector('#controls-table tbody');
    const empty = document.getElementById('controls-empty');
    if (!tbody) return;
    let items = [...state.controls];
    if (overviewFilters.status) {
      items = items.filter(c => c.status === overviewFilters.status);
    }
    if (overviewFilters.lastCheck) {
      items = items.filter(c => c.last_check_result === overviewFilters.lastCheck);
    }
    if (overviewFilters.noOwner) {
      items = items.filter(c => !c.owner_user_id);
    }
    if (overviewFilters.noChecks) {
      items = items.filter(c => !c.last_check_at);
    }
    tbody.innerHTML = '';
    if (!items.length) {
      if (empty) empty.hidden = false;
      return;
    }
    if (empty) empty.hidden = true;
    items.forEach(c => {
      const tr = document.createElement('tr');
      tr.dataset.controlId = c.id;
      tr.classList.add('clickable-row');
      const owner = c.owner_user_id && typeof UserDirectory !== 'undefined' ? UserDirectory.name(c.owner_user_id) : '-';
      const lastCheck = c.last_check_at ? formatDate(c.last_check_at) : '-';
      const tags = (c.tags || []).join(', ');
      const actions = [];
      if (hasPerm('controls.manage')) {
        actions.push(`<button class="btn btn-sm ghost" data-control-edit="${c.id}">${t('controls.actions.edit')}</button>`);
      }
      if (hasPerm('controls.checks.manage')) {
        actions.push(`<button class="btn btn-sm secondary" data-control-check="${c.id}">${t('controls.actions.addCheck')}</button>`);
      }
      tr.innerHTML = `
        <td>${escapeHtml(c.code)}</td>
        <td>${escapeHtml(c.title)}</td>
        <td>${escapeHtml(labelForType(c.control_type))}</td>
        <td>${escapeHtml(domainLabel(c.domain))}</td>
        <td>${escapeHtml(labelForStatus(c.status))}</td>
        <td>${escapeHtml(labelForRisk(c.risk_level))}</td>
        <td>${escapeHtml(owner)}</td>
        <td>${escapeHtml(lastCheck)}</td>
        <td>${escapeHtml(tags || '-')}</td>
        <td class="table-actions">${actions.join(' ') || '-'}</td>
      `;
      tbody.appendChild(tr);
    });
  }

  function populateControlOptions() {
    const controls = state.controls || [];
    populateSelectOptions('controls-filter-owner', controls, true, (c) => c.owner_user_id, (c) => ownerName(c.owner_user_id));
    populateUserSelect('controls-check-filter-owner', true);
    populateControlSelect('controls-check-filter-control', true);
    populateControlSelect('controls-violation-filter-control', true);
    populateControlSelect('check-control', false);
    populateControlSelect('violation-control', false);
    refreshDomainSelects();
  }

  function populateControlSelect(id, includeAll) {
    const select = document.getElementById(id);
    if (!select) return;
    const current = select.value;
    select.innerHTML = '';
    if (includeAll) {
      const opt = document.createElement('option');
      opt.value = '';
      opt.textContent = t('controls.filter.all');
      select.appendChild(opt);
    }
    (state.controls || []).forEach(c => {
      const opt = document.createElement('option');
      opt.value = c.id;
      opt.textContent = `${c.code} - ${c.title}`;
      select.appendChild(opt);
    });
    if (current) select.value = current;
  }

  function populateSelectOptions(id, items, includeAll, valueGetter, labelGetter) {
    const select = document.getElementById(id);
    if (!select) return;
    select.innerHTML = '';
    if (includeAll) {
      const opt = document.createElement('option');
      opt.value = '';
      opt.textContent = t('controls.filter.all');
      select.appendChild(opt);
    }
    const seen = new Set();
    items.forEach(item => {
      const value = valueGetter(item);
      if (!value || seen.has(value)) return;
      seen.add(value);
      const opt = document.createElement('option');
      opt.value = value;
      opt.textContent = labelGetter(item);
      select.appendChild(opt);
    });
  }

  function populateUserSelect(id, includeAll) {
    const select = document.getElementById(id);
    if (!select) return;
    select.innerHTML = '';
    if (includeAll) {
      const opt = document.createElement('option');
      opt.value = '';
      opt.textContent = t('controls.filter.all');
      select.appendChild(opt);
    }
    if (typeof UserDirectory === 'undefined' || !UserDirectory.all) return;
    UserDirectory.all().forEach(u => {
      const opt = document.createElement('option');
      opt.value = u.id;
      opt.textContent = u.full_name || u.username;
      select.appendChild(opt);
    });
  }

  function ownerName(ownerID) {
    if (!ownerID) return '-';
    if (typeof UserDirectory !== 'undefined') return UserDirectory.name(ownerID);
    return `#${ownerID}`;
  }

  async function getControlById(id) {
    if (!id) return null;
    const cached = state.controls.find(c => `${c.id}` === `${id}`);
    if (cached) return cached;
    try {
      return await Api.get(`/api/controls/${id}`);
    } catch (_) {
      return null;
    }
  }

  function openControlModal(id) {
    const modal = document.getElementById('control-modal');
    const alert = document.getElementById('control-modal-alert');
    if (!modal) return;
    if (alert) alert.hidden = true;
    const deleteBtn = document.getElementById('control-delete-btn');
    const titleEl = document.getElementById('control-modal-title');
    const linksSection = document.getElementById('control-links-section');
    const linksEmpty = document.getElementById('control-links-empty');
    const linkAdd = document.getElementById('control-link-add');
    const linkTarget = document.getElementById('control-link-target-id');
    const linkSearch = document.getElementById('control-link-search');
    resetControlForm();
    state.controlLinks = [];
    renderControlLinks();
    populateOwnerSelect('control-owner');
    refreshDomainSelects();
    if (!id) {
      if (titleEl) titleEl.textContent = t('controls.modal.createTitle');
      if (deleteBtn) deleteBtn.hidden = true;
      if (linksSection) linksSection.hidden = true;
      modal.hidden = false;
      return;
    }
    Api.get(`/api/controls/${id}`).then(control => {
      document.getElementById('control-id').value = control.id;
      document.getElementById('control-code').value = control.code || '';
      document.getElementById('control-title').value = control.title || '';
      document.getElementById('control-description').value = control.description_md || '';
      document.getElementById('control-type').value = control.control_type || '';
      document.getElementById('control-domain').value = control.domain || '';
      document.getElementById('control-owner').value = control.owner_user_id || '';
      document.getElementById('control-frequency').value = control.review_frequency || '';
      document.getElementById('control-status').value = control.status || '';
      document.getElementById('control-risk').value = control.risk_level || '';
      document.getElementById('control-tags').value = (control.tags || []).join(', ');
      if (titleEl) titleEl.textContent = t('controls.modal.editTitle');
      if (deleteBtn) deleteBtn.hidden = !hasPerm('controls.manage');
      if (linksSection) linksSection.hidden = !hasPerm('controls.view');
      if (linkAdd) linkAdd.disabled = !hasPerm('controls.manage');
      if (linkTarget) linkTarget.disabled = !hasPerm('controls.manage');
      if (linkSearch) linkSearch.disabled = !hasPerm('controls.manage');
      loadControlLinks(control.id);
      refreshLinkTargets();
      modal.hidden = false;
    });
  }

  function resetControlForm() {
    ['control-id', 'control-code', 'control-title', 'control-description', 'control-tags'].forEach(id => {
      const el = document.getElementById(id);
      if (el) el.value = '';
    });
  }

  function populateOwnerSelect(id) {
    const select = document.getElementById(id);
    if (!select) return;
    select.innerHTML = '';
    const emptyOpt = document.createElement('option');
    emptyOpt.value = '';
    emptyOpt.textContent = '-';
    select.appendChild(emptyOpt);
    if (typeof UserDirectory === 'undefined' || !UserDirectory.all) return;
    UserDirectory.all().forEach(u => {
      const opt = document.createElement('option');
      opt.value = u.id;
      opt.textContent = u.full_name || u.username;
      select.appendChild(opt);
    });
  }

  async function saveControl() {
    const alert = document.getElementById('control-modal-alert');
    if (alert) alert.hidden = true;
    const id = document.getElementById('control-id').value;
    const payload = {
      code: document.getElementById('control-code').value.trim(),
      title: document.getElementById('control-title').value.trim(),
      description_md: document.getElementById('control-description').value.trim(),
      control_type: document.getElementById('control-type').value,
      domain: document.getElementById('control-domain').value,
      owner_user_id: parseInt(document.getElementById('control-owner').value, 10) || null,
      review_frequency: document.getElementById('control-frequency').value,
      status: document.getElementById('control-status').value,
      risk_level: document.getElementById('control-risk').value,
      tags: splitList(document.getElementById('control-tags').value)
    };
    if (!payload.code || !payload.title) {
      showAlert(alert, t('controls.error.required'));
      return;
    }
    try {
      if (id) {
        await Api.put(`/api/controls/${id}`, payload);
      } else {
        await Api.post('/api/controls', payload);
      }
      closeModal('#control-modal');
      loadControls();
      loadOverview();
    } catch (err) {
      showAlert(alert, localizeError(err));
    }
  }

  async function deleteControl() {
    const id = document.getElementById('control-id').value;
    if (!id) return;
    const ok = window.confirm(t('common.confirm'));
    if (!ok) return;
    try {
      await Api.del(`/api/controls/${id}`);
      closeModal('#control-modal');
      loadControls();
      loadOverview();
    } catch (err) {
      showAlert(document.getElementById('control-modal-alert'), localizeError(err));
    }
  }

  async function loadControlLinks(controlId) {
    if (!controlId) return;
    try {
      const res = await Api.get(`/api/controls/${controlId}/links`);
      state.controlLinks = res.items || [];
    } catch (err) {
      state.controlLinks = [];
    }
    renderControlLinks();
  }

  async function ensureLinkOptions() {
    if (state.linkOptionsLoaded) return;
    try {
      const [docsRes, incidentsRes, tasksRes] = await Promise.all([
        Api.get('/api/docs/list?limit=200').catch(() => ({ items: [] })),
        Api.get('/api/incidents?limit=200').catch(() => ({ items: [] })),
        Api.get('/api/tasks?limit=200&include_archived=1').catch(() => ({ items: [] }))
      ]);
      state.linkOptions = {
        docs: docsRes.items || [],
        incidents: incidentsRes.items || [],
        tasks: tasksRes.items || []
      };
    } finally {
      state.linkOptionsLoaded = true;
    }
  }

  function refreshLinkTargets() {
    const type = document.getElementById('control-link-target-type')?.value || '';
    const select = document.getElementById('control-link-target-id');
    const search = (document.getElementById('control-link-search')?.value || '').toLowerCase().trim();
    if (!select) return;
    select.innerHTML = '';
    const placeholder = document.createElement('option');
    placeholder.value = '';
    placeholder.textContent = t('controls.links.targetPlaceholder');
    select.appendChild(placeholder);
    if (!type) return;
    ensureLinkOptions().then(() => {
      let items = [];
      if (type === 'doc') items = state.linkOptions.docs || [];
      if (type === 'incident') items = state.linkOptions.incidents || [];
      if (type === 'task') items = state.linkOptions.tasks || [];
      items
        .filter(item => linkOptionLabel(type, item).toLowerCase().includes(search))
        .forEach(item => {
          const opt = document.createElement('option');
          opt.value = linkOptionValue(type, item);
          opt.textContent = linkOptionLabel(type, item);
          select.appendChild(opt);
        });
    });
  }

  function refreshIncidentOptions() {
    const select = document.getElementById('violation-incident');
    const search = (document.getElementById('violation-incident-search')?.value || '').toLowerCase().trim();
    if (!select) return;
    select.innerHTML = '';
    const placeholder = document.createElement('option');
    placeholder.value = '';
    placeholder.textContent = t('controls.incidents.selectPlaceholder');
    select.appendChild(placeholder);
    ensureLinkOptions().then(() => {
      const items = state.linkOptions.incidents || [];
      items
        .filter(item => incidentOptionLabel(item).toLowerCase().includes(search))
        .forEach(item => {
          const opt = document.createElement('option');
          opt.value = linkOptionValue('incident', item);
          opt.textContent = incidentOptionLabel(item);
          select.appendChild(opt);
        });
    });
  }

  function renderControlLinks() {
    const list = document.getElementById('control-links-list');
    const empty = document.getElementById('control-links-empty');
    if (!list) return;
    list.innerHTML = '';
    if (!state.controlLinks.length) {
      if (empty) {
        empty.textContent = t('controls.links.empty');
        empty.hidden = false;
      }
      return;
    }
    if (empty) empty.hidden = true;
    state.controlLinks.forEach(link => {
      const row = document.createElement('div');
      row.className = 'link-item';
      const label = `${linkTypeLabel(link.target_type)} #${link.target_id}`;
      const relation = relationLabel(link.relation_type);
      const title = link.target_title ? ` - ${link.target_title}` : '';
      row.innerHTML = `
        <span>${escapeHtml(label)}${title ? ` <span class="muted">${escapeHtml(title)}</span>` : ''}</span>
        <span class="muted">${escapeHtml(relation)}</span>
      `;
      const actions = document.createElement('div');
      actions.className = 'table-actions';
      const openBtn = buildTargetOpenButton(link.target_type, link.target_id);
      if (openBtn) actions.appendChild(openBtn);
      if (hasPerm('controls.manage')) {
        const removeBtn = document.createElement('button');
        removeBtn.type = 'button';
        removeBtn.className = 'btn ghost btn-xs';
        removeBtn.textContent = t('controls.actions.delete');
        removeBtn.addEventListener('click', () => deleteControlLink(link.id));
        actions.appendChild(removeBtn);
      }
      row.appendChild(actions);
      list.appendChild(row);
    });
  }

  function linkOptionValue(type, item) {
    if (!item) return '';
    if (type === 'incident') {
      return item.reg_no || item.regNo || item.id || '';
    }
    return item.id || '';
  }

  function linkOptionLabel(type, item) {
    if (!item) return '';
    if (type === 'doc') {
      const reg = item.reg_no ? `${item.reg_no} ` : '';
      return `${reg}${item.title || ''}`.trim();
    }
    if (type === 'task') {
      return `#${item.id} ${item.title || ''}`.trim();
    }
    if (type === 'incident') {
      return incidentOptionLabel(item);
    }
    return `${item.id || ''}`.trim();
  }

  function incidentOptionLabel(inc) {
    if (!inc) return '';
    const reg = inc.reg_no || inc.regNo || `#${inc.id}`;
    const title = inc.title || '';
    const severity = inc.severity ? labelForIncidentSeverity(inc.severity) : '';
    const status = inc.status ? labelForIncidentStatus(inc.status) : '';
    const owner = inc.owner_name || '';
    const created = inc.created_at ? formatDate(inc.created_at) : '';
    const updated = inc.updated_at ? formatDate(inc.updated_at) : '';
    return `${reg} | ${title} | ${severity} | ${status} | ${owner} | ${created} | ${updated}`.replace(/\s+\|\s+$/, '').trim();
  }

  async function addControlLink() {
    const controlId = document.getElementById('control-id')?.value || '';
    if (!controlId) return;
    const targetType = document.getElementById('control-link-target-type')?.value || '';
    const targetId = document.getElementById('control-link-target-id')?.value || '';
    const relationType = document.getElementById('control-link-relation')?.value || '';
    if (relationType === 'violates' && targetType === 'incident') {
      showAlert(document.getElementById('control-modal-alert'), t('controls.links.violationInfo'));
      setTimeout(() => {
        const alert = document.getElementById('control-modal-alert');
        if (alert) alert.hidden = true;
      }, 2000);
    }
    if (!targetType || !targetId) {
      showAlert(document.getElementById('control-modal-alert'), t('controls.links.required'));
      return;
    }
    try {
      await Api.post(`/api/controls/${controlId}/links`, {
        target_type: targetType,
        target_id: targetId.trim(),
        relation_type: relationType
      });
      document.getElementById('control-link-target-id').value = '';
      document.getElementById('control-link-search').value = '';
      await loadControlLinks(controlId);
      refreshLinkTargets();
    } catch (err) {
      showAlert(document.getElementById('control-modal-alert'), localizeError(err));
    }
  }

  async function deleteControlLink(linkId) {
    const controlId = document.getElementById('control-id')?.value || '';
    if (!controlId || !linkId) return;
    if (!window.confirm(t('common.confirm'))) return;
    try {
      await Api.del(`/api/controls/${controlId}/links/${linkId}`);
      await loadControlLinks(controlId);
    } catch (err) {
      showAlert(document.getElementById('control-modal-alert'), localizeError(err));
    }
  }

  function openCheckModal(controlId) {
    const modal = document.getElementById('check-modal');
    const alert = document.getElementById('check-modal-alert');
    if (alert) alert.hidden = true;
    if (!modal) return;
    document.getElementById('check-notes').value = '';
    document.getElementById('check-evidence').value = '';
    document.getElementById('check-date').value = '';
    populateControlSelect('check-control', false);
    document.getElementById('check-control').value = controlId || '';
    modal.hidden = false;
  }

  async function saveCheck() {
    const alert = document.getElementById('check-modal-alert');
    if (alert) alert.hidden = true;
    const controlId = document.getElementById('check-control').value;
    if (!controlId) {
      showAlert(alert, t('controls.error.required'));
      return;
    }
    const payload = {
      result: document.getElementById('check-result').value,
      checked_at: datetimeToISO(document.getElementById('check-date').value),
      notes_md: document.getElementById('check-notes').value.trim(),
      evidence_links: splitList(document.getElementById('check-evidence').value)
    };
    try {
      await Api.post(`/api/controls/${controlId}/checks`, payload);
      closeModal('#check-modal');
      loadChecks();
      loadControls();
      loadOverview();
    } catch (err) {
      showAlert(alert, localizeError(err));
    }
  }

  async function loadChecks() {
    const params = new URLSearchParams();
    const controlId = document.getElementById('controls-check-filter-control')?.value || '';
    const result = document.getElementById('controls-check-filter-result')?.value || '';
    const owner = document.getElementById('controls-check-filter-owner')?.value || '';
    const from = document.getElementById('controls-check-filter-from')?.value || '';
    const to = document.getElementById('controls-check-filter-to')?.value || '';
    const fromISO = (typeof AppTime !== 'undefined' && AppTime.toISODate) ? AppTime.toISODate(from) : from;
    const toISO = (typeof AppTime !== 'undefined' && AppTime.toISODate) ? AppTime.toISODate(to) : to;
    if (controlId) params.set('control_id', controlId);
    if (result) params.set('result', result);
    if (owner) params.set('owner', owner);
    if (fromISO) params.set('date_from', fromISO);
    if (toISO) params.set('date_to', toISO);
    try {
      const res = await Api.get(`/api/checks${params.toString() ? `?${params}` : ''}`);
      state.checks = res.items || [];
      renderChecksTable();
    } catch (err) {
      console.error('checks load', err);
    }
  }

  function renderChecksTable() {
    const tbody = document.querySelector('#controls-checks-table tbody');
    const empty = document.getElementById('controls-checks-empty');
    if (!tbody) return;
    tbody.innerHTML = '';
    if (!state.checks.length) {
      if (empty) empty.hidden = false;
      return;
    }
    if (empty) empty.hidden = true;
    state.checks.forEach(check => {
      const tr = document.createElement('tr');
      tr.dataset.checkId = check.id;
      tr.classList.add('clickable-row');
      const notes = (check.notes_md || '').slice(0, 80);
      const checkedBy = check.checked_by ? ownerName(check.checked_by) : '-';
      const actions = hasPerm('controls.checks.manage')
        ? `<button class="btn btn-sm ghost" data-check-delete="${check.id}">${t('controls.actions.delete')}</button>`
        : '-';
      tr.innerHTML = `
        <td>${escapeHtml(check.control_code || '')}</td>
        <td>${escapeHtml(check.control_title || '')}</td>
        <td>${escapeHtml(labelForCheck(check.result))}</td>
        <td>${escapeHtml(formatDate(check.checked_at))}</td>
        <td>${escapeHtml(checkedBy)}</td>
        <td>${escapeHtml(notes || '-')}</td>
        <td>${actions}</td>
      `;
      tbody.appendChild(tr);
    });
  }

  async function deleteCheck(id) {
    if (!id) return;
    if (!window.confirm(t('common.confirm'))) return;
    try {
      await Api.del(`/api/checks/${id}`);
      loadChecks();
      loadControls();
      loadOverview();
    } catch (err) {
      console.error('delete check', err);
    }
  }

  function openViolationModal(controlId) {
    const modal = document.getElementById('violation-modal');
    const alert = document.getElementById('violation-modal-alert');
    if (alert) alert.hidden = true;
    if (!modal) return;
    document.getElementById('violation-summary').value = '';
    document.getElementById('violation-impact').value = '';
    document.getElementById('violation-incident').value = '';
    document.getElementById('violation-incident-search').value = '';
    document.getElementById('violation-date').value = '';
    populateControlSelect('violation-control', false);
    document.getElementById('violation-control').value = controlId || '';
    refreshIncidentOptions();
    modal.hidden = false;
  }

  async function saveViolation() {
    const alert = document.getElementById('violation-modal-alert');
    if (alert) alert.hidden = true;
    const controlId = document.getElementById('violation-control').value;
    if (!controlId) {
      showAlert(alert, t('controls.error.required'));
      return;
    }
    const incidentVal = document.getElementById('violation-incident').value.trim();
    const payload = {
      severity: document.getElementById('violation-severity').value,
      happened_at: datetimeToISO(document.getElementById('violation-date').value),
      summary: document.getElementById('violation-summary').value.trim(),
      impact_md: document.getElementById('violation-impact').value.trim()
    };
    if (incidentVal) {
      payload.incident_id = incidentVal;
    }
    try {
      await Api.post(`/api/controls/${controlId}/violations`, payload);
      closeModal('#violation-modal');
      loadViolations();
      loadControls();
      loadOverview();
    } catch (err) {
      showAlert(alert, localizeError(err));
    }
  }

  async function loadViolations() {
    const params = new URLSearchParams();
    const controlId = document.getElementById('controls-violation-filter-control')?.value || '';
    const severity = document.getElementById('controls-violation-filter-severity')?.value || '';
    const from = document.getElementById('controls-violation-filter-from')?.value || '';
    const to = document.getElementById('controls-violation-filter-to')?.value || '';
    const fromISO = (typeof AppTime !== 'undefined' && AppTime.toISODate) ? AppTime.toISODate(from) : from;
    const toISO = (typeof AppTime !== 'undefined' && AppTime.toISODate) ? AppTime.toISODate(to) : to;
    if (controlId) params.set('control_id', controlId);
    if (severity) params.set('severity', severity);
    if (fromISO) params.set('date_from', fromISO);
    if (toISO) params.set('date_to', toISO);
    try {
      const res = await Api.get(`/api/violations${params.toString() ? `?${params}` : ''}`);
      state.violations = res.items || [];
      renderViolationsTable();
    } catch (err) {
      console.error('violations load', err);
    }
  }

  function renderViolationsTable() {
    const tbody = document.querySelector('#controls-violations-table tbody');
    const empty = document.getElementById('controls-violations-empty');
    if (!tbody) return;
    tbody.innerHTML = '';
    if (!state.violations.length) {
      if (empty) empty.hidden = false;
      return;
    }
    if (empty) empty.hidden = true;
    state.violations.forEach(v => {
      const actions = hasPerm('controls.violations.manage')
        ? `<button class="btn btn-sm ghost" data-violation-delete="${v.id}">${t('controls.actions.delete')}</button>`
        : '-';
      const tr = document.createElement('tr');
      tr.dataset.violationId = v.id;
      tr.classList.add('clickable-row');
      const autoLabel = v.is_auto ? t('controls.violations.auto') : '-';
      const incidentLink = v.incident_id
        ? `<a href="/incidents?incident=${encodeURIComponent(v.incident_id)}">#${escapeHtml(v.incident_id)}</a>`
        : '-';
      tr.innerHTML = `
        <td>${escapeHtml(v.control_code || '')}</td>
        <td>${escapeHtml(v.control_title || '')}</td>
        <td>${escapeHtml(labelForRisk(v.severity))}</td>
        <td>${escapeHtml(autoLabel)}</td>
        <td>${escapeHtml((v.summary || '').slice(0, 80))}</td>
        <td>${escapeHtml(formatDate(v.happened_at))}</td>
        <td>${incidentLink}</td>
        <td>${actions}</td>
      `;
      tbody.appendChild(tr);
    });
  }

  async function deleteViolation(id) {
    if (!id) return;
    if (!window.confirm(t('common.confirm'))) return;
    try {
      await Api.del(`/api/violations/${id}`);
      loadViolations();
      loadControls();
      loadOverview();
    } catch (err) {
      console.error('delete violation', err);
    }
  }

  async function loadFrameworks() {
    if (!hasPerm('controls.frameworks.view')) return;
    try {
      const res = await Api.get('/api/frameworks');
      state.frameworks = res.items || [];
      renderFrameworksList();
    } catch (err) {
      console.error('frameworks load', err);
    }
  }

  function renderFrameworksList() {
    const list = document.getElementById('framework-list');
    const empty = document.getElementById('frameworks-empty');
    if (!list) return;
    list.innerHTML = '';
    if (!state.frameworks.length) {
      if (empty) empty.hidden = false;
      return;
    }
    if (empty) empty.hidden = true;
    state.frameworks.forEach(f => {
      const statusLabel = f.is_active ? '' : ` - ${t('controls.filter.inactiveOnly')}`;
      const btn = document.createElement('button');
      btn.className = `framework-card ${state.selectedFramework && state.selectedFramework.id === f.id ? 'active' : ''}`;
      btn.dataset.frameworkId = f.id;
      btn.innerHTML = `
        <div class="framework-title">${escapeHtml(f.name)}</div>
        <div class="framework-meta">${escapeHtml(f.version || '-')}${escapeHtml(statusLabel)}</div>
      `;
      btn.addEventListener('click', () => selectFramework(f.id));
      list.appendChild(btn);
    });
  }

  async function selectFramework(id) {
    const framework = state.frameworks.find(f => `${f.id}` === `${id}`);
    if (!framework) return;
    state.selectedFramework = framework;
    renderFrameworksList();
    await loadFrameworkItems(framework.id);
    await loadFrameworkMap(framework.id);
  }

  async function loadFrameworkItems(frameworkID) {
    if (!frameworkID) return;
    try {
      const res = await Api.get(`/api/frameworks/${frameworkID}/items`);
      state.frameworkItems = res.items || [];
      renderFrameworkItems();
    } catch (err) {
      console.error('framework items load', err);
    }
  }

  async function loadFrameworkMap(frameworkID) {
    if (!frameworkID) return;
    try {
      const res = await Api.get(`/api/frameworks/${frameworkID}/map`);
      state.frameworkMap = res.items || [];
      renderFrameworkItems();
    } catch (err) {
      console.error('framework map load', err);
    }
  }

  function renderFrameworkItems() {
    const tbody = document.querySelector('#framework-items-table tbody');
    const empty = document.getElementById('framework-items-empty');
    const title = document.getElementById('framework-items-title');
    const subtitle = document.getElementById('framework-items-subtitle');
    const itemBtn = document.getElementById('framework-item-create');
    if (!tbody) return;
    tbody.innerHTML = '';
    if (!state.selectedFramework) {
      if (title) title.textContent = t('controls.table.items');
      if (subtitle) subtitle.textContent = t('controls.empty.noSelection');
      if (empty) empty.hidden = true;
      if (itemBtn) itemBtn.disabled = true;
      return;
    }
    if (title) title.textContent = state.selectedFramework.name;
    if (subtitle) subtitle.textContent = state.selectedFramework.version || '';
    if (itemBtn) itemBtn.disabled = false;
    if (!state.frameworkItems.length) {
      if (empty) empty.hidden = false;
      return;
    }
    if (empty) empty.hidden = true;
    const mapByItem = new Map();
    state.frameworkMap.forEach(m => {
      if (!mapByItem.has(m.framework_item_id)) {
        mapByItem.set(m.framework_item_id, []);
      }
      mapByItem.get(m.framework_item_id).push(m.control_id);
    });
    state.frameworkItems.forEach(item => {
      const mappedIds = mapByItem.get(item.id) || [];
      const mappedLabels = mappedIds.map(id => {
        const c = state.controls.find(ctrl => ctrl.id === id);
        return c ? c.code : `#${id}`;
      });
      const actions = hasPerm('controls.frameworks.manage')
        ? `<button class="btn btn-sm ghost" data-framework-map="${item.id}">${t('controls.actions.mapControls')}</button>`
        : '-';
      const tr = document.createElement('tr');
      tr.innerHTML = `
        <td>${escapeHtml(item.code)}</td>
        <td>${escapeHtml(item.title)}</td>
        <td>${escapeHtml(mappedLabels.join(', ') || '-')}</td>
        <td>${actions}</td>
      `;
      tbody.appendChild(tr);
    });
  }

  async function openFrameworkDetails(id) {
    await selectFramework(id);
    openModal('#framework-items-modal');
  }

  function openFrameworkModal() {
    const alert = document.getElementById('framework-modal-alert');
    if (alert) alert.hidden = true;
    document.getElementById('framework-name').value = '';
    document.getElementById('framework-version').value = '';
    document.getElementById('framework-active').checked = true;
    openModal('#framework-modal');
  }

  async function saveFramework() {
    const alert = document.getElementById('framework-modal-alert');
    if (alert) alert.hidden = true;
    const payload = {
      name: document.getElementById('framework-name').value.trim(),
      version: document.getElementById('framework-version').value.trim(),
      is_active: document.getElementById('framework-active').checked
    };
    if (!payload.name) {
      showAlert(alert, t('controls.error.frameworkNameRequired'));
      return;
    }
    try {
      await Api.post('/api/frameworks', payload);
      closeModal('#framework-modal');
      loadFrameworks();
    } catch (err) {
      showAlert(alert, localizeError(err));
    }
  }

  function openFrameworkItemModal() {
    const alert = document.getElementById('framework-item-alert');
    if (alert) alert.hidden = true;
    if (!state.selectedFramework) {
      showAlert(alert, t('controls.empty.noSelection'));
      return;
    }
    document.getElementById('framework-item-code').value = '';
    document.getElementById('framework-item-title').value = '';
    document.getElementById('framework-item-description').value = '';
    openModal('#framework-item-modal');
  }

  async function saveFrameworkItem() {
    const alert = document.getElementById('framework-item-alert');
    if (alert) alert.hidden = true;
    if (!state.selectedFramework) {
      showAlert(alert, t('controls.empty.noSelection'));
      return;
    }
    const payload = {
      code: document.getElementById('framework-item-code').value.trim(),
      title: document.getElementById('framework-item-title').value.trim(),
      description_md: document.getElementById('framework-item-description').value.trim()
    };
    if (!payload.code || !payload.title) {
      showAlert(alert, t('controls.error.required'));
      return;
    }
    try {
      await Api.post(`/api/frameworks/${state.selectedFramework.id}/items`, payload);
      closeModal('#framework-item-modal');
      loadFrameworkItems(state.selectedFramework.id);
    } catch (err) {
      showAlert(alert, localizeError(err));
    }
  }

  function openFrameworkMapModal(itemId) {
    const alert = document.getElementById('framework-map-alert');
    if (alert) alert.hidden = true;
    const select = document.getElementById('framework-map-controls');
    if (!select) return;
    select.innerHTML = '';
    const controlsList = state.controls || [];
    controlsList.forEach(c => {
      const opt = document.createElement('option');
      opt.value = c.id;
      opt.textContent = `${c.code} - ${c.title}`;
      select.appendChild(opt);
    });
    document.getElementById('framework-map-item-id').value = itemId || '';
    openModal('#framework-map-modal');
  }

  async function saveFrameworkMap() {
    const alert = document.getElementById('framework-map-alert');
    if (alert) alert.hidden = true;
    const itemId = parseInt(document.getElementById('framework-map-item-id').value, 10);
    const select = document.getElementById('framework-map-controls');
    if (!itemId || !select) {
      showAlert(alert, t('controls.error.required'));
      return;
    }
    const selected = Array.from(select.options).filter(o => o.selected).map(o => parseInt(o.value, 10));
    if (!selected.length) {
      showAlert(alert, t('controls.error.required'));
      return;
    }
    try {
      for (const controlId of selected) {
        await Api.post('/api/frameworks/map', {
          framework_item_id: itemId,
          control_id: controlId
        });
      }
      closeModal('#framework-map-modal');
      if (state.selectedFramework) {
        loadFrameworkMap(state.selectedFramework.id);
      }
    } catch (err) {
      showAlert(alert, localizeError(err));
    }
  }

  function showAlert(el, msg) {
    if (!el) return;
    el.textContent = msg;
    el.hidden = false;
  }

  function closeModal(selector) {
    const modal = document.querySelector(selector);
    if (modal) modal.hidden = true;
  }

  function openModal(selector) {
    const modal = document.querySelector(selector);
    if (modal) modal.hidden = false;
  }

  async function openControlDetail(controlId) {
    const control = await getControlById(controlId);
    if (!control) return;
    const title = `${control.code || ''}`.trim() || t('controls.title');
    const subtitle = control.title || '';
    setDetailHeader(title, subtitle);
    renderDetailMain([
      { label: t('controls.field.code'), value: control.code || '-' },
      { label: t('controls.field.title'), value: control.title || '-' },
      { label: t('controls.field.type'), value: labelForType(control.control_type) },
      { label: t('controls.field.domain'), value: domainLabel(control.domain) },
      { label: t('controls.field.status'), value: labelForStatus(control.status) },
      { label: t('controls.field.risk'), value: labelForRisk(control.risk_level) },
      { label: t('controls.field.owner'), value: ownerName(control.owner_user_id) },
      { label: t('controls.field.frequency'), value: labelForFrequency(control.review_frequency) },
      { label: t('controls.field.tags'), value: (control.tags || []).join(', ') || '-' },
      { label: t('controls.table.lastCheck'), value: control.last_check_at ? formatDate(control.last_check_at) : '-' }
    ]);
    openModal('#control-detail-modal');
    emitControlDetailOpened(control.id);
    await loadControlHistory(control.id);
  }

  async function openCheckDetail(checkId) {
    const check = state.checks.find(item => `${item.id}` === `${checkId}`);
    if (!check) return;
    const control = await getControlById(check.control_id);
    const title = t('controls.detail.checkTitle');
    const subtitle = control ? `${control.code || ''} ${control.title || ''}`.trim() : '';
    setDetailHeader(title, subtitle);
    renderDetailMain([
      { label: t('controls.field.control'), value: subtitle || '-' },
      { label: t('controls.field.result'), value: labelForCheck(check.result) },
      { label: t('controls.field.checkedAt'), value: formatDate(check.checked_at) },
      { label: t('controls.table.checkedBy'), value: ownerName(check.checked_by) },
      { label: t('controls.field.notes'), value: check.notes_md || '-' , wide: true },
      { label: t('controls.field.evidence'), value: (check.evidence_links || []).join(', ') || '-' , wide: true }
    ]);
    openModal('#control-detail-modal');
    emitControlDetailOpened(check.control_id);
    await loadControlHistory(check.control_id);
  }

  async function openViolationDetail(violationId) {
    const violation = state.violations.find(item => `${item.id}` === `${violationId}`);
    if (!violation) return;
    const control = await getControlById(violation.control_id);
    const title = t('controls.detail.violationTitle');
    const subtitle = control ? `${control.code || ''} ${control.title || ''}`.trim() : '';
    setDetailHeader(title, subtitle);
    renderDetailMain([
      { label: t('controls.field.control'), value: subtitle || '-' },
      { label: t('controls.field.severity'), value: labelForRisk(violation.severity) },
      { label: t('controls.field.happenedAt'), value: formatDate(violation.happened_at) },
      { label: t('controls.field.summary'), value: violation.summary || '-' , wide: true },
      { label: t('controls.field.impact'), value: violation.impact_md || '-' , wide: true },
      { label: t('controls.field.incidentId'), value: violation.incident_id ? `#${violation.incident_id}` : '-' },
      { label: t('controls.table.auto'), value: violation.is_auto ? t('common.yes') : t('common.no') }
    ]);
    openModal('#control-detail-modal');
    emitControlDetailOpened(violation.control_id);
    await loadControlHistory(violation.control_id);
  }

  function emitControlDetailOpened(controlId) {
    try {
      document.dispatchEvent(new CustomEvent('controls:detailOpened', { detail: { controlId } }));
    } catch (_) {
      // ignore
    }
  }

  function setDetailHeader(title, subtitle) {
    const titleEl = document.getElementById('control-detail-title');
    const subtitleEl = document.getElementById('control-detail-subtitle');
    if (titleEl) titleEl.textContent = title || '-';
    if (subtitleEl) subtitleEl.textContent = subtitle || '';
  }

  function renderDetailMain(items) {
    const container = document.getElementById('control-detail-main');
    if (!container) return;
    container.innerHTML = '';
    items.forEach(item => {
      const wrapper = document.createElement('div');
      wrapper.className = `detail-item${item.wide ? ' wide' : ''}`;
      const label = document.createElement('div');
      label.className = 'detail-label';
      label.textContent = item.label || '';
      const value = document.createElement('div');
      value.className = 'detail-value';
      value.textContent = item.value ?? '-';
      wrapper.appendChild(label);
      wrapper.appendChild(value);
      container.appendChild(wrapper);
    });
  }

  async function loadControlHistory(controlId) {
    const checksBody = document.querySelector('#control-detail-checks-table tbody');
    const violationsBody = document.querySelector('#control-detail-violations-table tbody');
    if (checksBody) checksBody.innerHTML = '';
    if (violationsBody) violationsBody.innerHTML = '';
    try {
      const [checksRes, violationsRes] = await Promise.all([
        Api.get(`/api/controls/${controlId}/checks`),
        Api.get(`/api/controls/${controlId}/violations`)
      ]);
      renderDetailChecks(checksRes.items || []);
      renderDetailViolations(violationsRes.items || []);
    } catch (err) {
      renderDetailChecks([]);
      renderDetailViolations([]);
    }
  }

  function renderDetailChecks(items) {
    const tbody = document.querySelector('#control-detail-checks-table tbody');
    const empty = document.getElementById('control-detail-checks-empty');
    if (!tbody) return;
    tbody.innerHTML = '';
    if (!items.length) {
      if (empty) empty.hidden = false;
      return;
    }
    if (empty) empty.hidden = true;
    items.forEach(check => {
      const tr = document.createElement('tr');
      const checkedBy = ownerName(check.checked_by);
      const notes = (check.notes_md || '').slice(0, 120);
      tr.innerHTML = `
        <td>${escapeHtml(formatDate(check.checked_at))}</td>
        <td>${escapeHtml(labelForCheck(check.result))}</td>
        <td>${escapeHtml(checkedBy)}</td>
        <td>${escapeHtml(notes || '-')}</td>
      `;
      tbody.appendChild(tr);
    });
  }

  function renderDetailViolations(items) {
    const tbody = document.querySelector('#control-detail-violations-table tbody');
    const empty = document.getElementById('control-detail-violations-empty');
    if (!tbody) return;
    tbody.innerHTML = '';
    if (!items.length) {
      if (empty) empty.hidden = false;
      return;
    }
    if (empty) empty.hidden = true;
    items.forEach(v => {
      const tr = document.createElement('tr');
      const incident = v.incident_id ? `#${v.incident_id}` : '-';
      tr.innerHTML = `
        <td>${escapeHtml(formatDate(v.happened_at))}</td>
        <td>${escapeHtml(labelForRisk(v.severity))}</td>
        <td>${escapeHtml((v.summary || '').slice(0, 120))}</td>
        <td>${escapeHtml(incident)}</td>
      `;
      tbody.appendChild(tr);
    });
  }

  function escapeHtml(str) {
    return (str || '').toString().replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  }

  function formatDate(val) {
    if (!val) return '';
    try {
      if (typeof AppTime !== 'undefined' && AppTime.formatDateTime) {
        return AppTime.formatDateTime(val);
      }
      const d = new Date(val);
      const pad = (num) => `${num}`.padStart(2, '0');
      return `${pad(d.getDate())}.${pad(d.getMonth() + 1)}.${d.getFullYear()} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
    } catch (_) {
      return val;
    }
  }

  function splitList(raw) {
    if (!raw) return [];
    return raw.split(/[\n,;]/).map(v => v.trim()).filter(Boolean);
  }

  function datetimeToISO(value) {
    if (!value) return '';
    if (typeof AppTime !== 'undefined' && AppTime.toISODateTime) {
      return AppTime.toISODateTime(value);
    }
    const dt = new Date(value);
    if (Number.isNaN(dt.getTime())) return '';
    return dt.toISOString();
  }

  function labelForType(val, isBuiltin) {
    const raw = (val || '').toString();
    if (!raw) return '-';
    const key = raw.toLowerCase();
    const builtin = isBuiltin || DEFAULT_CONTROL_TYPES.includes(key);
    if (builtin) {
      const lookup = `controls.type.${key}`;
      const translated = t(lookup);
      return translated === lookup ? raw : translated;
    }
    return raw;
  }

  function labelForStatus(val) {
    const map = {
      implemented: t('controls.status.implemented'),
      partial: t('controls.status.partial'),
      not_implemented: t('controls.status.not_implemented'),
      not_applicable: t('controls.status.not_applicable')
    };
    return map[val] || val || '-';
  }

  function labelForRisk(val) {
    const map = {
      low: t('controls.risk.low'),
      medium: t('controls.risk.medium'),
      high: t('controls.risk.high'),
      critical: t('controls.risk.critical')
    };
    return map[val] || val || '-';
  }

  function labelForIncidentSeverity(val) {
    if (!val) return '-';
    const key = `incidents.severity.${String(val).toLowerCase()}`;
    const label = t(key);
    return label === key ? val : label;
  }

  function labelForIncidentStatus(val) {
    if (!val) return '-';
    const key = `incidents.status.${String(val).toLowerCase()}`;
    const label = t(key);
    return label === key ? val : label;
  }

  function labelForFrequency(val) {
    const map = {
      manual: t('controls.frequency.manual'),
      daily: t('controls.frequency.daily'),
      weekly: t('controls.frequency.weekly'),
      monthly: t('controls.frequency.monthly'),
      quarterly: t('controls.frequency.quarterly'),
      semiannual: t('controls.frequency.semiannual'),
      annual: t('controls.frequency.annual')
    };
    return map[val] || val || '-';
  }

  function labelForCheck(val) {
    const map = {
      pass: t('controls.check.pass'),
      partial: t('controls.check.partial'),
      fail: t('controls.check.fail'),
      not_applicable: t('controls.check.not_applicable')
    };
    return map[val] || val || '-';
  }

  function linkTypeLabel(val) {
    const map = {
      doc: t('controls.links.type.doc'),
      task: t('controls.links.type.task'),
      incident: t('controls.links.type.incident')
    };
    return map[val] || val || '-';
  }

  function relationLabel(val) {
    const map = {
      related: t('controls.links.relation.related'),
      evidence: t('controls.links.relation.evidence'),
      implements: t('controls.links.relation.implements'),
      violates: t('controls.links.relation.violates')
    };
    return map[val] || val || '-';
  }

  function buildTargetOpenButton(targetType, targetId) {
    if (!targetType || !targetId) return null;
    let href = '';
    if (targetType === 'doc') {
      href = `/docs/${targetId}`;
    } else if (targetType === 'incident') {
      href = `/incidents?incident=${encodeURIComponent(targetId)}`;
    } else if (targetType === 'task') {
      href = `/tasks/task/${targetId}`;
    }
    if (!href) return null;
    const btn = document.createElement('a');
    btn.className = 'btn ghost btn-xs';
    btn.href = href;
    btn.textContent = t('controls.links.open');
    return btn;
  }

  function localizeError(err) {
    const raw = (err && err.message ? err.message : '').trim();
    const msg = raw ? t(raw) : t('common.error');
    return msg || raw || 'error';
  }

  function mergeOptionLists(defaults, extras) {
    const seen = new Set();
    const out = [];
    [...(defaults || []), ...(extras || [])].forEach(item => {
      const clean = (item || '').toString().trim();
      if (!clean) return;
      const key = clean.toLowerCase();
      if (seen.has(key)) return;
      seen.add(key);
      out.push(clean);
    });
    return out;
  }

  function loadCustomOptions() {
    if (customLoaded) return;
    customLoaded = true;
    try {
      const raw = localStorage.getItem(CUSTOM_OPTIONS_KEY);
      if (!raw) return;
      const parsed = JSON.parse(raw);
      state.customDomains = Array.isArray(parsed.domains) ? mergeOptionLists([], parsed.domains) : [];
    } catch (_) {
      state.customDomains = [];
    }
  }

  function saveCustomOptions({ domains }) {
    if (domains) {
      state.customDomains = mergeOptionLists([], domains);
    }
    try {
      localStorage.setItem(CUSTOM_OPTIONS_KEY, JSON.stringify({ domains: state.customDomains }));
    } catch (_) {
      // ignore storage errors
    }
    emitOptionChange();
  }

  function emitOptionChange() {
    try {
      if (typeof document !== 'undefined' && typeof CustomEvent !== 'undefined') {
        document.dispatchEvent(new CustomEvent('controls:optionsChanged'));
      }
    } catch (_) {
      // ignored
    }
  }

  function getDomains() {
    if (!customLoaded) loadCustomOptions();
    return mergeOptionLists(DEFAULT_DOMAINS, state.customDomains);
  }

  return {
    init,
    state,
    getDomains,
    loadCustomOptions,
    saveCustomOptions
  };
})();

if (typeof window !== 'undefined') {
  window.ControlsPage = ControlsPage;
  window.addEventListener('DOMContentLoaded', () => {
    if (document.getElementById('controls-page')) {
      ControlsPage.init();
    }
  });
}
