(() => {
  const state = IncidentsPage.state;
  const { t, showError, escapeHtml } = IncidentsPage;

  async function loadDashboard() {
    try {
      const res = await Api.get('/api/incidents/dashboard');
      state.dashboard = res || { metrics: { open: 0, in_progress: 0, closed: 0, critical: 0 }, mine: [], attention: [], recent: [] };
      renderHome();
    } catch (err) {
      showError(err, 'incidents.forbidden');
    }
  }

  function renderHome() {
    const container = document.getElementById('incidents-home');
    if (!container) return;
    const metrics = (state.dashboard && state.dashboard.metrics) || { open: 0, in_progress: 0, closed: 0, critical: 0 };
    container.innerHTML = `
      <div class="dashboard-tiles">
        <div class="card dashboard-quick tile-square tile-quick">
          <div class="card-header">
            <h3>${t('incidents.quickActions')}</h3>
          </div>
          <div class="card-body quick-actions">
            <button class="btn primary" id="incident-quick-create">${t('incidents.quick.create')}</button>
            <button class="btn ghost" id="incident-quick-mine">${t('incidents.quick.mine')}</button>
            <button class="btn ghost" id="incident-quick-filters">${t('incidents.quick.filters')}</button>
          </div>
        </div>
        <div class="metrics-grid">
          <button type="button" class="metric-tile tile-square" data-filter-status="open">
            <div class="metric-value" id="incidents-metric-open">${metrics.open || 0}</div>
            <div class="metric-label">${t('incidents.metrics.open')}</div>
          </button>
          <button type="button" class="metric-tile tile-square" data-filter-status="in_progress">
            <div class="metric-value" id="incidents-metric-progress">${metrics.in_progress || 0}</div>
            <div class="metric-label">${t('incidents.metrics.inProgress')}</div>
          </button>
          <button type="button" class="metric-tile tile-square" data-filter-severity="critical">
            <div class="metric-value" id="incidents-metric-critical">${metrics.critical || 0}</div>
            <div class="metric-label">${t('incidents.metrics.critical')}</div>
          </button>
          <button type="button" class="metric-tile tile-square" data-filter-status="closed">
            <div class="metric-value" id="incidents-metric-closed">${metrics.closed || 0}</div>
            <div class="metric-label">${t('incidents.metrics.closed')}</div>
          </button>
        </div>
      </div>
      <div class="card dashboard-section">
        <div class="card-header">
          <h3>${t('incidents.dashboard.mine')}</h3>
        </div>
        <div class="card-body">
          <div class="table-responsive">
            <table class="data-table compact" id="incidents-dashboard-mine">
              <thead>
                <tr>
                  <th>${t('incidents.table.id')}</th>
                  <th>${t('incidents.table.title')}</th>
                  <th>${t('incidents.table.status')}</th>
                  <th>${t('incidents.table.updatedAt')}</th>
                </tr>
              </thead>
              <tbody></tbody>
            </table>
          </div>
        </div>
      </div>
      <div class="card dashboard-section">
        <div class="card-header">
          <h3>${t('incidents.dashboard.attention')}</h3>
        </div>
        <div class="card-body">
          <div class="table-responsive">
            <table class="data-table compact" id="incidents-dashboard-attention">
              <thead>
                <tr>
                  <th>${t('incidents.table.id')}</th>
                  <th>${t('incidents.table.title')}</th>
                  <th>${t('incidents.table.severity')}</th>
                  <th>${t('incidents.table.status')}</th>
                </tr>
              </thead>
              <tbody></tbody>
            </table>
          </div>
        </div>
      </div>
      <div class="card dashboard-section">
        <div class="card-header">
          <h3>${t('incidents.dashboard.recent')}</h3>
        </div>
        <div class="card-body">
          <div class="table-responsive">
            <table class="data-table compact" id="incidents-dashboard-recent">
              <thead>
                <tr>
                  <th>${t('incidents.table.id')}</th>
                  <th>${t('incidents.table.title')}</th>
                  <th>${t('incidents.table.status')}</th>
                  <th>${t('incidents.table.updatedAt')}</th>
                </tr>
              </thead>
              <tbody></tbody>
            </table>
          </div>
        </div>
      </div>`;
    bindDashboardActions();
    renderDashboardTable('incidents-dashboard-mine', state.dashboard.mine || [], 'incidents.dashboard.emptyMine', false, true);
    renderDashboardTable('incidents-dashboard-attention', state.dashboard.attention || [], 'incidents.dashboard.emptyAttention', true, false);
    renderDashboardTable('incidents-dashboard-recent', state.dashboard.recent || [], 'incidents.dashboard.emptyRecent', false, true);
  }

  function renderDashboardTable(tableId, items, emptyKey, includeSeverity, includeUpdated) {
    const tbody = document.querySelector(`#${tableId} tbody`);
    if (!tbody) return;
    tbody.innerHTML = '';
    if (!items.length) {
      const tr = document.createElement('tr');
      const cols = 3 + (includeSeverity ? 1 : 0) + (includeUpdated ? 1 : 0);
      tr.innerHTML = `<td colspan="${cols}">${escapeHtml(t(emptyKey))}</td>`;
      tbody.appendChild(tr);
      return;
    }
    items.forEach(incident => {
      const statusDisplay = IncidentsPage.getIncidentStatusDisplay
        ? IncidentsPage.getIncidentStatusDisplay(incident)
        : { status: incident.status, label: t(`incidents.status.${incident.status}`) };
      const tr = document.createElement('tr');
      tr.className = 'incident-row';
      tr.innerHTML = `
        <td>${escapeHtml(IncidentsPage.incidentLabel(incident))}</td>
        <td>${escapeHtml(incident.title || '')}</td>
        ${includeSeverity ? `<td><span class="badge severity-badge severity-${incident.severity}">${escapeHtml(t(`incidents.severity.${incident.severity}`))}</span></td>` : ''}
        <td><span class="badge status-badge status-${statusDisplay.status}">${escapeHtml(statusDisplay.label)}</span></td>
        ${includeUpdated ? `<td>${escapeHtml(IncidentsPage.formatDate(incident.updated_at))}</td>` : ''}`;
      tr.addEventListener('click', () => IncidentsPage.openIncidentTab(incident.id));
      tbody.appendChild(tr);
    });
  }

  function bindDashboardActions() {
    const createBtn = document.getElementById('incident-quick-create');
    const mineBtn = document.getElementById('incident-quick-mine');
    const filtersBtn = document.getElementById('incident-quick-filters');
    if (createBtn) createBtn.onclick = () => IncidentsPage.openCreateTab();
    if (mineBtn) {
      mineBtn.onclick = () => {
        if (IncidentsPage.openListWithFilters) {
          IncidentsPage.openListWithFilters({ scope: 'mine', status: '', severity: '', period: 'all' }, true);
        } else if (IncidentsPage.switchTab) {
          IncidentsPage.switchTab('list');
        }
      };
    }
    if (filtersBtn) {
      filtersBtn.onclick = () => {
        if (IncidentsPage.openListWithFilters) {
          IncidentsPage.openListWithFilters({}, false);
        } else if (IncidentsPage.switchTab) {
          IncidentsPage.switchTab('list');
        }
      };
    }
    document.querySelectorAll('#incidents-home .metric-tile').forEach(tile => {
      tile.addEventListener('click', () => {
        const status = tile.dataset.filterStatus || '';
        const severity = tile.dataset.filterSeverity || '';
        if (IncidentsPage.openListWithFilters) {
          IncidentsPage.openListWithFilters({
            status,
            severity,
            scope: 'all',
            period: 'all',
          }, true);
        } else if (IncidentsPage.switchTab) {
          IncidentsPage.switchTab('list');
        }
      });
    });
  }

  IncidentsPage.loadDashboard = loadDashboard;
  IncidentsPage.renderHome = renderHome;
  IncidentsPage.renderDashboardTable = renderDashboardTable;
  IncidentsPage.bindDashboardActions = bindDashboardActions;
})();
