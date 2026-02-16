(() => {
  const globalObj = typeof window !== 'undefined' ? window : globalThis;
  const AccountsPage = globalObj.AccountsPage || (globalObj.AccountsPage = {});
  const state = AccountsPage.state;
  const {
    permissionLabel,
    availablePermissions,
    renderSelectedOptions,
    enhanceMultiSelects,
    attachHintListener,
    renderTagList,
    fillSelect,
    fillSelectWithEmpty,
    setMultiSelect,
    showAlert,
    setText
  } = AccountsPage;

  async function loadReference() {
    const [rolesRes, templatesRes] = await Promise.all([
      Api.get('/api/accounts/roles'),
      Api.get('/api/accounts/role-templates')
    ]);
    setRoles(rolesRes.roles || []);
    state.roleTemplates = templatesRes.templates || [];
  }

  async function loadRoles() {
    try {
      const res = await Api.get('/api/accounts/roles');
      setRoles(res.roles || []);
    } catch (err) {
      console.error('roles load', err);
    }
  }

  async function loadRoleTemplates() {
    try {
      const res = await Api.get('/api/accounts/role-templates');
      state.roleTemplates = res.templates || [];
    } catch (err) {
      console.error('role templates', err);
    }
    return state.roleTemplates;
  }

  function setRoles(roles) {
    state.roles = roles || [];
    fillSelect(document.getElementById('user-roles'), state.roles.map(r => ({ value: r.name, label: r.name })));
    fillSelect(document.getElementById('group-roles'), state.roles.map(r => ({ value: r.name, label: r.name })));
    fillSelect(document.getElementById('group-detail-roles'), state.roles.map(r => ({ value: r.name, label: r.name })));
    fillSelectWithEmpty(document.getElementById('import-default-role'), state.roles.map(r => ({ value: r.name, label: r.name })));
    fillSelectWithEmpty(document.getElementById('filter-role'), state.roles.map(r => ({ value: r.name, label: r.name })), BerkutI18n.t('common.all') || 'All');
    renderRoles();
    renderRoleDetails();
  }

  function renderRoles() {
    const list = document.getElementById('roles-list');
    if (!list) return;
    const search = (document.getElementById('roles-search')?.value || '').toLowerCase().trim();
    list.innerHTML = '';
    const filtered = (state.roles || []).filter(r => {
      if (!search) return true;
      const name = (r.name || '').toLowerCase();
      const desc = (r.description || '').toLowerCase();
      return name.includes(search) || desc.includes(search);
    });
    if (!filtered.length) {
      const empty = document.createElement('div');
      empty.className = 'empty-state';
      empty.textContent = BerkutI18n.t('common.notAvailable') || '-';
      list.appendChild(empty);
      return;
    }
    filtered.forEach(r => {
      const tile = document.createElement('button');
      tile.type = 'button';
      tile.className = 'tile-card';
      if (state.selectedRoleId === r.id) tile.classList.add('active');
      const title = document.createElement('div');
      title.className = 'tile-title';
      title.textContent = r.name || '-';
      const desc = document.createElement('div');
      desc.className = 'tile-desc';
      desc.textContent = r.description || '';
      const meta = document.createElement('div');
      meta.className = 'tile-meta';
      const typeLabel = r.built_in
        ? (BerkutI18n.t('accounts.roleType.system') || 'System')
        : (r.template ? (BerkutI18n.t('accounts.roleType.template') || 'Template') : (BerkutI18n.t('accounts.roleType.custom') || 'Custom'));
      const permsLabel = BerkutI18n.t('accounts.permissions') || 'Permissions';
      meta.textContent = `${typeLabel} | ${permsLabel}: ${(r.permissions || []).length}`;
      tile.appendChild(title);
      tile.appendChild(desc);
      tile.appendChild(meta);
      tile.addEventListener('click', () => selectRole(r.id));
      list.appendChild(tile);
    });
  }

  function bindRoleDetails() {
    const editBtn = document.getElementById('role-edit-btn');
    const deleteBtn = document.getElementById('role-delete-btn');
    const saveBtn = document.getElementById('role-save-btn');
    const cancelBtn = document.getElementById('role-cancel-btn');
    const editLabel = BerkutI18n.t('common.edit') || 'Edit';
    const deleteLabel = BerkutI18n.t('common.delete') || 'Delete';
    const saveLabel = BerkutI18n.t('common.save') || 'Save';
    const cancelLabel = BerkutI18n.t('common.cancel') || 'Cancel';
    if (editBtn) {
      editBtn.title = editLabel;
      editBtn.setAttribute('aria-label', editLabel);
      editBtn.addEventListener('click', () => setRoleDetailEditState(true));
    }
    if (deleteBtn) {
      deleteBtn.title = deleteLabel;
      deleteBtn.setAttribute('aria-label', deleteLabel);
      deleteBtn.addEventListener('click', () => {
        const role = getSelectedRole();
        if (role) deleteRole(role);
      });
    }
    if (saveBtn) {
      saveBtn.title = saveLabel;
      saveBtn.setAttribute('aria-label', saveLabel);
      saveBtn.addEventListener('click', saveRoleDetails);
    }
    if (cancelBtn) {
      cancelBtn.title = cancelLabel;
      cancelBtn.setAttribute('aria-label', cancelLabel);
      cancelBtn.addEventListener('click', () => {
        setRoleDetailEditState(false);
        renderRoleDetails();
      });
    }
  }

  function selectRole(id) {
    state.roleEditMode = false;
    setRoleDetailEditState(false);
    state.selectedRoleId = id;
    renderRoles();
    renderRoleDetails();
  }

  function getSelectedRole() {
    if (!state.selectedRoleId) return null;
    return state.roles.find(r => r.id === state.selectedRoleId) || null;
  }

  function renderRoleDetails() {
    const role = getSelectedRole();
    const form = document.getElementById('role-detail-form');
    const empty = document.getElementById('role-detail-empty');
    const view = document.getElementById('role-detail-view');
    const title = document.getElementById('role-detail-title');
    const meta = document.getElementById('role-detail-meta');
    const editBtn = document.getElementById('role-edit-btn');
    const deleteBtn = document.getElementById('role-delete-btn');
    const actions = document.getElementById('role-detail-actions');
    const protectedLabel = BerkutI18n.t('accounts.roleSystemProtected') || 'Protected';
    if (!role) {
      state.roleEditMode = false;
      state.selectedRoleId = null;
      if (form) form.hidden = true;
      if (view) view.hidden = true;
      if (empty) empty.hidden = true;
      if (title) title.textContent = '';
      if (meta) meta.textContent = '';
      if (actions) actions.hidden = true;
      if (editBtn) editBtn.disabled = true;
      if (deleteBtn) deleteBtn.disabled = true;
      showAlert('role-detail-alert', '');
      return;
    }
    if (actions) actions.hidden = false;
    if (empty) empty.hidden = true;
    if (title) title.textContent = role.name || '-';
    const typeLabel = role.built_in
      ? (BerkutI18n.t('accounts.roleType.system') || 'System')
      : (role.template ? (BerkutI18n.t('accounts.roleType.template') || 'Template') : (BerkutI18n.t('accounts.roleType.custom') || 'Custom'));
    if (meta) meta.textContent = `${typeLabel} | ${(BerkutI18n.t('accounts.permissions') || 'Permissions')}: ${(role.permissions || []).length}`;
    const isProtected = !!role.built_in;
    if (editBtn) editBtn.disabled = isProtected;
    if (deleteBtn) {
      deleteBtn.disabled = isProtected;
      if (isProtected) deleteBtn.title = protectedLabel;
    }
    if (view) view.hidden = false;
    const nameInput = document.getElementById('role-detail-name');
    const typeInput = document.getElementById('role-detail-type');
    const descInput = document.getElementById('role-detail-description');
    const permsSel = document.getElementById('role-detail-permissions');
    if (nameInput) nameInput.value = role.name || '';
    if (typeInput) typeInput.value = typeLabel;
    if (descInput) descInput.value = role.description || '';
    if (permsSel) {
      const perms = availablePermissions();
      fillSelect(permsSel, perms.map(p => ({ value: p, label: permissionLabel(p) })));
      Array.from(permsSel.options).forEach(opt => {
        opt.dataset.label = opt.textContent;
      });
      setMultiSelect(permsSel, role.permissions || []);
      enhanceMultiSelects([permsSel.id]);
      renderSelectedOptions(permsSel, document.getElementById('role-detail-permissions-hint'));
      attachHintListener(permsSel, document.getElementById('role-detail-permissions-hint'));
    }
    setText('role-view-name', role.name || '-');
    setText('role-view-type', typeLabel);
    setText('role-view-description', role.description || (BerkutI18n.t('common.notAvailable') || '-'));
    renderTagList('role-view-permissions', (role.permissions || []).map(permissionLabel));
    setRoleDetailEditState(state.roleEditMode);
    showAlert('role-detail-alert', '');
  }

  function setRoleDetailEditState(enabled) {
    const role = getSelectedRole();
    if (enabled && (!role || role.built_in)) return;
    state.roleEditMode = enabled;
    const view = document.getElementById('role-detail-view');
    const form = document.getElementById('role-detail-form');
    const desc = document.getElementById('role-detail-description');
    const perms = document.getElementById('role-detail-permissions');
    if (desc) desc.disabled = !enabled;
    if (perms) perms.disabled = !enabled;
    const editBtn = document.getElementById('role-edit-btn');
    const deleteBtn = document.getElementById('role-delete-btn');
    const saveBtn = document.getElementById('role-save-btn');
    const cancelBtn = document.getElementById('role-cancel-btn');
    if (view) view.hidden = enabled;
    if (form) form.hidden = !enabled;
    if (editBtn) editBtn.hidden = enabled;
    if (deleteBtn) deleteBtn.hidden = enabled;
    if (saveBtn) saveBtn.hidden = !enabled;
    if (cancelBtn) cancelBtn.hidden = !enabled;
  }

  async function saveRoleDetails() {
    const role = getSelectedRole();
    if (!role) return;
    const desc = (document.getElementById('role-detail-description')?.value || '').trim();
    const perms = AccountsPage.getSelectedValues(document.getElementById('role-detail-permissions'));
    try {
      await Api.put(`/api/accounts/roles/${role.id}`, { description: desc, permissions: perms });
      setRoleDetailEditState(false);
      state.selectedRoleId = role.id;
      await loadRoles();
    } catch (err) {
      showAlert('role-detail-alert', err.message || BerkutI18n.t('common.error'));
    }
  }

  async function deleteRole(role) {
    if (!confirm('Delete this role?')) return;
    try {
      await Api.del(`/api/accounts/roles/${role.id}`);
      await loadRoles();
      if (state.selectedRoleId === role.id) {
        state.selectedRoleId = null;
        renderRoleDetails();
      }
    } catch (err) {
      alert(err.message || (BerkutI18n.t('common.error') || 'Error'));
    }
  }

  function bindRoleCreateModal() {
    const openBtn = document.getElementById('open-role-create');
    const modal = document.getElementById('role-create-modal');
    if (!openBtn || !modal) return;
    const form = document.getElementById('role-create-form');
    const select = document.getElementById('role-create-permissions');
    const hint = document.getElementById('role-create-permissions-hint');
    const selectAllBtn = document.getElementById('role-create-select-all');
    const clearAllBtn = document.getElementById('role-create-clear-all');
    openBtn.addEventListener('click', () => openRoleCreateModal());
    document.getElementById('close-role-create').onclick = closeRoleCreateModal;
    document.getElementById('cancel-role-create').onclick = closeRoleCreateModal;
    if (form) {
      form.onsubmit = async (e) => {
        e.preventDefault();
        const name = (document.getElementById('role-create-name')?.value || '').trim();
        if (!name) {
          showAlert('role-create-alert', BerkutI18n.t('accounts.roleNameRequired') || 'Name required');
          return;
        }
        const description = (document.getElementById('role-create-description')?.value || '').trim();
        const permissions = AccountsPage.getSelectedValues(select);
        try {
          if (state.editingRoleId) {
            await Api.put(`/api/accounts/roles/${state.editingRoleId}`, { description, permissions });
          } else {
            await Api.post('/api/accounts/roles', { name, description, permissions });
          }
          closeRoleCreateModal();
          await loadRoles();
        } catch (err) {
          showAlert('role-create-alert', err.message || BerkutI18n.t('common.error'));
        }
      };
    }
    if (select) {
      select.addEventListener('change', () => renderSelectedOptions(select, hint));
      select.addEventListener('selectionrefresh', () => renderSelectedOptions(select, hint));
    }
    if (selectAllBtn && select) {
      selectAllBtn.addEventListener('click', () => {
        Array.from(select.options).forEach(opt => {
          opt.selected = true;
        });
        renderSelectedOptions(select, hint);
        select.dispatchEvent(new Event('change', { bubbles: true }));
      });
    }
    if (clearAllBtn && select) {
      clearAllBtn.addEventListener('click', () => {
        Array.from(select.options).forEach(opt => {
          opt.selected = false;
        });
        renderSelectedOptions(select, hint);
        select.dispatchEvent(new Event('change', { bubbles: true }));
      });
    }
  }

  function openRoleCreateModal(role) {
    const modal = document.getElementById('role-create-modal');
    const select = document.getElementById('role-create-permissions');
    const hint = document.getElementById('role-create-permissions-hint');
    if (!modal || !select) return;
    state.editingRoleId = role ? role.id : null;
    const title = modal.querySelector('h3');
    if (title) {
      title.textContent = state.editingRoleId ? (BerkutI18n.t('accounts.roleEdit') || 'Edit role') : (BerkutI18n.t('accounts.roleCreate') || 'Create role');
    }
    const perms = availablePermissions();
    fillSelect(select, perms.map(p => ({ value: p, label: permissionLabel(p) })));
    Array.from(select.options).forEach(opt => {
      opt.dataset.label = opt.textContent;
    });
    enhanceMultiSelects([select.id]);
    renderSelectedOptions(select, hint);
    const nameInput = document.getElementById('role-create-name');
    const descInput = document.getElementById('role-create-description');
    if (nameInput) {
      nameInput.value = role ? role.name : '';
      if (role) {
        nameInput.setAttribute('disabled', 'true');
      } else {
        nameInput.removeAttribute('disabled');
      }
    }
    if (descInput) {
      descInput.value = role ? (role.description || '') : '';
    }
    if (role && select) {
      setMultiSelect(select, role.permissions || []);
      renderSelectedOptions(select, hint);
    }
    showAlert('role-create-alert', '');
    modal.hidden = false;
  }

  function closeRoleCreateModal() {
    const modal = document.getElementById('role-create-modal');
    if (modal) modal.hidden = true;
    state.editingRoleId = null;
    showAlert('role-create-alert', '');
  }

  function bindRoleTemplateModal() {
    const openBtn = document.getElementById('open-role-template');
    const modal = document.getElementById('role-template-modal');
    if (!openBtn || !modal) return;
    const form = document.getElementById('role-template-form');
    const select = document.getElementById('role-template-select');
    const nameInput = document.getElementById('role-template-name');
    const descInput = document.getElementById('role-template-description');
    openBtn.addEventListener('click', () => openRoleTemplateModal());
    document.getElementById('close-role-template').onclick = closeRoleTemplateModal;
    document.getElementById('cancel-role-template').onclick = closeRoleTemplateModal;
    select.addEventListener('change', () => renderTemplatePreview(select.value, false));
    form.onsubmit = async (e) => {
      e.preventDefault();
      const tplId = select.value;
      const name = (nameInput.value || '').trim();
      const description = (descInput.value || '').trim();
      if (!tplId) {
        showAlert('role-template-alert', BerkutI18n.t('errors.roleTemplateNotFound') || 'Template not found');
        return;
      }
      if (!name) {
        showAlert('role-template-alert', BerkutI18n.t('accounts.roleNameRequired') || 'Name required');
        return;
      }
      try {
        await Api.post('/api/accounts/roles/from-template', { template_id: tplId, name, description });
        closeRoleTemplateModal();
        await loadRoles();
      } catch (err) {
        showAlert('role-template-alert', err.message || BerkutI18n.t('common.error'));
      }
    };
  }

  async function openRoleTemplateModal() {
    const modal = document.getElementById('role-template-modal');
    if (!modal) return;
    await loadRoleTemplates();
    const select = document.getElementById('role-template-select');
    fillSelect(select, (state.roleTemplates || []).map(t => ({ value: t.id, label: t.name })));
    const first = state.roleTemplates[0];
    if (first) {
      select.value = select.value || first.id;
      renderTemplatePreview(select.value, true);
    } else {
      renderTemplatePreview('', true);
    }
    document.getElementById('role-template-name').value = '';
    showAlert('role-template-alert', '');
    modal.hidden = false;
  }

  function closeRoleTemplateModal() {
    const modal = document.getElementById('role-template-modal');
    if (modal) {
      modal.hidden = true;
    }
    showAlert('role-template-alert', '');
  }

  function renderTemplatePreview(templateId, resetDescription) {
    const preview = document.getElementById('role-template-permissions');
    if (!preview) return;
    preview.innerHTML = '';
    const tpl = (state.roleTemplates || []).find(t => t.id === templateId);
    if (!tpl) {
      if (resetDescription) {
        const descInput = document.getElementById('role-template-description');
        if (descInput) {
          descInput.value = '';
        }
      }
      return;
    }
    tpl.permissions.forEach(p => {
      const li = document.createElement('li');
      li.textContent = permissionLabel(p);
      li.title = p;
      preview.appendChild(li);
    });
    if (resetDescription) {
      const descInput = document.getElementById('role-template-description');
      if (descInput) {
        descInput.value = tpl.description || '';
      }
    }
  }

  AccountsPage.loadReference = loadReference;
  AccountsPage.loadRoles = loadRoles;
  AccountsPage.loadRoleTemplates = loadRoleTemplates;
  AccountsPage.setRoles = setRoles;
  AccountsPage.renderRoles = renderRoles;
  AccountsPage.bindRoleDetails = bindRoleDetails;
  AccountsPage.selectRole = selectRole;
  AccountsPage.getSelectedRole = getSelectedRole;
  AccountsPage.renderRoleDetails = renderRoleDetails;
  AccountsPage.setRoleDetailEditState = setRoleDetailEditState;
  AccountsPage.saveRoleDetails = saveRoleDetails;
  AccountsPage.deleteRole = deleteRole;
  AccountsPage.bindRoleCreateModal = bindRoleCreateModal;
  AccountsPage.openRoleCreateModal = openRoleCreateModal;
  AccountsPage.closeRoleCreateModal = closeRoleCreateModal;
  AccountsPage.bindRoleTemplateModal = bindRoleTemplateModal;
  AccountsPage.openRoleTemplateModal = openRoleTemplateModal;
  AccountsPage.closeRoleTemplateModal = closeRoleTemplateModal;
  AccountsPage.renderTemplatePreview = renderTemplatePreview;
})();
