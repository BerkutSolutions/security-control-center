const AccessesPage = (() => {
  const API_ENDPOINT = '/api/accesses';
  const state = {
    rows: [],
    activeViewId: '',
    actor: 'system',
    serviceFilter: '',
    serviceFilterOptions: [],
  };

  function init() {
    const root = document.getElementById('accesses-page');
    if (!root) return;
    clearLegacyLocalMirror();
    loadRowsRemote();
    fetchActor();
    bindActions();
    bindCreateForm();
    bindEditForm();
    bindSupplementForm();
    bindDismissalForm();
    bindViewActions();
    bindDirectoriesEvents();
    bindServiceFilter();
    renderServiceFilterChoices();
    renderCards();
  }

  function hasApiGet() {
    return typeof Api !== 'undefined' && typeof Api.get === 'function';
  }

  function hasApiPut() {
    return typeof Api !== 'undefined' && typeof Api.put === 'function';
  }

  function hasApiPost() {
    return typeof Api !== 'undefined' && typeof Api.post === 'function';
  }

  function nowISO() {
    return new Date().toISOString();
  }

  function formatDT(value) {
    if (window.AppTime?.formatDateTime) return AppTime.formatDateTime(value);
    return String(value || '-');
  }

  function cleanService(raw) {
    return String(raw || '').trim().toUpperCase();
  }

  function cleanText(raw) {
    return String(raw || '').trim();
  }

  function t(key) {
    return (window.BerkutI18n && BerkutI18n.t ? (BerkutI18n.t(key) || key) : key);
  }

  function currentLang() {
    return (typeof BerkutI18n !== 'undefined' && BerkutI18n.currentLang && BerkutI18n.currentLang() === 'en') ? 'en' : 'ru';
  }

  function normalizeISODate(raw) {
    const value = cleanText(raw);
    if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return '';
    const parts = value.split('-').map((x) => parseInt(x, 10));
    const year = parts[0];
    const month = parts[1];
    const day = parts[2];
    if (!year || !month || !day) return '';
    const dt = new Date(year, month - 1, day);
    if (Number.isNaN(dt.getTime())) return '';
    if (dt.getFullYear() !== year || dt.getMonth() !== month - 1 || dt.getDate() !== day) return '';
    const pad = (n) => String(n).padStart(2, '0');
    return `${year}-${pad(month)}-${pad(day)}`;
  }

  function dedupeServices(list) {
    return Array.from(new Set((list || []).map(cleanService).filter(Boolean)));
  }

  function normalizeEvent(event) {
    if (!event || typeof event !== 'object') return null;
    return {
      at: event.at || nowISO(),
      type: String(event.type || 'update'),
      added: dedupeServices(event.added || []),
      removed: dedupeServices(event.removed || []),
      details: cleanText(event.details || ''),
      by: cleanText(event.by || ''),
    };
  }

  function normalizeRow(row) {
    if (!row || typeof row !== 'object') return null;
    const user = cleanText(row.user);
    if (!user) return null;
    const created = row.created_at || nowISO();
    const updated = row.updated_at || created;
    const services = dedupeServices(row.services || []);
    const historyRaw = Array.isArray(row.history) ? row.history : [];
    const history = historyRaw.map(normalizeEvent).filter(Boolean);
    return {
      id: String(row.id || `${Date.now()}_${Math.random().toString(16).slice(2, 8)}`),
      user,
      services,
      position: cleanText(row.position || ''),
      department: cleanText(row.department || ''),
      blocked: !!row.blocked,
      created_at: created,
      updated_at: updated,
      history: history.length ? history : [{
        at: created,
        type: 'create',
        added: services,
        removed: [],
        details: '',
      }],
    };
  }

  async function loadRowsRemote() {
    if (!hasApiGet()) return;
    try {
      const payload = await Api.get(API_ENDPOINT);
      const items = Array.isArray(payload?.items) ? payload.items : [];
      state.rows = items.map(normalizeRow).filter(Boolean);
      renderServiceFilterChoices();
      renderCards();
    } catch (err) {
      showAlert(sanitizeError(err) || t('accesses.errors.saveFailed'));
    }
  }

  function buildAuditEvent(type, row, extra = {}) {
    const eventType = cleanText(type).toLowerCase();
    if (!eventType) return null;
    return {
      type: eventType,
      user: cleanText(row?.user || extra.user || ''),
      services: dedupeServices(row?.services || extra.services || []).map(serviceLabel),
      details: cleanText(extra.details || ''),
    };
  }

  async function saveRows(eventMeta = null) {
    return saveRowsRemote(eventMeta);
  }

  function clearLegacyLocalMirror() {
    try {
      localStorage.removeItem('berkut.accesses.users');
    } catch (_) {
      // ignore
    }
  }

  async function saveRowsRemote(eventMeta = null) {
    if (!hasApiPut()) {
      showAlert(t('accesses.errors.saveFailed'));
      return false;
    }
    try {
      const payload = { items: state.rows };
      if (eventMeta && typeof eventMeta === 'object' && cleanText(eventMeta.type)) {
        payload.event = {
          type: cleanText(eventMeta.type).toLowerCase(),
          user: cleanText(eventMeta.user || ''),
          services: Array.isArray(eventMeta.services) ? eventMeta.services.map((s) => cleanText(s)).filter(Boolean) : [],
          details: cleanText(eventMeta.details || ''),
        };
      }
      await Api.put(API_ENDPOINT, payload);
      return true;
    } catch (err) {
      showAlert(sanitizeError(err) || t('accesses.errors.saveFailed'));
      return false;
    }
  }

  function cloneRows() {
    return JSON.parse(JSON.stringify(state.rows || []));
  }

  function restoreRows(snapshot) {
    state.rows = Array.isArray(snapshot) ? snapshot.map(normalizeRow).filter(Boolean) : [];
    renderServiceFilterChoices();
    renderCards();
    if (state.activeViewId) {
      const exists = getRowById(state.activeViewId);
      if (exists) openViewModal(state.activeViewId);
      else closeModal('#accesses-view-modal');
    }
  }

  async function persistOrRollback(snapshot, eventMeta = null) {
    const ok = await saveRows(eventMeta);
    if (ok) return true;
    restoreRows(snapshot);
    return false;
  }

  function sanitizeError(err) {
    const msg = String(err?.message || err || '').trim();
    if (!msg) return t('common.serverError');
    const translated = t(msg);
    return translated === msg ? msg : translated;
  }

  function getRowById(id) {
    return state.rows.find(r => r.id === id) || null;
  }

  function serviceLabel(code) {
    return window.ServiceDirectory?.label ? ServiceDirectory.label(code) : code;
  }

  function compareServices(prev, next) {
    const a = new Set(dedupeServices(prev));
    const b = new Set(dedupeServices(next));
    return {
      added: Array.from(b).filter(v => !a.has(v)),
      removed: Array.from(a).filter(v => !b.has(v)),
    };
  }

  function logEvent(row, event) {
    const ev = normalizeEvent(event);
    if (!ev) return;
    if (!ev.by) ev.by = state.actor || 'system';
    row.history = Array.isArray(row.history) ? row.history : [];
    row.history.push(ev);
    row.updated_at = ev.at;
  }

  async function fetchActor() {
    try {
      const res = await fetch('/api/auth/me', { credentials: 'include' });
      if (!res.ok) return;
      const data = await res.json();
      const login = cleanText(
        data?.user?.username ||
        data?.user?.login ||
        data?.username ||
        data?.login ||
        data?.user?.full_name ||
        ''
      );
      if (!login) return;
      state.actor = login;
      migrateSystemActors(login);
    } catch (_) {
      // Keep fallback actor.
    }
  }

  function migrateSystemActors(actor) {
    if (!actor || actor === 'system') return;
    let changed = false;
    state.rows.forEach((row) => {
      if (!Array.isArray(row.history)) return;
      row.history = row.history.map((ev) => {
        if (!ev || typeof ev !== 'object') return ev;
        const by = cleanText(ev.by || '');
        if (by && by.toLowerCase() !== 'system') return ev;
        changed = true;
        return { ...ev, by: actor };
      });
    });
    if (changed) {
      saveRows(buildAuditEvent('cleanup', null, { details: 'migrate_system_actor' })).catch(() => {});
      renderCards();
      if (state.activeViewId) openViewModal(state.activeViewId);
    }
  }

  function bindActions() {
    const openBtn = document.getElementById('accesses-open-create');
    if (openBtn) {
      openBtn.addEventListener('click', () => {
        renderServicesSelect('accesses-create-services', 'accesses-create-services-hint', []);
        openModal('#accesses-create-modal');
      });
    }
  }

  function bindDirectoriesEvents() {
    document.addEventListener('services:changed', async () => {
      const before = cloneRows();
      const known = new Set((window.ServiceDirectory?.codes ? ServiceDirectory.codes() : []).map(cleanService));
      state.rows.forEach((row) => {
        const before = dedupeServices(row.services);
        const after = before.filter(code => known.has(code));
        const diff = compareServices(before, after);
        if (!diff.removed.length) return;
        row.services = after;
        logEvent(row, {
          at: nowISO(),
          type: 'cleanup',
          added: [],
          removed: diff.removed,
        });
      });
      await persistOrRollback(before, buildAuditEvent('cleanup', null, { details: 'services_changed' }));
      renderServiceFilterChoices();
      renderCards();
      if (state.activeViewId) openViewModal(state.activeViewId);
    });
  }

  function bindServiceFilter() {
    const input = document.getElementById('accesses-service-filter');
    const dropdown = document.getElementById('accesses-service-filter-dropdown');
    if (!input || !dropdown) return;
    let hideTimer = null;
    const sync = () => {
      state.serviceFilter = String(input.value || '').trim().toLowerCase();
      renderCards();
      renderServiceFilterChoices(input.value || '');
      dropdown.hidden = false;
    };
    input.addEventListener('input', sync);
    input.addEventListener('focus', () => {
      if (hideTimer) window.clearTimeout(hideTimer);
      renderServiceFilterChoices(input.value || '');
      dropdown.hidden = false;
    });
    input.addEventListener('blur', () => {
      hideTimer = window.setTimeout(() => { dropdown.hidden = true; }, 120);
    });
    input.addEventListener('keydown', (e) => {
      if (e.key === 'Escape') {
        input.value = '';
        state.serviceFilter = '';
        dropdown.hidden = true;
        renderCards();
      }
    });
    dropdown.addEventListener('mousedown', (e) => {
      const item = e.target.closest('.accesses-service-search-item');
      if (!item) return;
      e.preventDefault();
      const value = item.dataset.value || '';
      input.value = value;
      state.serviceFilter = value.trim().toLowerCase();
      dropdown.hidden = true;
      renderCards();
    });
  }

  function serviceCollator() {
    return new Intl.Collator(['ru', 'en'], { sensitivity: 'base', numeric: true });
  }

  function sortedServiceDirectory() {
    const collator = serviceCollator();
    return (window.ServiceDirectory?.all ? ServiceDirectory.all() : [])
      .map((item) => ({
        code: cleanService(item?.code),
        label: cleanText(item?.label || item?.code),
      }))
      .filter((item) => item.code && item.label)
      .sort((a, b) => collator.compare(a.label, b.label));
  }

  function renderServiceFilterChoices(filterText = '') {
    const dropdown = document.getElementById('accesses-service-filter-dropdown');
    if (!dropdown) return;
    const q = String(filterText || '').trim().toLowerCase();
    const all = sortedServiceDirectory().map((item) => item.label);
    state.serviceFilterOptions = all;
    const filtered = q
      ? all.filter((label) => label.toLowerCase().includes(q))
      : all;
    if (!filtered.length) {
      dropdown.innerHTML = `<div class="accesses-service-search-empty">${escapeHtml(BerkutI18n.t('common.empty'))}</div>`;
      return;
    }
    dropdown.innerHTML = filtered
      .map((label) => `<button type="button" class="accesses-service-search-item" data-value="${escapeHtml(label)}">${escapeHtml(label)}</button>`)
      .join('');
  }

  function selectedServices(selectId) {
    const select = document.getElementById(selectId);
    return dedupeServices(Array.from(select?.selectedOptions || []).map(o => o.value));
  }

  function renderServicesSelect(selectId, hintId, selectedValues, excludedValues = []) {
    const select = document.getElementById(selectId);
    const hint = document.getElementById(hintId);
    if (!select) return;
    const selected = new Set(dedupeServices(selectedValues || []));
    const excluded = new Set(dedupeServices(excludedValues || []));
    const options = sortedServiceDirectory();
    select.innerHTML = '';
    options.forEach(item => {
      const code = cleanService(item.code);
      if (excluded.has(code)) return;
      const opt = document.createElement('option');
      opt.value = code;
      opt.textContent = item.label;
      opt.dataset.label = item.label;
      opt.selected = selected.has(code);
      select.appendChild(opt);
    });
    enhanceMultiSelectLikeDocs(select);
    bindTagHintLikeDocs(select, hint);
    return select.options.length;
  }

  function bindCreateForm() {
    const form = document.getElementById('accesses-create-form');
    if (!form) return;
    form.addEventListener('submit', async (e) => {
      e.preventDefault();
      const before = cloneRows();
      const user = cleanText(document.getElementById('accesses-create-user')?.value);
      const services = selectedServices('accesses-create-services');
      const position = cleanText(document.getElementById('accesses-create-position')?.value);
      const department = cleanText(document.getElementById('accesses-create-department')?.value);
      if (!user) return showAlert(BerkutI18n.t('accesses.errors.userRequired'));
      if (!services.length) return showAlert(BerkutI18n.t('accesses.errors.servicesRequired'));

      const existing = state.rows.find(r => r.user.toLowerCase() === user.toLowerCase());
      if (existing) {
        const diff = compareServices(existing.services, services);
        existing.services = services;
        existing.position = position;
        existing.department = department;
        logEvent(existing, {
          at: nowISO(),
          type: 'update',
          added: diff.added,
          removed: diff.removed,
          details: buildProfileDetails(position, department),
          by: state.actor,
        });
      } else {
        const created = nowISO();
        const row = normalizeRow({
          id: `${Date.now()}_${Math.random().toString(16).slice(2, 8)}`,
          user,
          services,
          position,
          department,
          blocked: false,
          created_at: created,
          updated_at: created,
          history: [{
            at: created,
            type: 'create',
            added: services,
            removed: [],
            details: buildProfileDetails(position, department),
            by: state.actor,
          }],
        });
        if (row) {
          state.rows.unshift(row);
        }
      }
      const ok = await persistOrRollback(before, buildAuditEvent(existing ? 'edit' : 'create', existing || { user, services }, {
        details: buildProfileDetails(position, department),
      }));
      if (!ok) return;
      if (existing) {
        await sendAccessesNotification('edit', existing);
      } else {
        const createdRow = state.rows.find((r) => r.user.toLowerCase() === user.toLowerCase());
        if (createdRow) await sendAccessesNotification('create', createdRow);
      }
      form.reset();
      closeModal('#accesses-create-modal');
      clearAlert();
      renderCards();
    });
  }

  function bindEditForm() {
    const form = document.getElementById('accesses-edit-form');
    if (!form) return;
    form.addEventListener('submit', async (e) => {
      e.preventDefault();
      const before = cloneRows();
      const id = String(document.getElementById('accesses-edit-id')?.value || '');
      const row = getRowById(id);
      if (!row) return;
      const nextUser = cleanText(document.getElementById('accesses-edit-user')?.value);
      const nextServices = selectedServices('accesses-edit-services');
      const position = cleanText(document.getElementById('accesses-edit-position')?.value);
      const department = cleanText(document.getElementById('accesses-edit-department')?.value);
      if (!nextUser) return showAlert(BerkutI18n.t('accesses.errors.userRequired'));
      if (!nextServices.length) return showAlert(BerkutI18n.t('accesses.errors.servicesRequired'));

      const diff = compareServices(row.services, nextServices);
      const renamed = row.user !== nextUser;
      row.user = nextUser;
      row.services = nextServices;
      row.position = position;
      row.department = department;
      logEvent(row, {
        at: nowISO(),
        type: 'edit',
        added: diff.added,
        removed: diff.removed,
        details: (renamed ? `${BerkutI18n.t('accesses.history.rename')}: ${nextUser}; ` : '') + buildProfileDetails(position, department),
        by: state.actor,
      });
      const ok = await persistOrRollback(before, buildAuditEvent('edit', row, {
        details: (renamed ? `${BerkutI18n.t('accesses.history.rename')}: ${nextUser}; ` : '') + buildProfileDetails(position, department),
      }));
      if (!ok) return;
      await sendAccessesNotification('edit', row);
      closeModal('#accesses-edit-modal');
      clearAlert();
      renderCards();
      if (state.activeViewId === row.id) openViewModal(row.id);
    });
  }

  function bindSupplementForm() {
    const form = document.getElementById('accesses-supplement-form');
    if (!form) return;
    form.addEventListener('submit', async (e) => {
      e.preventDefault();
      const before = cloneRows();
      const id = String(document.getElementById('accesses-supplement-id')?.value || '');
      const row = getRowById(id);
      if (!row) return;
      const selected = selectedServices('accesses-supplement-services');
      if (!selected.length) return showAlert(BerkutI18n.t('accesses.errors.servicesRequired'));
      const merged = dedupeServices([...(row.services || []), ...selected]);
      const diff = compareServices(row.services || [], merged);
      row.services = merged;
      logEvent(row, {
        at: nowISO(),
        type: 'supplement',
        added: diff.added,
        removed: diff.removed,
        details: '',
        by: state.actor,
      });
      const ok = await persistOrRollback(before, buildAuditEvent('supplement', row));
      if (!ok) return;
      await sendAccessesNotification('supplement', row);
      closeModal('#accesses-supplement-modal');
      clearAlert();
      renderCards();
      if (state.activeViewId === row.id) openViewModal(row.id);
    });
  }

  function bindViewActions() {
    const toggleBtn = document.getElementById('accesses-view-toggle-block');
    if (!toggleBtn) return;
    toggleBtn.addEventListener('click', async () => {
      const before = cloneRows();
      const row = getRowById(state.activeViewId);
      if (!row) return;
      row.blocked = !row.blocked;
      logEvent(row, {
        at: nowISO(),
        type: row.blocked ? 'blocked' : 'unblocked',
        added: [],
        removed: [],
        by: state.actor,
      });
      const eventType = row.blocked ? 'blocked' : 'unblocked';
      const ok = await persistOrRollback(before, buildAuditEvent(eventType, row));
      if (!ok) return;
      await sendAccessesNotification(eventType, row);
      renderCards();
      openViewModal(row.id);
    });
  }

  function showAlert(text) {
    const box = document.getElementById('accesses-alert');
    if (!box) return;
    box.textContent = text || '';
    box.hidden = !text;
    if (text && window.AppToast?.show) {
      AppToast.show(text, 'error', 5000, { source: 'accesses-notifications' });
    }
  }

  function clearAlert() {
    showAlert('');
  }

  function statusText(row) {
    return row.blocked ? BerkutI18n.t('accesses.status.blocked') : BerkutI18n.t('accesses.status.active');
  }

  function buildProfileDetails(position, department) {
    const parts = [];
    if (position) parts.push(`${BerkutI18n.t('accesses.form.position')}: ${position}`);
    if (department) parts.push(`${BerkutI18n.t('accesses.form.department')}: ${department}`);
    return parts.join(' | ');
  }

  function dismissalWeekday(dateValue) {
    const iso = normalizeISODate(dateValue);
    if (!iso) return '';
    const dt = new Date(`${iso}T00:00:00`);
    if (Number.isNaN(dt.getTime())) return '';
    const locale = currentLang() === 'en' ? 'en-US' : 'ru-RU';
    return dt.toLocaleDateString(locale, { weekday: 'long' });
  }

  function dismissalDetails(dateValue) {
    const iso = normalizeISODate(dateValue);
    if (!iso) return '';
    const dt = new Date(`${iso}T00:00:00`);
    let dateLabel = iso;
    if (window.AppTime?.formatDate) {
      dateLabel = AppTime.formatDate(dt);
    } else {
      const locale = currentLang() === 'en' ? 'en-US' : 'ru-RU';
      dateLabel = dt.toLocaleDateString(locale);
    }
    const weekday = dismissalWeekday(dateValue);
    if (!weekday) return '';
    return `${BerkutI18n.t('accesses.actions.dismissal')}: ${dateLabel} (${weekday})`;
  }

  function bindDismissalForm() {
    const form = document.getElementById('accesses-dismissal-form');
    const dateInput = document.getElementById('accesses-dismissal-date');
    const weekdayHint = document.getElementById('accesses-dismissal-weekday');
    if (!form || !dateInput) return;
    dateInput.lang = currentLang();
    const refreshHint = () => {
      if (!weekdayHint) return;
      const details = dismissalDetails(dateInput.value);
      weekdayHint.textContent = details || (BerkutI18n.t('common.notSelected') || '-');
    };
    dateInput.addEventListener('change', refreshHint);
    dateInput.addEventListener('input', refreshHint);
    form.addEventListener('submit', async (e) => {
      e.preventDefault();
      const before = cloneRows();
      const rowID = cleanText(document.getElementById('accesses-dismissal-id')?.value);
      const row = getRowById(rowID);
      if (!row) return;
      const isoDate = normalizeISODate(dateInput.value);
      if (!isoDate) {
        showAlert(BerkutI18n.t('accesses.dismissal.invalidDate') || 'Неверная дата увольнения');
        return;
      }
      const details = dismissalDetails(isoDate);
      logEvent(row, {
        at: nowISO(),
        type: 'dismissal',
        added: [],
        removed: [],
        details,
        by: state.actor,
      });
      const ok = await persistOrRollback(before, buildAuditEvent('dismissal', row, { details }));
      if (!ok) return;
      closeModal('#accesses-dismissal-modal');
      clearAlert();
      renderCards();
      if (state.activeViewId === row.id) openViewModal(row.id);
      await sendAccessesNotification('dismissal', row, { dismissalDate: isoDate });
    });
    refreshHint();
  }

  async function sendAccessesNotification(eventType, row, extra = {}) {
    if (!hasApiPost()) return false;
    try {
      const res = await Api.post('/api/notifications/accesses-event', {
        event_type: eventType,
        user: cleanText(row?.user || ''),
        position: cleanText(row?.position || ''),
        department: cleanText(row?.department || ''),
        services: dedupeServices(row?.services || []).map(serviceLabel),
        actor: cleanText(state.actor || ''),
        occurred_at: nowISO(),
        dismissal_date: normalizeISODate(extra.dismissalDate || ''),
      });
      if (res?.status === 'skipped') {
        const reason = String(res?.reason || 'unknown');
        showAlert(BerkutI18n.t(`notifications.accesses.skipped.${reason}`) || BerkutI18n.t('notifications.accesses.sendFailed'));
        return false;
      }
      if (String(eventType) === 'test' && window.AppToast?.show) {
        AppToast.show(t('notifications.accesses.testSent'), 'success', 4000, { source: 'accesses-notifications-test' });
      }
      return true;
    } catch (_) {
      showAlert(BerkutI18n.t('notifications.accesses.sendFailed'));
      return false;
    }
  }

  function handleDismissalAction(rowID) {
    const row = getRowById(rowID);
    if (!row) return;
    const idEl = document.getElementById('accesses-dismissal-id');
    const dateInput = document.getElementById('accesses-dismissal-date');
    const weekdayHint = document.getElementById('accesses-dismissal-weekday');
    if (idEl) idEl.value = row.id;
    if (dateInput) {
      const today = new Date();
      const pad = (n) => String(n).padStart(2, '0');
      dateInput.value = `${today.getFullYear()}-${pad(today.getMonth() + 1)}-${pad(today.getDate())}`;
      dateInput.lang = currentLang();
    }
    if (weekdayHint && dateInput) {
      const details = dismissalDetails(dateInput.value);
      weekdayHint.textContent = details || (BerkutI18n.t('common.notSelected') || '-');
    }
    openModal('#accesses-dismissal-modal');
  }

  async function testAccessNotification(rowID) {
    const row = getRowById(rowID);
    if (!row) return;
    await sendAccessesNotification('test', row);
  }

  function changeSummary(row) {
    const last = Array.isArray(row.history) && row.history.length ? row.history[row.history.length - 1] : null;
    if (!last) return '-';
    const plus = (last.added || []).map(serviceLabel).join(', ');
    const minus = (last.removed || []).map(serviceLabel).join(', ');
    const parts = [];
    if (plus) parts.push(`+ ${plus}`);
    if (minus) parts.push(`- ${minus}`);
    if (last.details) parts.push(last.details);
    if (!parts.length) {
      const typeKey = `accesses.history.type.${last.type}`;
      const txt = BerkutI18n.t(typeKey);
      return txt && txt !== typeKey ? txt : last.type;
    }
    return parts.join(' | ');
  }

  function renderCards() {
    const tbody = document.getElementById('accesses-rows');
    if (!tbody) return;
    tbody.innerHTML = '';
    const visibleRows = state.rows.filter(matchesServiceFilter);
    if (!visibleRows.length) {
      const tr = document.createElement('tr');
      tr.className = 'placeholder';
      const emptyKey = state.serviceFilter ? 'accesses.emptyFiltered' : 'accesses.empty';
      tr.innerHTML = `<td colspan="7">${escapeHtml(BerkutI18n.t(emptyKey))}</td>`;
      tbody.appendChild(tr);
      return;
    }
    visibleRows.forEach((row) => {
      const tr = document.createElement('tr');
      tr.className = `accesses-row ${row.blocked ? 'is-blocked' : ''}`;
      tr.addEventListener('click', (e) => {
        if (e.target.closest('button')) return;
        openViewModal(row.id);
      });

      tr.innerHTML = `
        <td><strong>${escapeHtml(row.user)}</strong></td>
        <td>${escapeHtml(row.position || '-')}</td>
        <td>${escapeHtml(row.department || '-')}</td>
        <td class="accesses-services-count-cell">${escapeHtml(servicesCountLabel(row.services || []))}</td>
        <td><span class="pill ${row.blocked ? 'pill-muted' : ''}">${escapeHtml(statusText(row))}</span></td>
        <td>${escapeHtml(formatDT(row.updated_at))}</td>
        <td class="actions"></td>
      `;

      const actions = tr.querySelector('.actions');
      if (actions) {
        actions.appendChild(actionButton('common.edit', () => openEditModal(row.id)));
        actions.appendChild(actionButton('accesses.actions.supplement', () => openSupplementModal(row.id), 'primary'));
        actions.appendChild(actionButton('accesses.actions.testNotification', () => testAccessNotification(row.id)));
        actions.appendChild(actionButton('accesses.actions.dismissal', () => handleDismissalAction(row.id)));
        actions.appendChild(actionButton('common.delete', () => deleteRow(row.id), 'danger'));
      }
      tbody.appendChild(tr);
    });
  }

  function servicesPills(services) {
    const list = dedupeServices(services);
    if (!list.length) return '-';
    return list.map((code) => {
      const label = serviceLabel(code);
      const idx = Math.abs(hashString(code)) % 8;
      return `<span class="accesses-service-pill tone-${idx}">${escapeHtml(label)}</span>`;
    }).join('');
  }

  function servicesCountLabel(services) {
    const count = dedupeServices(services).length;
    return String(count);
  }

  function matchesServiceFilter(row) {
    if (!state.serviceFilter) return true;
    const q = state.serviceFilter;
    return dedupeServices(row.services || []).some((code) => {
      const label = String(serviceLabel(code) || '').toLowerCase();
      return code.toLowerCase().includes(q) || label.includes(q);
    });
  }

  function hashString(input) {
    let h = 0;
    const s = String(input || '');
    for (let i = 0; i < s.length; i += 1) h = ((h << 5) - h + s.charCodeAt(i)) | 0;
    return h;
  }

  function actionButton(i18nKey, onClick, style = 'ghost') {
    const btn = document.createElement('button');
    btn.type = 'button';
    btn.className = `btn ${style}`;
    btn.textContent = BerkutI18n.t(i18nKey);
    btn.addEventListener('click', (e) => {
      e.stopPropagation();
      onClick();
    });
    return btn;
  }

  function openEditModal(id) {
    const row = getRowById(id);
    if (!row) return;
    const idEl = document.getElementById('accesses-edit-id');
    const userEl = document.getElementById('accesses-edit-user');
    const posEl = document.getElementById('accesses-edit-position');
    const depEl = document.getElementById('accesses-edit-department');
    if (idEl) idEl.value = row.id;
    if (userEl) userEl.value = row.user;
    if (posEl) posEl.value = row.position || '';
    if (depEl) depEl.value = row.department || '';
    renderServicesSelect('accesses-edit-services', 'accesses-edit-services-hint', row.services);
    openModal('#accesses-edit-modal');
  }

  function openSupplementModal(id) {
    const row = getRowById(id);
    if (!row) return;
    const idEl = document.getElementById('accesses-supplement-id');
    const submitBtn = document.getElementById('accesses-supplement-submit');
    const hint = document.getElementById('accesses-supplement-services-hint');
    if (idEl) idEl.value = row.id;
    const available = renderServicesSelect('accesses-supplement-services', 'accesses-supplement-services-hint', [], row.services || []);
    if (submitBtn) submitBtn.disabled = !available;
    if (!available && hint) {
      hint.textContent = BerkutI18n.t('accesses.supplement.noneLeft') || BerkutI18n.t('common.empty') || '-';
    }
    openModal('#accesses-supplement-modal');
  }

  function enhanceMultiSelectLikeDocs(sel) {
    if (!sel) return;
    const parent = sel.parentElement;
    if (parent) {
      const selector = `[data-for="${sel.id}"]`;
      const existingSearch = parent.querySelector(selector);
      if (!existingSearch) {
        const search = document.createElement('input');
        search.type = 'search';
        search.className = 'input accesses-select-search';
        search.dataset.for = sel.id;
        search.placeholder = BerkutI18n.t('accesses.filters.servicePlaceholder') || '';
        search.addEventListener('input', () => {
          const q = String(search.value || '').trim().toLowerCase();
          let visible = 0;
          Array.from(sel.options).forEach((opt) => {
            const label = String(opt.dataset.label || opt.textContent || '').toLowerCase();
            const code = String(opt.value || '').toLowerCase();
            const show = !q || label.includes(q) || code.includes(q);
            opt.hidden = !show;
            if (show) visible += 1;
          });
          sel.size = Math.max(4, Math.min(10, visible || 4));
        });
        parent.insertBefore(search, sel);
      } else {
        existingSearch.value = '';
      }
    }
    Array.from(sel.options).forEach((opt) => { opt.hidden = false; });
    sel.multiple = true;
    sel.setAttribute('multiple', 'multiple');
    if (!sel.size || sel.size < 2) sel.size = 8;
    const refresh = () => {
      Array.from(sel.options).forEach((opt) => {
        const base = opt.dataset.label || opt.textContent;
        opt.dataset.label = base;
        opt.textContent = base;
      });
      sel.dispatchEvent(new Event('selectionrefresh', { bubbles: false }));
    };
    const toggle = (opt) => {
      opt.selected = !opt.selected;
      refresh();
      sel.dispatchEvent(new Event('change', { bubbles: true }));
    };
    if (!sel.dataset.accessesEnhanced) {
      sel.dataset.accessesEnhanced = '1';
      sel.addEventListener('mousedown', (e) => {
        const opt = e.target.closest('option');
        if (!opt) return;
        e.preventDefault();
        toggle(opt);
      });
      sel.addEventListener('change', refresh);
      sel.addEventListener('dblclick', (e) => {
        const opt = e.target.closest('option');
        if (!opt) return;
        toggle(opt);
      });
    }
    refresh();
  }

  function bindTagHintLikeDocs(selectEl, hintEl) {
    if (!selectEl || !hintEl) return;
    const render = () => {
      const options = Array.from(selectEl.selectedOptions || []);
      hintEl.innerHTML = '';
      if (!options.length) {
        hintEl.textContent = BerkutI18n.t('common.notSelected') || BerkutI18n.t('common.empty') || '-';
        return;
      }
      options.forEach((opt) => {
        const tag = document.createElement('span');
        tag.className = 'tag';
        tag.textContent = opt.dataset.label || opt.textContent || opt.value;
        const remove = document.createElement('button');
        remove.type = 'button';
        remove.className = 'tag-remove';
        remove.textContent = 'x';
        remove.addEventListener('click', () => {
          opt.selected = false;
          selectEl.dispatchEvent(new Event('change', { bubbles: true }));
        });
        tag.appendChild(remove);
        hintEl.appendChild(tag);
      });
    };
    if (!selectEl.dataset.accessesHintBound) {
      selectEl.dataset.accessesHintBound = '1';
      selectEl.addEventListener('change', render);
      selectEl.addEventListener('selectionrefresh', render);
    }
    render();
  }

  function openViewModal(id) {
    const row = getRowById(id);
    if (!row) return;
    state.activeViewId = row.id;
    setText('accesses-view-user', row.user);
    setText('accesses-view-position', row.position || '-');
    setText('accesses-view-department', row.department || '-');
    setViewServices(row.services || []);
    setText('accesses-view-updated', formatDT(row.updated_at));
    setText('accesses-view-status', statusText(row));
    setText('accesses-view-created-by', createdBy(row));
    const toggleBtn = document.getElementById('accesses-view-toggle-block');
    if (toggleBtn) {
      toggleBtn.textContent = row.blocked ? BerkutI18n.t('accesses.actions.unblock') : BerkutI18n.t('accesses.actions.block');
      toggleBtn.classList.toggle('danger', !row.blocked);
      toggleBtn.classList.toggle('primary', row.blocked);
    }
    renderHistory(row);
    openModal('#accesses-view-modal');
  }

  function createdBy(row) {
    const history = Array.isArray(row?.history) ? row.history : [];
    const createEvent = history.find(ev => String(ev?.type || '') === 'create') || history[0];
    const by = cleanText(createEvent?.by || '');
    return by || '-';
  }

  function renderHistory(row) {
    const list = document.getElementById('accesses-activity-list');
    if (!list) return;
    list.innerHTML = '';
    const events = (row.history || []).slice().sort((a, b) => String(b.at).localeCompare(String(a.at)));
    if (!events.length) {
      const empty = document.createElement('div');
      empty.className = 'muted';
      empty.textContent = BerkutI18n.t('accesses.history.empty');
      list.appendChild(empty);
      return;
    }
    events.forEach((ev) => {
      const typeKey = `accesses.history.type.${ev.type}`;
      const typeLabel = BerkutI18n.t(typeKey) === typeKey ? ev.type : BerkutI18n.t(typeKey);
      const lines = [];
      if ((ev.added || []).length) lines.push(`+ ${(ev.added || []).map(serviceLabel).join(', ')}`);
      if ((ev.removed || []).length) lines.push(`- ${(ev.removed || []).map(serviceLabel).join(', ')}`);
      if (ev.details) lines.push(ev.details);
      if (ev.by) lines.push(`${BerkutI18n.t('accesses.history.by')}: ${ev.by}`);
      const item = document.createElement('div');
      item.className = 'accesses-activity-item';
      item.innerHTML = `
        <div class="accesses-activity-head">
          <strong>${escapeHtml(typeLabel)}</strong>
          <span class="muted">${escapeHtml(formatDT(ev.at))}</span>
        </div>
        <div class="accesses-activity-body">${escapeHtml(lines.join(' | ') || '-')}</div>
      `;
      list.appendChild(item);
    });
  }

  function deleteRow(id) {
    const row = getRowById(id);
    if (!row) return;
    const doDelete = async () => {
      const before = cloneRows();
      state.rows = state.rows.filter(r => r.id !== id);
      const ok = await persistOrRollback(before, buildAuditEvent('delete', row));
      if (!ok) return;
      renderCards();
      if (state.activeViewId === id) {
        closeModal('#accesses-view-modal');
        state.activeViewId = '';
      }
      await sendAccessesNotification('delete', row);
    };
    if (window.AppConfirm?.ask) {
      window.AppConfirm.ask(BerkutI18n.t('accesses.deleteConfirm'), {
        title: BerkutI18n.t('common.confirm'),
        confirmText: BerkutI18n.t('common.delete'),
        cancelText: BerkutI18n.t('common.cancel'),
        danger: true,
      }).then(ok => { if (ok) doDelete(); });
      return;
    }
    if (window.confirm(BerkutI18n.t('accesses.deleteConfirm'))) doDelete();
  }

  function openModal(selector) {
    const el = document.querySelector(selector);
    if (el) el.hidden = false;
  }

  function closeModal(selector) {
    const el = document.querySelector(selector);
    if (el) el.hidden = true;
  }

  function setText(id, value) {
    const el = document.getElementById(id);
    if (el) el.textContent = value || '-';
  }

  function setViewServices(services) {
    const el = document.getElementById('accesses-view-services');
    if (!el) return;
    const html = servicesPills(services);
    if (html === '-') {
      el.textContent = '-';
      return;
    }
    el.innerHTML = html;
  }

  function escapeHtml(str) {
    return (str || '').toString().replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  }

  return { init };
})();
