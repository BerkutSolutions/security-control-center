(() => {
  const state = TasksPage.state;
  const { t, hasPermission, ensureTemplates } = TasksPage;

  function initTemplatesHome() {
    const homeOpenBtn = document.getElementById('tasks-templates-open');
    const homeCreateBtn = document.getElementById('tasks-template-create');
    if (homeOpenBtn) homeOpenBtn.addEventListener('click', () => TasksPage.openTemplates && TasksPage.openTemplates());
    if (homeCreateBtn) homeCreateBtn.addEventListener('click', () => TasksPage.openTemplates && TasksPage.openTemplates(null, true));
  }

  async function renderTemplatesHome() {
    const list = document.getElementById('tasks-templates-home-list');
    const empty = document.getElementById('tasks-templates-home-empty');
    if (!list || !empty) return;
    list.innerHTML = '';
    if (!hasPermission('tasks.templates.view')) {
      empty.hidden = false;
      return;
    }
    try {
      await ensureBoards();
      if (ensureTemplates) {
        await ensureTemplates(true);
      } else {
        const res = await Api.get('/api/tasks/templates?include_inactive=1');
        state.templates = res.items || [];
      }
    } catch (_) {
      state.templates = [];
    }
    if (!state.templates.length) {
      empty.hidden = false;
      return;
    }
    empty.hidden = true;
    const items = state.templates.slice(0, 6);
    items.forEach(tpl => {
      const card = document.createElement('div');
      card.className = 'template-card';
      const board = state.boards.find(b => b.id === tpl.board_id);
      const meta = [
        board?.name ? board.name : `#${tpl.board_id}`,
        tpl.priority ? t(`tasks.priority.${tpl.priority}`) : '',
        tpl.is_active ? t('tasks.templateActive') : t('common.disabled')
      ].filter(Boolean).join(' - ');
      card.innerHTML = `
        <div class="template-head">
          <div>
            <div class="template-name">${TasksPage.escapeHtml(tpl.title_template || '')}</div>
            <div class="muted">${TasksPage.escapeHtml(meta)}</div>
          </div>
          <div class="template-actions">
            ${hasPermission('tasks.templates.manage') ? `<button class="btn ghost btn-sm" data-edit="${tpl.id}">${t('common.edit')}</button>` : ''}
          </div>
        </div>
        <div class="muted">${TasksPage.escapeHtml(tpl.description_template || '')}</div>
      `;
      const editBtn = card.querySelector('[data-edit]');
      if (editBtn) editBtn.onclick = () => TasksPage.openTemplates && TasksPage.openTemplates(tpl.id);
      list.appendChild(card);
    });
    if (state.templates.length > items.length) {
      const more = document.createElement('div');
      more.className = 'muted';
      more.textContent = t('tasks.templatesHomeMore');
      list.appendChild(more);
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

  TasksPage.initTemplatesHome = initTemplatesHome;
  TasksPage.renderTemplatesHome = renderTemplatesHome;
})();
