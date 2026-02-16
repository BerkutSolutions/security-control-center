(() => {
  const globalObj = typeof window !== 'undefined' ? window : globalThis;
  const DashboardPage = globalObj.DashboardPage || (globalObj.DashboardPage = {});
  const { t } = DashboardPage;
  const state = DashboardPage.state || {};

  function openDetailModal(detailKey) {
    const modal = document.getElementById('dashboard-detail-modal');
    const titleEl = document.getElementById('dashboard-detail-title');
    const body = document.getElementById('dashboard-detail-body');
    const closeBtn = document.getElementById('dashboard-detail-close');
    if (!modal || !body || !titleEl) return;
    titleEl.textContent = detailTitle(detailKey);
    body.innerHTML = `<div class="muted">${t('dashboard.detail.loading')}</div>`;
    modal.hidden = false;
    if (closeBtn) {
      closeBtn.onclick = () => DashboardPage.closeModal(modal);
    }
    if (!modal.dataset.bound) {
      modal.dataset.bound = '1';
      modal.addEventListener('click', (e) => {
        if (!e.target.classList.contains('modal-backdrop')) return;
        DashboardPage.closeModal(modal);
      });
    }
    loadDetailItems(detailKey, body);
  }

  function detailTitle(detailKey) {
    switch (detailKey) {
      case 'docs_on_approval':
        return t('dashboard.detail.docsOnApproval');
      case 'docs_returned':
        return t('dashboard.detail.docsReturned');
      case 'docs_approved_30d':
        return t('dashboard.detail.docsApproved');
      case 'approvals_pending':
        return t('dashboard.detail.approvalsPending');
      case 'incidents_open':
        return t('dashboard.detail.incidentsOpen');
      case 'incidents_critical':
        return t('dashboard.detail.incidentsCritical');
      case 'incidents_new':
        return t('dashboard.detail.incidentsNew');
      case 'incidents_closed':
        return t('dashboard.detail.incidentsClosed');
      case 'incidents_assigned':
        return t('dashboard.detail.incidentsAssigned');
      case 'tasks_total':
        return t('dashboard.detail.tasksTotal');
      case 'tasks_mine':
      case 'tasks_assigned':
        return t('dashboard.detail.tasksMine');
      case 'tasks_overdue':
        return t('dashboard.detail.tasksOverdue');
      case 'tasks_blocked':
        return t('dashboard.detail.tasksBlocked');
      case 'tasks_completed_30d':
        return t('dashboard.detail.tasksCompleted');
      case 'accounts_blocked':
        return t('dashboard.detail.accountsBlocked');
      default:
        return t('dashboard.detail.title');
    }
  }

  async function loadDetailItems(detailKey, body) {
    try {
      const items = await getDetailItems(detailKey);
      if (!items.length) {
        body.innerHTML = `<div class="muted">${t('dashboard.detail.empty')}</div>`;
        return;
      }
      body.innerHTML = '';
      const list = document.createElement('div');
      list.className = 'dashboard-detail-list';
      items.forEach(item => {
        const row = document.createElement('button');
        row.type = 'button';
        row.className = 'dashboard-detail-row';
        row.innerHTML = `
          <div class="detail-title">${escapeHtml(item.title || '-')}</div>
          <div class="detail-meta">${escapeHtml(item.meta || '')}</div>`;
        if (item.onClick) {
          row.addEventListener('click', () => item.onClick(item));
        } else {
          row.disabled = true;
        }
        list.appendChild(row);
      });
      body.appendChild(list);
    } catch (err) {
      body.innerHTML = `<div class="muted">${t('dashboard.detail.error')}</div>`;
    }
  }

  async function getDetailItems(detailKey) {
    switch (detailKey) {
      case 'docs_on_approval':
        return listDocs({ status: 'review' });
      case 'docs_returned':
        return listDocs({ status: 'returned', mine: true });
      case 'docs_approved_30d':
        return listDocs({ status: 'approved', days: 30 });
      case 'approvals_pending':
        return listApprovals();
      case 'incidents_open':
        return listIncidents({ statusIn: openStatusList() });
      case 'incidents_critical':
        return listIncidents({ statusIn: openStatusList(), severity: 'critical' });
      case 'incidents_new':
        return listIncidents({ statusIn: openStatusList(), days: 7 });
      case 'incidents_closed':
        return listIncidents({ status: 'closed' });
      case 'incidents_assigned':
        return listIncidents({ assigned: true, statusIn: openStatusList() });
      case 'tasks_total':
        return listTasks({});
      case 'tasks_mine':
      case 'tasks_assigned':
        return listTasks({ mine: true });
      case 'tasks_overdue':
        return listTasks({ overdue: true });
      case 'tasks_blocked':
        return listTasks({ blocked: true });
      case 'tasks_completed_30d':
        return listTasks({ completedDays: 30 });
      default:
        return [];
    }
  }

  function openStatusList() {
    return ['draft', 'open', 'in_progress', 'contained', 'resolved', 'waiting', 'waiting_info', 'approval'];
  }

  async function listDocs(opts = {}) {
    const params = new URLSearchParams();
    if (opts.status) params.set('status', opts.status);
    if (opts.mine) params.set('mine', '1');
    const res = await Api.get(`/api/docs?${params.toString()}`);
    const items = res.items || [];
    const cutoff = opts.days ? Date.now() - opts.days * 86400000 : null;
    return items.filter(doc => {
      if (!cutoff) return true;
      const updated = new Date(doc.updated_at || doc.created_at || Date.now()).getTime();
      return updated >= cutoff;
    }).map(doc => ({
      id: doc.id,
      title: `${doc.reg_number || '#'} ${doc.title || ''}`.trim(),
      meta: t(`docs.status.${doc.status}`),
      onClick: () => {
        openDocument(doc.id);
        closeDetailModal();
      }
    }));
  }

  async function listApprovals() {
    const res = await Api.get('/api/approvals?status=review');
    const approvals = res.items || [];
    if (!approvals.length) return [];
    const docs = await Api.get('/api/docs?status=review');
    const docMap = {};
    (docs.items || []).forEach(doc => {
      docMap[doc.id] = doc;
    });
    return approvals.map(ap => {
      const doc = docMap[ap.doc_id] || {};
      return {
        id: ap.doc_id,
        title: `${doc.reg_number || '#'} ${doc.title || ''}`.trim(),
        meta: t('docs.status.review'),
        onClick: () => {
          openDocument(ap.doc_id);
          closeDetailModal();
        }
      };
    });
  }

  async function listIncidents(opts = {}) {
    const params = new URLSearchParams();
    if (opts.status) params.set('status', opts.status);
    if (opts.severity) params.set('severity', opts.severity);
    if (opts.assigned) params.set('assigned_to_me', '1');
    if (opts.statusIn && opts.statusIn.length) {
      params.set('status_in', opts.statusIn.join(','));
    }
    const res = await Api.get(`/api/incidents?${params.toString()}`);
    const items = res.items || [];
    const cutoff = opts.days ? Date.now() - opts.days * 86400000 : null;
    return items.filter(inc => {
      if (!cutoff) return true;
      const created = new Date(inc.created_at || Date.now()).getTime();
      return created >= cutoff;
    }).map(inc => ({
      id: inc.id,
      title: `${inc.reg_no || '#'} ${inc.title || ''}`.trim(),
      meta: t(`incidents.status.${(inc.status || '').toLowerCase()}`),
      onClick: () => {
        openIncident(inc.id);
        closeDetailModal();
      }
    }));
  }

  async function listTasks(opts = {}) {
    const params = new URLSearchParams();
    params.set('limit', '200');
    const res = await Api.get(`/api/tasks?${params.toString()}`);
    const items = res.items || [];
    const now = Date.now();
    const cutoff = opts.completedDays ? now - opts.completedDays * 86400000 : null;
    const currentUserId = state.currentUser?.id || null;
    return items.filter(task => {
      if (opts.mine) {
        if (!currentUserId) return false;
        const assigned = Array.isArray(task.assigned_to) ? task.assigned_to : [];
        if (!assigned.includes(currentUserId)) return false;
      }
      if (opts.overdue) {
        if (!task.due_date || task.closed_at) return false;
        const due = new Date(task.due_date).getTime();
        if (Number.isNaN(due) || due >= now) return false;
      }
      if (opts.blocked) {
        if (!task.is_blocked || task.closed_at) return false;
      }
      if (cutoff !== null) {
        if (!task.closed_at) return false;
        const closed = new Date(task.closed_at).getTime();
        if (Number.isNaN(closed) || closed < cutoff) return false;
      }
      return true;
    }).map(task => ({
      id: task.id,
      title: task.title || `#${task.id}`,
      meta: taskMeta(task),
      onClick: () => {
        openTask(task.id);
        closeDetailModal();
      }
    }));
  }

  function taskMeta(task) {
    const meta = [];
    if (task.priority) {
      meta.push(t(`tasks.priority.${task.priority}`));
    }
    if (task.due_date) {
      meta.push(`${t('tasks.fields.dueDateShort')}: ${formatDate(task.due_date)}`);
    }
    if (task.is_blocked) {
      meta.push(t('tasks.blocks.badge'));
    }
    return meta.filter(Boolean).join(' | ');
  }

  function openDocument(docId) {
    if (!docId) return;
    if (typeof DocsPage !== 'undefined' && DocsPage.openDocTab && document.getElementById('docs-page')) {
      DocsPage.openDocTab(docId, 'view');
    } else {
      window.__pendingDocOpen = docId;
    }
    navigateToPath('/docs');
  }

  function openIncident(incidentId) {
    if (!incidentId) return;
    if (typeof IncidentsPage !== 'undefined' && IncidentsPage.openIncidentTab && document.getElementById('incidents-page')) {
      IncidentsPage.openIncidentTab(incidentId);
    } else {
      window.__pendingIncidentOpen = incidentId;
    }
    navigateToPath('/incidents');
  }

  function openTask(taskId) {
    if (!taskId) return;
    if (typeof TasksPage !== 'undefined' && TasksPage.openTask && document.getElementById('tasks-page')) {
      TasksPage.openTask(taskId);
    } else {
      window.__pendingTaskOpen = taskId;
    }
    navigateToPath(`/tasks/task/${taskId}`);
  }

  function navigateToPath(path) {
    const next = path.startsWith('/') ? path : `/${path}`;
    if (window.location.pathname === next) return;
    window.history.pushState({}, '', next);
    window.dispatchEvent(new PopStateEvent('popstate'));
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

  function closeDetailModal() {
    const modal = document.getElementById('dashboard-detail-modal');
    if (modal && DashboardPage.closeModal) {
      DashboardPage.closeModal(modal);
    }
  }

  DashboardPage.openDetailModal = openDetailModal;
})();
