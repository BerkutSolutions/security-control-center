(() => {
  const globalObj = typeof window !== 'undefined' ? window : globalThis;
  const AccountsPage = globalObj.AccountsPage || (globalObj.AccountsPage = {});
  const state = AccountsPage.state || (AccountsPage.state = {
    roles: [],
    groups: [],
    users: [],
    roleTemplates: [],
    sessions: [],
    selected: new Set(),
    departments: [],
    editingUserId: null,
    editingGroupId: null,
    editingRoleId: null,
    selectedGroupId: null,
    selectedRoleId: null,
    groupEditMode: false,
    roleEditMode: false,
    groupDetails: {},
    importState: { id: null, headers: [], preview: [], createdUsers: [] },
    currentBulkAction: null,
    bulkPasswords: [],
    passwordVisible: false,
    referenceLoaded: false,
  });

  const MENU_OPTIONS = [
    { value: 'dashboard', labelKey: 'nav.dashboard' },
    { value: 'tasks', labelKey: 'nav.tasks' },
    { value: 'controls', labelKey: 'nav.controls' },
    { value: 'assets', labelKey: 'nav.assets' },
    { value: 'software', labelKey: 'nav.software' },
    { value: 'findings', labelKey: 'nav.findings' },
    { value: 'monitoring', labelKey: 'nav.monitoring' },
    { value: 'docs', labelKey: 'nav.docs' },
    { value: 'approvals', labelKey: 'nav.approvals' },
    { value: 'incidents', labelKey: 'nav.incidents' },
    { value: 'reports', labelKey: 'nav.reports' },
    { value: 'accounts', labelKey: 'nav.accounts' },
    { value: 'settings', labelKey: 'nav.settings' },
    { value: 'backups', labelKey: 'nav.backups' },
    { value: 'logs', labelKey: 'nav.logs' },
  ];
  const CLEARANCE_TAGS = [
    { value: 'COMMERCIAL_SECRET', labelKey: 'accounts.clearanceTag.commercial_secret' },
    { value: 'PERSONAL_DATA', labelKey: 'accounts.clearanceTag.personal_data' },
  ];

  AccountsPage.MENU_OPTIONS = MENU_OPTIONS;
  AccountsPage.CLEARANCE_TAGS = CLEARANCE_TAGS;

  function permissionLabel(permission) {
    const key = `accounts.permission.${permission}`;
    const label = BerkutI18n.t(key);
    if (label && label !== key) return label;
    return (permission || '').replace(/\./g, ' / ').replace(/_/g, ' ');
  }

  function permissionListLabel(perms) {
    return (perms || []).map(permissionLabel).join(', ');
  }

  function enhanceMultiSelects(ids = []) {
    ids.forEach(id => {
      const sel = document.getElementById(id);
      if (!sel) return;
      sel.multiple = true;
      sel.setAttribute('multiple', 'multiple');
      if (!sel.size || sel.size < 6) sel.size = 6;
      let suppressNotify = false;
      const refresh = () => {
        Array.from(sel.options).forEach(opt => {
          const base = opt.dataset.label || opt.textContent;
          opt.dataset.label = base;
          opt.textContent = base;
        });
        if (!suppressNotify) {
          suppressNotify = true;
          sel.dispatchEvent(new Event('selectionrefresh', { bubbles: false }));
          suppressNotify = false;
        }
      };
      const toggle = (opt) => {
        opt.selected = !opt.selected;
        refresh();
        sel.dispatchEvent(new Event('change', { bubbles: true }));
      };
      if (!sel.dataset.enhanced) {
        sel.dataset.enhanced = '1';
        sel.addEventListener('mousedown', (e) => {
          const opt = e.target.closest('option');
          if (!opt) return;
          e.preventDefault();
          toggle(opt);
        });
        sel.addEventListener('dblclick', (e) => {
          const opt = e.target.closest('option');
          if (!opt) return;
          toggle(opt);
        });
      }
      refresh();
    });
  }

  function availablePermissions() {
    const set = new Set();
    (state.roles || []).forEach(r => {
      (r.permissions || []).forEach(p => set.add(p));
    });
    (state.roleTemplates || []).forEach(t => {
      (t.permissions || []).forEach(p => set.add(p));
    });
    return Array.from(set).sort();
  }

  function renderSelectedOptions(selectEl, hintEl) {
    if (!selectEl || !hintEl) return;
    const selected = Array.from(selectEl.options).filter(o => o.selected);
    if (!selected.length) {
      const emptyLabel = BerkutI18n.t('common.notAvailable');
      hintEl.textContent = emptyLabel && emptyLabel !== 'common.notAvailable' ? emptyLabel : '-';
      return;
    }
    hintEl.innerHTML = '';
    selected.forEach(opt => {
      const tag = document.createElement('span');
      tag.className = 'tag';
      tag.textContent = opt.dataset.label || opt.textContent;
      hintEl.appendChild(tag);
    });
  }

  function renderTagList(containerId, items) {
    const container = document.getElementById(containerId);
    if (!container) return;
    container.innerHTML = '';
    if (!items || !items.length) {
      container.textContent = BerkutI18n.t('common.notAvailable') || '-';
      return;
    }
    items.forEach(text => {
      const tag = document.createElement('span');
      tag.className = 'tag';
      tag.textContent = text;
      container.appendChild(tag);
    });
  }

  function renderCommaList(containerId, items) {
    const container = document.getElementById(containerId);
    if (!container) return;
    if (!items || !items.length) {
      container.textContent = BerkutI18n.t('common.notAvailable') || '-';
      return;
    }
    container.textContent = items.join(', ');
  }

  function attachHintListener(selectEl, hintEl) {
    if (!selectEl || !hintEl || selectEl.dataset.hintBound) return;
    selectEl.dataset.hintBound = '1';
    selectEl.addEventListener('change', () => renderSelectedOptions(selectEl, hintEl));
    selectEl.addEventListener('selectionrefresh', () => renderSelectedOptions(selectEl, hintEl));
  }

  function clearanceLevelLabel(level) {
    if (typeof ClassificationDirectory !== 'undefined' && ClassificationDirectory.labelByLevel) {
      return ClassificationDirectory.labelByLevel(level);
    }
    const map = {
      0: 'accounts.clearanceLevel.public',
      1: 'accounts.clearanceLevel.internal',
      2: 'accounts.clearanceLevel.confidential',
      3: 'accounts.clearanceLevel.restricted',
    };
    const key = map[Number(level)];
    return key ? (BerkutI18n.t(key) || key) : `${BerkutI18n.t('accounts.clearanceLevel.generic') || 'Level'} ${level}`;
  }

  function levelByCode(code) {
    const all = ['PUBLIC', 'INTERNAL', 'CONFIDENTIAL', 'RESTRICTED', 'SECRET', 'TOP_SECRET', 'SPECIAL_IMPORTANCE'];
    const idx = all.indexOf(String(code || '').toUpperCase());
    return idx >= 0 ? idx : 0;
  }

  function getClearanceOptions() {
    if (typeof ClassificationDirectory !== 'undefined' && ClassificationDirectory.all) {
      return ClassificationDirectory.all().map((item) => ({
        value: `${levelByCode(item.code)}`,
        label: item.label || item.code,
      }));
    }
    return [
      { value: '0', label: clearanceLevelLabel(0) },
      { value: '1', label: clearanceLevelLabel(1) },
      { value: '2', label: clearanceLevelLabel(2) },
      { value: '3', label: clearanceLevelLabel(3) },
    ];
  }

  function syncClearanceSelects() {
    const ids = ['group-detail-clearance', 'group-clearance', 'user-clearance'];
    const options = getClearanceOptions();
    ids.forEach((id) => {
      const select = document.getElementById(id);
      if (!select) return;
      const prev = `${select.value || ''}`;
      fillSelect(select, options);
      if (options.some((opt) => opt.value === prev)) {
        select.value = prev;
      }
    });
    const filter = document.getElementById('filter-clearance');
    if (filter) {
      const prev = `${filter.value || ''}`;
      fillSelectWithEmpty(filter, options, BerkutI18n.t('accounts.filterClearanceAll') || BerkutI18n.t('common.all') || 'All');
      if (prev && options.some((opt) => opt.value === prev)) {
        filter.value = prev;
      }
    }
  }

  function clearanceTagLabel(tag) {
    const raw = (tag || '').toString();
    if (typeof TagDirectory !== 'undefined' && TagDirectory.label) {
      const label = TagDirectory.label(raw);
      if (label && label !== raw) return label;
    }
    const map = {
      COMMERCIAL_SECRET: 'accounts.clearanceTag.commercial_secret',
      PERSONAL_DATA: 'accounts.clearanceTag.personal_data',
    };
    const key = map[raw.toUpperCase()];
    return key ? (BerkutI18n.t(key) || key) : raw;
  }

  function groupImpactLines(group) {
    if (!group) return [];
    return [
      `${BerkutI18n.t('accounts.groups.roles') || 'Roles'}: ${(group.roles || []).join(', ') || '-'}`,
      `${BerkutI18n.t('accounts.groups.clearance') || 'Clearance'}: ${group.clearance_level || 0}`,
      `${BerkutI18n.t('accounts.groupMenuAccess') || 'Menu'}: ${(group.menu_permissions || []).join(', ') || '-'}`,
    ];
  }

  function renderClearanceTags(containerId, selected) {
    const container = document.getElementById(containerId);
    if (!container) return;
    container.innerHTML = '';
    const selectedSet = new Set((selected || []).map(t => t.toUpperCase()));
    const tags = (typeof TagDirectory !== 'undefined' && TagDirectory.codes)
      ? TagDirectory.codes()
      : CLEARANCE_TAGS.map(t => t.value);
    tags.forEach(value => {
      const label = document.createElement('label');
      label.className = 'checkbox-inline';
      const input = document.createElement('input');
      input.type = 'checkbox';
      input.value = value;
      input.checked = selectedSet.has(value.toUpperCase());
      label.appendChild(input);
      const span = document.createElement('span');
      span.textContent = clearanceTagLabel(value) || value;
      label.appendChild(span);
      container.appendChild(label);
    });
  }

  function setPasswordVisibility(visible) {
    state.passwordVisible = visible;
    const pwdInput = document.getElementById('user-password');
    const confirmInput = document.getElementById('user-password-confirm');
    const toggleBtn = document.getElementById('toggle-password-visibility');
    if (!pwdInput || !confirmInput || !toggleBtn) return;
    pwdInput.type = visible ? 'text' : 'password';
    confirmInput.type = visible ? 'text' : 'password';
    const labelKey = visible ? 'accounts.passwordHide' : 'accounts.passwordShow';
    const label = BerkutI18n.t(labelKey) || (visible ? 'Hide password' : 'Show password');
    toggleBtn.setAttribute('title', label);
    toggleBtn.setAttribute('aria-label', label);
    toggleBtn.classList.toggle('active', visible);
  }

  function generatePassword() {
    const length = 16;
    const sets = [
      'ABCDEFGHJKLMNPQRSTUVWXYZ',
      'abcdefghijkmnopqrstuvwxyz',
      '23456789',
      '!@#$%^&*()-_=+[]{}<>?'
    ];
    const all = sets.join('');
    const values = new Uint32Array(length);
    if (window.crypto && window.crypto.getRandomValues) {
      window.crypto.getRandomValues(values);
    } else {
      for (let i = 0; i < values.length; i += 1) {
        values[i] = Math.floor(Math.random() * all.length);
      }
    }
    const chars = [];
    sets.forEach((set, idx) => {
      chars.push(set[values[idx] % set.length]);
    });
    for (let i = sets.length; i < length; i += 1) {
      chars.push(all[values[i] % all.length]);
    }
    for (let i = chars.length - 1; i > 0; i -= 1) {
      const j = values[i] % (i + 1);
      [chars[i], chars[j]] = [chars[j], chars[i]];
    }
    return chars.join('');
  }

  function groupPermissions(perms) {
    const grouped = {};
    (perms || []).forEach(p => {
      const parts = (p || '').split('.');
      const prefix = parts.length > 1 ? parts[0] : 'other';
      if (!grouped[prefix]) grouped[prefix] = [];
      grouped[prefix].push(p);
    });
    Object.keys(grouped).forEach(k => grouped[k].sort());
    return grouped;
  }

  function formatDate(value) {
    if (!value) return '';
    try {
      if (typeof AppTime !== 'undefined' && AppTime.formatDateTime) {
        return AppTime.formatDateTime(value);
      }
      const d = new Date(value);
      if (Number.isNaN(d.getTime())) return value || '';
      const pad = (num) => `${num}`.padStart(2, '0');
      return `${pad(d.getDate())}.${pad(d.getMonth() + 1)}.${d.getFullYear()} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
    } catch (_) {
      return value || '';
    }
  }

  function showAlert(id, text) {
    const el = document.getElementById(id);
    if (!el) return;
    el.textContent = text;
    el.hidden = !text;
  }

  function setText(id, value) {
    const el = document.getElementById(id);
    if (el) el.textContent = value;
  }

  function escapeHtml(str) {
    return (str || '').toString().replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  }

  function debounce(fn, wait) {
    let t;
    return (...args) => {
      clearTimeout(t);
      t = setTimeout(() => fn(...args), wait);
    };
  }

  function fillSelect(selectEl, options) {
    if (!selectEl) return;
    selectEl.innerHTML = '';
    options.forEach(opt => {
      const el = document.createElement('option');
      el.value = opt.value;
      el.textContent = opt.label;
      selectEl.appendChild(el);
    });
  }

  function fillSelectWithEmpty(selectEl, options, placeholder) {
    if (!selectEl) return;
    const list = [{ value: '', label: placeholder || '-' }].concat(options || []);
    fillSelect(selectEl, list);
  }

  function getSelectedValues(selectEl) {
    return Array.from(selectEl.selectedOptions || []).map(o => o.value);
  }

  function setMultiSelect(selectEl, values) {
    const set = new Set(values.map(String));
    Array.from(selectEl.options || []).forEach(opt => {
      opt.selected = set.has(opt.value);
    });
  }

  function renderMenuOptions(containerId = 'group-menu') {
    const container = document.getElementById(containerId);
    if (!container) return;
    container.innerHTML = '';
    MENU_OPTIONS.forEach(opt => {
      const label = document.createElement('label');
      label.className = 'checkbox-inline';
      const input = document.createElement('input');
      input.type = 'checkbox';
      input.value = opt.value;
      label.appendChild(input);
      const span = document.createElement('span');
      span.textContent = BerkutI18n.t(opt.labelKey) || opt.value;
      label.appendChild(span);
      container.appendChild(label);
    });
  }

  function getCheckedValues(container) {
    if (!container) return [];
    return Array.from(container.querySelectorAll('input[type="checkbox"]:checked')).map(i => i.value);
  }

  function setCheckedValues(container, values) {
    if (!container) return;
    const set = new Set((values || []).map(String));
    container.querySelectorAll('input[type="checkbox"]').forEach(input => {
      input.checked = set.has(input.value);
    });
  }

  function renderGroupImpact(group, targetId = 'group-impact') {
    const target = document.getElementById(targetId);
    if (!target) return;
    target.innerHTML = '';
    if (!group) {
      target.textContent = BerkutI18n.t('common.notAvailable') || 'N/A';
      return;
    }
    const items = [];
    items.push(`${BerkutI18n.t('accounts.groups.roles') || 'Roles'}: ${(group.roles || []).join(', ') || '-'}`);
    items.push(`${BerkutI18n.t('accounts.groups.clearance') || 'Clearance'}: ${group.clearance_level || 0}`);
    items.push(`${BerkutI18n.t('accounts.groups.menuAccess') || 'Menu'}: ${(group.menu_permissions || []).join(', ') || '-'}`);
    items.forEach(text => {
      const div = document.createElement('div');
      div.textContent = text;
      target.appendChild(div);
    });
  }

  async function init() {
    const root = document.getElementById('accounts-page');
    if (!root) return;
    syncClearanceSelects();
    document.addEventListener('classifications:changed', syncClearanceSelects);
    if (AccountsPage.bindTabs) AccountsPage.bindTabs();
    if (AccountsPage.bindUserModal) AccountsPage.bindUserModal();
    if (AccountsPage.bindGroupModal) AccountsPage.bindGroupModal();
    if (AccountsPage.bindGroupDetails) AccountsPage.bindGroupDetails();
    if (AccountsPage.bindRoleCreateModal) AccountsPage.bindRoleCreateModal();
    if (AccountsPage.bindRoleDetails) AccountsPage.bindRoleDetails();
    if (AccountsPage.bindRoleTemplateModal) AccountsPage.bindRoleTemplateModal();
    if (AccountsPage.bindImportUI) AccountsPage.bindImportUI();
    const initialTab = AccountsPage.getInitialTab ? AccountsPage.getInitialTab() : 'accounts-dashboard';
    if (AccountsPage.switchTab) {
      await AccountsPage.switchTab(initialTab);
    }
    const refreshBtn = document.getElementById('refresh-users');
    if (refreshBtn) refreshBtn.addEventListener('click', () => AccountsPage.loadUsers && AccountsPage.loadUsers());
    const openCreate = document.getElementById('open-create');
    if (openCreate) openCreate.addEventListener('click', () => AccountsPage.openUserModal && AccountsPage.openUserModal());
    const openGroupCreate = document.getElementById('open-group-create');
    if (openGroupCreate) openGroupCreate.addEventListener('click', () => AccountsPage.openGroupModal && AccountsPage.openGroupModal());
    const groupsSearch = document.getElementById('groups-search');
    if (groupsSearch) groupsSearch.addEventListener('input', debounce(() => AccountsPage.renderGroups && AccountsPage.renderGroups(), 200));
    const rolesSearch = document.getElementById('roles-search');
    if (rolesSearch) rolesSearch.addEventListener('input', debounce(() => AccountsPage.renderRoles && AccountsPage.renderRoles(), 200));
    const filterStatus = document.getElementById('filter-status');
    if (filterStatus) filterStatus.addEventListener('change', () => AccountsPage.loadUsers && AccountsPage.loadUsers());
    const filterDept = document.getElementById('filter-dept');
    if (filterDept) filterDept.addEventListener('change', () => AccountsPage.loadUsers && AccountsPage.loadUsers());
    const filterRole = document.getElementById('filter-role');
    if (filterRole) filterRole.addEventListener('change', () => AccountsPage.loadUsers && AccountsPage.loadUsers());
    const filterGroup = document.getElementById('filter-group');
    if (filterGroup) filterGroup.addEventListener('change', () => AccountsPage.loadUsers && AccountsPage.loadUsers());
    const filterPassword = document.getElementById('filter-password');
    if (filterPassword) filterPassword.addEventListener('change', () => AccountsPage.loadUsers && AccountsPage.loadUsers());
    const filterClearance = document.getElementById('filter-clearance');
    if (filterClearance) filterClearance.addEventListener('change', () => AccountsPage.loadUsers && AccountsPage.loadUsers());
    const filterSearch = document.getElementById('filter-search');
    if (filterSearch) filterSearch.addEventListener('input', debounce(() => AccountsPage.loadUsers && AccountsPage.loadUsers(), 300));
    const selectAll = document.getElementById('select-all-users');
    if (selectAll) {
      selectAll.addEventListener('change', (e) => AccountsPage.toggleSelectAll && AccountsPage.toggleSelectAll(e.target.checked));
    }
    if (AccountsPage.bindBulkActions) AccountsPage.bindBulkActions();
  }

  function bindTabs() {
    document.querySelectorAll('.tab-btn').forEach(btn => {
      btn.addEventListener('click', (e) => {
        e.preventDefault();
        if (AccountsPage.switchTab) AccountsPage.switchTab(btn.dataset.tab);
        updateUrlForTab(btn.dataset.tab, false);
      });
    });
  }

  function getInitialTab() {
    const tabFromPath = tabForPath(window.location.pathname || '');
    if (tabFromPath) return tabFromPath;
    const params = new URLSearchParams(window.location.search || '');
    const tab = params.get('tab');
    const valid = new Set(['accounts-dashboard', 'accounts-groups', 'accounts-users']);
    return valid.has(tab) ? tab : 'accounts-dashboard';
  }

  async function ensureReferenceLoaded() {
    if (state.referenceLoaded) return;
    if (AccountsPage.loadReference) {
      await AccountsPage.loadReference();
    }
    state.referenceLoaded = true;
  }

  async function switchTab(targetId) {
    document.querySelectorAll('.tab-btn').forEach(btn => {
      btn.classList.toggle('active', btn.dataset.tab === targetId);
    });
    document.querySelectorAll('.tab-panel').forEach(panel => {
      panel.hidden = panel.id !== targetId;
    });
    if (targetId === 'accounts-dashboard' && AccountsPage.loadDashboard) {
      await AccountsPage.loadDashboard();
    }
    if (targetId === 'accounts-groups') {
      await ensureReferenceLoaded();
      if (AccountsPage.loadGroups) await AccountsPage.loadGroups();
    }
    if (targetId === 'accounts-users') {
      await ensureReferenceLoaded();
      if (AccountsPage.loadUsers) await AccountsPage.loadUsers();
    }
  }

  function tabForPath(pathname) {
    const parts = (pathname || '').split('/').filter(Boolean);
    if (parts[0] !== 'accounts') return '';
    if (parts[1] === 'groups') return 'accounts-groups';
    if (parts[1] === 'users') return 'accounts-users';
    return 'accounts-dashboard';
  }

  function pathForTab(tabId) {
    if (tabId === 'accounts-groups') return '/accounts/groups';
    if (tabId === 'accounts-users') return '/accounts/users';
    return '/accounts';
  }

  function updateUrlForTab(tabId, replace) {
    const nextPath = pathForTab(tabId);
    if (!nextPath || window.location.pathname === nextPath) return;
    if (replace) {
      window.history.replaceState({}, '', nextPath);
    } else {
      window.history.pushState({}, '', nextPath);
    }
  }

  AccountsPage.permissionLabel = permissionLabel;
  AccountsPage.permissionListLabel = permissionListLabel;
  AccountsPage.enhanceMultiSelects = enhanceMultiSelects;
  AccountsPage.availablePermissions = availablePermissions;
  AccountsPage.renderSelectedOptions = renderSelectedOptions;
  AccountsPage.renderTagList = renderTagList;
  AccountsPage.renderCommaList = renderCommaList;
  AccountsPage.attachHintListener = attachHintListener;
  AccountsPage.clearanceLevelLabel = clearanceLevelLabel;
  AccountsPage.clearanceTagLabel = clearanceTagLabel;
  AccountsPage.groupImpactLines = groupImpactLines;
  AccountsPage.renderClearanceTags = renderClearanceTags;
  AccountsPage.setPasswordVisibility = setPasswordVisibility;
  AccountsPage.generatePassword = generatePassword;
  AccountsPage.groupPermissions = groupPermissions;
  AccountsPage.formatDate = formatDate;
  AccountsPage.showAlert = showAlert;
  AccountsPage.setText = setText;
  AccountsPage.escapeHtml = escapeHtml;
  AccountsPage.debounce = debounce;
  AccountsPage.fillSelect = fillSelect;
  AccountsPage.fillSelectWithEmpty = fillSelectWithEmpty;
  AccountsPage.getSelectedValues = getSelectedValues;
  AccountsPage.setMultiSelect = setMultiSelect;
  AccountsPage.renderMenuOptions = renderMenuOptions;
  AccountsPage.getCheckedValues = getCheckedValues;
  AccountsPage.setCheckedValues = setCheckedValues;
  AccountsPage.renderGroupImpact = renderGroupImpact;
  AccountsPage.syncClearanceSelects = syncClearanceSelects;
  AccountsPage.init = init;
  AccountsPage.bindTabs = bindTabs;
  AccountsPage.getInitialTab = getInitialTab;
  AccountsPage.ensureReferenceLoaded = ensureReferenceLoaded;
  AccountsPage.switchTab = switchTab;
  AccountsPage.updateUrlForTab = updateUrlForTab;

  if (typeof window !== 'undefined') {
    window.AccountsPage = AccountsPage;
    window.addEventListener('DOMContentLoaded', () => {
      if (document.getElementById('accounts-page')) {
        AccountsPage.init();
      }
    });
    window.addEventListener('popstate', () => {
      if (document.getElementById('accounts-page')) {
        const nextTab = AccountsPage.getInitialTab ? AccountsPage.getInitialTab() : 'accounts-dashboard';
        if (AccountsPage.switchTab) AccountsPage.switchTab(nextTab);
      }
    });
  }
})();
