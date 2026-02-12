(() => {
  const els = {};
  const state = { editingId: null, items: [] };

  function bindMaintenance() {
    els.list = document.getElementById('monitoring-maintenance-list');
    els.alert = document.getElementById('monitoring-maintenance-alert');
    els.newBtn = document.getElementById('monitoring-maintenance-new');
    els.modal = document.getElementById('maintenance-modal');
    els.title = document.getElementById('maintenance-modal-title');
    els.form = document.getElementById('maintenance-form');
    els.save = document.getElementById('maintenance-save');
    els.name = document.getElementById('maintenance-name');
    els.monitor = document.getElementById('maintenance-monitor');
    els.tags = document.getElementById('maintenance-tags');
    els.tagsHint = document.querySelector('[data-tag-hint="maintenance-tags"]');
    els.tagInput = document.getElementById('maintenance-tags-new');
    els.tagAdd = document.getElementById('maintenance-tags-add');
    els.startsAt = document.getElementById('maintenance-starts-at');
    els.endsAt = document.getElementById('maintenance-ends-at');
    els.timezone = document.getElementById('maintenance-timezone');
    els.recurring = document.getElementById('maintenance-recurring');
    els.rrule = document.getElementById('maintenance-rrule');
    els.active = document.getElementById('maintenance-active');
    els.modalAlert = document.getElementById('maintenance-modal-alert');

    const canView = MonitoringPage.hasPermission('monitoring.maintenance.view')
      || MonitoringPage.hasPermission('monitoring.maintenance.manage');
    if (!canView) {
      const card = els.list?.closest('.card');
      if (card) card.hidden = true;
      return;
    }
    if (els.newBtn) {
      els.newBtn.addEventListener('click', () => openModal());
      const canManage = MonitoringPage.hasPermission('monitoring.maintenance.manage');
      els.newBtn.disabled = !canManage;
      els.newBtn.classList.toggle('disabled', !canManage);
    }
    if (els.save) {
      els.save.addEventListener('click', submitForm);
    }
    if (els.tagAdd) {
      els.tagAdd.addEventListener('click', (e) => {
        e.preventDefault();
        addTagOption();
      });
    }
    if (els.recurring) {
      els.recurring.addEventListener('change', () => toggleRecurring());
    }
    [els.startsAt, els.endsAt].forEach(input => {
      if (input) input.inputMode = 'numeric';
    });
    document.querySelectorAll('[data-close="#maintenance-modal"]').forEach(btn => {
      btn.addEventListener('click', () => {
        if (els.modal) els.modal.hidden = true;
      });
    });
    populateMonitorOptions();
    populateTagOptions();
    populateTimezoneOptions();
    enhanceMultiSelect(els.tags);
    if (MonitoringPage.bindTagHint) {
      MonitoringPage.bindTagHint(els.tags, els.tagsHint);
    }
    loadMaintenance();
  }

  async function loadMaintenance() {
    if (!MonitoringPage.hasPermission('monitoring.maintenance.view')) return;
    try {
      const res = await Api.get('/api/monitoring/maintenance');
      state.items = res.items || [];
      renderList(state.items);
    } catch (err) {
      console.error('maintenance list', err);
    }
  }

  function renderList(items) {
    if (!els.list) return;
    els.list.innerHTML = '';
    const header = document.createElement('div');
    header.className = 'monitoring-table-row header maintenance-header';
    header.innerHTML = `
      <div>${MonitoringPage.t('monitoring.maintenance.field.name')}</div>
      <div>${MonitoringPage.t('monitoring.maintenance.field.scope')}</div>
      <div>${MonitoringPage.t('monitoring.maintenance.field.window')}</div>
      <div>${MonitoringPage.t('monitoring.maintenance.field.recurring')}</div>
      <div>${MonitoringPage.t('monitoring.maintenance.field.active')}</div>
      <div></div>
    `;
    els.list.appendChild(header);
    if (!items.length) {
      const empty = document.createElement('div');
      empty.className = 'muted';
      empty.textContent = MonitoringPage.t('monitoring.maintenance.empty');
      els.list.appendChild(empty);
      return;
    }
    items.forEach(item => {
      const row = document.createElement('div');
      row.className = 'monitoring-table-row';
      row.innerHTML = `
        <div>${item.name || '-'}</div>
        <div>${scopeLabel(item)}</div>
        <div>${MonitoringPage.formatDate(item.starts_at)} - ${MonitoringPage.formatDate(item.ends_at)}</div>
        <div>${item.is_recurring ? MonitoringPage.t('common.yes') : MonitoringPage.t('common.no')}</div>
        <div>${item.is_active ? MonitoringPage.t('common.enable') : MonitoringPage.t('common.disabled')}</div>
        <div class="row-actions"></div>
      `;
      const actions = row.querySelector('.row-actions');
      if (actions && MonitoringPage.hasPermission('monitoring.maintenance.manage')) {
        const edit = document.createElement('button');
        edit.className = 'btn ghost';
        edit.textContent = MonitoringPage.t('common.edit');
        edit.addEventListener('click', () => openModal(item));
        const del = document.createElement('button');
        del.className = 'btn ghost danger';
        del.textContent = MonitoringPage.t('common.delete');
        del.addEventListener('click', () => deleteItem(item));
        actions.appendChild(edit);
        actions.appendChild(del);
      }
      els.list.appendChild(row);
    });
  }

  function openModal(item) {
    if (!els.modal) return;
    state.editingId = item?.id || null;
    MonitoringPage.hideAlert(els.modalAlert);
    els.form?.reset();
    populateMonitorOptions();
    populateTagOptions(item?.tags || []);
    populateTimezoneOptions();
    if (item) {
      els.title.textContent = MonitoringPage.t('monitoring.maintenance.editTitle');
      els.name.value = item.name || '';
      els.monitor.value = item.monitor_id || '';
      setSelectedOptions(els.tags, item.tags || []);
      els.startsAt.value = toInputValue(item.starts_at);
      els.endsAt.value = toInputValue(item.ends_at);
      els.timezone.value = item.timezone || defaultTimezone();
      els.recurring.checked = !!item.is_recurring;
      els.rrule.value = item.rrule_text || '';
      els.active.checked = !!item.is_active;
    } else {
      els.title.textContent = MonitoringPage.t('monitoring.maintenance.createTitle');
      els.timezone.value = defaultTimezone();
      els.recurring.checked = false;
      els.active.checked = true;
    }
    toggleRecurring();
    els.modal.hidden = false;
  }

  async function submitForm() {
    MonitoringPage.hideAlert(els.modalAlert);
    const payload = buildPayload();
    if (!payload) return;
    try {
      if (state.editingId) {
        await Api.put(`/api/monitoring/maintenance/${state.editingId}`, payload);
      } else {
        await Api.post('/api/monitoring/maintenance', payload);
      }
      els.modal.hidden = true;
      state.editingId = null;
      await loadMaintenance();
      MonitoringPage.refreshEventsCenter?.();
    } catch (err) {
      MonitoringPage.showAlert(els.modalAlert, MonitoringPage.sanitizeErrorMessage(err.message || err), false);
    }
  }

  async function deleteItem(item) {
    if (!item?.id) return;
    const confirmed = window.confirm(MonitoringPage.t('monitoring.maintenance.confirmDelete'));
    if (!confirmed) return;
    try {
      await Api.del(`/api/monitoring/maintenance/${item.id}`);
      await loadMaintenance();
      MonitoringPage.refreshEventsCenter?.();
    } catch (err) {
      MonitoringPage.showAlert(els.alert, MonitoringPage.sanitizeErrorMessage(err.message || err), false);
    }
  }

  function buildPayload() {
    const startsAt = fromInputValue(els.startsAt.value);
    const endsAt = fromInputValue(els.endsAt.value);
    if (!startsAt || !endsAt) {
      MonitoringPage.showAlert(els.modalAlert, MonitoringPage.t('monitoring.error.invalidWindow'), false);
      return null;
    }
    const payload = {
      name: els.name.value.trim(),
      monitor_id: els.monitor.value ? parseInt(els.monitor.value, 10) : null,
      tags: getSelectedOptions(els.tags),
      starts_at: startsAt,
      ends_at: endsAt,
      timezone: els.timezone.value.trim() || defaultTimezone(),
      is_recurring: !!els.recurring.checked,
      rrule_text: els.rrule.value.trim(),
      is_active: !!els.active.checked,
    };
    if (!payload.name) {
      MonitoringPage.showAlert(els.modalAlert, MonitoringPage.t('monitoring.error.nameRequired'), false);
      return null;
    }
    if (payload.is_recurring && !payload.rrule_text) {
      MonitoringPage.showAlert(els.modalAlert, MonitoringPage.t('monitoring.error.invalidRRule'), false);
      return null;
    }
    return payload;
  }

  function toggleRecurring() {
    if (!els.rrule) return;
    els.rrule.disabled = !els.recurring.checked;
  }

  function scopeLabel(item) {
    if (item.monitor_id) {
      const mon = (MonitoringPage.state.monitors || []).find(m => m.id === item.monitor_id);
      return mon?.name || `#${item.monitor_id}`;
    }
    if (item.tags && item.tags.length) {
      return `${MonitoringPage.t('monitoring.maintenance.tagsScope')}: ${item.tags.join(', ')}`;
    }
    return MonitoringPage.t('monitoring.maintenance.allMonitors');
  }

  function populateMonitorOptions() {
    if (!els.monitor) return;
    els.monitor.innerHTML = '';
    const all = document.createElement('option');
    all.value = '';
    all.textContent = MonitoringPage.t('monitoring.maintenance.allMonitors');
    els.monitor.appendChild(all);
    (MonitoringPage.state.monitors || []).forEach(mon => {
      const opt = document.createElement('option');
      opt.value = mon.id;
      opt.textContent = mon.name || `#${mon.id}`;
      els.monitor.appendChild(opt);
    });
  }

  function populateTagOptions(selected = []) {
    if (!els.tags) return;
    const existing = new Set(selected || []);
    if (typeof TagDirectory !== 'undefined' && TagDirectory.all) {
      TagDirectory.all().forEach(tag => existing.add(tag.code || tag));
    }
    els.tags.innerHTML = '';
    Array.from(existing).sort().forEach(tag => {
      const opt = document.createElement('option');
      opt.value = tag;
      opt.textContent = (typeof TagDirectory !== 'undefined' && TagDirectory.label)
        ? (TagDirectory.label(tag) || tag)
        : tag;
      opt.dataset.label = opt.textContent;
      els.tags.appendChild(opt);
    });
    if (MonitoringPage.bindTagHint) {
      MonitoringPage.bindTagHint(els.tags, els.tagsHint);
    }
  }

  function defaultTimezone() {
    if (typeof AppTime !== 'undefined' && AppTime.getTimeZone) {
      return AppTime.getTimeZone();
    }
    return 'Europe/Moscow';
  }

  function populateTimezoneOptions() {
    if (!els.timezone) return;
    const zones = (typeof AppTime !== 'undefined' && AppTime.timeZones)
      ? AppTime.timeZones()
      : ['Europe/Moscow', 'UTC'];
    const current = els.timezone.value || defaultTimezone();
    els.timezone.innerHTML = '';
    zones.forEach(zone => {
      const opt = document.createElement('option');
      opt.value = zone;
      opt.textContent = zone;
      opt.selected = zone === current;
      els.timezone.appendChild(opt);
    });
    if (!els.timezone.value) {
      els.timezone.value = current;
    }
  }

  function addTagOption() {
    const val = (els.tagInput.value || '').trim();
    if (!val) return;
    const opt = document.createElement('option');
    opt.value = val;
    opt.textContent = val;
    opt.selected = true;
    els.tags.appendChild(opt);
    els.tagInput.value = '';
  }

  function enhanceMultiSelect(select) {
    if (!select) return;
    select.multiple = true;
    if (!select.size || select.size < 4) select.size = 4;
  }

  function getSelectedOptions(select) {
    if (!select) return [];
    return Array.from(select.selectedOptions).map(o => o.value);
  }

  function setSelectedOptions(select, values) {
    if (!select) return;
    const set = new Set(values || []);
    Array.from(select.options).forEach(opt => {
      opt.selected = set.has(opt.value);
    });
  }

  function toInputValue(value) {
    if (!value) return '';
    if (typeof AppTime !== 'undefined' && AppTime.formatDateTime) {
      return AppTime.formatDateTime(value);
    }
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return '';
    const pad = (num) => `${num}`.padStart(2, '0');
    return `${pad(date.getDate())}.${pad(date.getMonth() + 1)}.${date.getFullYear()} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
  }

  function fromInputValue(value) {
    if (!value) return '';
    if (typeof AppTime !== 'undefined' && AppTime.toISODateTime) {
      return AppTime.toISODateTime(value);
    }
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return '';
    return date.toISOString();
  }

  if (typeof MonitoringPage !== 'undefined') {
    MonitoringPage.bindMaintenance = bindMaintenance;
    MonitoringPage.refreshMaintenanceOptions = () => {
      populateMonitorOptions();
      populateTagOptions();
    };
  }
})();
