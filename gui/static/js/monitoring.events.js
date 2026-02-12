(() => {
  const els = {};

  function bindEventsCenter() {
    if (!MonitoringPage.hasPermission('monitoring.events.view')) {
      const card = document.getElementById('monitor-events-center');
      if (card) card.hidden = true;
      return;
    }
    els.range = document.getElementById('monitor-events-center-range');
    els.type = document.getElementById('monitor-events-center-type');
    els.monitor = document.getElementById('monitor-events-center-monitor');
    els.tags = document.getElementById('monitor-events-center-tags');
    els.tagsHint = document.querySelector('[data-tag-hint="monitor-events-center-tags"]');
    els.list = document.getElementById('monitor-events-center-list');
    els.refresh = document.getElementById('monitor-events-center-refresh');

    [els.range, els.type, els.monitor, els.tags].forEach(el => {
      if (!el) return;
      el.addEventListener('change', () => loadEvents());
    });
    if (els.refresh) {
      els.refresh.addEventListener('click', () => loadEvents());
    }
    populateMonitorOptions();
    populateTagOptions();
    enhanceMultiSelect(els.tags);
    if (MonitoringPage.bindTagHint) {
      MonitoringPage.bindTagHint(els.tags, els.tagsHint);
    }
    loadEvents();
  }

  async function loadEvents() {
    if (!MonitoringPage.hasPermission('monitoring.events.view')) return;
    const params = new URLSearchParams();
    if (els.range?.value) params.set('range', els.range.value);
    if (els.type?.value) params.set('type', els.type.value);
    if (els.monitor?.value) params.set('monitor_id', els.monitor.value);
    const tags = getSelectedOptions(els.tags);
    if (tags.length) params.set('tag', tags.join(','));
    params.set('limit', '50');
    try {
      const res = await Api.get(`/api/monitoring/events?${params.toString()}`);
      renderList(res.items || []);
    } catch (err) {
      console.error('events feed', err);
    }
  }

  function renderList(items) {
    if (!els.list) return;
    els.list.innerHTML = '';
    if (!items.length) {
      const empty = document.createElement('div');
      empty.className = 'muted';
      empty.textContent = MonitoringPage.t('monitoring.noEvents');
      els.list.appendChild(empty);
      return;
    }
    items.forEach(ev => {
      const row = document.createElement('div');
      row.className = `monitoring-event ${statusClass(ev.event_type)}`;
      row.innerHTML = `
        <div>
          <div>${statusLabel(ev.event_type)} - ${ev.monitor_name || `#${ev.monitor_id}`}</div>
          <div class="event-meta">${MonitoringPage.sanitizeErrorMessage(ev.message || '')}</div>
        </div>
        <div class="event-meta">${MonitoringPage.formatDate(ev.ts)}</div>
      `;
      row.addEventListener('click', () => {
        if (MonitoringPage.state.monitors?.length) {
          MonitoringPage.state.selectedId = ev.monitor_id;
          MonitoringPage.loadMonitors?.();
        }
      });
      els.list.appendChild(row);
    });
  }

  function populateMonitorOptions() {
    if (!els.monitor) return;
    const current = els.monitor.value;
    els.monitor.innerHTML = '';
    const all = document.createElement('option');
    all.value = '';
    all.textContent = MonitoringPage.t('common.all');
    els.monitor.appendChild(all);
    (MonitoringPage.state.monitors || []).forEach(mon => {
      const opt = document.createElement('option');
      opt.value = mon.id;
      opt.textContent = mon.name || `#${mon.id}`;
      els.monitor.appendChild(opt);
    });
    if (current) {
      els.monitor.value = current;
    }
  }

  function populateTagOptions() {
    if (!els.tags) return;
    const existing = new Set();
    if (typeof TagDirectory !== 'undefined' && TagDirectory.all) {
      TagDirectory.all().forEach(tag => existing.add(tag.code || tag));
    }
    (MonitoringPage.state.monitors || []).forEach(m => (m.tags || []).forEach(t => existing.add(t)));
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

  function enhanceMultiSelect(select) {
    if (!select) return;
    select.multiple = true;
    if (!select.size || select.size < 4) select.size = 4;
  }

  function getSelectedOptions(select) {
    if (!select) return [];
    return Array.from(select.selectedOptions).map(o => o.value);
  }

  function statusClass(status) {
    const val = (status || '').toLowerCase();
    if (val === 'up') return 'up';
    if (val === 'paused') return 'paused';
    if (val === 'maintenance_start' || val === 'maintenance_end') return 'maintenance';
    return 'down';
  }

  function statusLabel(status) {
    const val = (status || '').toLowerCase();
    if (val === 'maintenance_start') return MonitoringPage.t('monitoring.event.maintenanceStart');
    if (val === 'maintenance_end') return MonitoringPage.t('monitoring.event.maintenanceEnd');
    if (val === 'tls_expiring') return MonitoringPage.t('monitoring.event.tlsExpiring');
    return MonitoringPage.t(`monitoring.status.${val}`);
  }

  if (typeof MonitoringPage !== 'undefined') {
    MonitoringPage.bindEventsCenter = bindEventsCenter;
    MonitoringPage.refreshEventsCenter = loadEvents;
    MonitoringPage.refreshEventsFilters = () => {
      populateMonitorOptions();
      populateTagOptions();
    };
  }
})();
