(() => {
  const globalObj = typeof window !== 'undefined' ? window : globalThis;
  const AccountsPage = globalObj.AccountsPage || (globalObj.AccountsPage = {});
  const state = AccountsPage.state;
  const {
    renderClearanceTags,
    setPasswordVisibility,
    generatePassword,
    setMultiSelect,
    getCheckedValues,
    renderSelectedOptions,
    renderCommaList,
    groupPermissions,
    permissionLabel,
    clearanceLevelLabel,
    clearanceTagLabel,
    setText,
    escapeHtml,
    formatDate,
    showAlert,
    fillSelect
  } = AccountsPage;

  function populateDepartments(users) {
    const set = new Set();
    (users || []).forEach(u => {
      if (u.department) set.add(u.department);
    });
    state.departments = Array.from(set).sort();
    AccountsPage.fillSelectWithEmpty(
      document.getElementById('filter-dept'),
      state.departments.map(d => ({ value: d, label: d })),
      BerkutI18n.t('common.all') || 'All'
    );
  }

  async function loadUsers() {
    const params = new URLSearchParams();
    const status = document.getElementById('filter-status').value;
    const dept = (document.getElementById('filter-dept').value || '').trim();
    const role = document.getElementById('filter-role').value;
    const group = document.getElementById('filter-group').value;
    const password = document.getElementById('filter-password').value;
    const clearance = document.getElementById('filter-clearance').value;
    const search = (document.getElementById('filter-search').value || '').trim();
    if (status) params.set('status', status);
    if (dept) params.set('department', dept);
    if (role) params.set('role', role);
    if (group) params.set('group_id', group);
    if (password) params.set('password_status', password);
    if (clearance) {
      params.set('clearance_min', clearance);
      params.set('clearance_max', clearance);
    }
    if (search) params.set('q', search);
    try {
      const query = params.toString();
      const res = await Api.get(`/api/accounts/users${query ? `?${query}` : ''}`);
      state.users = res.users || [];
      state.selected = new Set();
      const selectAll = document.getElementById('select-all-users');
      if (selectAll) selectAll.checked = false;
      populateDepartments(state.users);
      fillSelect(document.getElementById('group-users'), state.users.map(u => ({ value: u.id, label: `${u.username} (${u.full_name || ''})` })));
      fillSelect(document.getElementById('group-detail-users'), state.users.map(u => ({ value: u.id, label: `${u.username} (${u.full_name || ''})` })));
      if (state.selectedGroupId && AccountsPage.renderGroupDetails) AccountsPage.renderGroupDetails();
      renderUsers();
      updateBulkBar();
    } catch (err) {
      console.error('users load', err);
    }
  }

  function renderUsers() {
    const tbody = document.querySelector('#users-table tbody');
    tbody.innerHTML = '';
    if (!state.users.length) {
      const tr = document.createElement('tr');
      tr.innerHTML = `<td colspan="9">${BerkutI18n.t('accounts.noResults') || 'No users'}</td>`;
      tbody.appendChild(tr);
      updateSelectAllCheckbox();
      return;
    }
    const canReset2FA = Array.isArray(state.currentUser?.roles) && state.currentUser.roles.some(r => String(r || '').toLowerCase() === 'superadmin');
    state.users.forEach(u => {
      const status = userStatus(u);
      const statusLines = [status];
      if (u.lock_reason) {
        statusLines.push(`${BerkutI18n.t('accounts.lockReasonPrompt') || 'Reason'}: ${u.lock_reason}`);
      }
      if (u.password_set === false) {
        statusLines.push(BerkutI18n.t('accounts.passwordNotSetIndicator') || 'Password not set');
      }
      const statusHtml = statusLines.map(line => `<div class="status-line">${escapeHtml(line)}</div>`).join('');
      const tr = document.createElement('tr');
      const checked = state.selected.has(u.id);
      tr.innerHTML = `
        <td><input type="checkbox" class="user-select" data-id="${u.id}" ${checked ? 'checked' : ''}></td>
        <td>${escapeHtml(u.username)}</td>
        <td>${escapeHtml(u.full_name || '')}</td>
        <td>${escapeHtml(u.department || '')}</td>
        <td>${escapeHtml(u.position || '')}</td>
        <td>${escapeHtml((u.roles || []).join(', '))}</td>
        <td>${escapeHtml((u.groups || []).map(g => g.name).join(', '))}</td>
        <td>${statusHtml}</td>
        <td class="actions">
          <button class="btn ghost" data-action="edit">${BerkutI18n.t('accounts.edit')}</button>
          <button class="btn ghost" data-action="lock">${BerkutI18n.t('accounts.lock') || 'Lock'}</button>
          <button class="btn ghost" data-action="reset">${BerkutI18n.t('accounts.reset')}</button>
          ${canReset2FA ? `<button class="btn ghost danger" data-action="reset2fa">${BerkutI18n.t('accounts.reset2fa') || 'Reset 2FA'}</button>` : ''}
        </td>`;
      tr.querySelector('[data-action="edit"]').onclick = () => openUserModal(u);
      tr.querySelector('[data-action="lock"]').onclick = () => toggleLock(u);
      tr.querySelector('[data-action="reset"]').onclick = () => resetPassword(u);
      const reset2faBtn = tr.querySelector('[data-action="reset2fa"]');
      if (reset2faBtn) reset2faBtn.onclick = () => reset2FA(u);
      const checkbox = tr.querySelector('.user-select');
      if (checkbox) {
        checkbox.addEventListener('change', (e) => toggleUserSelection(u.id, e.target.checked));
      }
      tbody.appendChild(tr);
    });
    updateSelectAllCheckbox();
  }

  function userStatus(u) {
    if (u.lock_stage >= 6) {
      return BerkutI18n.t('accounts.lockedPermanent') || 'Locked';
    }
    if (u.locked_until && new Date(u.locked_until) > new Date()) {
      const formatted = (typeof AppTime !== 'undefined' && AppTime.formatDateTime)
        ? AppTime.formatDateTime(u.locked_until)
        : (() => {
          const dt = new Date(u.locked_until);
          const pad = (num) => `${num}`.padStart(2, '0');
          return `${pad(dt.getDate())}.${pad(dt.getMonth() + 1)}.${dt.getFullYear()} ${pad(dt.getHours())}:${pad(dt.getMinutes())}`;
        })();
      return (BerkutI18n.t('accounts.lockedUntil') || 'Locked until') + ' ' + formatted;
    }
    return u.active ? BerkutI18n.t('accounts.active') : BerkutI18n.t('accounts.disabled');
  }

  function toggleUserSelection(userId, selected) {
    if (selected) {
      state.selected.add(userId);
    } else {
      state.selected.delete(userId);
    }
    updateSelectAllCheckbox();
    updateBulkBar();
  }

  function toggleSelectAll(checked) {
    if (checked) {
      state.selected = new Set(state.users.map(u => u.id));
    } else {
      state.selected = new Set();
    }
    document.querySelectorAll('.user-select').forEach(cb => {
      cb.checked = checked;
    });
    updateSelectAllCheckbox();
    updateBulkBar();
  }

  function updateSelectAllCheckbox() {
    const selectAll = document.getElementById('select-all-users');
    if (!selectAll) return;
    const total = state.users.length;
    const selected = state.selected.size;
    selectAll.indeterminate = selected > 0 && selected < total;
    selectAll.checked = total > 0 && selected === total;
  }

  function updateBulkBar() {
    const bar = document.getElementById('bulk-actions');
    const countEl = document.getElementById('bulk-selected-count');
    const count = state.selected.size;
    if (countEl) countEl.textContent = count;
    if (bar) {
      bar.hidden = count === 0;
    }
  }

  function bindUserModal() {
    const modal = document.getElementById('user-modal');
    const form = document.getElementById('user-form');
    const toggleBtn = document.getElementById('toggle-password-visibility');
    const generateBtn = document.getElementById('generate-password');
    document.getElementById('close-modal').onclick = closeUserModal;
    document.getElementById('cancel-modal').onclick = closeUserModal;
    if (toggleBtn) {
      toggleBtn.onclick = () => setPasswordVisibility(!state.passwordVisible);
    }
    if (generateBtn) {
      const genLabel = BerkutI18n.t('accounts.passwordGenerate') || 'Generate password';
      generateBtn.setAttribute('title', genLabel);
      generateBtn.setAttribute('aria-label', genLabel);
      generateBtn.onclick = () => {
        const pwd = generatePassword();
        const pwdInput = document.getElementById('user-password');
        const confirmInput = document.getElementById('user-password-confirm');
        if (pwdInput) pwdInput.value = pwd;
        if (confirmInput) confirmInput.value = pwd;
        setPasswordVisibility(false);
      };
    }
    form.onsubmit = async (e) => {
      e.preventDefault();
      const payload = collectUserForm(form);
      if (payload.password && payload.password !== payload.password_confirm) {
        showAlert('modal-alert', BerkutI18n.t('accounts.passwordMismatch'));
        return;
      }
      try {
        if (!state.editingUserId) {
          await Api.post('/api/accounts/users', payload);
        } else {
          await Api.put(`/api/accounts/users/${state.editingUserId}`, payload);
        }
        closeUserModal();
        await loadUsers();
      } catch (err) {
        showAlert('modal-alert', err.message || BerkutI18n.t('common.error'));
      }
    };
    if (modal) {
      const rolesSel = document.getElementById('user-roles');
      const groupsSel = document.getElementById('user-groups');
      renderSelectedOptions(rolesSel, document.getElementById('user-roles-hint'));
      renderSelectedOptions(groupsSel, document.getElementById('user-groups-hint'));
    }
  }

  function openUserModal(user) {
    const modal = document.getElementById('user-modal');
    const form = document.getElementById('user-form');
    form.reset();
    setPasswordVisibility(false);
    state.editingUserId = user ? user.id : null;
    document.getElementById('modal-title').textContent = user ? BerkutI18n.t('accounts.edit') : BerkutI18n.t('accounts.create');
    if (user) {
      form.username.value = user.username;
      form.username.setAttribute('disabled', 'true');
      form.full_name.value = user.full_name || '';
      form.department.value = user.department || '';
      form.position.value = user.position || '';
      form.clearance_level.value = user.clearance_level != null ? user.clearance_level : '';
      form.status.value = user.active ? 'active' : 'disabled';
      renderClearanceTags('user-clearance-tags', user.clearance_tags || []);
      setMultiSelect(form.roles, user.roles || []);
      setMultiSelect(form.groups, (user.groups || []).map(g => g.id));
    } else {
      form.username.removeAttribute('disabled');
      renderClearanceTags('user-clearance-tags', []);
      setMultiSelect(form.roles, []);
      setMultiSelect(form.groups, []);
    }
    state.sessions = [];
    const sessionsPanel = document.getElementById('user-sessions');
    if (sessionsPanel) {
      sessionsPanel.hidden = !user;
    }
    if (user) {
      loadSessions(user.id);
      const killAll = document.getElementById('kill-all-sessions');
      if (killAll) {
        killAll.onclick = () => killAllSessions(user.id);
      }
    }
    renderEffective(user);
    modal.hidden = false;
  }

  async function loadSessions(userId) {
    const panel = document.getElementById('user-sessions');
    if (!panel || !userId) return;
    try {
      const res = await Api.get(`/api/accounts/users/${userId}/sessions`);
      state.sessions = res.sessions || [];
    } catch (err) {
      state.sessions = [];
      console.error('sessions load', err);
    }
    renderSessions();
  }

  function renderSessions() {
    const panel = document.getElementById('user-sessions');
    const empty = document.getElementById('sessions-empty');
    const tbody = document.querySelector('#sessions-table tbody');
    if (!panel || !tbody) return;
    tbody.innerHTML = '';
    if (!state.sessions || !state.sessions.length) {
      if (empty) empty.hidden = false;
      panel.hidden = false;
      return;
    }
    if (empty) empty.hidden = true;
    state.sessions.forEach(s => {
      const sid = s.id || s.ID;
      const tr = document.createElement('tr');
      tr.innerHTML = `
        <td>${escapeHtml(s.ip || s.IP || '')}</td>
        <td class="ua">${escapeHtml(s.user_agent || s.UserAgent || '')}</td>
        <td>${formatDate(s.created_at || s.CreatedAt)}</td>
        <td>${formatDate(s.last_seen_at || s.last_seen || s.LastSeenAt)}</td>
        <td class="actions"><button class="btn ghost" data-action="kill">${BerkutI18n.t('accounts.killSession') || 'Kill'}</button></td>`;
      tr.querySelector('[data-action="kill"]').onclick = () => killSession(sid);
      tbody.appendChild(tr);
    });
    panel.hidden = false;
  }

  async function killSession(sessionId) {
    if (!sessionId) return;
    try {
      await Api.post(`/api/accounts/sessions/${sessionId}/kill`, {});
      await loadSessions(state.editingUserId);
    } catch (err) {
      alert(err.message || (BerkutI18n.t('common.error') || 'Error'));
    }
  }

  async function killAllSessions(userId) {
    if (!userId) return;
    try {
      await Api.post(`/api/accounts/users/${userId}/sessions/kill_all`, {});
      await loadSessions(userId);
    } catch (err) {
      alert(err.message || (BerkutI18n.t('common.error') || 'Error'));
    }
  }

  function closeUserModal() {
    document.getElementById('user-modal').hidden = true;
    document.getElementById('modal-alert').hidden = true;
    state.editingUserId = null;
  }

  function collectUserForm(form) {
    const data = Object.fromEntries(new FormData(form).entries());
    const payload = {
      username: (data.username || '').trim(),
      full_name: (data.full_name || '').trim(),
      department: (data.department || '').trim(),
      position: (data.position || '').trim(),
      status: data.status || 'active',
      clearance_level: Number(data.clearance_level || 0),
      clearance_tags: getCheckedValues(document.getElementById('user-clearance-tags')),
      require_password_change: form.require_password_change.checked,
      roles: AccountsPage.getSelectedValues(form.roles),
      groups: AccountsPage.getSelectedValues(form.groups).map(Number)
    };
    if (data.password) {
      payload.password = data.password;
      payload.password_confirm = data.password_confirm;
    }
    return payload;
  }

  async function toggleLock(user) {
    const locked = (user.lock_stage >= 6) || (user.locked_until && new Date(user.locked_until) > new Date());
    const reason = prompt(BerkutI18n.t('accounts.lockReasonPrompt') || 'Reason?') || '';
    if (locked) {
      await Api.post(`/api/accounts/users/${user.id}/unlock`, { reason });
    } else {
      await Api.post(`/api/accounts/users/${user.id}/lock`, { reason, minutes: 60 });
    }
    await loadUsers();
  }

  async function resetPassword(user) {
    const pwd = prompt(BerkutI18n.t('accounts.newPasswordPrompt'));
    if (!pwd) return;
    await Api.post(`/api/accounts/users/${user.id}/reset-password`, { password: pwd, require_change: true });
    await loadUsers();
  }

  async function reset2FA(user) {
    if (!user || !user.id) return;
    const msg = BerkutI18n.t('accounts.reset2faConfirm') || 'Reset 2FA for this user?';
    if (!confirm(msg)) return;
    try {
      await Api.post(`/api/accounts/users/${user.id}/reset-2fa`, {});
      await loadUsers();
    } catch (err) {
      alert(err.message || (BerkutI18n.t('common.error') || 'Error'));
    }
  }

  function renderEffective(user) {
    const panel = document.getElementById('user-effective');
    if (!panel) return;
    if (!user) {
      panel.hidden = true;
      return;
    }
    panel.hidden = false;
    const roles = user.effective_roles || user.roles || [];
    setText('effective-roles', roles.join(', ') || '-');
    const permsGrouped = document.getElementById('effective-permissions-grouped');
    if (permsGrouped) {
      permsGrouped.innerHTML = '';
      const grouped = groupPermissions(user.effective_permissions || []);
      Object.keys(grouped).sort().forEach(prefix => {
        const block = document.createElement('details');
        block.open = true;
        const summary = document.createElement('summary');
        summary.textContent = prefix || (BerkutI18n.t('common.all') || 'all');
        block.appendChild(summary);
        const ul = document.createElement('ul');
        grouped[prefix].forEach(p => {
          const li = document.createElement('li');
          li.textContent = permissionLabel(p);
          li.title = p;
          ul.appendChild(li);
        });
        block.appendChild(ul);
        permsGrouped.appendChild(block);
      });
    }
    const clearanceLevel = user.effective_clearance_level != null ? user.effective_clearance_level : user.clearance_level;
    const clearanceTags = user.effective_clearance_tags || user.clearance_tags || [];
    const clearanceLabel = clearanceLevelLabel(clearanceLevel);
    const tagsLabel = clearanceTags.length ? clearanceTags.map(clearanceTagLabel).join(', ') : '-';
    setText('effective-clearance', `${clearanceLabel} (${tagsLabel})`);
    setText('effective-menu', (user.effective_menu_permissions || []).join(', ') || '-');
    const sources = document.getElementById('effective-sources');
    if (sources) {
      const direct = user.roles || [];
      const groupRoles = [];
      const groupNames = [];
      (user.groups || []).forEach(g => {
        if (g.roles) {
          g.roles.forEach(r => {
            if (!groupRoles.includes(r)) groupRoles.push(r);
          });
        }
        if (g.name) groupNames.push(g.name);
      });
      const parts = [];
      if (direct.length) parts.push(`${BerkutI18n.t('accounts.roles') || 'Roles'}: ${direct.join(', ')}`);
      if (groupRoles.length) parts.push(`${BerkutI18n.t('accounts.groups.title') || 'Groups'}: ${groupRoles.join(', ')}${groupNames.length ? ` (${groupNames.join(', ')})` : ''}`);
      sources.textContent = parts.join(' • ');
    }
    const groupsList = document.getElementById('effective-groups');
    if (groupsList) {
      groupsList.innerHTML = '';
      (user.groups || []).forEach(g => {
        const div = document.createElement('div');
        div.className = 'pill';
        div.textContent = `${g.name} • ${(g.roles || []).join(', ') || '-' } • ${BerkutI18n.t('accounts.groups.clearance') || 'Clearance'} ${g.clearance_level || 0}`;
        groupsList.appendChild(div);
      });
    }
  }

  AccountsPage.populateDepartments = populateDepartments;
  AccountsPage.loadUsers = loadUsers;
  AccountsPage.renderUsers = renderUsers;
  AccountsPage.userStatus = userStatus;
  AccountsPage.toggleUserSelection = toggleUserSelection;
  AccountsPage.toggleSelectAll = toggleSelectAll;
  AccountsPage.updateSelectAllCheckbox = updateSelectAllCheckbox;
  AccountsPage.updateBulkBar = updateBulkBar;
  AccountsPage.bindUserModal = bindUserModal;
  AccountsPage.openUserModal = openUserModal;
  AccountsPage.loadSessions = loadSessions;
  AccountsPage.renderSessions = renderSessions;
  AccountsPage.killSession = killSession;
  AccountsPage.killAllSessions = killAllSessions;
  AccountsPage.closeUserModal = closeUserModal;
  AccountsPage.collectUserForm = collectUserForm;
  AccountsPage.toggleLock = toggleLock;
  AccountsPage.resetPassword = resetPassword;
  AccountsPage.renderEffective = renderEffective;
})();
