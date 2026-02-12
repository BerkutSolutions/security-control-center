(() => {
  const els = {};
  const modalState = { editingId: null };

  function bindModal() {
    els.modal = document.getElementById('monitor-modal');
    els.title = document.getElementById('monitor-modal-title');
    els.alert = document.getElementById('monitor-modal-alert');
    els.form = document.getElementById('monitor-form');
    els.save = document.getElementById('monitor-save');
    els.type = document.getElementById('monitor-type');
    els.name = document.getElementById('monitor-name');
    els.url = document.getElementById('monitor-url');
    els.host = document.getElementById('monitor-host');
    els.port = document.getElementById('monitor-port');
    els.interval = document.getElementById('monitor-interval');
    els.timeout = document.getElementById('monitor-timeout');
    els.retries = document.getElementById('monitor-retries');
    els.retryInterval = document.getElementById('monitor-retry-interval');
    els.method = document.getElementById('monitor-method');
    els.allowedStatus = document.getElementById('monitor-allowed-status');
    els.headers = document.getElementById('monitor-headers');
    els.body = document.getElementById('monitor-body');
    els.bodyType = document.getElementById('monitor-body-type');
    els.tags = document.getElementById('monitor-tags');
    els.tagsHint = document.querySelector('[data-tag-hint="monitor-tags"]');
    els.sla = document.getElementById('monitor-sla-target');
    els.autoIncident = document.getElementById('monitor-auto-incident');
    els.incidentSeverity = document.getElementById('monitor-incident-severity');
    els.notifyTLS = document.getElementById('monitor-notify-tls');
    els.ignoreTLS = document.getElementById('monitor-ignore-tls');
    els.notifications = document.getElementById('monitor-notifications-list');
    els.tagInput = document.getElementById('monitor-tags-new');
    els.tagAdd = document.getElementById('monitor-tags-add');
    document.querySelectorAll('[data-close="#monitor-modal"]').forEach(btn => {
      btn.addEventListener('click', () => {
        if (els.modal) els.modal.hidden = true;
      });
    });

    if (els.type) {
      els.type.addEventListener('change', () => toggleTypeFields(els.type.value));
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
    if (els.tags && DocsPage?.enhanceMultiSelects) {
      DocsPage.enhanceMultiSelects([els.tags.id]);
    }
    if (els.tags && MonitoringPage.bindTagHint) {
      MonitoringPage.bindTagHint(els.tags, els.tagsHint);
    }
    if (els.autoIncident) {
      els.autoIncident.addEventListener('change', () => applyIncidentControlState());
    }
    toggleTypeFields('http');
  }

  async function openMonitorModal(monitor) {
    if (!els.modal) return;
    modalState.editingId = monitor?.id || null;
    MonitoringPage.hideAlert(els.alert);
    els.form?.reset();
    fillTagOptions(monitor?.tags || []);
    if (monitor) {
      els.title.textContent = MonitoringPage.t('monitoring.modal.editTitle');
      els.type.value = monitor.type || 'http';
      els.name.value = monitor.name || '';
      els.url.value = monitor.url || '';
      els.host.value = monitor.host || '';
      els.port.value = monitor.port || '';
      els.interval.value = monitor.interval_sec || '';
      els.timeout.value = monitor.timeout_sec || '';
      els.retries.value = monitor.retries || 0;
      els.retryInterval.value = monitor.retry_interval_sec || '';
      els.method.value = monitor.method || 'GET';
      els.allowedStatus.value = (monitor.allowed_status || []).join(', ');
      els.headers.value = JSON.stringify(monitor.headers || {}, null, 2);
      els.body.value = monitor.request_body || '';
      els.bodyType.value = monitor.request_body_type || 'none';
      setSelectedOptions(els.tags, monitor.tags || []);
      if (els.sla) {
        els.sla.value = monitor.sla_target_pct || '';
      }
      if (els.autoIncident) {
        els.autoIncident.checked = !!monitor.auto_incident;
      }
      if (els.notifyTLS) {
        els.notifyTLS.checked = monitor.notify_tls_expiring !== false;
      }
      if (els.ignoreTLS) {
        els.ignoreTLS.checked = !!monitor.ignore_tls_errors;
      }
      if (els.incidentSeverity) {
        els.incidentSeverity.value = monitor.incident_severity || 'low';
      }
    } else {
      els.title.textContent = MonitoringPage.t('monitoring.modal.createTitle');
      const defaults = MonitoringPage.state.settings || {};
      els.type.value = 'http';
      els.method.value = 'GET';
      els.bodyType.value = 'none';
      els.interval.value = defaults.default_interval_sec || 30;
      els.timeout.value = defaults.default_timeout_sec || 20;
      els.retryInterval.value = defaults.default_retry_interval_sec || 30;
      els.retries.value = defaults.default_retries ?? 2;
      els.allowedStatus.value = '200-299';
      if (els.sla) {
        els.sla.value = defaults.default_sla_target_pct || 90;
      }
      if (els.autoIncident) {
        els.autoIncident.checked = false;
      }
      if (els.notifyTLS) {
        els.notifyTLS.checked = true;
      }
      if (els.ignoreTLS) {
        els.ignoreTLS.checked = false;
      }
      if (els.incidentSeverity) {
        els.incidentSeverity.value = 'low';
      }
    }
    await renderNotificationLinks(monitor?.id || null);
    toggleIncidentFields();
    applyIncidentControlState();
    toggleTypeFields(els.type.value);
    els.modal.hidden = false;
  }

  async function submitForm() {
    MonitoringPage.hideAlert(els.alert);
    const payload = buildPayload();
    if (!payload) return;
    try {
      let id = modalState.editingId;
      if (modalState.editingId) {
        await Api.put(`/api/monitoring/monitors/${modalState.editingId}`, payload);
      } else {
        const created = await Api.post('/api/monitoring/monitors', payload);
        id = created?.id || created?.ID || null;
      }
      if (id && MonitoringPage.hasPermission('monitoring.notifications.manage')) {
        await saveMonitorNotifications(id);
      }
      els.modal.hidden = true;
      modalState.editingId = null;
      await MonitoringPage.loadMonitors?.();
    } catch (err) {
      MonitoringPage.showAlert(els.alert, MonitoringPage.sanitizeErrorMessage(err.message || err), false);
    }
  }

  function buildPayload() {
    const type = els.type.value;
    const headers = parseHeaders();
    if (headers === null) return null;
    const payload = {
      type,
      name: els.name.value.trim(),
      interval_sec: parseInt(els.interval.value, 10) || 0,
      timeout_sec: parseInt(els.timeout.value, 10) || 0,
      retries: parseInt(els.retries.value, 10) || 0,
      retry_interval_sec: parseInt(els.retryInterval.value, 10) || 0,
      method: els.method.value,
      allowed_status: splitList(els.allowedStatus.value),
      request_body: els.body.value || '',
      request_body_type: els.bodyType.value,
      headers: headers,
      tags: getSelectedOptions(els.tags),
    };
    if (els.autoIncident && MonitoringPage.hasPermission('monitoring.incidents.link')) {
      payload.auto_incident = !!els.autoIncident.checked;
      payload.incident_severity = els.incidentSeverity?.value || 'low';
    }
    if (els.notifyTLS) {
      payload.notify_tls_expiring = !!els.notifyTLS.checked;
    }
    if (els.ignoreTLS) {
      payload.ignore_tls_errors = !!els.ignoreTLS.checked;
    }
    if (els.sla && els.sla.value !== '') {
      const sla = parseFloat(els.sla.value);
      if (!Number.isNaN(sla) && sla > 0) {
        payload.sla_target_pct = sla;
      }
    }
    if (type === 'http') {
      payload.url = els.url.value.trim();
    } else {
      payload.host = els.host.value.trim();
      payload.port = parseInt(els.port.value, 10) || 0;
    }
    return payload;
  }

  function parseHeaders() {
    const raw = (els.headers.value || '').trim();
    if (!raw) return {};
    try {
      const parsed = JSON.parse(raw);
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
        throw new Error('invalid');
      }
      return parsed;
    } catch (err) {
      MonitoringPage.showAlert(els.alert, MonitoringPage.t('monitoring.error.invalidHeaders'), false);
      return null;
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

  function fillTagOptions(selected) {
    if (!els.tags) return;
    els.tags.innerHTML = '';
    const existing = new Set(selected || []);
    if (typeof TagDirectory !== 'undefined' && TagDirectory.all) {
      TagDirectory.all().forEach(tag => existing.add(tag.code || tag));
    }
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

  function toggleTypeFields(type) {
    const isHTTP = type === 'http';
    document.getElementById('monitor-url-field').hidden = !isHTTP;
    document.getElementById('monitor-method-field').hidden = !isHTTP;
    document.getElementById('monitor-status-field').hidden = !isHTTP;
    document.getElementById('monitor-headers-field').hidden = !isHTTP;
    document.getElementById('monitor-body-field').hidden = !isHTTP;
    document.getElementById('monitor-host-field').hidden = isHTTP;
    document.getElementById('monitor-port-field').hidden = isHTTP;
    if (els.notifyTLS) els.notifyTLS.closest('.form-field').hidden = !isHTTP;
    if (els.ignoreTLS) els.ignoreTLS.closest('.form-field').hidden = !isHTTP;
  }

  function toggleIncidentFields() {
    const canLink = MonitoringPage.hasPermission('monitoring.incidents.link');
    [els.autoIncident, els.incidentSeverity].forEach(el => {
      if (!el) return;
      el.closest('.form-field').hidden = !canLink;
      if (!canLink) el.disabled = true;
    });
  }

  function applyIncidentControlState() {
    if (!els.autoIncident || !els.incidentSeverity) return;
    const enabled = MonitoringPage.hasPermission('monitoring.incidents.link');
    els.incidentSeverity.disabled = !enabled;
  }

  async function renderNotificationLinks(monitorId) {
    if (!els.notifications) return;
    const canManage = MonitoringPage.hasPermission('monitoring.notifications.manage');
    const canView = canManage || MonitoringPage.hasPermission('monitoring.notifications.view');
    if (!canView) {
      els.notifications.closest('.form-field').hidden = true;
      return;
    }
    els.notifications.closest('.form-field').hidden = false;
    const channels = await MonitoringPage.ensureNotificationChannels?.();
    let activeIds = [];
    if (monitorId && canView) {
      try {
        const res = await Api.get(`/api/monitoring/monitors/${monitorId}/notifications`);
        activeIds = (res.items || []).filter(i => i.enabled).map(i => i.notification_channel_id);
      } catch (_) {
        activeIds = [];
      }
    }
    els.notifications.innerHTML = '';
    if (!channels || !channels.length) {
      const empty = document.createElement('div');
      empty.className = 'muted';
      empty.textContent = MonitoringPage.t('monitoring.notifications.empty');
      els.notifications.appendChild(empty);
      return;
    }
    channels.forEach(ch => {
      const row = document.createElement('label');
      row.className = 'tag-option';
      row.innerHTML = `
        <input type="checkbox" value="${ch.id}">
        <span>${escapeHtml(ch.name)} (${escapeHtml(ch.type)})</span>`;
      const input = row.querySelector('input');
      if (input) {
        input.checked = activeIds.includes(ch.id);
        input.disabled = !canManage;
      }
      els.notifications.appendChild(row);
    });
  }

  async function saveMonitorNotifications(monitorId) {
    if (!els.notifications) return;
    const selected = Array.from(els.notifications.querySelectorAll('input[type="checkbox"]:checked')).map(i => parseInt(i.value, 10)).filter(Boolean);
    const items = selected.map(id => ({ notification_channel_id: id, enabled: true }));
    await Api.put(`/api/monitoring/monitors/${monitorId}/notifications`, { items });
  }

  function escapeHtml(str) {
    return (str || '').toString().replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  }

  function splitList(raw) {
    if (!raw) return [];
    return raw.split(',').map(v => v.trim()).filter(Boolean);
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

  if (typeof MonitoringPage !== 'undefined') {
    MonitoringPage.bindModal = bindModal;
    MonitoringPage.openMonitorModal = openMonitorModal;
  }
})();
