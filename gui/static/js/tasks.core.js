const TasksPage = (() => {
  const state = {
    spaces: [],
    spaceMap: {},
    spaceSummary: {},
    spaceId: null,
    boardsBySpace: {},
    boards: [],
    boardId: null,
    columnsByBoard: {},
    subcolumnsByBoard: {},
    subcolumnsByColumn: {},
    tasksByBoard: {},
    archivedTasks: [],
    tasks: [],
    taskMap: new Map(),
    templates: [],
    templatesLoaded: false,
    templatesIncludeInactive: false,
    recurringRules: [],
    currentUser: null,
    usersLoaded: false,
    filters: { search: '', mine: false, overdue: false, high: false, archived: false },
    card: { taskId: null, original: null, dirty: false, templateId: null, ruleId: null },
    drag: { active: false, taskId: null, boardId: null },
  };

  function t(key) {
    const value = BerkutI18n.t(key);
    return value === key ? key : value;
  }

  function hasPermission(perm) {
    if (!perm) return true;
    const perms = Array.isArray(state.currentUser?.permissions) ? state.currentUser.permissions : [];
    if (!perms.length) return true;
    return perms.includes(perm);
  }

  function showAlert(el, msg) {
    const target = typeof el === 'string' ? document.getElementById(el) : el;
    if (!target) return;
    target.textContent = msg || '';
    target.hidden = !msg;
  }

  function hideAlert(el) {
    const target = typeof el === 'string' ? document.getElementById(el) : el;
    if (!target) return;
    target.hidden = true;
  }

  function openModal(id) {
    const el = document.getElementById(id);
    if (el) el.hidden = false;
  }

  function closeModal(id) {
    const el = document.getElementById(id);
    if (el) el.hidden = true;
  }

  function setConnectionBanner(visible) {
    const el = document.getElementById('tasks-connection-banner');
    if (el) el.hidden = !visible;
  }

  function isNetworkError(err) {
    const msg = (err?.message || '').toLowerCase();
    return msg.includes('failed to fetch') || msg.includes('networkerror') || msg.includes('network error');
  }

  function resolveErrorMessage(err, fallbackKey) {
    const raw = (err && err.message ? err.message : '').trim();
    if (raw) {
      const translated = t(raw);
      if (translated && translated !== raw) return translated;
      if (raw === 'forbidden') return t('common.accessDenied') || raw;
      if (raw === 'unauthorized') return t('common.accessDenied') || raw;
      if (raw === 'bad request') return t('common.error') || raw;
      if (raw === 'server error') return t('common.error') || raw;
      return raw;
    }
    return t(fallbackKey || 'common.error');
  }

  function showError(err, fallbackKey) {
    const msg = resolveErrorMessage(err, fallbackKey);
    window.alert(msg || 'Error');
  }

  function parseDate(value) {
    if (!value) return null;
    if (value instanceof Date) return Number.isNaN(value.getTime()) ? null : value;
    const d = new Date(value);
    return Number.isNaN(d.getTime()) ? null : d;
  }

  function formatDateTime(value) {
    const d = parseDate(value);
    if (!d) return value || '';
    if (typeof AppTime !== 'undefined' && AppTime.formatDateTime) {
      return AppTime.formatDateTime(d);
    }
    const pad = (num) => `${num}`.padStart(2, '0');
    return `${pad(d.getDate())}.${pad(d.getMonth() + 1)}.${d.getFullYear()} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
  }

  function formatDateShort(value) {
    const d = parseDate(value);
    if (!d) return '';
    if (typeof AppTime !== 'undefined' && AppTime.formatDate) {
      const formatted = AppTime.formatDate(d);
      return formatted === '-' ? '' : formatted;
    }
    const pad = (num) => `${num}`.padStart(2, '0');
    return `${pad(d.getDate())}.${pad(d.getMonth() + 1)}.${d.getFullYear()}`;
  }

  function toInputDate(value) {
    const d = parseDate(value);
    if (!d) return '';
    if (typeof AppTime !== 'undefined' && AppTime.toISODate) {
      const iso = AppTime.toISODate(AppTime.formatDate(d));
      if (iso) return iso;
    }
    const pad = (num) => `${num}`.padStart(2, '0');
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
  }

  function toISODate(dateValue) {
    const raw = (dateValue || '').trim();
    if (!raw) return '';
    const isoDate = (typeof AppTime !== 'undefined' && AppTime.toISODate) ? AppTime.toISODate(raw) : raw;
    if (!isoDate) return '';
    return `${isoDate}T00:00:00Z`;
  }

  function confirmAction(opts = {}) {
    const modal = document.getElementById('tasks-confirm-modal');
    if (!modal) {
      return Promise.resolve(window.confirm(opts.message || ''));
    }
    const title = document.getElementById('tasks-confirm-title');
    const message = document.getElementById('tasks-confirm-message');
    const yes = document.getElementById('tasks-confirm-yes');
    const no = document.getElementById('tasks-confirm-no');
    const close = document.getElementById('tasks-confirm-close');
    if (title) title.textContent = opts.title || t('common.confirm');
    if (message) message.textContent = opts.message || '';
    if (yes) yes.textContent = opts.confirmText || t('common.confirm');
    if (no) no.textContent = opts.cancelText || t('common.cancel');
    modal.hidden = false;
    return new Promise((resolve) => {
      const cleanup = () => {
        modal.hidden = true;
        yes?.removeEventListener('click', onYes);
        no?.removeEventListener('click', onNo);
        close?.removeEventListener('click', onNo);
      };
      const onYes = () => { cleanup(); resolve(true); };
      const onNo = () => { cleanup(); resolve(false); };
      yes?.addEventListener('click', onYes);
      no?.addEventListener('click', onNo);
      close?.addEventListener('click', onNo);
    });
  }

  async function loadCurrentUser() {
    const res = await Api.get('/api/auth/me');
    state.currentUser = res.user;
    return state.currentUser;
  }

  async function ensureUserDirectory() {
    if (state.usersLoaded) return;
    const dir = (typeof window !== 'undefined' && window.UserDirectory)
      ? window.UserDirectory
      : (typeof UserDirectory !== 'undefined' ? UserDirectory : null);
    if (dir && dir.load) {
      await dir.load();
      state.usersLoaded = true;
    }
  }

  function escapeHtml(str) {
    return (str || '').toString().replace(/[&<>"']/g, c => ({
      '&': '&amp;',
      '<': '&lt;',
      '>': '&gt;',
      '"': '&quot;',
      "'": '&#39;'
    }[c]));
  }

  const TAG_PALETTE = [
    { bg: 'rgba(96, 141, 255, 0.22)', border: 'rgba(96, 141, 255, 0.55)', text: '#dbe7ff' },
    { bg: 'rgba(82, 190, 150, 0.2)', border: 'rgba(82, 190, 150, 0.55)', text: '#d6fff2' },
    { bg: 'rgba(255, 163, 92, 0.2)', border: 'rgba(255, 163, 92, 0.55)', text: '#ffe3c9' },
    { bg: 'rgba(236, 120, 170, 0.22)', border: 'rgba(236, 120, 170, 0.5)', text: '#ffd6ea' },
    { bg: 'rgba(144, 120, 255, 0.2)', border: 'rgba(144, 120, 255, 0.55)', text: '#e5dcff' },
    { bg: 'rgba(120, 200, 235, 0.2)', border: 'rgba(120, 200, 235, 0.55)', text: '#d7f4ff' },
    { bg: 'rgba(200, 200, 120, 0.18)', border: 'rgba(200, 200, 120, 0.55)', text: '#f4f0c7' },
    { bg: 'rgba(180, 140, 100, 0.2)', border: 'rgba(180, 140, 100, 0.55)', text: '#f1e0cf' }
  ];

  function hashTag(tag) {
    const str = (tag || '').toString().toLowerCase();
    let hash = 0;
    for (let i = 0; i < str.length; i += 1) {
      hash = ((hash << 5) - hash) + str.charCodeAt(i);
      hash |= 0;
    }
    return Math.abs(hash);
  }

  function tagColors(tag) {
    const idx = hashTag(tag) % TAG_PALETTE.length;
    return TAG_PALETTE[idx];
  }

  function applyTagStyle(el, tag) {
    if (!el) return;
    const colors = tagColors(tag);
    el.style.background = colors.bg;
    el.style.borderColor = colors.border;
    el.style.color = colors.text;
  }

  async function ensureTemplates(includeInactive = false) {
    if (state.templatesLoaded && (!includeInactive || state.templatesIncludeInactive)) {
      return state.templates;
    }
    const query = includeInactive ? '?include_inactive=1' : '';
    const res = await Api.get(`/api/tasks/templates${query}`);
    state.templates = res.items || [];
    state.templatesLoaded = true;
    if (includeInactive) state.templatesIncludeInactive = true;
    return state.templates;
  }

  function populatePrioritySelect(select, selected) {
    if (!select) return;
    const options = ['low', 'medium', 'high', 'critical'];
    select.innerHTML = '';
    options.forEach(val => {
      const opt = document.createElement('option');
      opt.value = val;
      opt.textContent = t(`tasks.priority.${val}`);
      if (selected && selected === val) opt.selected = true;
      select.appendChild(opt);
    });
  }

  function populateAssigneesSelect(select, selected = []) {
    if (!select) return;
    const dir = (typeof window !== 'undefined' && window.UserDirectory)
      ? window.UserDirectory
      : (typeof UserDirectory !== 'undefined' ? UserDirectory : null);
    select.innerHTML = '';
    const selectedSet = new Set((selected || []).map(v => `${v}`));
    (dir?.all ? dir.all() : []).forEach(user => {
      const opt = document.createElement('option');
      opt.value = user.username || `${user.id}`;
      opt.textContent = user.full_name || user.username;
      if (selectedSet.has(opt.value) || selectedSet.has(`${user.id}`)) opt.selected = true;
      select.appendChild(opt);
    });
  }

  return {
    state,
    t,
    hasPermission,
    showAlert,
    hideAlert,
    openModal,
    closeModal,
    setConnectionBanner,
    isNetworkError,
    resolveErrorMessage,
    showError,
    parseDate,
    formatDateTime,
    formatDateShort,
    toInputDate,
    toISODate,
    confirmAction,
    loadCurrentUser,
    ensureUserDirectory,
    escapeHtml,
    applyTagStyle,
    ensureTemplates,
    populatePrioritySelect,
    populateAssigneesSelect
  };
})();

if (typeof window !== 'undefined') {
  window.TasksPage = TasksPage;
}
