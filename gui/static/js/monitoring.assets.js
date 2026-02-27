(() => {
  const els = {};
  const state = {
    monitorId: null,
    linked: [],
    options: [],
    optionsLoaded: false,
    selected: new Set(),
    search: '',
    denied: false
  };

  function t(key) {
    return (typeof MonitoringPage !== 'undefined' && MonitoringPage.t) ? (MonitoringPage.t(key) || key) : key;
  }

  function hasPerm(perm) {
    return typeof MonitoringPage !== 'undefined' && MonitoringPage.hasPermission ? MonitoringPage.hasPermission(perm) : true;
  }

  function bindAssets() {
    const page = document.getElementById('monitoring-page');
    if (!page) return;
    els.wrap = document.getElementById('monitor-assets-wrap');
    els.list = document.getElementById('monitor-detail-assets');
    els.empty = document.getElementById('monitor-detail-assets-empty');
    els.edit = document.getElementById('monitor-assets-edit');
    els.modal = document.getElementById('monitor-assets-modal');
    els.alert = document.getElementById('monitor-assets-alert');
    els.search = document.getElementById('monitor-assets-search');
    els.optionsList = document.getElementById('monitor-assets-list');
    els.save = document.getElementById('monitor-assets-save');
    if (els.edit) {
      els.edit.addEventListener('click', () => openModal());
    }
    if (els.search) {
      els.search.addEventListener('input', () => {
        state.search = (els.search.value || '').toLowerCase().trim();
        renderOptions();
      });
    }
    if (els.save) {
      els.save.addEventListener('click', () => save());
    }
  }

  async function loadMonitorAssets(monitorId) {
    state.monitorId = monitorId || null;
    state.denied = false;
    if (!els.wrap || !els.list) return;
    if (!monitorId || !hasPerm('assets.view')) {
      hide();
      return;
    }
    try {
      const res = await Api.get(`/api/monitoring/monitors/${monitorId}/assets`);
      state.linked = res.items || [];
      show();
      renderLinked();
    } catch (err) {
      const msg = (err && err.message ? err.message : '').trim();
      if (msg === 'forbidden' || msg === 'unauthorized') {
        state.denied = true;
        hide();
        return;
      }
      state.linked = [];
      show();
      renderLinked();
    }
  }

  function show() {
    if (els.wrap) els.wrap.hidden = false;
    if (els.edit) {
      const canManage = hasPerm('monitoring.manage') && hasPerm('assets.view');
      els.edit.hidden = !canManage;
      els.edit.disabled = !canManage;
    }
  }

  function hide() {
    if (els.wrap) els.wrap.hidden = true;
  }

  function renderLinked() {
    if (!els.list || !els.empty) return;
    els.list.innerHTML = '';
    const items = Array.isArray(state.linked) ? state.linked : [];
    if (!items.length) {
      els.empty.hidden = false;
      return;
    }
    els.empty.hidden = true;
    items.forEach((a) => {
      const chip = document.createElement('a');
      chip.className = 'monitor-item-tag';
      chip.href = `/assets?asset=${encodeURIComponent(a.id)}`;
      const typeLabel = a.type ? (t(`assets.type.${a.type}`) || a.type) : '';
      chip.textContent = typeLabel ? `${a.name || `#${a.id}`} (${typeLabel})` : (a.name || `#${a.id}`);
      chip.title = chip.textContent;
      els.list.appendChild(chip);
    });
  }

  async function openModal() {
    if (!els.modal || !state.monitorId) return;
    if (!hasPerm('monitoring.manage') || !hasPerm('assets.view')) return;
    if (els.alert) els.alert.hidden = true;
    state.search = '';
    if (els.search) els.search.value = '';
    state.selected = new Set((state.linked || []).map(x => x.id));
    els.modal.hidden = false;
    await ensureOptions();
    renderOptions();
  }

  async function ensureOptions() {
    if (state.optionsLoaded) return;
    try {
      const res = await Api.get('/api/assets/list?limit=200').catch(() => ({ items: [] }));
      state.options = res.items || [];
    } finally {
      state.optionsLoaded = true;
    }
  }

  function renderOptions() {
    if (!els.optionsList) return;
    els.optionsList.innerHTML = '';
    const items = Array.isArray(state.options) ? state.options : [];
    const filtered = state.search
      ? items.filter((a) => optionLabel(a).toLowerCase().includes(state.search))
      : items.slice();
    if (!filtered.length) {
      const empty = document.createElement('div');
      empty.className = 'muted';
      empty.textContent = t('common.empty');
      els.optionsList.appendChild(empty);
      return;
    }
    filtered.forEach((a) => {
      const row = document.createElement('label');
      row.className = 'checkbox compact';
      const input = document.createElement('input');
      input.type = 'checkbox';
      input.checked = state.selected.has(a.id);
      input.addEventListener('change', () => {
        if (input.checked) state.selected.add(a.id);
        else state.selected.delete(a.id);
      });
      const span = document.createElement('span');
      span.textContent = optionLabel(a);
      row.appendChild(input);
      row.appendChild(span);
      els.optionsList.appendChild(row);
    });
  }

  function optionLabel(a) {
    if (!a) return '-';
    const name = a.name || `#${a.id}`;
    const typeLabel = a.type ? (t(`assets.type.${a.type}`) || a.type) : '';
    const suffix = typeLabel ? ` (${typeLabel})` : '';
    return `#${a.id} ${name}${suffix}`.trim();
  }

  async function save() {
    if (!state.monitorId || !hasPerm('monitoring.manage') || !hasPerm('assets.view')) return;
    try {
      const assetIDs = Array.from(state.selected || []).map(x => parseInt(String(x), 10)).filter(x => Number.isFinite(x) && x > 0);
      await Api.put(`/api/monitoring/monitors/${state.monitorId}/assets`, { asset_ids: assetIDs });
      await loadMonitorAssets(state.monitorId);
      if (els.modal) els.modal.hidden = true;
    } catch (err) {
      const msg = (err && err.message ? err.message : '').trim();
      if (els.alert) {
        els.alert.textContent = t(msg || 'common.error');
        els.alert.hidden = false;
      }
    }
  }

  if (typeof MonitoringPage !== 'undefined') {
    MonitoringPage.bindAssets = bindAssets;
    MonitoringPage.loadMonitorAssets = loadMonitorAssets;
  }
})();

