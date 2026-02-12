const ApprovalsPage = (() => {
  let approvals = [];
  let docsCache = {};
  let versionsCache = {};
  let current = null;
  let me = null;

  function getUserDirectory() {
    if (typeof window !== 'undefined' && window.UserDirectory) return window.UserDirectory;
    if (typeof UserDirectory !== 'undefined') return UserDirectory;
    return {
      load: async () => ({}),
      name: (id) => `#${id}`,
      all: () => [],
      get: () => null,
    };
  }
  async function init() {
    const page = document.getElementById('approvals-page');
    if (!page) return;
    await getUserDirectory().load();
    me = await loadMe();
    bindUI();
    await loadApprovals();
    const pending = parseApprovalRoute();
    if (pending) {
      openApproval(pending);
    }
  }

  function bindUI() {
    const statusSel = document.getElementById('approvals-status');
    if (statusSel) statusSel.onchange = () => loadApprovals();
    const refresh = document.getElementById('approvals-refresh');
    if (refresh) refresh.onclick = () => loadApprovals();
    document.querySelectorAll('[data-close="#approval-detail-modal"]').forEach(btn => {
      btn.onclick = () => {
        closeModal('#approval-detail-modal');
        resetApprovalPath();
      };
    });
    const modal = document.getElementById('approval-detail-modal');
    modal?.addEventListener('click', (e) => {
      if (e.target.classList.contains('modal-backdrop')) {
        resetApprovalPath();
      }
    });
    document.addEventListener('keydown', (e) => {
      if (e.key !== 'Escape') return;
      if (modal && !modal.hidden) resetApprovalPath();
    });
    const commentBtn = document.getElementById('comment-submit');
    if (commentBtn) commentBtn.onclick = () => submitComment();
  }

  async function loadApprovals() {
    const statusSel = document.getElementById('approvals-status');
    const qs = statusSel && statusSel.value ? `?status=${statusSel.value}` : '';
    try {
      const res = await Api.get(`/api/approvals${qs}`);
      approvals = res.items || [];
      await preloadDocs(approvals);
      renderTable();
    } catch (err) {
      console.error('load approvals', err);
      renderTable([]);
    }
  }

  async function preloadDocs(list) {
    const ids = Array.from(new Set(list.map(a => a.doc_id))).filter(id => !docsCache[id]);
    const fetched = await Promise.all(ids.map(id => Api.get(`/api/docs/${id}`).catch(() => null)));
    fetched.forEach(doc => {
      if (doc) docsCache[doc.id] = doc;
    });
  }

  function renderTable() {
    const tbody = document.querySelector('#approvals-table tbody');
    if (!tbody) return;
    tbody.innerHTML = '';
    if (!approvals.length) {
      const tr = document.createElement('tr');
      tr.innerHTML = `<td colspan="4">${BerkutI18n.t('approvals.empty')}</td>`;
      tbody.appendChild(tr);
      return;
    }
    approvals.forEach(ap => {
      const doc = docsCache[ap.doc_id] || {};
      const tr = document.createElement('tr');
      tr.innerHTML = `
        <td>${escapeHtml(doc.title || `#${ap.doc_id}`)}</td>
        <td><span class="badge status-${ap.status}">${DocUI.statusLabel(ap.status)}</span></td>
        <td>${escapeHtml(ap.message || '')}</td>
        <td>${formatDate(ap.updated_at || ap.created_at)}</td>
      `;
      tr.onclick = () => openApproval(ap.id);
      tbody.appendChild(tr);
    });
  }

  async function openApproval(id) {
    const alertBox = document.getElementById('approval-detail-alert');
    hideAlert(alertBox);
    try {
      updateApprovalPath(id);
      const res = await Api.get(`/api/approvals/${id}`);
      current = res.approval;
      const participants = res.participants || [];
      const doc = docsCache[current.doc_id] || await Api.get(`/api/docs/${current.doc_id}`);
      docsCache[current.doc_id] = doc;
      await renderDetail(doc, current, participants);
      await loadComments();
      openModal('#approval-detail-modal');
    } catch (err) {
      showAlert(alertBox, err.message || 'failed');
    }
  }

  function parseApprovalRoute() {
    const parts = window.location.pathname.split('/').filter(Boolean);
    if (parts[0] !== 'approvals') return null;
    const id = parseInt(parts[1] || '', 10);
    return Number.isFinite(id) ? id : null;
  }

  function updateApprovalPath(id) {
    if (!id) return;
    const next = `/approvals/${id}`;
    if (window.location.pathname !== next) {
      window.history.replaceState({}, '', next);
    }
  }

  function resetApprovalPath() {
    if (window.location.pathname !== '/approvals') {
      window.history.replaceState({}, '', '/approvals');
    }
  }

  async function renderDetail(doc, ap, parts) {
    const titleEl = document.getElementById('approval-doc-title');
    const metaEl = document.getElementById('approval-doc-meta');
    const statusEl = document.getElementById('approval-status');
    const levelEl = document.getElementById('approval-classification');
    const versionEl = document.getElementById('approval-version');
    const initiatorEl = document.getElementById('approval-initiator');
    const reasonEl = document.getElementById('approval-reason');
    const requestEl = document.getElementById('approval-request');
    const updatedEl = document.getElementById('approval-updated');
    if (titleEl) titleEl.textContent = `${doc.title || ''} (${doc.reg_number || ''})`;
    if (metaEl) metaEl.textContent = `${DocUI.levelName(doc.classification_level)} • ${formatDate(ap.updated_at || ap.created_at)}`;
    if (statusEl) statusEl.innerHTML = `<span class="badge status-${ap.status}">${DocUI.statusLabel(ap.status)}</span>`;
    if (levelEl) levelEl.textContent = DocUI.levelName(doc.classification_level);
    const versionInfo = await loadVersionInfo(doc);
    if (versionEl) versionEl.textContent = versionInfo.version ? `v${versionInfo.version}` : '-';
    const initiatorName = getUserDirectory().name(ap.created_by || doc.created_by);
    if (initiatorEl) initiatorEl.textContent = initiatorName || '-';
    if (reasonEl) reasonEl.textContent = versionInfo.reason || '-';
    if (requestEl) requestEl.textContent = ap.message || '-';
    if (updatedEl) updatedEl.textContent = formatDate(ap.updated_at || ap.created_at);
    renderParticipants(parts);
    const stages = buildStages(ap, parts);
    renderStages(stages, ap);
  }

  async function loadVersionInfo(doc) {
    if (!doc) return { version: '', reason: '' };
    if (versionsCache[doc.id]) return versionsCache[doc.id];
    try {
      const res = await Api.get(`/api/docs/${doc.id}/versions`);
      const list = (res.versions || []).slice().sort((a, b) => (b.version || 0) - (a.version || 0));
      const latest = list[0] || {};
      const info = { version: latest.version || doc.current_version || '', reason: latest.reason || '' };
      versionsCache[doc.id] = info;
      return info;
    } catch (err) {
      return { version: doc.current_version || '', reason: '' };
    }
  }

  function renderParticipants(parts = []) {
    const participantsEl = document.getElementById('participants-list');
    if (!participantsEl) return;
    participantsEl.innerHTML = '';
    const seen = new Set();
    parts.forEach(p => {
      if (seen.has(p.user_id)) return;
      seen.add(p.user_id);
      const li = document.createElement('li');
      const name = getUserDirectory().name(p.user_id);
      li.textContent = name;
      participantsEl.appendChild(li);
    });
  }

  function buildStages(ap, parts = []) {
    const map = new Map();
    parts.forEach(p => {
      const stageNum = p.stage || 1;
      const key = stageNum;
      const item = map.get(key) || { stage: stageNum, name: '', message: '', approvers: [], observers: [], decidedAt: null, status: 'pending' };
      if (!item.name && p.stage_name) item.name = p.stage_name;
      if (!item.message && p.stage_message) item.message = p.stage_message;
      if (p.role === 'approver') {
        item.approvers.push(p);
      } else if (p.role === 'observer') {
        item.observers.push(p);
      }
      map.set(key, item);
    });
    const stages = Array.from(map.values()).sort((a, b) => a.stage - b.stage);
    stages.forEach(stage => {
      let decidedAt = null;
      let hasReject = false;
      let allApproved = true;
      if (!stage.approvers.length) {
        allApproved = false;
      }
      stage.approvers.forEach(p => {
        if (p.decision === 'reject') hasReject = true;
        if (!p.decision) allApproved = false;
        if (p.decided_at) {
          const dt = new Date(p.decided_at);
          if (!decidedAt || dt > decidedAt) decidedAt = dt;
        }
      });
      if (hasReject) {
        stage.status = 'rejected';
      } else if (allApproved && stage.approvers.length) {
        stage.status = 'approved';
      } else {
        stage.status = 'pending';
      }
      stage.decidedAt = decidedAt;
      if (!stage.name) {
        stage.name = `${BerkutI18n.t('docs.approvalStage') || 'Этап'} ${stage.stage}`;
      }
    });
    stages.forEach(stage => {
      const currentStage = ap.current_stage || 1;
      if (ap.status === 'review' && stage.stage > currentStage) {
        stage.status = 'locked';
      }
      if (ap.status === 'approved' && stage.status === 'pending') {
        stage.status = 'approved';
      }
      if (ap.status === 'returned' && stage.status === 'pending') {
        stage.status = 'rejected';
      }
    });
    return stages;
  }

  function renderStages(stages = [], ap) {
    const container = document.getElementById('approval-stages-container');
    if (!container) return;
    container.innerHTML = '';
    if (!stages.length) {
      container.textContent = BerkutI18n.t('approvals.empty');
      return;
    }
    const currentStage = ap.current_stage || 1;
    stages.forEach(stage => {
      const card = document.createElement('div');
      card.className = `stage-card status-${stage.status}${stage.stage === currentStage && ap.status === 'review' ? ' current-stage' : ''}`;
      const statusText = stageStatusLabel(stage.status);
      const statusMeta = stage.decidedAt ? `${statusText} ${formatDate(stage.decidedAt)}` : statusText;
      const approversList = stage.approvers.map(p => `<div class="person-chip">${escapeHtml(getUserDirectory().name(p.user_id))}</div>`).join('') || `<div class="muted">${BerkutI18n.t('approvals.stage.noApprovers') || '-'}</div>`;
      const observersList = stage.observers.map(p => `<div class="person-chip muted">${escapeHtml(getUserDirectory().name(p.user_id))}</div>`).join('') || `<div class="muted">${BerkutI18n.t('approvals.stage.noObservers') || '-'}</div>`;
      const messageText = stage.message ? escapeHtml(stage.message) : (BerkutI18n.t('approvals.stage.noMessage') || '');
      card.innerHTML = `
        <div class="stage-card-head">
          <div>
            <div class="stage-label">${BerkutI18n.t('docs.approvalStage') || 'Этап'} ${stage.stage}</div>
            <div class="stage-name">${escapeHtml(stage.name || '')}</div>
          </div>
          <div class="stage-status">${statusMeta}</div>
        </div>
        <div class="stage-message">${messageText}</div>
        <div class="stage-people">
          <div>
            <div class="muted">${BerkutI18n.t('approvals.role.approver')}</div>
            <div class="people-list">${approversList}</div>
          </div>
          <div>
            <div class="muted">${BerkutI18n.t('approvals.role.observer')}</div>
            <div class="people-list">${observersList}</div>
          </div>
        </div>
      `;
      const isMyStage = ap.status === 'review' && stage.stage === currentStage;
      const myEntry = stage.approvers.find(p => me && p.user_id === me.id);
      const canDecide = isMyStage && myEntry && (!myEntry.decision || myEntry.decision === '');
      if (canDecide) {
        const actions = document.createElement('div');
        actions.className = 'stage-decision';
        actions.innerHTML = `
          <h4>${BerkutI18n.t('approvals.decisionTitle')}</h4>
          <textarea class="decision-comment textarea" data-stage="${stage.stage}" rows="2" placeholder="${escapeHtml(BerkutI18n.t('approvals.decisionPlaceholder') || '')}"></textarea>
          <div class="decision-actions">
            <button class="btn success" data-stage-action="approve" data-stage="${stage.stage}">${BerkutI18n.t('approvals.approve')}</button>
            <button class="btn danger" data-stage-action="reject" data-stage="${stage.stage}">${BerkutI18n.t('approvals.reject')}</button>
          </div>
        `;
        const approveBtn = actions.querySelector('[data-stage-action="approve"]');
        const rejectBtn = actions.querySelector('[data-stage-action="reject"]');
        if (approveBtn) approveBtn.onclick = () => submitDecision('approve', stage.stage);
        if (rejectBtn) rejectBtn.onclick = () => submitDecision('reject', stage.stage);
        card.appendChild(actions);
      } else if (isMyStage && myEntry && myEntry.decision) {
        const info = document.createElement('div');
        info.className = 'stage-decision readonly';
        info.textContent = `${BerkutI18n.t('approvals.decisionTitle')}: ${BerkutI18n.t(`approvals.decision.${myEntry.decision}`)}`;
        card.appendChild(info);
      }
      container.appendChild(card);
    });
  }

  function stageStatusLabel(status) {
    switch (status) {
      case 'approved':
        return BerkutI18n.t('approvals.decision.approve');
      case 'rejected':
        return BerkutI18n.t('approvals.decision.reject');
      case 'locked':
        return BerkutI18n.t('approvals.stage.locked') || BerkutI18n.t('approvals.pending');
      default:
        return BerkutI18n.t('approvals.pending');
    }
  }

  async function loadComments() {
    const list = document.getElementById('comments-list');
    if (!current || !list) return;
    list.innerHTML = '';
    try {
      const res = await Api.get(`/api/approvals/${current.id}/comments`);
      (res.comments || []).forEach(c => {
        const item = document.createElement('div');
        item.className = 'comment-item';
        item.innerHTML = `<div class="comment-meta">${escapeHtml(c.author || getUserDirectory().name(c.user_id))} • ${formatDate(c.created_at)}</div><div class="comment-text">${escapeHtml(c.comment)}</div>`;
        list.appendChild(item);
      });
    } catch (err) {
      console.warn('load comments', err);
    }
  }

  async function submitComment() {
    if (!current) return;
    const textEl = document.getElementById('comment-text');
    if (!textEl) return;
    const text = textEl.value.trim();
    if (!text) return;
    await Api.post(`/api/approvals/${current.id}/comments`, { comment: text });
    textEl.value = '';
    await loadComments();
  }

  async function submitDecision(decision, stage) {
    if (!current) return;
    const commentEl = stage ? document.querySelector(`.decision-comment[data-stage="${stage}"]`) : document.getElementById('decision-comment');
    const alertBox = document.getElementById('approval-detail-alert');
    const comment = (commentEl && commentEl.value || '').trim();
    if (!comment) {
      showAlert(alertBox, BerkutI18n.t('approvals.decisionPlaceholder'));
      return;
    }
    try {
      await Api.post(`/api/approvals/${current.id}/decision`, { decision, comment });
      await loadApprovals();
      if (typeof DocsPage !== 'undefined' && DocsPage.hasPermission && DocsPage.hasPermission('docs.view')) {
        await DocsPage.loadDocs();
      }
      await openApproval(current.id);
    } catch (err) {
      showAlert(alertBox, err.message || 'error');
    }
  }

  async function loadMe() {
    try {
      const res = await Api.get('/api/auth/me');
      return res.user;
    } catch (err) {
      return null;
    }
  }

  function escapeHtml(str) {
    return (str || '').toString().replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', '\'': '&#39;' }[c]));
  }

  function formatDate(d) {
    if (!d) return '-';
    try {
      if (typeof AppTime !== 'undefined' && AppTime.formatDateTime) {
        return AppTime.formatDateTime(d);
      }
      const dt = new Date(d);
      const pad = (n) => (n < 10 ? `0${n}` : `${n}`);
      const day = pad(dt.getDate());
      const mon = pad(dt.getMonth() + 1);
      const year = dt.getFullYear();
      const hh = pad(dt.getHours());
      const mm = pad(dt.getMinutes());
      return `${day}.${mon}.${year} ${hh}:${mm}`;
    } catch (_) {
      return d;
    }
  }

  function hideAlert(el) {
    if (el) el.hidden = true;
  }

  function showAlert(el, msg) {
    if (!el) return;
    el.textContent = msg;
    el.hidden = false;
  }

  function openModal(sel) {
    const el = document.querySelector(sel);
    if (el) el.hidden = false;
  }

  function closeModal(sel) {
    const el = document.querySelector(sel);
    if (el) el.hidden = true;
  }

  return { init, refresh: loadApprovals };
})();

if (typeof window !== 'undefined') {
  window.ApprovalsPage = ApprovalsPage;
}
