(() => {
  const globalObj = typeof window !== 'undefined' ? window : globalThis;
  const AppCompat = globalObj.AppCompat || (globalObj.AppCompat = {});
  let shown = false;
  let pollTimer = null;
  let activeJobId = null;
  let pendingFullReset = null;

  function t(key) {
    return (globalObj.BerkutI18n && BerkutI18n.t(key)) || key;
  }

  function escapeHtml(str) {
    return String(str || '')
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;');
  }

  function statusLabel(status) {
    return t(`compat.status.${status || 'unknown'}`);
  }

  function requiredResetToken() {
    return 'RESET';
  }

  function showAlert(err) {
    const el = document.getElementById('compat-wizard-alert');
    if (!el) return;
    const msg = err && err.message ? String(err.message) : String(err || '');
    el.textContent = msg || t('common.serverError');
    el.hidden = false;
  }

  function hideAlert() {
    const el = document.getElementById('compat-wizard-alert');
    if (el) el.hidden = true;
  }

  function showConfirmAlert(msg) {
    const el = document.getElementById('compat-confirm-alert');
    if (!el) return;
    el.textContent = msg || '';
    el.hidden = !msg;
  }

  function hideConfirmAlert() {
    showConfirmAlert('');
  }

  function showWizardView() {
    stopPolling();
    const wizard = document.getElementById('compat-wizard-view');
    const jobView = document.getElementById('compat-job-view');
    if (wizard) wizard.hidden = false;
    if (jobView) jobView.hidden = true;
    hideAlert();
  }

  function showJobView() {
    const wizard = document.getElementById('compat-wizard-view');
    const jobView = document.getElementById('compat-job-view');
    if (wizard) wizard.hidden = true;
    if (jobView) jobView.hidden = false;
    hideAlert();
  }

  function ensureWizardModal() {
    let modal = document.getElementById('compat-modal');
    if (modal) return modal;

    modal = document.createElement('div');
    modal.className = 'modal';
    modal.id = 'compat-modal';
    modal.hidden = true;
    modal.innerHTML = `
      <div class="modal-backdrop"></div>
      <div class="modal-body wide">
        <div class="modal-header">
          <div>
            <h3 class="compat-modal-title">${escapeHtml(t('compat.modal.title'))}</h3>
            <div class="muted compat-modal-subtitle">${escapeHtml(t('compat.modal.subtitle'))}</div>
          </div>
          <button id="compat-modal-close" class="btn ghost" data-close="#compat-modal">${escapeHtml(t('common.close'))}</button>
        </div>
        <div class="modal-content">
          <div id="compat-wizard-alert" class="alert" hidden></div>

          <div id="compat-wizard-view" class="card">
            <div class="card-body">
              <div class="muted">${escapeHtml(t('compat.modal.note'))}</div>
              <div class="table-responsive compat-table-wrapper">
                <table class="table">
                  <thead>
                    <tr>
                      <th>${escapeHtml(t('compat.table.module'))}</th>
                      <th>${escapeHtml(t('compat.table.status'))}</th>
                      <th>${escapeHtml(t('compat.table.details'))}</th>
                      <th>${escapeHtml(t('compat.table.actions'))}</th>
                    </tr>
                  </thead>
                  <tbody id="compat-table-body"></tbody>
                </table>
              </div>
            </div>
          </div>

          <div id="compat-job-view" class="card" hidden>
            <div class="card-body">
              <div class="progress-section">
                <div class="progress-meta">
                  <div><span class="label">${escapeHtml(t('compat.job.id'))}</span><span id="compat-job-id">-</span></div>
                  <div><span class="label">${escapeHtml(t('compat.job.status'))}</span><span id="compat-job-status">-</span></div>
                  <div><span class="label">${escapeHtml(t('compat.job.progress'))}</span><span id="compat-job-progress">0%</span></div>
                </div>
                <progress id="compat-job-progressbar" class="progress-native" max="100" value="0"></progress>
              </div>
              <div class="log-card compat-log-card">
                <div class="log-actions">
                  <div class="muted">${escapeHtml(t('compat.job.log'))}</div>
                  <button id="compat-job-cancel" class="btn danger btn-sm">${escapeHtml(t('compat.job.cancel'))}</button>
                  <button id="compat-job-back" class="btn ghost btn-sm">${escapeHtml(t('compat.job.back'))}</button>
                </div>
                <div id="compat-job-log" class="log-stream" aria-label="job log"></div>
              </div>
            </div>
          </div>
        </div>
      </div>
    `;
    document.body.appendChild(modal);

    const cleanup = () => stopPolling();
    modal.addEventListener('click', (e) => {
      if (e.target && e.target.classList && e.target.classList.contains('modal-backdrop')) cleanup();
    });
    document.getElementById('compat-modal-close')?.addEventListener('click', cleanup);
    document.addEventListener('keydown', (e) => {
      if (e.key === 'Escape' && modal && !modal.hidden) cleanup();
    });

    document.getElementById('compat-job-back')?.addEventListener('click', () => showWizardView());
    document.getElementById('compat-job-cancel')?.addEventListener('click', async () => {
      if (!activeJobId) return;
      try {
        await Api.post(`/api/app/jobs/${activeJobId}/cancel`, {});
      } catch (err) {
        showAlert(err);
      }
    });

    return modal;
  }

  function ensureFullResetConfirmModal() {
    let modal = document.getElementById('compat-reset-confirm-modal');
    if (modal) return modal;
    modal = document.createElement('div');
    modal.className = 'modal';
    modal.id = 'compat-reset-confirm-modal';
    modal.hidden = true;
    modal.innerHTML = `
      <div class="modal-backdrop"></div>
      <div class="modal-body">
        <div class="modal-header">
          <h3 class="compat-confirm-title">${escapeHtml(t('compat.confirm.title'))}</h3>
          <button class="btn ghost" data-close="#compat-reset-confirm-modal">${escapeHtml(t('common.close'))}</button>
        </div>
        <div class="modal-content">
          <div id="compat-confirm-alert" class="alert" hidden></div>
          <div class="card">
            <div class="card-body">
              <div class="muted" id="compat-confirm-text"></div>
              <div class="compat-confirm-fields">
                <label class="muted">${escapeHtml(t('compat.confirm.prompt'))}</label>
                <input id="compat-confirm-input" class="input" placeholder="${escapeHtml(t('compat.confirm.placeholder'))}" />
                <div class="muted compat-confirm-hint">${escapeHtml(t('compat.confirm.required'))}: <code>${escapeHtml(requiredResetToken())}</code></div>
              </div>
            </div>
          </div>
          <div class="form-actions">
            <button id="compat-confirm-cancel" class="btn ghost" data-close="#compat-reset-confirm-modal">${escapeHtml(t('common.cancel'))}</button>
            <button id="compat-confirm-ok" class="btn danger">${escapeHtml(t('compat.confirm.action'))}</button>
          </div>
        </div>
      </div>
    `;
    document.body.appendChild(modal);

    document.getElementById('compat-confirm-ok')?.addEventListener('click', async () => {
      const input = (document.getElementById('compat-confirm-input')?.value || '').trim();
      if (input !== requiredResetToken()) {
        showConfirmAlert(t('compat.confirm.mismatch'));
        return;
      }
      hideConfirmAlert();
      const req = pendingFullReset;
      pendingFullReset = null;
      modal.hidden = true;
      if (!req) return;
      await startJob(req);
    });
    return modal;
  }

  function openFullResetConfirm(item) {
    pendingFullReset = { type: 'reinit', scope: 'module', module_id: item.module_id, mode: 'full' };
    const modal = ensureFullResetConfirmModal();
    const textEl = document.getElementById('compat-confirm-text');
    const inputEl = document.getElementById('compat-confirm-input');
    if (textEl) {
      const title = t(item.title_i18n_key) || item.module_id || '-';
      textEl.textContent = t('compat.confirm.text').replace('{module}', title);
    }
    if (inputEl) inputEl.value = '';
    hideConfirmAlert();
    modal.hidden = false;
    inputEl?.focus();
  }

  function renderItems(items) {
    const tbody = document.getElementById('compat-table-body');
    if (!tbody) return;
    tbody.innerHTML = '';

    (items || []).forEach((it) => {
      const title = t(it.title_i18n_key) || it.module_id || '-';
      const details = t(it.details_i18n_key) || '';
      const status = statusLabel(it.status);
      const lastErr = it.last_error ? `\n${t('compat.lastError')}: ${it.last_error}` : '';
      const canPartial = it.status && it.status !== 'ok' && !!it.has_partial_adapt;
      const canFull = it.status && it.status !== 'ok' && !!it.has_full_reset;
      const versions = (it && typeof it.expected_schema_version === 'number')
        ? `schema ${Number(it.applied_schema_version || 0)}/${Number(it.expected_schema_version || 0)}, behavior ${Number(it.applied_behavior_version || 0)}/${Number(it.expected_behavior_version || 0)}`
        : '';

      const tr = document.createElement('tr');
      tr.innerHTML = `
        <td>${escapeHtml(title)}</td>
        <td><span class="badge">${escapeHtml(status)}</span></td>
        <td class="muted">${escapeHtml(details + lastErr)}${versions ? `<div class="compat-version-row muted">${escapeHtml(versions)}</div>` : ''}</td>
        <td></td>
      `;

      const actions = document.createElement('div');
      actions.className = 'compat-actions';

      const partialBtn = document.createElement('button');
      partialBtn.className = 'btn secondary btn-sm';
      partialBtn.textContent = t('compat.action.partial');
      partialBtn.disabled = !canPartial;
      partialBtn.addEventListener('click', async () => {
        await startJob({ type: 'adapt', scope: 'module', module_id: it.module_id, mode: 'partial' });
      });
      actions.appendChild(partialBtn);

      const fullBtn = document.createElement('button');
      fullBtn.className = 'btn danger btn-sm';
      fullBtn.textContent = t('compat.action.full');
      fullBtn.disabled = !canFull;
      fullBtn.addEventListener('click', () => openFullResetConfirm(it));
      actions.appendChild(fullBtn);

      tr.querySelector('td:last-child')?.appendChild(actions);
      tbody.appendChild(tr);
    });
  }

  async function startJob(req) {
    if (!globalObj.Api || typeof Api.post !== 'function') return;
    try {
      hideAlert();
      const res = await Api.post('/api/app/jobs', req);
      const id = res && res.id ? Number(res.id) : 0;
      if (!id) throw new Error(t('compat.job.createFailed'));
      await openJob(id);
    } catch (err) {
      const msg = err && err.message ? String(err.message) : '';
      if (msg.toLowerCase().includes('forbidden') || msg.toLowerCase().includes('unauthorized')) {
        showAlert(new Error(t('common.accessDenied')));
        return;
      }
      showAlert(err);
    }
  }

  async function openJob(id) {
    activeJobId = id;
    showJobView();
    updateJobUI({ id, status: 'queued', progress: 0, log_json: '[]' });
    await pollJob(id);
    pollTimer = setInterval(() => pollJob(id), 1000);
  }

  function stopPolling() {
    if (pollTimer) {
      clearInterval(pollTimer);
      pollTimer = null;
    }
    activeJobId = null;
  }

  async function pollJob(id) {
    if (!id) return;
    try {
      const res = await Api.get(`/api/app/jobs/${id}`);
      updateJobUI(res && res.job ? res.job : null);
    } catch (err) {
      stopPolling();
      showAlert(err);
    }
  }

  function safeParseJSON(raw) {
    try {
      return JSON.parse(raw || '[]');
    } catch (_) {
      return [];
    }
  }

  function formatTS(raw) {
    const s = String(raw || '').trim();
    if (!s) return '';
    const d = new Date(s);
    if (isNaN(d.getTime())) return s;
    return d.toLocaleString();
  }

  function updateJobUI(job) {
    if (!job) return;
    const idEl = document.getElementById('compat-job-id');
    const stEl = document.getElementById('compat-job-status');
    const prEl = document.getElementById('compat-job-progress');
    const progressEl = document.getElementById('compat-job-progressbar');
    const logEl = document.getElementById('compat-job-log');
    const cancelBtn = document.getElementById('compat-job-cancel');

    const status = String(job.status || '').trim();
    const progress = Number(job.progress || 0);
    if (idEl) idEl.textContent = String(job.id || '-');
    if (stEl) stEl.textContent = t(`compat.jobStatus.${status || 'unknown'}`);
    if (prEl) prEl.textContent = `${Math.max(0, Math.min(100, progress))}%`;
    if (progressEl && typeof progressEl.value !== 'undefined') {
      progressEl.value = Math.max(0, Math.min(100, progress));
    }

    if (cancelBtn) {
      cancelBtn.disabled = !(status === 'queued' || status === 'running');
    }

    if (logEl) {
      const items = safeParseJSON(job.log_json);
      logEl.innerHTML = '';
      (items || []).slice(-200).forEach((line) => {
        const ts = formatTS(line.ts);
        const lvl = String(line.level || 'info').toLowerCase();
        const msg = String(line.msg || '');
        const fields = line.fields && typeof line.fields === 'object' ? line.fields : null;
        const extra = fields ? ` ${JSON.stringify(fields)}` : '';
        const span = document.createElement('span');
        span.className = `log-line level-${lvl}`;
        span.innerHTML = `<span class="ts">${escapeHtml(ts)}</span><span class="lvl">${escapeHtml(lvl.toUpperCase())}</span> ${escapeHtml(msg + extra)}`;
        logEl.appendChild(span);
      });
      logEl.scrollTop = logEl.scrollHeight;
    }

    if (status === 'finished' || status === 'failed' || status === 'canceled') {
      stopPolling();
    }
  }

  async function checkAndShowWizard() {
    if (!globalObj.Api || typeof Api.get !== 'function') return;
    if (shown) return;
    shown = true;

    let report;
    try {
      report = await Api.get('/api/app/compat');
    } catch (err) {
      console.warn('compat check failed', err);
      return;
    }
    const items = report?.items || [];
    const needs = items.filter(i => i && i.status && i.status !== 'ok');
    if (!needs.length) return;

    const modal = ensureWizardModal();
    showWizardView();
    renderItems(needs);
    modal.hidden = false;
  }

  AppCompat.checkAndNotify = checkAndShowWizard;
  AppCompat.checkAndShowWizard = checkAndShowWizard;

})();
