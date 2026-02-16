(() => {
  const state = TasksPage.state;
  const {
    t,
    hasPermission,
    showAlert,
    hideAlert,
    openModal,
    closeModal,
    resolveErrorMessage,
    toISODate,
    formatDateTime,
    ensureTemplates,
    populatePrioritySelect,
    populateAssigneesSelect
  } = TasksPage;

  let currentTemplateId = null;
  let currentLinks = [];

  function initTemplates() {
    const closeBtn = document.querySelector('#task-templates-modal [data-close]');
    const newBtn = document.getElementById('task-template-new');
    const form = document.getElementById('task-template-form');
    const deleteBtn = document.getElementById('task-template-delete');
    const linkAddBtn = document.getElementById('task-template-link-add');
    const boardSelect = document.getElementById('task-template-board');

    if (closeBtn) closeBtn.addEventListener('click', () => closeModal('task-templates-modal'));
    if (newBtn) newBtn.addEventListener('click', () => editTemplate(null));
    if (linkAddBtn) linkAddBtn.addEventListener('click', addTemplateLink);
    if (boardSelect) boardSelect.addEventListener('change', async (e) => {
      const boardId = parseInt(e.target.value || '0', 10);
      await TasksPage.loadColumns(boardId, true, false);
      populateTemplateColumns(boardId);
    });
    if (deleteBtn) deleteBtn.addEventListener('click', () => deleteTemplate(currentTemplateId));
    if (form) {
      form.addEventListener('submit', async (e) => {
        e.preventDefault();
        await saveTemplate();
      });
    }
  }

  async function openTemplates(selectId, forceNew = false) {
    if (!hasPermission('tasks.templates.view')) return;
    hideAlert('task-templates-alert');
    try {
      await ensureBoards();
      await TasksPage.ensureUserDirectory();
      if (ensureTemplates) {
        await ensureTemplates(true);
      } else {
        const res = await Api.get('/api/tasks/templates?include_inactive=1');
        state.templates = res.items || [];
      }
      renderTemplatesList();
      if (forceNew) {
        editTemplate(null);
      } else {
        editTemplate(selectId || null);
      }
      if (TasksPage.renderTemplatesHome) {
        TasksPage.renderTemplatesHome();
      }
      openModal('task-templates-modal');
    } catch (err) {
      showAlert('task-templates-alert', resolveErrorMessage(err, 'common.error'));
    }
  }

  async function ensureBoards() {
    if (state.boards.length) return;
    try {
      const res = await Api.get('/api/tasks/boards');
      state.boards = res.items || [];
    } catch (_) {
      state.boards = [];
    }
  }

  function renderTemplatesList() {
    const container = document.getElementById('task-templates-list');
    if (!container) return;
    container.innerHTML = '';
    if (!state.templates.length) {
      container.textContent = t('tasks.empty.noTemplates');
      return;
    }
    state.templates.forEach(tpl => {
      const card = document.createElement('div');
      card.className = 'template-card';
      const board = state.boards.find(b => b.id === tpl.board_id);
      const meta = [
        board?.name ? board.name : `#${tpl.board_id}`,
        tpl.priority ? t(`tasks.priority.${tpl.priority}`) : '',
        tpl.is_active ? t('tasks.templateActive') : t('common.disabled')
      ].filter(Boolean).join(' · ');
      card.innerHTML = `
        <div class="template-head">
          <div>
            <div class="template-name">${TasksPage.escapeHtml(tpl.title_template || '')}</div>
            <div class="muted">${TasksPage.escapeHtml(meta)}</div>
          </div>
          <div class="template-actions">
            ${hasPermission('tasks.create') ? `<button class="btn secondary" data-use="${tpl.id}" data-i18n="tasks.actions.createFromTemplate">${t('tasks.actions.createFromTemplate')}</button>` : ''}
            ${hasPermission('tasks.templates.manage') ? `<button class="btn ghost" data-edit="${tpl.id}" data-i18n="common.edit">${t('common.edit')}</button>` : ''}
            ${hasPermission('tasks.templates.manage') ? `<button class="btn ghost danger" data-delete="${tpl.id}" data-i18n="common.delete">${t('common.delete')}</button>` : ''}
          </div>
        </div>
        <div class="muted">${TasksPage.escapeHtml(tpl.description_template || '')}</div>
      `;
      const useBtn = card.querySelector('[data-use]');
      if (useBtn) useBtn.onclick = () => createTaskFromTemplate(tpl.id);
      const editBtn = card.querySelector('[data-edit]');
      if (editBtn) editBtn.onclick = () => editTemplate(tpl.id);
      const delBtn = card.querySelector('[data-delete]');
      if (delBtn) delBtn.onclick = () => deleteTemplate(tpl.id);
      container.appendChild(card);
    });
  }


  function resetTemplateForm() {
    currentTemplateId = null;
    currentLinks = [];
    const form = document.getElementById('task-template-form');
    const deleteBtn = document.getElementById('task-template-delete');
    if (form) form.hidden = false;
    if (deleteBtn) deleteBtn.hidden = true;
    const boardSelect = document.getElementById('task-template-board');
    if (boardSelect) {
      boardSelect.innerHTML = '';
      state.boards.forEach(board => {
        const opt = document.createElement('option');
        opt.value = board.id;
        opt.textContent = board.name;
        if (board.id === state.boardId) opt.selected = true;
        boardSelect.appendChild(opt);
      });
    }
    const boardId = parseInt(boardSelect?.value || '0', 10);
    if (boardId) populateTemplateColumns(boardId);
    if (populatePrioritySelect) populatePrioritySelect(document.getElementById('task-template-priority'));
    if (populateAssigneesSelect) populateAssigneesSelect(document.getElementById('task-template-assignees'));
    setValue('task-template-title', '');
    setValue('task-template-description', '');
    setValue('task-template-due-days', '0');
    setValue('task-template-checklist', '');
    const active = document.getElementById('task-template-active');
    if (active) active.checked = true;
    renderTemplateLinks();
    hideAlert('task-template-manage-alert');
  }

  function editTemplate(id) {
    const form = document.getElementById('task-template-form');
    const deleteBtn = document.getElementById('task-template-delete');
    if (!hasPermission('tasks.templates.manage')) {
      currentTemplateId = null;
      if (form) form.hidden = true;
      if (deleteBtn) deleteBtn.hidden = true;
      return;
    }
    if (!id) {
      resetTemplateForm();
      return;
    }
    const tpl = state.templates.find(t => `${t.id}` === `${id}`);
    if (!tpl) return;
    currentTemplateId = tpl.id;
    currentLinks = (tpl.links_template || []).map(l => ({ ...l }));
    if (form) form.hidden = false;
    if (deleteBtn) deleteBtn.hidden = false;
    const boardSelect = document.getElementById('task-template-board');
    if (boardSelect) {
      boardSelect.innerHTML = '';
      state.boards.forEach(board => {
        const opt = document.createElement('option');
        opt.value = board.id;
        opt.textContent = board.name;
        if (board.id === tpl.board_id) opt.selected = true;
        boardSelect.appendChild(opt);
      });
    }
    populateTemplateColumns(tpl.board_id, tpl.column_id);
    if (populatePrioritySelect) populatePrioritySelect(document.getElementById('task-template-priority'));
    setValue('task-template-priority', tpl.priority || 'medium');
    setValue('task-template-title', tpl.title_template || '');
    setValue('task-template-description', tpl.description_template || '');
    setValue('task-template-due-days', `${tpl.default_due_days || 0}`);
    setValue('task-template-checklist', (tpl.checklist_template || []).map(i => i.text).join('\n'));
    if (populateAssigneesSelect) populateAssigneesSelect(document.getElementById('task-template-assignees'), tpl.default_assignees || []);
    const active = document.getElementById('task-template-active');
    if (active) active.checked = tpl.is_active !== false;
    renderTemplateLinks();
    hideAlert('task-template-manage-alert');
  }

  function populateTemplateColumns(boardId, selectedId) {
    const columnSelect = document.getElementById('task-template-column');
    if (!columnSelect) return;
    if (!boardId) {
      columnSelect.innerHTML = '';
      return;
    }
    const hasLoaded = Object.prototype.hasOwnProperty.call(state.columnsByBoard, boardId);
    const columns = state.columnsByBoard[boardId] || [];
    if (!hasLoaded && TasksPage.loadColumns) {
      TasksPage.loadColumns(boardId, true, false).then(() => {
        populateTemplateColumns(boardId, selectedId);
      });
    }
    columnSelect.innerHTML = '';
    columns.forEach(col => {
      const opt = document.createElement('option');
      opt.value = col.id;
      opt.textContent = col.name;
      if (selectedId && col.id === selectedId) opt.selected = true;
      columnSelect.appendChild(opt);
    });
  }

  function renderTemplateLinks() {
    const list = document.getElementById('task-template-links-list');
    if (!list) return;
    list.innerHTML = '';
    if (!currentLinks.length) {
      const empty = document.createElement('div');
      empty.className = 'muted';
      empty.textContent = t('tasks.empty.noLinks');
      list.appendChild(empty);
      return;
    }
    currentLinks.forEach((link, idx) => {
      const row = document.createElement('div');
      row.className = 'task-link';
      const label = document.createElement('span');
      label.textContent = `${t(`tasks.links.${link.target_type}`)} #${link.target_id}`;
      row.appendChild(label);
      const remove = document.createElement('button');
      remove.type = 'button';
      remove.className = 'btn ghost btn-sm';
      remove.textContent = t('tasks.actions.remove');
      remove.addEventListener('click', () => {
        currentLinks = currentLinks.filter((_, i) => i !== idx);
        renderTemplateLinks();
      });
      row.appendChild(remove);
      list.appendChild(row);
    });
  }

  function addTemplateLink() {
    const type = document.getElementById('task-template-link-type')?.value || '';
    const idVal = (document.getElementById('task-template-link-id')?.value || '').trim();
    if (!type || !idVal) return;
    currentLinks.push({ target_type: type, target_id: idVal });
    document.getElementById('task-template-link-id').value = '';
    renderTemplateLinks();
  }

  async function saveTemplate() {
    if (!hasPermission('tasks.templates.manage')) return;
    hideAlert('task-template-manage-alert');
    const boardId = parseInt(document.getElementById('task-template-board')?.value || '0', 10);
    const columnId = parseInt(document.getElementById('task-template-column')?.value || '0', 10);
    const title = (document.getElementById('task-template-title')?.value || '').trim();
    const description = (document.getElementById('task-template-description')?.value || '').trim();
    const priority = document.getElementById('task-template-priority')?.value || 'medium';
    const dueDaysRaw = (document.getElementById('task-template-due-days')?.value || '0').trim();
    const dueDays = parseInt(dueDaysRaw || '0', 10);
    const assignees = Array.from(document.getElementById('task-template-assignees')?.selectedOptions || [])
      .map(opt => opt.value)
      .filter(Boolean);
    const checklistText = (document.getElementById('task-template-checklist')?.value || '').trim();
    const checklist = checklistText
      ? checklistText.split('\n').map(line => ({ text: line.trim(), done: false })).filter(i => i.text)
      : [];
    const active = document.getElementById('task-template-active')?.checked !== false;
    if (!title) {
      showAlert('task-template-manage-alert', t('tasks.templateTitleRequired'));
      return;
    }
    if (!columnId) {
      showAlert('task-template-manage-alert', t('tasks.columnRequired'));
      return;
    }
    const payload = {
      board_id: boardId,
      column_id: columnId,
      title_template: title,
      description_template: description,
      priority,
      default_due_days: Number.isNaN(dueDays) ? 0 : dueDays,
      default_assignees: assignees,
      checklist_template: checklist,
      links_template: currentLinks,
      is_active: active
    };
    try {
      if (currentTemplateId) {
        await Api.put(`/api/tasks/templates/${currentTemplateId}`, payload);
      } else {
        await Api.post('/api/tasks/templates', payload);
      }
      await openTemplates(currentTemplateId);
    } catch (err) {
      showAlert('task-template-manage-alert', resolveErrorMessage(err, 'common.error'));
    }
  }

  async function deleteTemplate(id) {
    if (!hasPermission('tasks.templates.manage') || !id) return;
    if (!confirm(t('tasks.templateDeleteConfirm'))) return;
    try {
      await Api.del(`/api/tasks/templates/${id}`);
      await openTemplates();
    } catch (err) {
      showAlert('task-template-manage-alert', resolveErrorMessage(err, 'common.error'));
    }
  }

  async function createTaskFromTemplate(id) {
    if (!hasPermission('tasks.create')) return;
    try {
      const task = await Api.post(`/api/tasks/templates/${id}/create-task`, {});
      if (task && task.board_id === state.boardId) {
        state.tasks.push(task);
        state.taskMap.set(task.id, task);
        if (TasksPage.renderBoard) TasksPage.renderBoard();
      }
    } catch (err) {
      showAlert('task-templates-alert', resolveErrorMessage(err, 'common.error'));
    }
  }

  function describeTemplate(id) {
    if (!id) return '';
    const tpl = state.templates.find(ti => `${ti.id}` === `${id}`);
    return tpl?.title_template || `#${id}`;
  }

  function setValue(id, val) {
    const el = document.getElementById(id);
    if (el) el.value = val;
  }

  TasksPage.initTemplates = initTemplates;
  TasksPage.openTemplates = openTemplates;
  TasksPage.describeTemplate = describeTemplate;
})();
