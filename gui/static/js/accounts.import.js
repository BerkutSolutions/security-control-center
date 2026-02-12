(() => {
  const globalObj = typeof window !== 'undefined' ? window : globalThis;
  const AccountsPage = globalObj.AccountsPage || (globalObj.AccountsPage = {});
  const state = AccountsPage.state;
  const { showAlert, fillSelectWithEmpty, escapeHtml } = AccountsPage;

  function bindImportUI() {
    const openBtn = document.getElementById('open-import');
    const fileInput = document.getElementById('user-import-file');
    const cancelBtn = document.getElementById('cancel-user-import');
    const closeBtn = document.getElementById('close-user-import');
    const form = document.getElementById('user-import-form');
    if (openBtn && fileInput) {
      openBtn.onclick = () => {
        showAlert('user-import-alert', '');
        fileInput.value = '';
        fileInput.click();
      };
      fileInput.onchange = () => handleImportFile(fileInput.files);
    }
    if (cancelBtn) cancelBtn.onclick = closeImportModal;
    if (closeBtn) closeBtn.onclick = closeImportModal;
    if (form) {
      form.onsubmit = async (e) => {
        e.preventDefault();
        await submitImport();
      };
    }
    const downloadBtn = document.getElementById('download-import-passwords');
    if (downloadBtn) downloadBtn.onclick = downloadImportPasswords;
  }

  async function handleImportFile(files) {
    const file = files && files[0];
    if (!file) return;
    const fd = new FormData();
    fd.append('file', file);
    showAlert('user-import-alert', '');
    try {
      const res = await Api.upload('/api/accounts/import/upload', fd);
      state.importState.id = res.import_id;
      state.importState.headers = res.detected_headers || [];
      state.importState.preview = res.preview || [];
      const nameEl = document.getElementById('user-import-file-name');
      if (nameEl) nameEl.textContent = `${file.name} (${Math.round(file.size / 1024)} KB)`;
      renderImportPreview(state.importState.headers, state.importState.preview);
      populateImportMapping(state.importState.headers);
      const modal = document.getElementById('user-import-modal');
      if (modal) modal.hidden = false;
      const resultBox = document.getElementById('user-import-result');
      if (resultBox) resultBox.hidden = true;
    } catch (err) {
      showAlert('user-import-alert', err.message || (BerkutI18n.t('accounts.importErrorInvalidValue') || 'Import failed'));
    }
  }

  function populateImportMapping(headers) {
    const opts = (headers || []).map(h => ({ value: h, label: h }));
    const fields = [
      { id: 'import-map-login', match: ['login', 'username'] },
      { id: 'import-map-full_name', match: ['full_name', 'fullname', 'name'] },
      { id: 'import-map-department', match: ['department', 'dept'] },
      { id: 'import-map-position', match: ['position', 'title'] },
      { id: 'import-map-email', match: ['email', 'mail'] },
      { id: 'import-map-status', match: ['status', 'active'] },
      { id: 'import-map-roles', match: ['roles', 'role'] },
      { id: 'import-map-groups', match: ['groups', 'group'] },
      { id: 'import-map-clearance_level', match: ['clearance', 'clearance_level'] },
      { id: 'import-map-clearance_tags', match: ['clearance_tags', 'tags'] }
    ];
    fields.forEach(f => {
      fillSelectWithEmpty(document.getElementById(f.id), opts);
      const auto = autoSelectHeader(headers, f.match);
      const el = document.getElementById(f.id);
      if (el && auto) el.value = auto;
    });
    const temp = document.getElementById('import-temp-password');
    const mustChange = document.getElementById('import-must-change');
    if (temp) temp.checked = true;
    if (mustChange) mustChange.checked = true;
  }

  function autoSelectHeader(headers, candidates) {
    const normalized = (headers || []).map(h => h.toLowerCase());
    for (const cand of candidates) {
      const cleanCand = cand.replace(/[^a-z0-9]/g, '');
      const idx = normalized.findIndex(h => h === cand || h.replace(/[^a-z0-9]/g, '') === cleanCand);
      if (idx >= 0) return headers[idx];
    }
    return '';
  }

  function renderImportPreview(headers, rows) {
    const container = document.getElementById('user-import-preview');
    if (!container) return;
    if (!headers || !headers.length) {
      container.innerHTML = '';
      return;
    }
    const title = BerkutI18n.t('accounts.importPreview') || 'Preview';
    let html = `<h4>${escapeHtml(title)}</h4><div class="table-responsive"><table class="data-table"><thead><tr>`;
    headers.forEach(h => { html += `<th>${escapeHtml(h)}</th>`; });
    html += '</tr></thead><tbody>';
    (rows || []).slice(0, 10).forEach(r => {
      html += '<tr>';
      headers.forEach((_, idx) => {
        const val = (r && r[idx]) ? r[idx] : '';
        html += `<td>${escapeHtml(val || '')}</td>`;
      });
      html += '</tr>';
    });
    if (!rows || !rows.length) {
      html += `<tr><td colspan="${headers.length}">${escapeHtml(BerkutI18n.t('common.notAvailable') || '-')}</td></tr>`;
    }
    html += '</tbody></table></div>';
    container.innerHTML = html;
  }

  function collectImportMapping() {
    const mapping = {};
    const fields = ['login', 'full_name', 'department', 'position', 'email', 'status', 'roles', 'groups', 'clearance_level', 'clearance_tags'];
    fields.forEach(f => {
      const el = document.getElementById(`import-map-${f}`);
      if (el) {
        mapping[f] = el.value || '';
      }
    });
    return mapping;
  }

  async function submitImport() {
    const mapping = collectImportMapping();
    if (!state.importState.id) {
      showAlert('user-import-alert', BerkutI18n.t('accounts.importMapping') || 'Upload CSV first');
      return;
    }
    if (!mapping.login || !mapping.full_name) {
      showAlert('user-import-alert', BerkutI18n.t('accounts.importMapping') || 'Mapping is required');
      return;
    }
    const payload = {
      import_id: state.importState.id,
      mapping,
      options: {
        temp_password: document.getElementById('import-temp-password')?.checked ?? true,
        must_change_password: document.getElementById('import-must-change')?.checked ?? true
      },
      defaults: {
        default_role: document.getElementById('import-default-role')?.value || '',
        default_group: document.getElementById('import-default-group')?.value || ''
      }
    };
    try {
      const res = await Api.post('/api/accounts/import/commit', payload);
      renderImportResult(res);
      await AccountsPage.loadUsers();
    } catch (err) {
      showAlert('user-import-alert', err.message || (BerkutI18n.t('accounts.importErrorInvalidValue') || 'Import failed'));
    }
  }

  function renderImportResult(res) {
    const box = document.getElementById('user-import-result');
    if (!box) return;
    const summary = document.getElementById('user-import-summary');
    if (summary) {
      const total = res.total_rows != null ? res.total_rows : 0;
      const created = res.created_count || 0;
      const failed = res.failed_count || 0;
      const label = BerkutI18n.t('accounts.importResult') || 'Result';
      const errorsLabel = failed === 1 ? (BerkutI18n.t('common.error') || 'error') : (BerkutI18n.t('common.errors') || 'errors');
      summary.textContent = `${label}: ${created}/${total} (${failed} ${errorsLabel})`;
    }
    renderImportFailures(res.failures || []);
    renderImportPasswords(res.created_users || []);
    box.hidden = false;
  }

  function renderImportFailures(failures) {
    const container = document.getElementById('user-import-errors');
    if (!container) return;
    if (!failures.length) {
      container.innerHTML = '';
      container.hidden = true;
      return;
    }
    let html = '<div class="table-responsive"><table class="data-table"><thead><tr><th>' + (BerkutI18n.t('accounts.importPreview') || 'Row') + '</th><th>' + (BerkutI18n.t('accounts.importResult') || 'Reason') + '</th></tr></thead><tbody>';
    failures.forEach(f => {
      html += `<tr><td>${escapeHtml(f.row_number || '')}</td><td>${escapeHtml(reasonLabel(f))}</td></tr>`;
    });
    html += '</tbody></table></div>';
    container.innerHTML = html;
    container.hidden = false;
  }

  function renderImportPasswords(list) {
    const box = document.getElementById('user-import-passwords');
    const table = document.getElementById('user-import-passwords-table');
    if (!box || !table) return;
    const rows = (list || []).filter(u => u.temp_password);
    state.importState.createdUsers = rows;
    if (!rows.length) {
      table.innerHTML = '';
      box.hidden = true;
      return;
    }
    let html = '<div class="table-responsive"><table class="data-table"><thead><tr><th>Login</th><th>Password</th></tr></thead><tbody>';
    rows.forEach(r => {
      html += `<tr><td>${escapeHtml(r.login || r.Login || '')}</td><td>${escapeHtml(r.temp_password || '')}</td></tr>`;
    });
    html += '</tbody></table></div>';
    table.innerHTML = html;
    box.hidden = false;
  }

  function reasonLabel(failure) {
    const map = {
      already_exists: 'accounts.importErrorAlreadyExists',
      role_not_found: 'accounts.importErrorRoleNotFound',
      group_not_found: 'accounts.importErrorGroupNotFound',
      clearance_too_high: 'accounts.importErrorClearanceTooHigh',
      invalid_value: 'accounts.importErrorInvalidValue',
    };
    const key = map[failure.reason] || map.invalid_value;
    let text = BerkutI18n.t(key) || failure.reason || '';
    if (failure.detail) {
      text += ` (${failure.detail})`;
    }
    return text;
  }

  function closeImportModal() {
    const modal = document.getElementById('user-import-modal');
    if (modal) modal.hidden = true;
    const nameEl = document.getElementById('user-import-file-name');
    if (nameEl) nameEl.textContent = '';
    state.importState = { id: null, headers: [], preview: [], createdUsers: [] };
    renderImportPreview([], []);
    const resultBox = document.getElementById('user-import-result');
    if (resultBox) resultBox.hidden = true;
    showAlert('user-import-alert', '');
  }

  function downloadImportPasswords() {
    if (!state.importState.createdUsers || !state.importState.createdUsers.length) return;
    let csv = 'login,temp_password\n';
    state.importState.createdUsers.forEach(row => {
      csv += `${row.login || row.Login},${row.temp_password}\n`;
    });
    const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'import_passwords.csv';
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  }

  AccountsPage.bindImportUI = bindImportUI;
  AccountsPage.handleImportFile = handleImportFile;
  AccountsPage.populateImportMapping = populateImportMapping;
  AccountsPage.autoSelectHeader = autoSelectHeader;
  AccountsPage.renderImportPreview = renderImportPreview;
  AccountsPage.collectImportMapping = collectImportMapping;
  AccountsPage.submitImport = submitImport;
  AccountsPage.renderImportResult = renderImportResult;
  AccountsPage.renderImportFailures = renderImportFailures;
  AccountsPage.renderImportPasswords = renderImportPasswords;
  AccountsPage.reasonLabel = reasonLabel;
  AccountsPage.closeImportModal = closeImportModal;
  AccountsPage.downloadImportPasswords = downloadImportPasswords;
})();
