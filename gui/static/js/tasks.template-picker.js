(() => {
  const state = TasksPage.state;
  const {
    t,
    hasPermission,
    showAlert,
    hideAlert,
    openModal,
    closeModal,
    ensureTemplates
  } = TasksPage;

  let pickerContext = null;
  let pickerTemplates = [];

  function initTemplatePicker() {
    const closeBtn = document.getElementById('task-template-picker-close');
    const cancelBtn = document.getElementById('task-template-picker-cancel');
    const createBtn = document.getElementById('task-template-picker-create');
    const select = document.getElementById('task-template-picker-select');
    if (closeBtn) closeBtn.addEventListener('click', () => closeModal('task-template-picker-modal'));
    if (cancelBtn) cancelBtn.addEventListener('click', () => closeModal('task-template-picker-modal'));
    if (createBtn) createBtn.addEventListener('click', submitTemplatePicker);
    if (select && select.dataset.bound !== '1') {
      select.dataset.bound = '1';
      select.addEventListener('change', () => updatePickerMeta());
    }
  }

  async function openTemplatePicker(ctx) {
    if (!hasPermission('tasks.create')) return;
    pickerContext = ctx || {};
    hideAlert('task-template-picker-alert');
    const titleInput = document.getElementById('task-template-picker-title');
    if (titleInput) titleInput.value = (pickerContext.title || '');
    try {
      pickerTemplates = ensureTemplates ? await ensureTemplates(false) : [];
    } catch (_) {
      pickerTemplates = [];
    }
    renderPickerOptions();
    updatePickerMeta();
    openModal('task-template-picker-modal');
  }

  function renderPickerOptions() {
    const select = document.getElementById('task-template-picker-select');
    if (!select) return;
    select.innerHTML = '';
    const standard = document.createElement('option');
    standard.value = '0';
    standard.textContent = t('tasks.templateStandard');
    select.appendChild(standard);
    const boardId = pickerContext?.boardId || 0;
    const templates = (pickerTemplates || []).filter(tpl => tpl.is_active !== false && (!boardId || tpl.board_id === boardId));
    templates.forEach(tpl => {
      const opt = document.createElement('option');
      opt.value = `${tpl.id}`;
      opt.textContent = tpl.title_template || `#${tpl.id}`;
      select.appendChild(opt);
    });
    const preferred = pickerContext?.defaultTemplateId;
    if (preferred && templates.find(tpl => tpl.id === preferred)) {
      select.value = `${preferred}`;
    }
  }

  function updatePickerMeta() {
    const select = document.getElementById('task-template-picker-select');
    const meta = document.getElementById('task-template-picker-meta');
    if (!select || !meta) return;
    const templateId = parseInt(select.value || '0', 10);
    if (!templateId) {
      meta.textContent = t('tasks.templateStandardHint');
      return;
    }
    const tpl = (pickerTemplates || []).find(item => item.id === templateId);
    if (!tpl) {
      meta.textContent = '';
      return;
    }
    const metaParts = [
      tpl.priority ? t(`tasks.priority.${tpl.priority}`) : '',
      tpl.description_template || ''
    ].filter(Boolean);
    meta.textContent = metaParts.join(' - ');
  }

  async function submitTemplatePicker() {
    const select = document.getElementById('task-template-picker-select');
    if (!select) return;
    const templateId = parseInt(select.value || '0', 10);
    const titleInput = document.getElementById('task-template-picker-title');
    const title = (titleInput?.value || '').trim();
    const boardId = pickerContext?.boardId || 0;
    const columnId = pickerContext?.columnId || 0;
    const subcolumnId = pickerContext?.subcolumnId || null;
    const position = pickerContext?.position || 0;
    hideAlert('task-template-picker-alert');
    if (!columnId) {
      showAlert('task-template-picker-alert', t('tasks.columnRequired'));
      return;
    }
    try {
      let created = null;
      if (!templateId) {
        if (!title) {
          showAlert('task-template-picker-alert', t('tasks.titleRequired'));
          return;
        }
        created = await Api.post('/api/tasks', {
          board_id: boardId,
          column_id: columnId,
          subcolumn_id: subcolumnId || undefined,
          title,
          position
        });
      } else {
        const payload = { column_id: columnId };
        if (subcolumnId) payload.subcolumn_id = subcolumnId;
        if (title) payload.title = title;
        created = await Api.post(`/api/tasks/templates/${templateId}/create-task`, payload);
      }
      if (created && created.id) {
        state.taskMap.set(created.id, created);
      }
      if (TasksPage.loadTasks) await TasksPage.loadTasks(boardId);
      if (TasksPage.renderBoards) TasksPage.renderBoards(state.spaceId);
      closeModal('task-template-picker-modal');
    } catch (err) {
      showAlert('task-template-picker-alert', TasksPage.resolveErrorMessage(err, 'common.error'));
    }
  }

  TasksPage.initTemplatePicker = initTemplatePicker;
  TasksPage.openTemplatePicker = openTemplatePicker;
})();
