(() => {
  const U = (typeof MonitoringMaintenanceUtils !== 'undefined') ? MonitoringMaintenanceUtils : null;
  const els = {};
  const state = { editingId: null, items: [] };

  function bindMaintenance() {
    bindElements();
    const canView = MonitoringPage.hasPermission('monitoring.maintenance.view') || MonitoringPage.hasPermission('monitoring.maintenance.manage');
    if (!canView) {
      const panel = document.getElementById('monitoring-tab-maintenance');
      if (panel) panel.hidden = true;
      return;
    }
    const canManage = MonitoringPage.hasPermission('monitoring.maintenance.manage');
    if (els.newBtn) {
      els.newBtn.disabled = !canManage;
      els.newBtn.classList.toggle('disabled', !canManage);
      els.newBtn.addEventListener('click', () => openModal());
    }
    els.save?.addEventListener('click', submitForm);
    els.strategy?.addEventListener('change', toggleStrategyFields);
    if (els.monitors) {
      els.monitors.multiple = true;
      els.monitors.size = 8;
      els.monitors.addEventListener('mousedown', onMonitorToggle);
      els.monitors.addEventListener('change', renderMonitorHint);
    }
    document.querySelectorAll('[data-close="#maintenance-modal"]').forEach((btn) => {
      btn.addEventListener('click', () => {
        if (els.modal) els.modal.hidden = true;
      });
    });
    U?.buildMonthDayList(els.monthdays);
    populateMonitors([]);
    populateTimezones();
    toggleStrategyFields();
    loadMaintenance();
  }

  function bindElements() {
    els.list = document.getElementById('monitoring-maintenance-list');
    els.alert = document.getElementById('monitoring-maintenance-alert');
    els.newBtn = document.getElementById('monitoring-maintenance-new');
    els.modal = document.getElementById('maintenance-modal');
    els.title = document.getElementById('maintenance-modal-title');
    els.form = document.getElementById('maintenance-form');
    els.save = document.getElementById('maintenance-save');
    els.modalAlert = document.getElementById('maintenance-modal-alert');
    els.name = document.getElementById('maintenance-name');
    els.description = document.getElementById('maintenance-description');
    els.monitors = document.getElementById('maintenance-monitors');
    els.monitorsHint = document.getElementById('maintenance-monitors-hint');
    els.strategy = document.getElementById('maintenance-strategy');
    els.timezone = document.getElementById('maintenance-timezone');
    els.active = document.getElementById('maintenance-active');
    els.startsAt = document.getElementById('maintenance-starts-at');
    els.endsAt = document.getElementById('maintenance-ends-at');
    els.cron = document.getElementById('maintenance-cron');
    els.duration = document.getElementById('maintenance-duration');
    els.intervalDays = document.getElementById('maintenance-interval-days');
    els.intervalStart = document.getElementById('maintenance-window-start');
    els.intervalEnd = document.getElementById('maintenance-window-end');
    els.weekdayStart = document.getElementById('maintenance-weekday-start');
    els.weekdayEnd = document.getElementById('maintenance-weekday-end');
    els.monthdayStart = document.getElementById('maintenance-monthday-start');
    els.monthdayEnd = document.getElementById('maintenance-monthday-end');
    els.weekdays = document.getElementById('maintenance-weekdays');
    els.monthdays = document.getElementById('maintenance-monthdays');
    els.lastDay = document.getElementById('maintenance-last-day');
  }

  async function loadMaintenance() {
    try {
      const res = await Api.get('/api/monitoring/maintenance');
      state.items = Array.isArray(res.items) ? res.items : [];
      renderList();
    } catch (err) {
      showError(MonitoringPage.sanitizeErrorMessage(err.message || err));
    }
  }

  function renderList() {
    if (!els.list) return;
    els.list.innerHTML = '';
    const h = document.createElement('div');
    h.className = 'monitoring-table-row header maintenance-header';
    h.innerHTML = `
      <div>${MonitoringPage.t('monitoring.maintenance.field.name')}</div>
      <div>${MonitoringPage.t('monitoring.maintenance.field.monitors')}</div>
      <div>${MonitoringPage.t('monitoring.maintenance.field.strategy')}</div>
      <div>${MonitoringPage.t('monitoring.maintenance.field.window')}</div>
      <div>${MonitoringPage.t('monitoring.maintenance.field.state')}</div>
      <div></div>`;
    els.list.appendChild(h);
    if (!state.items.length) {
      const empty = document.createElement('div');
      empty.className = 'muted';
      empty.textContent = MonitoringPage.t('monitoring.maintenance.empty');
      els.list.appendChild(empty);
      return;
    }
    state.items.forEach((item) => {
      const row = document.createElement('div');
      row.className = 'monitoring-table-row';
      row.innerHTML = `
        <div><strong>${U?.escapeHtml(item.name || '-') || '-'}</strong></div>
        <div>${U?.escapeHtml(U?.monitorsLabel(item, MonitoringPage.state.monitors)) || '-'}</div>
        <div>${U?.escapeHtml(U?.strategyLabel(item.strategy, MonitoringPage.t)) || '-'}</div>
        <div>${U?.escapeHtml(U?.windowLabel(item, MonitoringPage.formatDate)) || '-'}</div>
        <div>${item.is_active ? MonitoringPage.t('monitoring.maintenance.state.enabled') : MonitoringPage.t('monitoring.maintenance.state.stopped')}</div>
        <div class="row-actions"></div>`;
      const actions = row.querySelector('.row-actions');
      if (actions && MonitoringPage.hasPermission('monitoring.maintenance.manage')) {
        addRowAction(actions, MonitoringPage.t('common.edit'), 'btn ghost', () => openModal(item));
        addRowAction(
          actions,
          item.is_active ? MonitoringPage.t('monitoring.maintenance.stop') : MonitoringPage.t('monitoring.maintenance.resume'),
          'btn ghost',
          () => toggleItemState(item),
        );
        addRowAction(actions, MonitoringPage.t('common.delete'), 'btn ghost danger', () => deleteItem(item));
      }
      els.list.appendChild(row);
    });
  }

  function addRowAction(root, text, cls, handler, disabled = false) {
    const btn = document.createElement('button');
    btn.className = cls;
    btn.textContent = text;
    btn.disabled = !!disabled;
    btn.addEventListener('click', handler);
    root.appendChild(btn);
  }

  function openModal(item) {
    if (!els.modal) return;
    state.editingId = item?.id || null;
    els.form?.reset();
    MonitoringPage.hideAlert(els.modalAlert);
    populateMonitors(item?.monitor_ids || []);
    populateTimezones(item?.timezone || U?.defaultTimezone());
    if (item) {
      fillForm(item);
      els.title.textContent = MonitoringPage.t('monitoring.maintenance.editTitle');
    } else {
      els.title.textContent = MonitoringPage.t('monitoring.maintenance.createTitle');
      els.strategy.value = 'single';
      els.active.checked = true;
      els.duration.value = 60;
      els.intervalDays.value = 1;
      ['intervalStart', 'weekdayStart', 'monthdayStart'].forEach((key) => { if (els[key]) els[key].value = '02:00'; });
      ['intervalEnd', 'weekdayEnd', 'monthdayEnd'].forEach((key) => { if (els[key]) els[key].value = '03:00'; });
      U?.markDays(els.weekdays, []);
      U?.markDays(els.monthdays, []);
      if (els.lastDay) els.lastDay.checked = false;
    }
    renderMonitorHint();
    toggleStrategyFields();
    els.modal.hidden = false;
  }

  function fillForm(item) {
    const schedule = item.schedule || {};
    els.name.value = item.name || '';
    els.description.value = item.description_md || '';
    els.strategy.value = item.strategy || 'single';
    els.active.checked = !!item.is_active;
    els.cron.value = schedule.cron_expression || '';
    els.duration.value = schedule.duration_min || 60;
    els.intervalDays.value = schedule.interval_days || 1;
    els.intervalStart.value = schedule.window_start || '02:00';
    els.intervalEnd.value = schedule.window_end || '03:00';
    els.weekdayStart.value = schedule.window_start || '02:00';
    els.weekdayEnd.value = schedule.window_end || '03:00';
    els.monthdayStart.value = schedule.window_start || '02:00';
    els.monthdayEnd.value = schedule.window_end || '03:00';
    const rangeStart = schedule.active_from || item.starts_at;
    const rangeEnd = schedule.active_until || item.ends_at;
    els.startsAt.value = U?.toInputValue(rangeStart) || '';
    els.endsAt.value = U?.toInputValue(rangeEnd) || '';
    U?.markDays(els.weekdays, schedule.weekdays || []);
    U?.markDays(els.monthdays, schedule.month_days || []);
    if (els.lastDay) els.lastDay.checked = !!schedule.use_last_day;
  }

  async function submitForm() {
    const payload = buildPayload();
    if (!payload) return;
    MonitoringPage.hideAlert(els.modalAlert);
    try {
      if (state.editingId) await Api.put(`/api/monitoring/maintenance/${state.editingId}`, payload);
      else await Api.post('/api/monitoring/maintenance', payload);
      els.modal.hidden = true;
      state.editingId = null;
      await loadMaintenance();
      MonitoringPage.refreshEventsCenter?.();
      MonitoringPage.refreshSLA?.();
    } catch (err) {
      MonitoringPage.showAlert(els.modalAlert, MonitoringPage.sanitizeErrorMessage(err.message || err), false);
    }
  }

  function buildPayload() {
    const strategy = (els.strategy?.value || 'single').toLowerCase();
    const monitorIDs = Array.from(els.monitors?.selectedOptions || []).map((opt) => Number(opt.value)).filter((v) => Number.isInteger(v) && v > 0);
    if (!monitorIDs.length) return modalError('monitoring.maintenance.error.monitorRequired');
    const payload = {
      name: (els.name?.value || '').trim(),
      description_md: (els.description?.value || '').trim(),
      monitor_ids: monitorIDs,
      timezone: els.timezone?.value || U?.defaultTimezone() || 'UTC',
      strategy,
      is_active: !!els.active?.checked,
      schedule: {},
    };
    if (!payload.name) return modalError('monitoring.error.nameRequired');
    const rangeStart = U?.fromInputValue(els.startsAt.value) || '';
    const rangeEnd = U?.fromInputValue(els.endsAt.value) || '';
    if (!rangeStart || !rangeEnd) return modalError('monitoring.error.invalidWindow');
    if (strategy === 'single') {
      payload.starts_at = rangeStart;
      payload.ends_at = rangeEnd;
      return payload;
    }
    payload.schedule.active_from = rangeStart;
    payload.schedule.active_until = rangeEnd;
    if (strategy === 'cron') {
      payload.schedule.cron_expression = (els.cron.value || '').trim();
      payload.schedule.duration_min = parseInt(els.duration.value, 10) || 0;
      if (!payload.schedule.cron_expression || payload.schedule.duration_min <= 0) return modalError('monitoring.maintenance.error.invalidCron');
    }
    if (strategy === 'interval') {
      payload.schedule.interval_days = parseInt(els.intervalDays.value, 10) || 0;
      payload.schedule.window_start = els.intervalStart.value || '';
      payload.schedule.window_end = els.intervalEnd.value || '';
      if (payload.schedule.interval_days <= 0 || !payload.schedule.window_start || !payload.schedule.window_end) return modalError('monitoring.maintenance.error.invalidInterval');
    }
    if (strategy === 'weekday') {
      payload.schedule.weekdays = U?.getCheckedInts(els.weekdays) || [];
      payload.schedule.window_start = els.weekdayStart.value || '';
      payload.schedule.window_end = els.weekdayEnd.value || '';
      if (!payload.schedule.weekdays.length || !payload.schedule.window_start || !payload.schedule.window_end) return modalError('monitoring.maintenance.error.invalidWeekday');
    }
    if (strategy === 'monthday') {
      payload.schedule.month_days = U?.getCheckedInts(els.monthdays) || [];
      payload.schedule.use_last_day = !!els.lastDay?.checked;
      payload.schedule.window_start = els.monthdayStart.value || '';
      payload.schedule.window_end = els.monthdayEnd.value || '';
      if ((!payload.schedule.month_days.length && !payload.schedule.use_last_day) || !payload.schedule.window_start || !payload.schedule.window_end) return modalError('monitoring.maintenance.error.invalidMonthday');
    }
    payload.starts_at = rangeStart;
    payload.ends_at = rangeEnd;
    return payload;
  }

  function modalError(key) {
    MonitoringPage.showAlert(els.modalAlert, MonitoringPage.t(key), false);
    return null;
  }

  async function stopItem(item) {
    if (!item?.id) return;
    try {
      await Api.post(`/api/monitoring/maintenance/${item.id}/stop`, {});
      await loadMaintenance();
      MonitoringPage.refreshEventsCenter?.();
      MonitoringPage.refreshSLA?.();
    } catch (err) { showError(MonitoringPage.sanitizeErrorMessage(err.message || err)); }
  }

  async function resumeItem(item) {
    if (!item?.id) return;
    try {
      await Api.put(`/api/monitoring/maintenance/${item.id}`, { is_active: true });
      await loadMaintenance();
      MonitoringPage.refreshEventsCenter?.();
      MonitoringPage.refreshSLA?.();
    } catch (err) { showError(MonitoringPage.sanitizeErrorMessage(err.message || err)); }
  }

  function toggleItemState(item) {
    if (!item?.id) return;
    if (item.is_active) {
      stopItem(item);
      return;
    }
    resumeItem(item);
  }

  async function deleteItem(item) {
    if (!item?.id || !window.confirm(MonitoringPage.t('monitoring.maintenance.confirmDelete'))) return;
    try {
      await Api.del(`/api/monitoring/maintenance/${item.id}`);
      await loadMaintenance();
      MonitoringPage.refreshEventsCenter?.();
      MonitoringPage.refreshSLA?.();
    } catch (err) { showError(MonitoringPage.sanitizeErrorMessage(err.message || err)); }
  }

  function toggleStrategyFields() {
    const strategy = (els.strategy?.value || 'single').toLowerCase();
    document.querySelectorAll('#maintenance-form .maintenance-strategy-block').forEach((node) => {
      node.hidden = node.dataset.strategy !== strategy;
    });
  }

  function onMonitorToggle(event) {
    const option = event.target;
    if (!(option instanceof HTMLOptionElement)) return;
    event.preventDefault();
    option.selected = !option.selected;
    renderMonitorHint();
  }

  function renderMonitorHint() {
    if (!els.monitorsHint || !els.monitors) return;
    els.monitorsHint.innerHTML = '';
    Array.from(els.monitors.selectedOptions).forEach((opt) => {
      const tag = document.createElement('span');
      tag.className = 'tag';
      tag.textContent = opt.textContent;
      const remove = document.createElement('button');
      remove.type = 'button';
      remove.className = 'tag-remove';
      remove.textContent = 'x';
      remove.addEventListener('click', () => {
        opt.selected = false;
        renderMonitorHint();
      });
      tag.appendChild(remove);
      els.monitorsHint.appendChild(tag);
    });
  }

  function populateMonitors(selectedIds) {
    if (!els.monitors) return;
    const selected = new Set((selectedIds || []).map((v) => Number(v)));
    els.monitors.innerHTML = '';
    (MonitoringPage.state.monitors || []).forEach((mon) => {
      const opt = document.createElement('option');
      opt.value = mon.id;
      opt.textContent = mon.name || `#${mon.id}`;
      opt.selected = selected.has(Number(mon.id));
      els.monitors.appendChild(opt);
    });
  }

  function populateTimezones(current) {
    if (!els.timezone) return;
    const zones = (typeof AppTime !== 'undefined' && AppTime.timeZones) ? AppTime.timeZones() : ['UTC'];
    const selected = current || U?.defaultTimezone() || 'UTC';
    els.timezone.innerHTML = '';
    zones.forEach((zone) => {
      const opt = document.createElement('option');
      opt.value = zone;
      opt.textContent = zone;
      opt.selected = zone === selected;
      els.timezone.appendChild(opt);
    });
    if (!els.timezone.value) els.timezone.value = selected;
  }

  function showError(message) {
    MonitoringPage.showAlert(els.alert, message, false);
  }

  if (typeof MonitoringPage !== 'undefined') {
    MonitoringPage.bindMaintenance = bindMaintenance;
    MonitoringPage.refreshMaintenanceOptions = () => populateMonitors([]);
    MonitoringPage.refreshMaintenanceList = loadMaintenance;
  }
})();
