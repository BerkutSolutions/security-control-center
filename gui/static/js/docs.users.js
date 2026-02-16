(() => {
  const state = DocsPage.state;

  async function loadCurrentUser() {
    try {
      const res = await Api.get('/api/auth/me');
      return res.user;
    } catch (err) {
      console.error('load me failed', err);
      return null;
    }
  }

  async function loadUsersIntoSelects() {
    await UserDirectory.load();
    const ownerSelects = ['create-owner', 'import-owner'];
    ownerSelects.forEach(id => {
      const sel = document.getElementById(id);
      if (!sel) return;
      sel.innerHTML = '';
      UserDirectory.all().forEach(u => {
        const opt = document.createElement('option');
        opt.value = u.id;
        opt.textContent = u.full_name || u.username;
        sel.appendChild(opt);
      });
      if (state.currentUser && UserDirectory.get(state.currentUser.id)) {
        sel.value = state.currentUser.id;
      }
    });
    const aclRoleSelects = ['create-acl-roles', 'import-acl-roles'];
    const roleOptions = ['superadmin', 'admin', 'security_officer', 'doc_admin', 'doc_editor', 'doc_reviewer', 'doc_viewer', 'auditor', 'manager', 'analyst'];
    aclRoleSelects.forEach(id => {
      const sel = document.getElementById(id);
      if (!sel) return;
      sel.innerHTML = '';
      roleOptions.forEach(r => {
        const opt = document.createElement('option');
        opt.value = r;
        opt.textContent = r;
        sel.appendChild(opt);
      });
      attachSelectedPreview(sel);
    });
    const aclUserSelects = ['create-acl-users', 'import-acl-users'];
    aclUserSelects.forEach(id => {
      const sel = document.getElementById(id);
      if (!sel) return;
      sel.innerHTML = '';
      UserDirectory.all().forEach(u => {
        const opt = document.createElement('option');
        opt.value = u.id;
        opt.textContent = u.full_name || u.username;
        sel.appendChild(opt);
      });
      attachSelectedPreview(sel);
    });
    DocsPage.enhanceMultiSelects(['create-acl-roles', 'create-acl-users', 'import-acl-roles', 'import-acl-users']);
  }

  function attachSelectedPreview(sel) {
    if (!sel) return;
    let preview = sel.parentElement?.querySelector(`.selected-hint[data-select-hint="${sel.id}"]`);
    if (!preview) preview = sel.nextElementSibling;
    if (!preview || (!preview.classList.contains('selected-preview') && !preview.classList.contains('selected-hint'))) {
      preview = document.createElement('div');
      preview.className = 'selected-hint';
      if (sel.id) preview.dataset.selectHint = sel.id;
      sel.parentElement.appendChild(preview);
    }
    const render = () => {
      const selected = Array.from(sel.options).filter(o => o.selected);
      preview.innerHTML = '';
      if (!selected.length) {
        preview.textContent = BerkutI18n.t('docs.stageEmptySelection') || '';
        return;
      }
      selected.forEach(opt => {
        const tag = document.createElement('span');
        tag.className = 'tag';
        tag.textContent = opt.textContent || '';
        const remove = document.createElement('button');
        remove.type = 'button';
        remove.className = 'tag-remove';
        remove.setAttribute('aria-label', BerkutI18n.t('common.delete') || 'Remove');
        remove.textContent = 'x';
        remove.addEventListener('click', (e) => {
          e.stopPropagation();
          opt.selected = false;
          sel.dispatchEvent(new Event('change', { bubbles: true }));
        });
        tag.appendChild(remove);
        preview.appendChild(tag);
      });
    };
    sel.addEventListener('change', render);
    sel.addEventListener('selectionrefresh', render);
    render();
  }

  DocsPage.loadCurrentUser = loadCurrentUser;
  DocsPage.loadUsersIntoSelects = loadUsersIntoSelects;
  DocsPage.attachSelectedPreview = attachSelectedPreview;
})();
