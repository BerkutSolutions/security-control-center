(() => {
  const globalObj = typeof window !== 'undefined' ? window : globalThis;
  const AccountsPage = globalObj.AccountsPage || (globalObj.AccountsPage = {});
  const state = AccountsPage.state;
  const {
    renderTagList,
    renderCommaList,
    renderClearanceTags,
    clearanceLevelLabel,
    clearanceTagLabel,
    groupImpactLines,
    renderMenuOptions,
    getCheckedValues,
    setCheckedValues,
    renderGroupImpact,
    fillSelect,
    fillSelectWithEmpty,
    setMultiSelect,
    enhanceMultiSelects,
    renderSelectedOptions,
    attachHintListener,
    showAlert,
    setText
  } = AccountsPage;

  async function loadGroups() {
    try {
      const res = await Api.get('/api/accounts/groups');
      state.groupDetails = {};
      setGroups(res.groups || []);
    } catch (err) {
      console.error('groups', err);
    }
  }

  function setGroups(groups) {
    state.groups = groups || [];
    fillSelect(document.getElementById('user-groups'), state.groups.map(g => ({ value: g.id, label: g.name })));
    fillSelectWithEmpty(document.getElementById('import-default-group'), state.groups.map(g => ({ value: g.name, label: g.name })));
    fillSelectWithEmpty(document.getElementById('filter-group'), state.groups.map(g => ({ value: g.id, label: g.name })), BerkutI18n.t('common.all') || 'All');
    renderGroups();
    renderGroupDetails();
  }

  function renderGroups() {
    const list = document.getElementById('groups-list');
    if (!list) return;
    const search = (document.getElementById('groups-search')?.value || '').toLowerCase().trim();
    list.innerHTML = '';
    const filtered = (state.groups || []).filter(g => {
      if (!search) return true;
      const name = (g.name || '').toLowerCase();
      const desc = (g.description || '').toLowerCase();
      return name.includes(search) || desc.includes(search);
    });
    if (!filtered.length) {
      const empty = document.createElement('div');
      empty.className = 'empty-state';
      empty.textContent = BerkutI18n.t('common.notAvailable') || '-';
      list.appendChild(empty);
      return;
    }
    filtered.forEach(g => {
      const tile = document.createElement('button');
      tile.type = 'button';
      tile.className = 'tile-card';
      if (state.selectedGroupId === g.id) tile.classList.add('active');
      const title = document.createElement('div');
      title.className = 'tile-title';
      title.textContent = g.name || '-';
      const desc = document.createElement('div');
      desc.className = 'tile-desc';
      desc.textContent = g.description || '';
      const meta = document.createElement('div');
      meta.className = 'tile-meta';
      const membersLabel = BerkutI18n.t('accounts.groupMembers') || 'Members';
      const rolesLabel = BerkutI18n.t('accounts.groupRoles') || 'Roles';
      const clearanceLabel = BerkutI18n.t('accounts.groups.clearance') || 'Clearance';
      meta.textContent = `${membersLabel}: ${g.user_count || 0} | ${rolesLabel}: ${(g.roles || []).length} | ${clearanceLabel}: ${g.clearance_level || 0}`;
      tile.appendChild(title);
      tile.appendChild(desc);
      tile.appendChild(meta);
      tile.addEventListener('click', () => selectGroup(g.id));
      list.appendChild(tile);
    });
  }

  function bindGroupDetails() {
    const editBtn = document.getElementById('group-edit-btn');
    const deleteBtn = document.getElementById('group-delete-btn');
    const saveBtn = document.getElementById('group-save-btn');
    const cancelBtn = document.getElementById('group-cancel-btn');
    const editLabel = BerkutI18n.t('common.edit') || 'Edit';
    const deleteLabel = BerkutI18n.t('common.delete') || 'Delete';
    const saveLabel = BerkutI18n.t('common.save') || 'Save';
    const cancelLabel = BerkutI18n.t('common.cancel') || 'Cancel';
    if (editBtn) {
      editBtn.title = editLabel;
      editBtn.setAttribute('aria-label', editLabel);
      editBtn.addEventListener('click', () => setGroupDetailEditState(true));
    }
    if (deleteBtn) {
      deleteBtn.title = deleteLabel;
      deleteBtn.setAttribute('aria-label', deleteLabel);
      deleteBtn.addEventListener('click', () => {
        const group = getSelectedGroup();
        if (group) deleteGroup(group.id);
      });
    }
    if (saveBtn) {
      saveBtn.title = saveLabel;
      saveBtn.setAttribute('aria-label', saveLabel);
      saveBtn.addEventListener('click', saveGroupDetails);
    }
    if (cancelBtn) {
      cancelBtn.title = cancelLabel;
      cancelBtn.setAttribute('aria-label', cancelLabel);
      cancelBtn.addEventListener('click', () => {
        setGroupDetailEditState(false);
        renderGroupDetails();
      });
    }
  }

  function selectGroup(id) {
    state.groupEditMode = false;
    setGroupDetailEditState(false);
    state.selectedGroupId = id;
    renderGroups();
    renderGroupDetails();
    loadGroupDetails(id);
  }

  function getSelectedGroup() {
    if (!state.selectedGroupId) return null;
    return state.groupDetails[state.selectedGroupId] || state.groups.find(g => g.id === state.selectedGroupId) || null;
  }

  async function loadGroupDetails(id) {
    if (!id) return;
    if (state.groupDetails[id] && state.groupDetails[id]._full) return;
    try {
      const res = await Api.get(`/api/accounts/groups/${id}`);
      if (res.group) {
        state.groupDetails[id] = { ...res.group, _full: true };
        if (state.selectedGroupId === id) renderGroupDetails();
      }
    } catch (err) {
      console.error('group details', err);
    }
  }

  function renderGroupDetails() {
    const group = getSelectedGroup();
    const form = document.getElementById('group-detail-form');
    const empty = document.getElementById('group-detail-empty');
    const view = document.getElementById('group-detail-view');
    const title = document.getElementById('group-detail-title');
    const meta = document.getElementById('group-detail-meta');
    const editBtn = document.getElementById('group-edit-btn');
    const deleteBtn = document.getElementById('group-delete-btn');
    const actions = document.getElementById('group-detail-actions');
    const protectedLabel = BerkutI18n.t('accounts.groupSystemProtected') || 'Protected';
    if (!group) {
      state.groupEditMode = false;
      state.selectedGroupId = null;
      if (form) form.hidden = true;
      if (view) view.hidden = true;
      if (empty) empty.hidden = true;
      if (title) title.textContent = '';
      if (meta) meta.textContent = '';
      if (actions) actions.hidden = true;
      if (editBtn) editBtn.disabled = true;
      if (deleteBtn) deleteBtn.disabled = true;
      showAlert('group-detail-alert', '');
      return;
    }
    if (actions) actions.hidden = false;
    if (empty) empty.hidden = true;
    if (title) title.textContent = group.name || '-';
    if (meta) {
      const membersLabel = BerkutI18n.t('accounts.groupMembers') || 'Members';
      const rolesLabel = BerkutI18n.t('accounts.groupRoles') || 'Roles';
      const clearanceLabel = BerkutI18n.t('accounts.groups.clearance') || 'Clearance';
      meta.textContent = `${membersLabel}: ${group.user_count || 0} | ${rolesLabel}: ${(group.roles || []).length} | ${clearanceLabel}: ${group.clearance_level || 0}`;
    }
    const isProtected = !!(group.built_in || group.system || group.is_system);
    if (editBtn) editBtn.disabled = isProtected;
    if (deleteBtn) {
      deleteBtn.disabled = isProtected;
      if (isProtected) deleteBtn.title = protectedLabel;
    }
    if (view) view.hidden = false;
    const nameInput = document.getElementById('group-detail-name');
    const descInput = document.getElementById('group-detail-description');
    const clearanceSel = document.getElementById('group-detail-clearance');
    if (nameInput) nameInput.value = group.name || '';
    if (descInput) descInput.value = group.description || '';
    if (clearanceSel) clearanceSel.value = group.clearance_level != null ? group.clearance_level : 0;
    renderClearanceTags('group-detail-clearance-tags', group.clearance_tags || []);
    renderMenuOptions('group-detail-menu');
    setCheckedValues(document.getElementById('group-detail-menu'), group.menu_permissions || []);
    const rolesSel = document.getElementById('group-detail-roles');
    const usersSel = document.getElementById('group-detail-users');
    const memberIds = Array.isArray(group.users) ? group.users.map(u => u.id || u) : [];
    if (rolesSel) setMultiSelect(rolesSel, group.roles || []);
    if (usersSel) setMultiSelect(usersSel, memberIds);
    renderGroupImpact(group, 'group-detail-impact');
    enhanceMultiSelects([rolesSel?.id, usersSel?.id].filter(Boolean));
    renderSelectedOptions(rolesSel, document.getElementById('group-detail-roles-hint'));
    renderSelectedOptions(usersSel, document.getElementById('group-detail-users-hint'));
    attachHintListener(rolesSel, document.getElementById('group-detail-roles-hint'));
    attachHintListener(usersSel, document.getElementById('group-detail-users-hint'));
    setText('group-view-name', group.name || '-');
    setText('group-view-description', group.description || (BerkutI18n.t('common.notAvailable') || '-'));
    setText('group-view-clearance', clearanceLevelLabel(group.clearance_level));
    renderCommaList('group-view-clearance-tags', (group.clearance_tags || []).map(clearanceTagLabel));
    renderCommaList('group-view-roles', group.roles || []);
    renderCommaList('group-view-menu', group.menu_permissions || []);
    const memberNames = [];
    (group.users || []).forEach(u => {
      if (u && typeof u === 'object') {
        const label = u.full_name ? `${u.username || ''} (${u.full_name})` : (u.username || u.name || '');
        if (label) memberNames.push(label);
      } else if (u != null) {
        const found = (state.users || []).find(item => String(item.id) === String(u));
        if (found) {
          memberNames.push(`${found.username} (${found.full_name || ''})`.trim());
        } else {
          memberNames.push(String(u));
        }
      }
    });
    renderCommaList('group-view-members', memberNames.length ? memberNames : []);
    renderCommaList('group-view-impact', groupImpactLines(group));
    setGroupDetailEditState(state.groupEditMode);
    showAlert('group-detail-alert', '');
    if (!state.groupDetails[group.id] || !state.groupDetails[group.id]._full) {
      loadGroupDetails(group.id);
    }
  }

  function setGroupDetailEditState(enabled) {
    const group = getSelectedGroup();
    const isProtected = group && (group.built_in || group.system || group.is_system);
    if (enabled && (!group || isProtected)) return;
    state.groupEditMode = enabled;
    const view = document.getElementById('group-detail-view');
    const form = document.getElementById('group-detail-form');
    if (form) {
      form.querySelectorAll('input, select, textarea').forEach(el => {
        el.disabled = !enabled;
      });
    }
    ['group-detail-clearance-tags', 'group-detail-menu'].forEach(id => {
      const container = document.getElementById(id);
      if (!container) return;
      container.querySelectorAll('input[type="checkbox"]').forEach(input => {
        input.disabled = !enabled;
      });
    });
    const editBtn = document.getElementById('group-edit-btn');
    const deleteBtn = document.getElementById('group-delete-btn');
    const saveBtn = document.getElementById('group-save-btn');
    const cancelBtn = document.getElementById('group-cancel-btn');
    if (view) view.hidden = enabled;
    if (form) form.hidden = !enabled;
    if (editBtn) editBtn.hidden = enabled;
    if (deleteBtn) deleteBtn.hidden = enabled;
    if (saveBtn) saveBtn.hidden = !enabled;
    if (cancelBtn) cancelBtn.hidden = !enabled;
  }

  async function saveGroupDetails() {
    const group = getSelectedGroup();
    if (!group) return;
    const form = document.getElementById('group-detail-form');
    if (!form) return;
    const name = (form.name.value || '').trim();
    if (!name) {
      showAlert('group-detail-alert', BerkutI18n.t('accounts.groupNameRequired') || 'Name required');
      return;
    }
    const payload = {
      name,
      description: (form.description.value || '').trim(),
      clearance_level: Number(form.clearance_level.value || 0),
      clearance_tags: getCheckedValues(document.getElementById('group-detail-clearance-tags')),
      roles: AccountsPage.getSelectedValues(form.roles),
      menu_permissions: getCheckedValues(document.getElementById('group-detail-menu')),
      users: AccountsPage.getSelectedValues(form.users),
    };
    try {
      await Api.put(`/api/accounts/groups/${group.id}`, payload);
      setGroupDetailEditState(false);
      state.selectedGroupId = group.id;
      await loadGroups();
      loadGroupDetails(group.id);
    } catch (err) {
      showAlert('group-detail-alert', err.message || BerkutI18n.t('common.error'));
    }
  }

  function bindGroupModal() {
    const openBtn = document.getElementById('open-group-create');
    const modal = document.getElementById('group-modal');
    if (!openBtn || !modal) return;
    openBtn.addEventListener('click', () => openGroupModal());
    document.getElementById('close-group-modal').onclick = closeGroupModal;
    document.getElementById('cancel-group-modal').onclick = closeGroupModal;
    const form = document.getElementById('group-form');
    if (form) {
      form.onsubmit = async (e) => {
        e.preventDefault();
        const payload = {
          name: (form.name.value || '').trim(),
          description: (form.description.value || '').trim(),
          clearance_level: Number(form.clearance_level.value || 0),
          clearance_tags: getCheckedValues(document.getElementById('group-clearance-tags')),
          roles: AccountsPage.getSelectedValues(form.roles),
          menu_permissions: getCheckedValues(document.getElementById('group-menu')),
          users: AccountsPage.getSelectedValues(form.users),
        };
        if (!payload.name) {
          showAlert('group-modal-alert', BerkutI18n.t('accounts.groupNameRequired') || 'Name required');
          return;
        }
        try {
          if (state.editingGroupId) {
            await Api.put(`/api/accounts/groups/${state.editingGroupId}`, payload);
          } else {
            await Api.post('/api/accounts/groups', payload);
          }
          closeGroupModal();
          await loadGroups();
        } catch (err) {
          showAlert('group-modal-alert', err.message || BerkutI18n.t('common.error'));
        }
      };
    }
  }

  async function openGroupModal(group) {
    const modal = document.getElementById('group-modal');
    const form = document.getElementById('group-form');
    form.reset();
    state.editingGroupId = group ? group.id : null;
    document.getElementById('group-modal-title').textContent = group ? (BerkutI18n.t('accounts.groups.edit') || 'Edit group') : BerkutI18n.t('accounts.groups.create');
    renderMenuOptions();
    let memberIds = [];
    if (group && group.id) {
      try {
        const res = await Api.get(`/api/accounts/groups/${group.id}`);
        group = res.group || group;
        memberIds = res.members || [];
      } catch (err) {
        console.error('group load', err);
      }
    }
    if (group) {
      form.name.value = group.name;
      form.description.value = group.description || '';
      form.clearance_level.value = group.clearance_level || 0;
      setMultiSelect(form.roles, group.roles || []);
      renderClearanceTags('group-clearance-tags', group.clearance_tags || []);
      setCheckedValues(document.getElementById('group-menu'), group.menu_permissions || []);
      setMultiSelect(form.users, memberIds.length ? memberIds : (group.users || []));
      renderGroupImpact(group);
    } else {
      setMultiSelect(form.roles, []);
      renderClearanceTags('group-clearance-tags', []);
      setCheckedValues(document.getElementById('group-menu'), []);
      setMultiSelect(form.users, []);
      renderGroupImpact(null);
    }
    const rolesSel = document.getElementById('group-roles');
    const usersSel = document.getElementById('group-users');
    const rolesHint = document.getElementById('group-roles-hint');
    const usersHint = document.getElementById('group-users-hint');
    enhanceMultiSelects([rolesSel?.id, usersSel?.id].filter(Boolean));
    renderSelectedOptions(rolesSel, rolesHint);
    renderSelectedOptions(usersSel, usersHint);
    attachHintListener(rolesSel, rolesHint);
    attachHintListener(usersSel, usersHint);
    modal.hidden = false;
  }

  function closeGroupModal() {
    document.getElementById('group-modal').hidden = true;
    document.getElementById('group-modal-alert').hidden = true;
    state.editingGroupId = null;
  }

  async function deleteGroup(id) {
    if (!confirm(BerkutI18n.t('accounts.groups.deleteConfirm') || 'Удалить группу?')) return;
    await Api.del(`/api/accounts/groups/${id}`);
    await loadGroups();
    if (state.selectedGroupId === id) {
      state.selectedGroupId = null;
      renderGroupDetails();
    }
  }

  AccountsPage.loadGroups = loadGroups;
  AccountsPage.setGroups = setGroups;
  AccountsPage.renderGroups = renderGroups;
  AccountsPage.bindGroupDetails = bindGroupDetails;
  AccountsPage.selectGroup = selectGroup;
  AccountsPage.getSelectedGroup = getSelectedGroup;
  AccountsPage.loadGroupDetails = loadGroupDetails;
  AccountsPage.renderGroupDetails = renderGroupDetails;
  AccountsPage.setGroupDetailEditState = setGroupDetailEditState;
  AccountsPage.saveGroupDetails = saveGroupDetails;
  AccountsPage.bindGroupModal = bindGroupModal;
  AccountsPage.openGroupModal = openGroupModal;
  AccountsPage.closeGroupModal = closeGroupModal;
  AccountsPage.deleteGroup = deleteGroup;
})();
