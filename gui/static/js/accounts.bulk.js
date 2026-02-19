(() => {
  const globalObj = typeof window !== 'undefined' ? window : globalThis;
  const AccountsPage = globalObj.AccountsPage || (globalObj.AccountsPage = {});
  const state = AccountsPage.state;
  const { fillSelectWithEmpty, showAlert, escapeHtml } = AccountsPage;

  function bindBulkActions() {
    ['assign_role', 'assign_group', 'reset_password', 'lock', 'unlock', 'disable', 'enable'].forEach(action => {
      const btn = document.querySelector(`[data-bulk="${action}"]`);
      if (btn) {
        btn.addEventListener('click', () => openBulkModal(action));
      }
    });
    const closeBtn = document.getElementById('close-bulk-modal');
    if (closeBtn) closeBtn.onclick = closeBulkModal;
    const cancelBtn = document.getElementById('bulk-cancel');
    if (cancelBtn) cancelBtn.onclick = closeBulkModal;
    const form = document.getElementById('bulk-form');
    if (form) {
      form.onsubmit = onBulkSubmit;
    }
    const download = document.getElementById('bulk-download-passwords');
    if (download) {
      download.onclick = downloadBulkPasswords;
    }
  }

  function openBulkModal(action) {
    if (!state.selected || state.selected.size === 0) {
      alert(BerkutI18n.t('accounts.selectAll') || 'Select users first');
      return;
    }
    state.currentBulkAction = action;
    state.bulkPasswords = [];
    const modal = document.getElementById('bulk-modal');
    const title = document.getElementById('bulk-modal-title');
    if (title) {
      title.textContent = bulkTitle(action);
    }
    buildBulkForm(action);
    showAlert('bulk-modal-alert', '');
    const resultBox = document.getElementById('bulk-result');
    if (resultBox) resultBox.hidden = true;
    modal.hidden = false;
  }

  function closeBulkModal() {
    const modal = document.getElementById('bulk-modal');
    if (modal) modal.hidden = true;
    showAlert('bulk-modal-alert', '');
    const resultBox = document.getElementById('bulk-result');
    if (resultBox) resultBox.hidden = true;
    state.bulkPasswords = [];
  }

  function bulkTitle(action) {
    const map = {
      assign_role: BerkutI18n.t('accounts.bulkAssignRole'),
      assign_group: BerkutI18n.t('accounts.bulkAssignGroup'),
      reset_password: BerkutI18n.t('accounts.bulkResetPassword'),
      lock: BerkutI18n.t('accounts.bulkLock'),
      unlock: BerkutI18n.t('accounts.bulkUnlock'),
      disable: BerkutI18n.t('accounts.bulkDisable'),
      enable: BerkutI18n.t('accounts.bulkEnable'),
    };
    return map[action] || BerkutI18n.t('accounts.bulkActions') || 'Bulk action';
  }

  function buildBulkForm(action) {
    const fields = document.getElementById('bulk-form-fields');
    if (!fields) return;
    fields.innerHTML = '';
    const resultBox = document.getElementById('bulk-result');
    if (resultBox) resultBox.hidden = true;
    const pwdBox = document.getElementById('bulk-passwords');
    if (pwdBox) pwdBox.hidden = true;
    if (action === 'assign_role') {
      const wrap = document.createElement('div');
      wrap.className = 'form-field';
      wrap.innerHTML = `<label>${bulkTitle(action)}</label><select id="bulk-role-select"></select>`;
      fields.appendChild(wrap);
      fillSelectWithEmpty(document.getElementById('bulk-role-select'), state.roles.map(r => ({ value: r.name, label: r.name })), '');
    } else if (action === 'assign_group') {
      const wrap = document.createElement('div');
      wrap.className = 'form-field';
      wrap.innerHTML = `<label>${bulkTitle(action)}</label><select id="bulk-group-select"></select>`;
      fields.appendChild(wrap);
      fillSelectWithEmpty(document.getElementById('bulk-group-select'), state.groups.map(g => ({ value: g.id, label: g.name })), '');
    } else if (action === 'reset_password') {
      const generate = document.createElement('div');
      generate.className = 'form-field checkbox';
      generate.innerHTML = `<label><input type="checkbox" id="bulk-generate-password" checked> ${BerkutI18n.t('accounts.importOptionTempPassword') || 'Generate temporary passwords'}</label>`;
      const passField = document.createElement('div');
      passField.className = 'form-field';
      passField.innerHTML = `<label>${BerkutI18n.t('accounts.password') || 'Password'}</label><input id="bulk-password-input" type="password" disabled autocomplete="new-password">`;
      const mustChange = document.createElement('div');
      mustChange.className = 'form-field checkbox';
      mustChange.innerHTML = `<label><input type="checkbox" id="bulk-must-change" checked> ${BerkutI18n.t('accounts.importOptionMustChange') || 'Require password change'}</label>`;
      fields.appendChild(generate);
      fields.appendChild(passField);
      fields.appendChild(mustChange);
      const genInput = generate.querySelector('input');
      const pwdInput = passField.querySelector('input');
      if (genInput && pwdInput) {
        genInput.onchange = () => {
          pwdInput.disabled = genInput.checked;
          if (!genInput.checked) {
            pwdInput.focus();
          }
        };
      }
    } else if (action === 'lock') {
      const reason = document.createElement('div');
      reason.className = 'form-field required';
      reason.innerHTML = `<label>${BerkutI18n.t('accounts.lockReasonPrompt') || 'Reason'}</label><input id="bulk-lock-reason" required>`;
      const minutes = document.createElement('div');
      minutes.className = 'form-field';
      minutes.innerHTML = `<label>${BerkutI18n.t('accounts.lockDuration') || 'Duration (minutes)'}</label><input id="bulk-lock-minutes" type="number" min="1" value="60">`;
      fields.appendChild(reason);
      fields.appendChild(minutes);
    } else if (action === 'unlock') {
      const reason = document.createElement('div');
      reason.className = 'form-field';
      reason.innerHTML = `<label>${BerkutI18n.t('accounts.lockReasonPrompt') || 'Reason'}</label><input id="bulk-unlock-reason">`;
      fields.appendChild(reason);
    } else if (action === 'disable' || action === 'enable') {
      const reason = document.createElement('div');
      reason.className = 'form-field';
      reason.innerHTML = `<label>${BerkutI18n.t('accounts.lockReasonPrompt') || 'Reason'}</label><input id="bulk-status-reason">`;
      fields.appendChild(reason);
    }
  }

  async function onBulkSubmit(e) {
    e.preventDefault();
    if (!state.currentBulkAction) return;
    const payload = collectBulkPayload(state.currentBulkAction);
    if (payload === false) return;
    await performBulkAction(state.currentBulkAction, payload);
  }

  function collectBulkPayload(action) {
    if (action === 'assign_role') {
      const select = document.getElementById('bulk-role-select');
      if (!select || !select.value) {
        showAlert('bulk-modal-alert', BerkutI18n.t('accounts.roleRequired') || 'Role required');
        return false;
      }
      return { role_id: select.value };
    }
    if (action === 'assign_group') {
      const select = document.getElementById('bulk-group-select');
      if (!select || !select.value) {
        showAlert('bulk-modal-alert', BerkutI18n.t('accounts.bulkAssignGroup') || 'Group required');
        return false;
      }
      return { group_id: Number(select.value) };
    }
    if (action === 'reset_password') {
      const generate = document.getElementById('bulk-generate-password');
      const pwdInput = document.getElementById('bulk-password-input');
      const mustChange = document.getElementById('bulk-must-change');
      const payload = { temp_password: '', must_change: true };
      if (generate && !generate.checked) {
        payload.temp_password = (pwdInput?.value || '').trim();
        if (!payload.temp_password) {
          showAlert('bulk-modal-alert', BerkutI18n.t('accounts.passwordMismatch') || 'Password required');
          return false;
        }
      }
      if (mustChange) {
        payload.must_change = mustChange.checked;
      }
      return payload;
    }
    if (action === 'lock') {
      const reason = (document.getElementById('bulk-lock-reason')?.value || '').trim();
      const minutes = Number(document.getElementById('bulk-lock-minutes')?.value || 60);
      if (!reason) {
        showAlert('bulk-modal-alert', BerkutI18n.t('accounts.lockReasonPrompt') || 'Reason required');
        return false;
      }
      return { reason, minutes };
    }
    if (action === 'unlock') {
      return { reason: (document.getElementById('bulk-unlock-reason')?.value || '').trim() };
    }
    if (action === 'disable' || action === 'enable') {
      return { reason: (document.getElementById('bulk-status-reason')?.value || '').trim() };
    }
    return {};
  }

  async function performBulkAction(action, payload) {
    if (!state.selected || state.selected.size === 0) {
      showAlert('bulk-modal-alert', BerkutI18n.t('accounts.selectAll') || 'Select users');
      return;
    }
    try {
      const res = await Api.post('/api/accounts/users/bulk', {
        action,
        user_ids: Array.from(state.selected),
        payload,
      });
      renderBulkResult(action, res || {});
      await AccountsPage.loadUsers();
    } catch (err) {
      showAlert('bulk-modal-alert', err.message || (BerkutI18n.t('common.error') || 'Error'));
    }
  }

  function renderBulkResult(action, res) {
    const box = document.getElementById('bulk-result');
    const body = document.getElementById('bulk-result-body');
    const passwordsBox = document.getElementById('bulk-passwords');
    const pwdTable = document.getElementById('bulk-password-table');
    if (pwdTable) pwdTable.innerHTML = '';
    state.bulkPasswords = [];
    if (!box || !body) return;
    const success = res.success_count || 0;
    const failures = res.failures || [];
    body.innerHTML = `<p>${bulkTitle(action)}: ${success} / ${(success + failures.length) || state.selected.size}</p>`;
    if (failures.length) {
      const table = document.createElement('table');
      table.className = 'data-table';
      table.innerHTML = `<thead><tr><th>ID</th><th>${BerkutI18n.t('accounts.bulkFailureReason') || 'Reason'}</th></tr></thead>`;
      const tbody = document.createElement('tbody');
      failures.forEach(f => {
        const tr = document.createElement('tr');
        tr.innerHTML = `<td>${escapeHtml(f.user_id || f.UserID || '')}</td><td>${escapeHtml(describeFailure(f))}</td>`;
        tbody.appendChild(tr);
      });
      table.appendChild(tbody);
      body.appendChild(table);
    }
    if (Array.isArray(res.passwords) && res.passwords.length && passwordsBox && pwdTable) {
      state.bulkPasswords = res.passwords;
      let html = '<table class="data-table"><thead><tr><th>Login</th><th>Password</th></tr></thead><tbody>';
      res.passwords.forEach(p => {
        html += `<tr><td>${escapeHtml(p.login || p.Login || '')}</td><td>${escapeHtml(p.temp_password || '')}</td></tr>`;
      });
      html += '</tbody></table>';
      pwdTable.innerHTML = html;
      passwordsBox.hidden = false;
    } else if (passwordsBox) {
      passwordsBox.hidden = true;
    }
    box.hidden = false;
  }

  function describeFailure(failure) {
    const map = {
      server_error: BerkutI18n.t('common.error') || 'Error',
      not_found: BerkutI18n.t('common.notFound') || 'Not found',
      role_not_found: BerkutI18n.t('accounts.importErrorRoleNotFound'),
      group_not_found: BerkutI18n.t('accounts.importErrorGroupNotFound'),
      clearance_too_high: BerkutI18n.t('accounts.clearanceTooHigh'),
      clearance_tags_not_allowed: BerkutI18n.t('accounts.clearanceTagsNotAllowed'),
      invalid_password: BerkutI18n.t('accounts.passwordNotSetIndicator') || 'Invalid password',
      password_reused: BerkutI18n.t('accounts.passwordReuseDenied'),
      last_superadmin: BerkutI18n.t('accounts.lastSuperadminProtected'),
      self_lockout: BerkutI18n.t('accounts.selfLockoutPrevented'),
      forbidden: BerkutI18n.t('common.accessDenied'),
      invalid_payload: BerkutI18n.t('common.error') || 'Error',
    };
    let text = map[failure.reason] || map.invalid_payload || failure.reason || 'Error';
    if (failure.detail) {
      text += ` (${failure.detail})`;
    }
    return text;
  }

  function downloadBulkPasswords() {
    if (!state.bulkPasswords || !state.bulkPasswords.length) return;
    let csv = 'login,temp_password\n';
    state.bulkPasswords.forEach(row => {
      csv += `${row.login || row.Login},${row.temp_password}\n`;
    });
    const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'bulk_passwords.csv';
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  }

  AccountsPage.bindBulkActions = bindBulkActions;
  AccountsPage.openBulkModal = openBulkModal;
  AccountsPage.closeBulkModal = closeBulkModal;
  AccountsPage.bulkTitle = bulkTitle;
  AccountsPage.buildBulkForm = buildBulkForm;
  AccountsPage.onBulkSubmit = onBulkSubmit;
  AccountsPage.collectBulkPayload = collectBulkPayload;
  AccountsPage.performBulkAction = performBulkAction;
  AccountsPage.renderBulkResult = renderBulkResult;
  AccountsPage.describeFailure = describeFailure;
  AccountsPage.downloadBulkPasswords = downloadBulkPasswords;
})();
