const NotificationsPage = (() => {
  const state = {
    channels: [],
    deliveries: [],
    permissions: [],
    accessesTypes: [],
    accessesChannelId: null,
    modal: { editingId: null, tokenVisible: false, originalToken: '' },
  };
  const els = {};
  const DEFAULT_ACCESSES_TYPES = ['create', 'edit', 'supplement', 'blocked', 'unblocked', 'delete', 'dismissal'];

  function t(key) {
    return (window.BerkutI18n && BerkutI18n.t ? (BerkutI18n.t(key) || key) : key);
  }

  function hasPermission(perm) {
    if (!perm) return true;
    const perms = Array.isArray(state.permissions) ? state.permissions : [];
    if (!perms.length) return true;
    return perms.includes(perm);
  }

  async function init() {
    const root = document.getElementById('notifications-page');
    if (!root) return;
    await loadCurrentUser();
    bindTabs();
    bindPage();
    bindModal();
    await Promise.all([loadChannels(), loadDeliveries(), loadSettings()]);
  }

  async function loadCurrentUser() {
    try {
      const me = await Api.get('/api/auth/me');
      state.permissions = Array.isArray(me?.user?.permissions) ? me.user.permissions : [];
    } catch (_) {
      state.permissions = [];
    }
  }

  function bindTabs() {
    const initial = tabFromPath() || 'notifications-tab-channels';
    switchTab(initial);
    document.querySelectorAll('#notifications-tabs .tab-btn').forEach((btn) => {
      btn.addEventListener('click', () => switchTab(btn.dataset.tab || 'notifications-tab-channels'));
    });
  }

  function switchTab(tabId) {
    const nextTab = normalizeTabId(tabId);
    document.querySelectorAll('#notifications-tabs .tab-btn').forEach((btn) => {
      btn.classList.toggle('active', btn.dataset.tab === nextTab);
    });
    document.querySelectorAll('#notifications-page .tab-panel').forEach((panel) => {
      panel.hidden = panel.dataset.tab !== nextTab;
    });
    updateTabPath(nextTab);
  }

  function normalizeTabId(tabId) {
    if (tabId === 'notifications-tab-settings') return tabId;
    return 'notifications-tab-channels';
  }

  function tabFromPath() {
    const parts = window.location.pathname.split('/').filter(Boolean);
    if (parts[0] !== 'notifications') return null;
    if (parts[1] === 'settings') return 'notifications-tab-settings';
    if (parts[1] === 'channels') return 'notifications-tab-channels';
    return null;
  }

  function updateTabPath(tabId) {
    const suffix = tabId === 'notifications-tab-settings' ? 'settings' : 'channels';
    const next = `/notifications/${suffix}`;
    if (window.location.pathname !== next) {
      window.history.replaceState({}, '', next);
    }
  }

  function bindPage() {
    els.cards = document.getElementById('notifications-channel-cards');
    els.alert = document.getElementById('notifications-channel-alert');
    els.newBtn = document.getElementById('notifications-channel-new');
    els.deliveryList = document.getElementById('notifications-delivery-list');
    els.deliveryRefresh = document.getElementById('notifications-delivery-refresh');
    els.settingsAlert = document.getElementById('notifications-settings-alert');
    els.settingsSave = document.getElementById('notifications-settings-save');
    els.settingsMonitoring = document.getElementById('notifications-settings-monitoring-enabled');
    els.settingsAccesses = document.getElementById('notifications-settings-accesses-enabled');
    els.settingsAccessesTypes = document.getElementById('notifications-settings-accesses-types');
    els.settingsAccessesTypesHint = document.getElementById('notifications-settings-accesses-types-hint');
    els.settingsAccessesChannel = document.getElementById('notifications-settings-accesses-channel');

    if (els.newBtn) {
      els.newBtn.addEventListener('click', () => openModal());
      if (!hasPermission('monitoring.notifications.manage')) els.newBtn.disabled = true;
    }
    if (els.deliveryRefresh) {
      els.deliveryRefresh.addEventListener('click', () => loadDeliveries());
    }
    if (els.settingsSave) {
      els.settingsSave.addEventListener('click', () => saveSettings());
      if (!hasPermission('monitoring.settings.manage') && !hasPermission('accounts.view')) els.settingsSave.disabled = true;
    }
    bindAccessesTypesHint();
  }

  function bindModal() {
    els.modal = document.getElementById('notification-modal');
    els.modalTitle = document.getElementById('notification-modal-title');
    els.modalAlert = document.getElementById('notification-modal-alert');
    els.modalSave = document.getElementById('notification-save');
    els.modalForm = document.getElementById('notification-form');
    els.type = document.getElementById('notification-type');
    els.name = document.getElementById('notification-name');
    els.token = document.getElementById('notification-token');
    els.tokenToggle = document.getElementById('notification-token-toggle');
    els.chatId = document.getElementById('notification-chat-id');
    els.threadId = document.getElementById('notification-thread-id');
    els.template = document.getElementById('notification-template');
    els.quietEnabled = document.getElementById('notification-quiet-enabled');
    els.quietStart = document.getElementById('notification-quiet-start');
    els.quietEnd = document.getElementById('notification-quiet-end');
    els.quietTz = document.getElementById('notification-quiet-tz');
    els.silent = document.getElementById('notification-silent');
    els.protect = document.getElementById('notification-protect');
    els.default = document.getElementById('notification-default');
    els.active = document.getElementById('notification-active');
    els.applyAll = document.getElementById('notification-apply-all');
    els.applyAllRow = document.getElementById('notification-apply-all-row');

    if (els.modalSave) {
      els.modalSave.addEventListener('click', saveChannel);
    }
    if (els.tokenToggle && els.token) {
      els.tokenToggle.addEventListener('click', () => toggleTokenVisibility());
    }
  }

  async function loadChannels() {
    try {
      const res = await Api.get('/api/monitoring/notifications');
      state.channels = Array.isArray(res?.items) ? res.items : [];
      renderChannels();
      renderAccessesChannelSelect();
    } catch (err) {
      showAlert(els.alert, sanitizeError(err), false);
      state.channels = [];
      renderAccessesChannelSelect();
    }
  }

  async function loadDeliveries() {
    if (!els.deliveryList) return;
    try {
      const res = await Api.get('/api/monitoring/notifications/deliveries?limit=100');
      state.deliveries = Array.isArray(res?.items) ? res.items : [];
      renderDeliveries(state.deliveries);
      renderChannels();
    } catch (err) {
      renderDeliveries([]);
      showAlert(els.alert, sanitizeError(err), false);
    }
  }

  async function loadSettings() {
    try {
      const settings = await Api.get('/api/notifications/settings');
      if (els.settingsMonitoring) els.settingsMonitoring.checked = !!settings.monitoring_enabled;
      if (els.settingsAccesses) els.settingsAccesses.checked = !!settings.accesses_enabled;
      state.accessesTypes = normalizeTypes(settings.accesses_types);
      state.accessesChannelId = parseChannelID(settings.accesses_channel_id);
      applyTypesToSelect();
      renderAccessesChannelSelect();
    } catch (_) {
      if (els.settingsMonitoring) els.settingsMonitoring.checked = true;
      if (els.settingsAccesses) els.settingsAccesses.checked = true;
      state.accessesTypes = DEFAULT_ACCESSES_TYPES.slice();
      state.accessesChannelId = null;
      applyTypesToSelect();
      renderAccessesChannelSelect();
    }
  }

  async function saveSettings() {
    if (!hasPermission('monitoring.settings.manage') && !hasPermission('accounts.view')) return;
    hideAlert(els.settingsAlert);
    const payload = {
      monitoring_enabled: !!els.settingsMonitoring?.checked,
      accesses_enabled: !!els.settingsAccesses?.checked,
      accesses_types: selectedAccessesTypes(),
      accesses_channel_id: selectedAccessesChannelId(),
    };
    try {
      await Api.put('/api/notifications/settings', payload);
      state.accessesChannelId = parseChannelID(payload.accesses_channel_id);
      renderAccessesChannelSelect();
      showAlert(els.settingsAlert, t('notifications.settings.saved'), true);
      document.dispatchEvent(new CustomEvent('notifications:settings-changed', { detail: payload }));
    } catch (err) {
      showAlert(els.settingsAlert, sanitizeError(err), false);
    }
  }

  function normalizeTypes(incoming) {
    const allowed = new Set(DEFAULT_ACCESSES_TYPES);
    const list = Array.isArray(incoming) ? incoming : [];
    const out = [];
    list.forEach((raw) => {
      const val = String(raw || '').trim().toLowerCase();
      if (!allowed.has(val) || out.includes(val)) return;
      out.push(val);
    });
    return out.length ? out : DEFAULT_ACCESSES_TYPES.slice();
  }

  function selectedAccessesTypes() {
    const select = els.settingsAccessesTypes;
    if (!select) return DEFAULT_ACCESSES_TYPES.slice();
    return normalizeTypes(Array.from(select.selectedOptions || []).map((opt) => opt.value));
  }

  function applyTypesToSelect() {
    const select = els.settingsAccessesTypes;
    if (!select) return;
    const selected = new Set(state.accessesTypes.length ? state.accessesTypes : DEFAULT_ACCESSES_TYPES);
    Array.from(select.options).forEach((opt) => {
      opt.selected = selected.has(String(opt.value || '').trim().toLowerCase());
    });
    renderAccessesTypesHint();
  }

  function parseChannelID(raw) {
    if (raw === null || raw === undefined || raw === '') return null;
    const parsed = Number(raw);
    if (!Number.isFinite(parsed) || parsed <= 0) return null;
    return Math.trunc(parsed);
  }

  function selectedAccessesChannelId() {
    return parseChannelID(els.settingsAccessesChannel?.value);
  }

  function renderAccessesChannelSelect() {
    const select = els.settingsAccessesChannel;
    if (!select) return;
    const channels = Array.isArray(state.channels) ? state.channels : [];
    const options = ['<option value="">' + escapeHtml(t('common.notSelected') || '-') + '</option>'];
    channels.forEach((ch) => {
      if (!ch || !ch.id) return;
      options.push('<option value="' + escapeHtml(String(ch.id)) + '">' + escapeHtml(ch.name || ('#' + ch.id)) + '</option>');
    });
    if (state.accessesChannelId && !channels.some((ch) => Number(ch?.id) === Number(state.accessesChannelId))) {
      options.push('<option value="' + escapeHtml(String(state.accessesChannelId)) + '">' + escapeHtml('#' + state.accessesChannelId) + '</option>');
    }
    select.innerHTML = options.join('');
    select.value = state.accessesChannelId ? String(state.accessesChannelId) : '';
  }

  function bindAccessesTypesHint() {
    if (!els.settingsAccessesTypes || !els.settingsAccessesTypesHint) return;
    enhanceAccessesTypesSelect(els.settingsAccessesTypes);
    els.settingsAccessesTypes.addEventListener('change', () => renderAccessesTypesHint());
    renderAccessesTypesHint();
  }

  function enhanceAccessesTypesSelect(select) {
    if (!select || select.dataset.enhanced === '1') return;
    select.dataset.enhanced = '1';
    select.multiple = true;
    if (!select.size || select.size < 2) select.size = 6;
    select.addEventListener('mousedown', (e) => {
      const option = e.target.closest('option');
      if (!option) return;
      e.preventDefault();
      option.selected = !option.selected;
      select.dispatchEvent(new Event('change', { bubbles: true }));
    });
  }

  function renderAccessesTypesHint() {
    const hint = els.settingsAccessesTypesHint;
    const select = els.settingsAccessesTypes;
    if (!hint || !select) return;
    const selected = Array.from(select.selectedOptions || []);
    hint.innerHTML = '';
    if (!selected.length) {
      hint.textContent = t('common.notSelected') || '-';
      return;
    }
    selected.forEach((opt) => {
      const tag = document.createElement('span');
      tag.className = 'tag';
      tag.textContent = opt.textContent || opt.value;
      const remove = document.createElement('button');
      remove.type = 'button';
      remove.className = 'tag-remove';
      remove.textContent = 'x';
      remove.addEventListener('click', () => {
        opt.selected = false;
        renderAccessesTypesHint();
      });
      tag.appendChild(remove);
      hint.appendChild(tag);
    });
  }

  function renderChannels() {
    if (!els.cards) return;
    const canManage = hasPermission('monitoring.notifications.manage');
    if (!state.channels.length) {
      els.cards.innerHTML = `<div class="empty-state">${t('monitoring.notifications.empty')}</div>`;
      return;
    }
    const cards = state.channels.map((item) => {
      const status = item.is_active ? t('common.active') : t('common.disabled');
      const lastSent = lastDeliveryText(item.id);
      return `
        <article class="notifications-channel-card" data-id="${item.id}">
          <div class="notifications-channel-head">
            <div>
              <strong>${escapeHtml(item.name || '-')}</strong>
              <div class="muted">${escapeHtml(formatChannelType(item.type || 'telegram'))}</div>
            </div>
            <span class="pill ${item.is_active ? '' : 'pill-muted'}">${escapeHtml(status)}</span>
          </div>
          <div class="notifications-channel-meta">
            <span>${t('monitoring.notifications.chat')}: ${escapeHtml(item.telegram_chat_id || '-')}</span>
            <span>${t('notifications.channels.lastSent')}: ${escapeHtml(lastSent)}</span>
          </div>
          <div class="notifications-channel-actions">
            <button class="btn ghost notify-edit"${canManage ? '' : ' disabled'}>${t('common.edit')}</button>
            <button class="btn ghost notify-test"${canManage ? '' : ' disabled'}>${t('monitoring.notifications.test')}</button>
            <button class="btn ghost danger notify-delete"${canManage ? '' : ' disabled'}>${t('common.delete')}</button>
          </div>
        </article>`;
    }).join('');
    els.cards.innerHTML = cards;
    els.cards.querySelectorAll('.notifications-channel-card').forEach((card) => {
      const id = parseInt(card.dataset.id || '0', 10);
      const item = state.channels.find((row) => row.id === id);
      if (!item) return;
      card.addEventListener('click', (e) => {
        if (e.target.closest('button')) return;
        if (!hasPermission('monitoring.notifications.manage')) return;
        openModal(item);
      });
    });
    els.cards.querySelectorAll('.notify-edit').forEach((btn) => {
      btn.addEventListener('click', (e) => {
        const id = parseInt(e.target.closest('.notifications-channel-card')?.dataset.id || '0', 10);
        const item = state.channels.find((row) => row.id === id);
        if (item) openModal(item);
      });
    });
    els.cards.querySelectorAll('.notify-delete').forEach((btn) => {
      btn.addEventListener('click', async (e) => {
        const id = parseInt(e.target.closest('.notifications-channel-card')?.dataset.id || '0', 10);
        if (!id) return;
        const ok = await (window.AppConfirm?.ask
          ? window.AppConfirm.ask(t('monitoring.notifications.confirmDelete'), {
            title: t('common.confirm'),
            confirmText: t('common.delete'),
            cancelText: t('common.cancel'),
            danger: true,
          })
          : Promise.resolve(window.confirm(t('monitoring.notifications.confirmDelete'))));
        if (!ok) return;
        try {
          await Api.del(`/api/monitoring/notifications/${id}`);
          await loadChannels();
        } catch (err) {
          showAlert(els.alert, sanitizeError(err), false);
        }
      });
    });
    els.cards.querySelectorAll('.notify-test').forEach((btn) => {
      btn.addEventListener('click', async (e) => {
        const id = parseInt(e.target.closest('.notifications-channel-card')?.dataset.id || '0', 10);
        if (!id) return;
        try {
          await Api.post(`/api/monitoring/notifications/${id}/test`);
          showAlert(els.alert, t('monitoring.notifications.testSent'), true);
        } catch (err) {
          showAlert(els.alert, sanitizeError(err), false);
        }
      });
    });
  }

  function renderDeliveries(items) {
    if (!els.deliveryList) return;
    if (!items.length) {
      els.deliveryList.innerHTML = `<div class="empty-state">${t('monitoring.notifications.deliveryEmpty')}</div>`;
      return;
    }
    const rows = items.map((item) => `
      <tr>
        <td>${formatDate(item.created_at)}</td>
        <td>${escapeHtml(formatDeliveryEventType(item.event_type || ''))}</td>
        <td>${escapeHtml(formatDeliveryStatus(item.status || ''))}</td>
        <td>${escapeHtml(item.error || '')}</td>
        <td>${escapeHtml(item.body_preview || '')}</td>
      </tr>
    `).join('');
    els.deliveryList.innerHTML = `
      <table class="data-table compact">
        <thead>
          <tr>
            <th>${t('common.time')}</th>
            <th>${t('monitoring.filter.type')}</th>
            <th>${t('monitoring.notifications.deliveryStatus')}</th>
            <th>${t('common.error')}</th>
            <th>${t('monitoring.notifications.deliveryMessage')}</th>
          </tr>
        </thead>
        <tbody>${rows}</tbody>
      </table>`;
  }

  function openModal(channel) {
    if (!els.modal) return;
    state.modal.editingId = channel?.id || null;
    state.modal.tokenVisible = false;
    state.modal.originalToken = channel?.telegram_bot_token || '';
    hideAlert(els.modalAlert);
    els.modalForm?.reset();
    if (els.applyAllRow) els.applyAllRow.hidden = !!channel;
    if (channel) {
      els.modalTitle.textContent = t('monitoring.notifications.editTitle');
      els.type.value = channel.type || 'telegram';
      els.name.value = channel.name || '';
      els.token.value = channel.telegram_bot_token || '';
      els.chatId.value = channel.telegram_chat_id || '';
      els.threadId.value = channel.telegram_thread_id || '';
      els.template.value = channel.template_text || '';
      els.quietEnabled.checked = !!channel.quiet_hours_enabled;
      els.quietStart.value = channel.quiet_hours_start || '';
      els.quietEnd.value = channel.quiet_hours_end || '';
      els.quietTz.value = channel.quiet_hours_tz || defaultQuietTimezone();
      els.silent.checked = !!channel.silent;
      els.protect.checked = !!channel.protect_content;
      els.default.checked = !!channel.is_default;
      els.active.checked = !!channel.is_active;
    } else {
      els.modalTitle.textContent = t('monitoring.notifications.createTitle');
      els.type.value = 'telegram';
      els.template.value = '{message}';
      els.quietEnabled.checked = false;
      els.quietStart.value = '';
      els.quietEnd.value = '';
      els.quietTz.value = defaultQuietTimezone();
      els.default.checked = false;
      els.active.checked = true;
    }
    if (els.token) els.token.type = 'password';
    els.modal.hidden = false;
  }

  async function saveChannel() {
    if (!hasPermission('monitoring.notifications.manage')) return;
    hideAlert(els.modalAlert);
    const payload = buildPayload();
    if (!payload) return;
    try {
      if (state.modal.editingId) {
        await Api.put(`/api/monitoring/notifications/${state.modal.editingId}`, payload);
      } else {
        await Api.post('/api/monitoring/notifications', payload);
      }
      if (els.modal) els.modal.hidden = true;
      state.modal.editingId = null;
      await loadChannels();
    } catch (err) {
      showAlert(els.modalAlert, sanitizeError(err), false);
    }
  }

  function buildPayload() {
    const name = (els.name?.value || '').trim();
    let token = (els.token?.value || '').trim();
    if (state.modal.editingId && token) {
      if (token.includes('*') || token === (state.modal.originalToken || '').trim()) token = '';
    }
    const chatId = (els.chatId?.value || '').trim();
    if (!name) {
      showAlert(els.modalAlert, t('monitoring.notifications.nameRequired'), false);
      return null;
    }
    if (!state.modal.editingId && (!token || !chatId)) {
      showAlert(els.modalAlert, t('monitoring.notifications.telegramRequired'), false);
      return null;
    }
    return {
      type: 'telegram',
      name,
      telegram_bot_token: token,
      telegram_chat_id: chatId,
      telegram_thread_id: els.threadId?.value ? (parseInt(els.threadId.value, 10) || null) : null,
      template_text: (els.template?.value || '').trim(),
      quiet_hours_enabled: !!els.quietEnabled?.checked,
      quiet_hours_start: (els.quietStart?.value || '').trim(),
      quiet_hours_end: (els.quietEnd?.value || '').trim(),
      quiet_hours_tz: (els.quietTz?.value || '').trim(),
      silent: !!els.silent?.checked,
      protect_content: !!els.protect?.checked,
      is_default: !!els.default?.checked,
      is_active: !!els.active?.checked,
      apply_to_all: !!els.applyAll?.checked,
    };
  }

  async function toggleTokenVisibility() {
    if (!els.token) return;
    const nextVisible = !state.modal.tokenVisible;
    if (nextVisible && state.modal.editingId && (els.token.value || '').includes('*')) {
      try {
        const res = await Api.get(`/api/monitoring/notifications/${state.modal.editingId}/token`);
        const raw = (res?.telegram_bot_token || '').toString();
        els.token.value = raw;
        state.modal.originalToken = raw.trim();
      } catch (err) {
        showAlert(els.modalAlert, sanitizeError(err), false);
        return;
      }
    }
    state.modal.tokenVisible = nextVisible;
    els.token.type = state.modal.tokenVisible ? 'text' : 'password';
    if (els.tokenToggle) els.tokenToggle.classList.toggle('active', state.modal.tokenVisible);
  }

  function lastDeliveryText(channelID) {
    const match = (state.deliveries || []).find((item) => Number(item?.notification_id) === Number(channelID));
    if (!match) return t('common.notSelected') || '-';
    return formatDate(match.created_at);
  }

  function defaultQuietTimezone() {
    if (typeof AppTime !== 'undefined' && AppTime.getTimeZone) return AppTime.getTimeZone() || 'UTC';
    if (typeof Preferences !== 'undefined' && Preferences.load) return Preferences.load().timeZone || 'UTC';
    return 'UTC';
  }

  function showAlert(el, msg, ok) {
    if (!el) return;
    const text = String(msg || '').trim();
    el.textContent = text;
    el.hidden = !text;
    el.classList.toggle('success', !!ok);
  }

  function hideAlert(el) {
    showAlert(el, '', false);
  }

  function sanitizeError(err) {
    const msg = String(err?.message || err || '').trim();
    if (!msg) return t('common.serverError');
    const translated = t(msg);
    return translated === msg ? msg : translated;
  }

  function formatDate(value) {
    if (!value) return '-';
    if (window.AppTime?.formatDateTime) return AppTime.formatDateTime(value);
    return String(value);
  }

  function escapeHtml(str) {
    return (str || '').toString().replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  }

  function translateKey(key, fallback) {
    const translated = t(key);
    if (translated === key) return fallback;
    return translated;
  }

  function formatChannelType(rawType) {
    const type = String(rawType || '').trim().toLowerCase();
    if (!type) return '-';
    return translateKey(`notifications.channels.type.${type}`, rawType);
  }

  function formatDeliveryEventType(rawType) {
    const type = String(rawType || '').trim().toLowerCase();
    if (!type) return '';
    if (type === 'tls_expiring') {
      return translateKey('monitoring.event.tlsExpiring', rawType);
    }
    if (type.startsWith('accesses.')) {
      const eventType = type.slice('accesses.'.length);
      return translateKey(`accesses.history.type.${eventType}`, rawType);
    }
    const monitoringStatus = translateKey(`monitoring.status.${type}`, '');
    if (monitoringStatus) return monitoringStatus;
    return translateKey(`notifications.delivery.event.${type}`, rawType);
  }

  function formatDeliveryStatus(rawStatus) {
    const status = String(rawStatus || '').trim().toLowerCase();
    if (!status) return '';
    return translateKey(`notifications.delivery.status.${status}`, rawStatus);
  }

  return { init };
})();

if (typeof window !== 'undefined') {
  window.NotificationsPage = NotificationsPage;
}
