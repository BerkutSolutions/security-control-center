(() => {
  const globalObj = typeof window !== 'undefined' ? window : globalThis;
  const DashboardPage = globalObj.DashboardPage || (globalObj.DashboardPage = {});
  const state = DashboardPage.state;
  const { t } = DashboardPage;

  const FRAME_ITEMS = {
    summary: [
      { key: 'docs_on_approval', labelKey: 'dashboard.summary.docsOnApproval', detailKey: 'docs_on_approval' },
      { key: 'tasks_overdue', labelKey: 'dashboard.summary.tasksOverdue', detailKey: 'tasks_overdue' },
      { key: 'incidents_open', labelKey: 'dashboard.summary.incidentsOpen', detailKey: 'incidents_open' },
      { key: 'accounts_blocked', labelKey: 'dashboard.summary.accountsBlocked', detailKey: 'accounts_blocked' }
    ],
    todo: [
      { key: 'approvals_pending', labelKey: 'dashboard.todo.approvals', detailKey: 'approvals_pending' },
      { key: 'tasks_assigned', labelKey: 'dashboard.todo.tasks', detailKey: 'tasks_assigned' },
      { key: 'docs_returned', labelKey: 'dashboard.todo.docsReturned', detailKey: 'docs_returned' },
      { key: 'incidents_assigned', labelKey: 'dashboard.todo.incidentsAssigned', detailKey: 'incidents_assigned' }
    ],
    incidents: [
      { key: 'open', labelKey: 'dashboard.incidents.open', detailKey: 'incidents_open' },
      { key: 'critical', labelKey: 'dashboard.incidents.critical', detailKey: 'incidents_critical' },
      { key: 'new_last_7d', labelKey: 'dashboard.incidents.new7d', detailKey: 'incidents_new' },
      { key: 'closed', labelKey: 'dashboard.incidents.closed', detailKey: 'incidents_closed' }
    ],
    incident_chart: [
      { key: 'status_draft', labelKey: 'incidents.status.draft' },
      { key: 'status_open', labelKey: 'incidents.status.open' },
      { key: 'status_in_progress', labelKey: 'incidents.status.in_progress' },
      { key: 'status_contained', labelKey: 'incidents.status.contained' },
      { key: 'status_resolved', labelKey: 'incidents.status.resolved' },
      { key: 'status_waiting', labelKey: 'incidents.status.waiting' },
      { key: 'status_waiting_info', labelKey: 'incidents.status.waiting_info' },
      { key: 'status_approval', labelKey: 'incidents.status.approval' },
      { key: 'status_closed', labelKey: 'incidents.status.closed' }
    ],
    documents: [
      { key: 'on_approval', labelKey: 'dashboard.documents.onApproval', detailKey: 'docs_on_approval' },
      { key: 'approved_30d', labelKey: 'dashboard.documents.approved30d', detailKey: 'docs_approved_30d' },
      { key: 'returned', labelKey: 'dashboard.documents.returned', detailKey: 'docs_returned' }
    ],
    tasks: [
      { key: 'total', labelKey: 'dashboard.tasks.total', detailKey: 'tasks_total' },
      { key: 'mine', labelKey: 'dashboard.tasks.mine', detailKey: 'tasks_mine' },
      { key: 'overdue', labelKey: 'dashboard.tasks.overdue', detailKey: 'tasks_overdue' },
      { key: 'blocked', labelKey: 'dashboard.tasks.blocked', detailKey: 'tasks_blocked' },
      { key: 'completed_30d', labelKey: 'dashboard.tasks.completed', detailKey: 'tasks_completed_30d' }
    ]
  };
  function frameTitleMap() {
    const map = {};
    (state.frames || []).forEach((f) => {
      if (f && f.id) map[f.id] = f.title;
    });
    return map;
  }
  function renderFrames() {
    if (!state.grid) return;
    state.grid.innerHTML = '';
    if (!state.layout) return;
    const hidden = new Set(state.layout?.hidden || []);
    const frameMap = frameTitleMap();
    let visibleCount = 0;
    const order = state.layout?.order || [];
    order.forEach((id, idx) => {
      if (hidden.has(id)) return;
      const titleKey = frameMap[id] || `dashboard.frame.${id}`;
      const renderer = FRAME_RENDERERS[id];
      const frame = renderer ? renderer(id, titleKey) : renderPlaceholderFrame(id, titleKey);
      if (frame) {
        state.grid.appendChild(frame);
        if (DashboardPage.positionFrame) {
          DashboardPage.positionFrame(frame, id, idx);
        }
        visibleCount++;
      }
    });
    if (DashboardPage.applyBoardHeight) {
      DashboardPage.applyBoardHeight();
    }
    if (state.empty) {
      state.empty.hidden = visibleCount > 0;
    }
  }
  function createFrameShell(id, titleKey) {
    const card = document.createElement('div');
    card.className = 'card dashboard-frame';
    card.dataset.frameId = id;
    card.draggable = !!state.editMode;
    const header = document.createElement('div');
    header.className = 'card-header dashboard-frame-header';
    const title = document.createElement('h3');
    title.textContent = t(titleKey);
    header.appendChild(title);
    const actions = document.createElement('div');
    actions.className = 'frame-actions';
    const settingsBtn = document.createElement('button');
    settingsBtn.className = 'btn ghost icon-btn frame-settings';
    settingsBtn.innerHTML = '&#9881;';
    settingsBtn.dataset.frameId = id;
    settingsBtn.setAttribute('aria-label', t('dashboard.frameSettings'));
    settingsBtn.hidden = !state.editMode;
    settingsBtn.addEventListener('click', () => openFrameSettings(id, titleKey));
    const hideBtn = document.createElement('button');
    hideBtn.className = 'btn ghost icon-btn frame-hide';
    hideBtn.innerHTML = '&#10006;';
    hideBtn.dataset.frameId = id;
    hideBtn.setAttribute('aria-label', t('dashboard.hideFrame'));
    hideBtn.hidden = !state.editMode;
    hideBtn.addEventListener('click', () => DashboardPage.toggleFrameVisibility(id, false));
    actions.appendChild(settingsBtn);
    actions.appendChild(hideBtn);
    header.appendChild(actions);
    const body = document.createElement('div');
    body.className = 'card-body';
    const resizeHandle = document.createElement('div');
    resizeHandle.className = 'frame-resize-handle';
    resizeHandle.dataset.frameId = id;
    card.appendChild(header);
    card.appendChild(body);
    card.appendChild(resizeHandle);
    if (DashboardPage.bindDragResize) {
      DashboardPage.bindDragResize(card, resizeHandle);
    }
    return { card, body };
  }
  function renderPlaceholderFrame(id, titleKey) {
    const shell = createFrameShell(id, titleKey);
    shell.body.innerHTML = `<div class="muted">${t('dashboard.framePlaceholder')}</div>`;
    return shell.card;
  }
  function renderSummaryFrame(id, titleKey) {
    const shell = createFrameShell(id, titleKey);
    shell.card.classList.add('frame-summary');
    const summary = state.data.summary || {};
    const items = FRAME_ITEMS.summary.filter(item => isItemVisible(id, item.key));
    const grid = document.createElement('div');
    grid.className = 'dashboard-mini-metrics column';
    items.forEach((item) => {
      const card = document.createElement('div');
      card.className = 'mini-metric';
      const valueRaw = summary[item.key];
      if (valueRaw === null || typeof valueRaw === 'undefined') {
        card.classList.add('metric-disabled');
      }
      const value = document.createElement('div');
      value.className = 'mini-metric-value';
      value.textContent = valueRaw === null || typeof valueRaw === 'undefined' ? '-' : `${valueRaw}`;
      const label = document.createElement('div');
      label.className = 'mini-metric-label muted';
      label.textContent = t(item.labelKey);
      card.appendChild(value);
      card.appendChild(label);
      if (valueRaw !== null && typeof valueRaw !== 'undefined') {
        card.classList.add('clickable');
        card.addEventListener('click', () => {
          if (state.editMode) return;
          if (DashboardPage.openDetailModal) {
            DashboardPage.openDetailModal(item.detailKey);
          }
        });
      }
      grid.appendChild(card);
    });
    shell.body.appendChild(grid);
    return shell.card;
  }
  function renderTodoFrame(id, titleKey) {
    const shell = createFrameShell(id, titleKey);
    shell.card.classList.add('frame-todo');
    const todo = state.data.todo || {};
    const items = FRAME_ITEMS.todo.filter(item => isItemVisible(id, item.key));
    const list = document.createElement('div');
    list.className = 'dashboard-list';
    items.forEach((item) => {
      const row = document.createElement('div');
      row.className = 'dashboard-list-row';
      const valueRaw = todo[item.key];
      if (valueRaw === null || typeof valueRaw === 'undefined') {
        row.classList.add('row-disabled');
      }
      const label = document.createElement('div');
      label.className = 'dashboard-list-label';
      label.textContent = t(item.labelKey);
      const meta = document.createElement('div');
      meta.className = 'dashboard-list-meta';
      const count = document.createElement('span');
      count.className = 'pill';
      count.textContent = valueRaw === null || typeof valueRaw === 'undefined' ? '-' : `${valueRaw}`;
      meta.appendChild(count);
      if (valueRaw !== null && typeof valueRaw !== 'undefined') {
        row.classList.add('clickable');
        row.addEventListener('click', () => {
          if (state.editMode) return;
          if (DashboardPage.openDetailModal) {
            DashboardPage.openDetailModal(item.detailKey);
          }
        });
      }
      row.appendChild(label);
      row.appendChild(meta);
      list.appendChild(row);
    });
    shell.body.appendChild(list);
    return shell.card;
  }
  function renderIncidentsFrame(id, titleKey) {
    const shell = createFrameShell(id, titleKey);
    shell.card.classList.add('frame-incidents');
    const incidents = state.data.incidents || {};
    const items = FRAME_ITEMS.incidents.filter(item => isItemVisible(id, item.key));
    const grid = document.createElement('div');
    grid.className = 'dashboard-mini-metrics';
    items.forEach((item) => {
      const tile = document.createElement('div');
      tile.className = 'mini-metric';
      const value = document.createElement('div');
      value.className = 'mini-metric-value';
      const raw = incidents[item.key];
      value.textContent = raw === null || typeof raw === 'undefined' ? '-' : `${raw}`;
      const label = document.createElement('div');
      label.className = 'mini-metric-label muted';
      label.textContent = t(item.labelKey);
      tile.appendChild(value);
      tile.appendChild(label);
      if (raw === null || typeof raw === 'undefined') {
        tile.classList.add('metric-disabled');
      } else {
        tile.classList.add('clickable');
        tile.addEventListener('click', () => {
          if (state.editMode) return;
          if (DashboardPage.openDetailModal) {
            DashboardPage.openDetailModal(item.detailKey);
          }
        });
      }
      grid.appendChild(tile);
    });
    shell.body.appendChild(grid);
    return shell.card;
  }
  function renderTasksFrame(id, titleKey) {
    const shell = createFrameShell(id, titleKey);
    shell.card.classList.add('frame-tasks');
    const tasks = state.data.tasks || {};
    const items = FRAME_ITEMS.tasks.filter(item => isItemVisible(id, item.key));
    const grid = document.createElement('div');
    grid.className = 'dashboard-mini-metrics';
    items.forEach((item) => {
      const tile = document.createElement('div');
      tile.className = 'mini-metric';
      const value = document.createElement('div');
      value.className = 'mini-metric-value';
      const raw = tasks[item.key];
      value.textContent = raw === null || typeof raw === 'undefined' ? '-' : `${raw}`;
      const label = document.createElement('div');
      label.className = 'mini-metric-label muted';
      label.textContent = t(item.labelKey);
      tile.appendChild(value);
      tile.appendChild(label);
      if (raw === null || typeof raw === 'undefined') {
        tile.classList.add('metric-disabled');
      } else {
        tile.classList.add('clickable');
        tile.addEventListener('click', () => {
          if (state.editMode) return;
          if (DashboardPage.openDetailModal) {
            DashboardPage.openDetailModal(item.detailKey);
          }
        });
      }
      grid.appendChild(tile);
    });
    shell.body.appendChild(grid);
    return shell.card;
  }
  function renderDocumentsFrame(id, titleKey) {
    const shell = createFrameShell(id, titleKey);
    shell.card.classList.add('frame-documents');
    const docs = state.data.documents || {};
    const items = FRAME_ITEMS.documents.filter(item => isItemVisible(id, item.key));
    const list = document.createElement('div');
    list.className = 'dashboard-mini-metrics';
    items.forEach((item) => {
      const tile = document.createElement('div');
      tile.className = 'mini-metric';
      const value = document.createElement('div');
      value.className = 'mini-metric-value';
      const raw = docs[item.key];
      value.textContent = raw === null || typeof raw === 'undefined' ? '-' : `${raw}`;
      const label = document.createElement('div');
      label.className = 'mini-metric-label muted';
      label.textContent = t(item.labelKey);
      tile.appendChild(value);
      tile.appendChild(label);
      if (raw === null || typeof raw === 'undefined') {
        tile.classList.add('metric-disabled');
      } else {
        tile.classList.add('clickable');
        tile.addEventListener('click', () => {
          if (state.editMode) return;
          if (DashboardPage.openDetailModal) {
            DashboardPage.openDetailModal(item.detailKey);
          }
        });
      }
      list.appendChild(tile);
    });
    shell.body.appendChild(list);
    return shell.card;
  }
  function renderIncidentChartFrame(id, titleKey) {
    const shell = createFrameShell(id, titleKey);
    shell.card.classList.add('frame-incident-chart');
    const incidents = state.data.incidents || {};
    const items = FRAME_ITEMS.incident_chart.filter(item => isItemVisible(id, item.key));
    const counts = items.map(item => ({
      key: item.key,
      label: t(item.labelKey),
      value: toNumber(incidents[item.key])
    }));
    const total = Math.max(1, counts.reduce((acc, item) => acc + item.value, 0));
    const chart = document.createElement('div');
    chart.className = 'dashboard-chart';
    if (!counts.length) {
      chart.innerHTML = `<div class="muted">${t('dashboard.framePlaceholder')}</div>`;
      shell.body.appendChild(chart);
      return shell.card;
    }
    chart.innerHTML = counts.map(item => {
      const pct = Math.round((item.value / total) * 100);
      const muted = item.key === 'status_closed' ? ' muted' : '';
      return `
        <div class="chart-row">
          <div class="chart-label">${item.label}</div>
          <div class="chart-bar${muted}">
            <svg class="chart-svg" viewBox="0 0 100 1" preserveAspectRatio="none" aria-hidden="true">
              <rect class="chart-fill" width="${pct}" height="1"></rect>
            </svg>
          </div>
          <div class="chart-value">${item.value}</div>
        </div>`;
    }).join('');
    shell.body.appendChild(chart);
    return shell.card;
  }
  function renderActivityFrame(id, titleKey) {
    const shell = createFrameShell(id, titleKey);
    shell.card.classList.add('frame-activity');
    const container = document.createElement('div');
    container.className = 'dashboard-activity';
    container.innerHTML = `<div class="muted">${t('dashboard.activity.loading')}</div>`;
    shell.body.appendChild(container);
    loadActivity(id, container);
    return shell.card;
  }
  function toNumber(value) {
    const num = typeof value === 'number' ? value : parseInt(value, 10);
    return Number.isFinite(num) ? num : 0;
  }
  function isItemVisible(frameId, key) {
    const settings = DashboardPage.getFrameSettings ? DashboardPage.getFrameSettings(frameId) : {};
    const items = settings.items || {};
    if (Object.prototype.hasOwnProperty.call(items, key)) {
      return !!items[key];
    }
    return true;
  }
  function openFrameSettings(id, titleKey) {
    if (!state.editMode) return;
    const modal = document.getElementById('dashboard-frame-settings-modal');
    const titleEl = document.getElementById('dashboard-frame-settings-title');
    const body = document.getElementById('dashboard-frame-settings-body');
    if (!modal || !body) return;
    modal.dataset.frameId = id;
    if (titleEl) titleEl.textContent = t(titleKey);
    body.innerHTML = '';
    const hidden = new Set(state.layout.hidden || []);
    const showRow = document.createElement('label');
    showRow.className = 'checkbox-row';
    const showInput = document.createElement('input');
    showInput.type = 'checkbox';
    showInput.checked = !hidden.has(id);
    showInput.dataset.setting = 'visible';
    const showLabel = document.createElement('span');
    showLabel.textContent = t('dashboard.frameVisible');
    showRow.appendChild(showInput);
    showRow.appendChild(showLabel);
    body.appendChild(showRow);
    const frameItems = FRAME_ITEMS[id];
    if (frameItems && frameItems.length) {
      const itemsWrap = document.createElement('div');
      itemsWrap.className = 'dashboard-frame-settings-items';
      frameItems.forEach(item => {
        const row = document.createElement('label');
        row.className = 'checkbox-row';
        const input = document.createElement('input');
        input.type = 'checkbox';
        input.dataset.setting = `item:${item.key}`;
        input.checked = isItemVisible(id, item.key);
        const span = document.createElement('span');
        span.textContent = t(item.labelKey);
        row.appendChild(input);
        row.appendChild(span);
        itemsWrap.appendChild(row);
      });
      body.appendChild(itemsWrap);
    } else {
      const note = document.createElement('div');
      note.className = 'muted';
      note.textContent = t('dashboard.frameNoSettings');
      body.appendChild(note);
    }
    if (id === 'activity') {
      const activityWrap = document.createElement('div');
      activityWrap.className = 'dashboard-frame-settings-items';
      const onlyMineRow = document.createElement('label');
      onlyMineRow.className = 'checkbox-row';
      const onlyMine = document.createElement('input');
      onlyMine.type = 'checkbox';
      onlyMine.dataset.setting = 'activity:mine';
      onlyMine.checked = getActivitySetting(id, 'only_mine', false);
      const onlyMineLabel = document.createElement('span');
      onlyMineLabel.textContent = t('dashboard.activity.onlyMine');
      onlyMineRow.appendChild(onlyMine);
      onlyMineRow.appendChild(onlyMineLabel);
      activityWrap.appendChild(onlyMineRow);
      const limitRow = document.createElement('label');
      limitRow.className = 'checkbox-row';
      const limitLabel = document.createElement('span');
      limitLabel.textContent = t('dashboard.activity.limit');
      const limitInput = document.createElement('input');
      limitInput.type = 'number';
      limitInput.min = '5';
      limitInput.max = '50';
      limitInput.value = `${getActivitySetting(id, 'limit', 10)}`;
      limitInput.dataset.setting = 'activity:limit';
      limitRow.appendChild(limitLabel);
      limitRow.appendChild(limitInput);
      activityWrap.appendChild(limitRow);
      body.appendChild(activityWrap);
    }
    modal.hidden = false;
  }
  function getActivitySetting(frameId, key, fallback) {
    const settings = DashboardPage.getFrameSettings ? DashboardPage.getFrameSettings(frameId) : {};
    const activity = settings.activity || {};
    if (Object.prototype.hasOwnProperty.call(activity, key)) return activity[key];
    return fallback;
  }
  function saveFrameSettings() {
    const modal = document.getElementById('dashboard-frame-settings-modal');
    const body = document.getElementById('dashboard-frame-settings-body');
    if (!modal || !body) return;
    const id = modal.dataset.frameId;
    if (!id) return;
    const visibleInput = body.querySelector('input[data-setting="visible"]');
    if (visibleInput) {
      DashboardPage.toggleFrameVisibility(id, visibleInput.checked);
    }
    const settings = DashboardPage.getFrameSettings ? DashboardPage.getFrameSettings(id) : {};
    const itemInputs = body.querySelectorAll('input[data-setting^="item:"]');
    if (itemInputs.length) {
      const nextItems = { ...(settings.items || {}) };
      itemInputs.forEach(input => {
        const key = input.dataset.setting.replace('item:', '');
        nextItems[key] = input.checked;
      });
      settings.items = nextItems;
    }
    if (id === 'activity') {
      const mineInput = body.querySelector('input[data-setting="activity:mine"]');
      const limitInput = body.querySelector('input[data-setting="activity:limit"]');
      settings.activity = settings.activity || {};
      if (mineInput) settings.activity.only_mine = mineInput.checked;
      if (limitInput) {
        const val = parseInt(limitInput.value, 10);
        settings.activity.limit = Number.isFinite(val) ? Math.min(Math.max(val, 5), 50) : 10;
      }
    }
    if (DashboardPage.setDirty) DashboardPage.setDirty();
    if (DashboardPage.renderFrames) DashboardPage.renderFrames();
    if (DashboardPage.closeModal) DashboardPage.closeModal(modal);
  }
  async function loadActivity(frameId, container) {
    if (!container) return;
    try {
      const res = await Api.get('/api/logs');
      const items = Array.isArray(res.items) ? res.items.slice() : [];
      const settings = DashboardPage.getFrameSettings ? DashboardPage.getFrameSettings(frameId) : {};
      const activity = settings.activity || {};
      const onlyMine = !!activity.only_mine;
      const limit = Number.isFinite(parseInt(activity.limit, 10)) ? parseInt(activity.limit, 10) : 10;
      const username = state.currentUser?.username || '';
      const filtered = items.filter(item => {
        if (!onlyMine) return true;
        return (item.username || '') === username;
      }).slice(0, limit);
      if (!filtered.length) {
        container.innerHTML = `<div class="muted">${t('dashboard.activity.empty')}</div>`;
        return;
      }
      container.innerHTML = '';
      filtered.forEach(item => {
        const row = document.createElement('div');
        row.className = 'dashboard-activity-row';
        const label = translateAction(item.action || '');
        row.innerHTML = `
          <div class="dashboard-activity-title">${escapeHtml(label)}</div>
          <div class="dashboard-activity-meta">${escapeHtml(item.username || "")} - ${formatDate(item.created_at)}</div>`;
        container.appendChild(row);
      });
    } catch (err) {
      container.innerHTML = `<div class="muted">${t('dashboard.activity.error')}</div>`;
    }
  }
  function translateAction(action) {
    if (globalObj.LogsPage && typeof globalObj.LogsPage.prettyAction === "function") {
      return globalObj.LogsPage.prettyAction(action);
    }
    return action;
  }

  function escapeHtml(str) {
    return (str || '').toString().replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  }
  function formatDate(value) {
    if (!value) return '';
    if (typeof AppTime !== 'undefined' && AppTime.formatDateTime) {
      return AppTime.formatDateTime(value);
    }
    const dt = new Date(value);
    if (Number.isNaN(dt.getTime())) return value;
    const pad = (num) => `${num}`.padStart(2, '0');
    return `${pad(dt.getDate())}.${pad(dt.getMonth() + 1)}.${dt.getFullYear()} ${pad(dt.getHours())}:${pad(dt.getMinutes())}`;
  }
  const FRAME_RENDERERS = {
    summary: renderSummaryFrame,
    tasks: renderTasksFrame,
    todo: renderTodoFrame,
    incidents: renderIncidentsFrame,
    documents: renderDocumentsFrame,
    incident_chart: renderIncidentChartFrame,
    activity: renderActivityFrame
  };

  DashboardPage.renderFrames = renderFrames;
  DashboardPage.frameTitleMap = frameTitleMap;
  DashboardPage.openFrameSettings = openFrameSettings;
  DashboardPage.saveFrameSettings = saveFrameSettings;
})();
