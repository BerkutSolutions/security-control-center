(() => {
  const els = {};

  function bindCerts() {
    const canView = MonitoringPage.hasPermission('monitoring.certs.view')
      || MonitoringPage.hasPermission('monitoring.certs.manage');
    if (!canView) {
      const panel = document.getElementById('monitoring-tab-cert');
      if (panel) panel.hidden = true;
      return;
    }
    els.refresh = document.getElementById('monitor-certs-refresh');
    els.expiring = document.getElementById('monitor-certs-expiring');
    els.status = document.getElementById('monitor-certs-status');
    els.tags = document.getElementById('monitor-certs-tags');
    els.tagsHint = document.querySelector('[data-tag-hint="monitor-certs-tags"]');
    els.list = document.getElementById('monitor-certs-list');
    els.notifyDays = document.getElementById('monitor-certs-notify-days');
    els.notifyAdd = document.getElementById('monitor-certs-notify-add');
    els.notifySave = document.getElementById('monitor-certs-notify-save');
    els.notifyTest = document.getElementById('monitor-certs-notify-test');
    els.notifyList = document.getElementById('monitor-certs-notify-list');
    els.notifyAlert = document.getElementById('monitor-certs-notify-alert');

    if (els.refresh) {
      els.refresh.addEventListener('click', () => loadCerts());
    }
    [els.expiring, els.status, els.tags].forEach(el => {
      if (!el) return;
      el.addEventListener('change', () => loadCerts());
    });
    populateTagOptions();
    enhanceMultiSelect(els.tags);
    if (MonitoringPage.bindTagHint) {
      MonitoringPage.bindTagHint(els.tags, els.tagsHint);
    }
    loadCerts();
    bindNotifySettings();
  }

  async function loadCerts() {
    if (!MonitoringPage.hasPermission('monitoring.certs.view')) return;
    const params = new URLSearchParams();
    if (els.expiring?.value) params.set('expiring_lt', els.expiring.value);
    if (els.status?.value) params.set('status', els.status.value);
    const tags = getSelectedOptions(els.tags);
    if (tags.length) params.set('tag', tags.join(','));
    const qs = params.toString();
    try {
      const res = await Api.get(`/api/monitoring/certs${qs ? `?${qs}` : ''}`);
      renderList(res.items || []);
    } catch (err) {
      console.error('certs list', err);
    }
  }

  function renderList(items) {
    if (!els.list) return;
    els.list.innerHTML = '';
    const header = document.createElement('div');
    header.className = 'monitoring-table-row header certs';
    header.innerHTML = `
      <div>${MonitoringPage.t('monitoring.certs.monitor')}</div>
      <div>${MonitoringPage.t('monitoring.certs.site')}</div>
      <div>${MonitoringPage.t('monitoring.certs.expires')}</div>
      <div>${MonitoringPage.t('monitoring.certs.daysLeft')}</div>
      <div>${MonitoringPage.t('monitoring.certs.issuer')}</div>
      <div>${MonitoringPage.t('monitoring.certs.commonName')}</div>
      <div>${MonitoringPage.t('monitoring.certs.checkedAt')}</div>
      <div>${MonitoringPage.t('monitoring.filter.status')}</div>
    `;
    els.list.appendChild(header);
    if (!items.length) {
      const empty = document.createElement('div');
      empty.className = 'muted';
      empty.textContent = MonitoringPage.t('monitoring.certs.empty');
      els.list.appendChild(empty);
      return;
    }
    items.forEach(item => {
      const row = document.createElement('div');
      row.className = 'monitoring-table-row certs';
      const daysLeft = item.days_left !== null && item.days_left !== undefined
        ? MonitoringPage.formatDaysLeft(item.days_left)
        : '-';
      const statusKey = (item.status || '').toLowerCase();
      const statusLabel = statusKey ? MonitoringPage.t(`monitoring.status.${statusKey}`) : '-';
      row.innerHTML = `
        <div>${item.name || `#${item.monitor_id}`}</div>
        <div>${item.url || '-'}</div>
        <div>${MonitoringPage.formatDate(item.not_after)}</div>
        <div class="${item.expiring_soon ? 'warning' : ''}">${daysLeft}</div>
        <div>${item.issuer || '-'}</div>
        <div>${item.common_name || '-'}</div>
        <div>${MonitoringPage.formatDate(item.checked_at)}</div>
        <div>${statusLabel}</div>
      `;
      els.list.appendChild(row);
    });
  }

  function populateTagOptions() {
    if (!els.tags) return;
    const existing = new Set();
    if (typeof TagDirectory !== 'undefined' && TagDirectory.all) {
      TagDirectory.all().forEach(tag => existing.add(tag.code || tag));
    }
    const selected = new Set(getSelectedOptions(els.tags));
    els.tags.innerHTML = '';
    Array.from(existing).sort().forEach(tag => {
      const opt = document.createElement('option');
      opt.value = tag;
      opt.textContent = (typeof TagDirectory !== 'undefined' && TagDirectory.label)
        ? (TagDirectory.label(tag) || tag)
        : tag;
      opt.selected = selected.has(tag);
      els.tags.appendChild(opt);
    });
    if (MonitoringPage.bindTagHint) {
      MonitoringPage.bindTagHint(els.tags, els.tagsHint);
    }
  }

  function bindNotifySettings() {
    const canManage = MonitoringPage.hasPermission('monitoring.certs.manage');
    hideNotifyAlert();
    if (els.notifyAdd) {
      els.notifyAdd.disabled = !canManage;
      els.notifyAdd.addEventListener('click', () => {
        if (!els.notifyDays) return;
        const val = parseInt(els.notifyDays.value, 10);
        if (!Number.isFinite(val) || val <= 0) return;
        els.notifyDays.value = `${val}`;
      });
    }
    if (els.notifySave) {
      els.notifySave.disabled = !canManage;
      els.notifySave.addEventListener('click', async () => {
        await saveNotifySettings();
      });
    }
    if (els.notifyTest) {
      els.notifyTest.disabled = !canManage;
      els.notifyTest.addEventListener('click', async () => {
        await testNotifySettings();
      });
    }
    if (els.notifyDays) {
      els.notifyDays.disabled = !canManage;
    }
    loadNotifySettings().then(() => renderNotifySettings());
    renderNotifyList();
  }

  async function loadNotifySettings() {
    if (MonitoringPage.state.settings) return;
    if (!MonitoringPage.hasPermission('monitoring.settings.manage')) return;
    try {
      const res = await Api.get('/api/monitoring/settings');
      MonitoringPage.state.settings = res;
    } catch (err) {
      console.error('load monitoring settings', err);
    }
  }

  function renderNotifySettings() {
    if (!els.notifyDays) return;
    const settings = MonitoringPage.state.settings || {};
    const days = settings.tls_expiring_days || 30;
    els.notifyDays.value = `${days}`;
  }

  function renderNotifyList() {
    if (!els.notifyList) return;
    const canManage = MonitoringPage.hasPermission('monitoring.certs.manage');
    const items = (MonitoringPage.state.monitors || [])
      .filter(m => (m.type || '').toLowerCase() === 'http' && (m.url || '').toLowerCase().startsWith('https'));
    els.notifyList.innerHTML = '';
    if (!items.length) {
      const empty = document.createElement('div');
      empty.className = 'muted';
      empty.textContent = MonitoringPage.t('monitoring.certs.notifyEmpty');
      els.notifyList.appendChild(empty);
      return;
    }
    items.forEach(mon => {
      const row = document.createElement('label');
      row.className = 'tag-option';
      row.innerHTML = `
        <input type="checkbox" value="${mon.id}">
        <span>${mon.name || `#${mon.id}`}</span>
      `;
      const input = row.querySelector('input');
      if (input) {
        input.checked = mon.notify_tls_expiring !== false;
        input.dataset.monitorId = `${mon.id}`;
        input.disabled = !canManage;
      }
      els.notifyList.appendChild(row);
    });
  }

  async function saveNotifySettings() {
    if (!MonitoringPage.hasPermission('monitoring.settings.manage')) return;
    const days = parseInt(els.notifyDays?.value, 10) || 0;
    if (days > 0) {
      try {
        const res = await Api.put('/api/monitoring/settings', { tls_expiring_days: days });
        MonitoringPage.state.settings = res;
      } catch (err) {
        console.error('save tls days', err);
      }
    }
    const list = Array.from(els.notifyList?.querySelectorAll('input[type="checkbox"]') || []);
    const updates = list.map(input => ({
      id: parseInt(input.dataset.monitorId, 10),
      notify: input.checked,
    })).filter(item => Number.isFinite(item.id));
    for (const item of updates) {
      const monitor = (MonitoringPage.state.monitors || []).find(m => m.id === item.id);
      if (!monitor) continue;
      const payload = { ...monitor, notify_tls_expiring: item.notify };
      try {
        await Api.put(`/api/monitoring/monitors/${monitor.id}`, payload);
        monitor.notify_tls_expiring = item.notify;
      } catch (err) {
        console.error('save tls notify monitor', err);
      }
    }
  }

  async function testNotifySettings() {
    const list = Array.from(els.notifyList?.querySelectorAll('input[type="checkbox"]:checked') || []);
    const ids = list.map(input => parseInt(input.dataset.monitorId, 10)).filter(Boolean);
    if (!ids.length) {
      showNotifyAlert('monitoring.certs.notifyNoMonitors');
      return;
    }
    try {
      await Api.post('/api/monitoring/certs/test-notification', { monitor_ids: ids });
      showNotifyAlert('monitoring.notifications.testSuccess', true);
    } catch (err) {
      const code = (err?.message || '').trim();
      if (code === 'monitoring.notifications.testFailed') {
        showNotifyAlert('monitoring.notifications.testFailed');
      } else if (code === 'monitoring.notifications.channelRequired') {
        showNotifyAlert('monitoring.notifications.channelRequired');
      } else if (code === 'monitoring.certs.notifyNoMonitors') {
        showNotifyAlert('monitoring.certs.notifyNoMonitors');
      } else {
        showNotifyAlert('common.error');
      }
    }
  }

  function showNotifyAlert(key, success = false) {
    const msg = MonitoringPage.t(key);
    MonitoringPage.showAlert?.(els.notifyAlert, msg || key, success);
  }

  function hideNotifyAlert() {
    MonitoringPage.hideAlert?.(els.notifyAlert);
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

  if (typeof MonitoringPage !== 'undefined') {
    MonitoringPage.bindCerts = bindCerts;
    MonitoringPage.refreshCertsNotifyList = renderNotifyList;
  }
})();
