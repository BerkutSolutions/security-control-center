(() => {
  const els = {};
  const HOST_TARGET_TYPES = new Set(['tcp', 'ping', 'dns', 'docker', 'steam', 'gamedig', 'mqtt', 'kafka_producer', 'mssql', 'mysql', 'mongodb', 'radius', 'redis', 'tailscale_ping']);

  function bindList() {
    const page = document.getElementById('monitoring-page');
    if (!page) return;
    els.search = document.getElementById('monitor-search');
    els.status = document.getElementById('monitor-filter-status');
    els.active = document.getElementById('monitor-filter-active');
    els.tags = document.getElementById('monitor-filter-tags');
    els.tagsHint = document.querySelector('[data-tag-hint="monitor-filter-tags"]');
    els.list = document.getElementById('monitor-list');
    els.refresh = document.getElementById('monitor-refresh');
    els.newBtn = document.getElementById('monitor-new-btn');

    if (els.search) {
      els.search.addEventListener('input', debounce(() => {
        MonitoringPage.state.filters.q = els.search.value.trim();
        MonitoringPage.loadMonitors?.();
      }, 300));
    }
    [els.status, els.active, els.tags].forEach(el => {
      if (!el) return;
      el.addEventListener('change', () => {
        syncFilters();
        MonitoringPage.loadMonitors?.();
      });
    });
    if (els.refresh) {
      els.refresh.addEventListener('click', () => MonitoringPage.loadMonitors?.());
    }
    if (els.newBtn) {
      els.newBtn.addEventListener('click', () => MonitoringPage.openMonitorModal?.());
      const canManage = MonitoringPage.hasPermission('monitoring.manage');
      els.newBtn.disabled = !canManage;
      els.newBtn.classList.toggle('disabled', !canManage);
    }
    populateTagOptions();
    enhanceMultiSelect(els.tags);
    if (MonitoringPage.bindTagHint) {
      MonitoringPage.bindTagHint(els.tags, els.tagsHint);
    }
  }

  async function loadMonitors() {
    if (!MonitoringPage.hasPermission('monitoring.view')) return;
    const qs = buildQuery();
    try {
      const res = await Api.get(`/api/monitoring/monitors${qs}`);
      const items = res.items || [];
      MonitoringPage.state.monitors = items;
      const hasMonitors = items.length > 0;
      const noMonitorsCard = document.getElementById('monitor-no-monitors');
      const emptyCard = document.getElementById('monitor-empty');
      const detailCard = document.getElementById('monitor-detail');
      const eventsCenterCard = document.getElementById('monitor-events-center');
      if (noMonitorsCard) noMonitorsCard.hidden = hasMonitors;
      if (emptyCard) emptyCard.hidden = !hasMonitors;
      if (detailCard && !hasMonitors) detailCard.hidden = true;
      if (eventsCenterCard) {
        const canEvents = MonitoringPage.hasPermission('monitoring.events.view');
        eventsCenterCard.hidden = !hasMonitors || !canEvents;
      }
      populateTagOptions(items);
      MonitoringPage.refreshEventsFilters?.();
      MonitoringPage.refreshCertsNotifyList?.();
      MonitoringPage.refreshMaintenanceOptions?.();
      renderList(items);
      if (!MonitoringPage.state.selectedId && items.length) {
        MonitoringPage.state.selectedId = items[0].id;
      }
      const selected = MonitoringPage.selectedMonitor();
      if (selected && MonitoringPage.loadDetail) {
        MonitoringPage.loadDetail(selected.id);
      } else if (!items.length) {
        MonitoringPage.clearDetail?.();
      }
    } catch (err) {
      console.error('monitor list', err);
    }
  }

  function buildQuery() {
    const f = MonitoringPage.state.filters;
    const params = new URLSearchParams();
    if (f.q) params.set('q', f.q);
    if (f.status) params.set('status', f.status);
    if (f.active) params.set('active', f.active);
    if (f.tags && f.tags.length) params.set('tag', f.tags.join(','));
    const qs = params.toString();
    return qs ? `?${qs}` : '';
  }

  function syncFilters() {
    MonitoringPage.state.filters.status = els.status?.value || '';
    MonitoringPage.state.filters.active = els.active?.value || '';
    MonitoringPage.state.filters.tags = getSelectedOptions(els.tags);
  }

  function renderList(items) {
    if (!els.list) return;
    els.list.innerHTML = '';
    if (!items.length) {
      const empty = document.createElement('div');
      empty.className = 'muted';
      empty.textContent = MonitoringPage.t('monitoring.emptyList');
      els.list.appendChild(empty);
      return;
    }
    items.forEach(item => {
      const card = document.createElement('div');
      card.className = 'monitor-item';
      if (item.id === MonitoringPage.state.selectedId) {
        card.classList.add('active');
      }
      const dot = document.createElement('span');
      dot.className = `status-dot ${statusClass(item.status)}`;
      const title = document.createElement('div');
      title.className = 'monitor-item-header';
      title.appendChild(dot);
      const name = document.createElement('span');
      name.textContent = item.name || `#${item.id}`;
      title.appendChild(name);
      const meta = document.createElement('div');
      meta.className = 'monitor-item-meta';
      meta.textContent = HOST_TARGET_TYPES.has((item.type || '').toLowerCase())
        ? (item.port ? `${item.host}:${item.port}` : (item.host || '-'))
        : (item.url || item.host || '-');
      if ((item.status || '').toLowerCase() === 'maintenance') {
        const badge = document.createElement('span');
        badge.className = 'status-badge maintenance';
        badge.textContent = MonitoringPage.t('monitoring.status.maintenance');
        meta.appendChild(badge);
      }
      card.appendChild(title);
      card.appendChild(meta);
      card.addEventListener('click', () => selectMonitor(item.id));
      els.list.appendChild(card);
    });
  }

  function selectMonitor(id) {
    MonitoringPage.state.selectedId = id;
    renderList(MonitoringPage.state.monitors);
    if (MonitoringPage.loadDetail) {
      MonitoringPage.loadDetail(id);
    }
  }

  function populateTagOptions(items = []) {
    if (!els.tags) return;
    const existing = new Set();
    if (typeof TagDirectory !== 'undefined' && TagDirectory.all) {
      TagDirectory.all().forEach(tag => existing.add(tag.code || tag));
    }
    items.forEach(m => (m.tags || []).forEach(t => existing.add(t)));
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
    if (val === 'maintenance') return 'maintenance';
    return 'down';
  }

  function debounce(fn, delay) {
    let t;
    return (...args) => {
      clearTimeout(t);
      t = setTimeout(() => fn(...args), delay);
    };
  }

  if (typeof MonitoringPage !== 'undefined') {
    MonitoringPage.bindList = bindList;
    MonitoringPage.loadMonitors = loadMonitors;
  }
})();
