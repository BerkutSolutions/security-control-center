(() => {
  const state = TasksPage.state;
  const { t, hasPermission, showAlert, hideAlert, openModal, closeModal, resolveErrorMessage, formatDateTime } = TasksPage;

  let currentRuleId = null;

  function initRecurring() {
    const closeBtn = document.querySelector('#task-recurring-modal [data-close]');
    const newBtn = document.getElementById('task-recurring-new');
    const form = document.getElementById('task-recurring-form');
    const typeSelect = document.getElementById('task-recurring-type');
    const runBtn = document.getElementById('task-recurring-run');

    if (closeBtn) closeBtn.addEventListener('click', () => closeModal('task-recurring-modal'));
    if (newBtn) newBtn.addEventListener('click', () => editRule(null));
    if (typeSelect) typeSelect.addEventListener('change', () => toggleRecurringFields(typeSelect.value));
    if (runBtn) {
      runBtn.hidden = !hasPermission('tasks.recurring.run');
      runBtn.addEventListener('click', runNow);
    }
    if (form) {
      form.addEventListener('submit', async (e) => {
        e.preventDefault();
        await saveRule();
      });
    }
    renderWeekdayCheckboxes();
    renderMonthOptions();
  }

  async function openRecurring(selectId) {
    if (!hasPermission('tasks.recurring.view')) return;
    hideAlert('task-recurring-alert');
    try {
      if (hasPermission('tasks.recurring.manage')) {
        await ensureTemplates();
      }
      const res = await Api.get('/api/tasks/recurring?include_inactive=1');
      state.recurringRules = res.items || [];
      renderRecurringList();
      editRule(selectId || null);
      openModal('task-recurring-modal');
    } catch (err) {
      showAlert('task-recurring-alert', resolveErrorMessage(err, 'common.error'));
    }
  }

  async function ensureTemplates() {
    if (!state.templates.length) {
      const res = await Api.get('/api/tasks/templates?include_inactive=1');
      state.templates = res.items || [];
    }
    populateTemplateSelect();
  }

  function renderRecurringList() {
    const tbody = document.getElementById('task-recurring-list');
    if (!tbody) return;
    tbody.innerHTML = '';
    if (!state.recurringRules.length) {
      const row = document.createElement('tr');
      row.className = 'placeholder';
      row.innerHTML = `<td colspan="6">${t('tasks.empty.noRecurring')}</td>`;
      tbody.appendChild(row);
      return;
    }
    state.recurringRules.forEach(rule => {
    const tpl = state.templates.find(tpl => tpl.id === rule.template_id);
    const templateTitle = tpl?.title_template || rule.template_title || `#${rule.template_id}`;
      const schedule = describeSchedule(rule);
      const nextRun = rule.next_run_at ? formatDateTime(rule.next_run_at) : '';
      const lastRun = rule.last_run_at ? formatDateTime(rule.last_run_at) : '';
      const row = document.createElement('tr');
      row.innerHTML = `
        <td>${TasksPage.escapeHtml(templateTitle)}</td>
        <td>${TasksPage.escapeHtml(schedule)}</td>
        <td>${TasksPage.escapeHtml(nextRun)}</td>
        <td>${TasksPage.escapeHtml(lastRun)}</td>
        <td>${rule.is_active ? t('tasks.templateActive') : t('common.disabled')}</td>
        <td class="actions"></td>
      `;
      const actions = row.querySelector('.actions');
      if (actions) {
        if (hasPermission('tasks.recurring.manage')) {
          const editBtn = document.createElement('button');
          editBtn.type = 'button';
          editBtn.className = 'btn ghost btn-sm';
          editBtn.textContent = t('common.edit');
          editBtn.addEventListener('click', () => editRule(rule.id));
          actions.appendChild(editBtn);

          const toggleBtn = document.createElement('button');
          toggleBtn.type = 'button';
          toggleBtn.className = 'btn ghost btn-sm';
          toggleBtn.textContent = rule.is_active ? t('common.disable') : t('common.enable');
          toggleBtn.addEventListener('click', () => toggleRule(rule));
          actions.appendChild(toggleBtn);
        }
        if (hasPermission('tasks.recurring.run')) {
          const runBtn = document.createElement('button');
          runBtn.type = 'button';
          runBtn.className = 'btn secondary btn-sm';
          runBtn.textContent = t('tasks.actions.runNow');
          runBtn.addEventListener('click', () => runRule(rule.id));
          actions.appendChild(runBtn);
        }
      }
      tbody.appendChild(row);
    });
  }

  function editRule(id) {
    const form = document.getElementById('task-recurring-form');
    if (!hasPermission('tasks.recurring.manage')) {
      currentRuleId = null;
      if (form) form.hidden = true;
      return;
    }
    if (!form) return;
    const runBtn = document.getElementById('task-recurring-run');
    currentRuleId = id || null;
    if (!id) {
      resetRuleForm();
      if (runBtn) runBtn.disabled = true;
      return;
    }
    const rule = state.recurringRules.find(r => `${r.id}` === `${id}`);
    if (!rule) return;
    if (runBtn) runBtn.disabled = !hasPermission('tasks.recurring.run');
    setValue('task-recurring-template', rule.template_id);
    setValue('task-recurring-type', rule.schedule_type);
    setValue('task-recurring-time', rule.time_of_day || '');
    document.getElementById('task-recurring-active').checked = rule.is_active !== false;
    applyScheduleConfig(rule);
    toggleRecurringFields(rule.schedule_type);
    hideAlert('task-recurring-manage-alert');
    form.hidden = false;
  }

  function resetRuleForm() {
    const form = document.getElementById('task-recurring-form');
    if (form) form.hidden = false;
    currentRuleId = null;
    setValue('task-recurring-template', '');
    setValue('task-recurring-type', 'daily');
    setValue('task-recurring-time', '');
    setValue('task-recurring-day', '');
    setValue('task-recurring-month-day', '');
    document.getElementById('task-recurring-active').checked = true;
    clearWeekdays();
    toggleRecurringFields('daily');
  }

  function populateTemplateSelect() {
    const select = document.getElementById('task-recurring-template');
    if (!select) return;
    select.innerHTML = '';
    state.templates.forEach(tpl => {
      const opt = document.createElement('option');
      opt.value = tpl.id;
      opt.textContent = tpl.is_active ? tpl.title_template : `${tpl.title_template} (${t('common.disabled')})`;
      select.appendChild(opt);
    });
  }

  function renderWeekdayCheckboxes() {
    const container = document.getElementById('task-recurring-weekdays');
    if (!container) return;
    container.innerHTML = '';
    const labels = [t('common.sun') || 'Sun', t('common.mon') || 'Mon', t('common.tue') || 'Tue', t('common.wed') || 'Wed', t('common.thu') || 'Thu', t('common.fri') || 'Fri', t('common.sat') || 'Sat'];
    labels.forEach((label, idx) => {
      const wrap = document.createElement('label');
      wrap.className = 'checkbox-inline';
      wrap.innerHTML = `<input type="checkbox" value="${idx}" /> <span>${label}</span>`;
      container.appendChild(wrap);
    });
  }

  function renderMonthOptions() {
    const select = document.getElementById('task-recurring-month');
    if (!select) return;
    select.innerHTML = '';
    for (let i = 1; i <= 12; i += 1) {
      const opt = document.createElement('option');
      opt.value = `${i}`;
      opt.textContent = `${i}`;
      select.appendChild(opt);
    }
  }

  function toggleRecurringFields(type) {
    const weekly = document.getElementById('task-recurring-weekdays-field');
    const monthly = document.getElementById('task-recurring-day-field');
    const monthField = document.getElementById('task-recurring-month-field');
    const monthDay = document.getElementById('task-recurring-month-day-field');
    if (weekly) weekly.hidden = type !== 'weekly';
    if (monthly) monthly.hidden = type !== 'monthly';
    if (monthField) monthField.hidden = !['quarterly', 'semiannual', 'annual'].includes(type);
    if (monthDay) monthDay.hidden = !['quarterly', 'semiannual', 'annual'].includes(type);
  }

  function applyScheduleConfig(rule) {
    const config = parseConfig(rule.schedule_config);
    clearWeekdays();
    if (rule.schedule_type === 'weekly') {
      const days = Array.isArray(config.weekdays) ? config.weekdays : [];
      days.forEach(d => {
        const checkbox = document.querySelector(`#task-recurring-weekdays input[value="${d}"]`);
        if (checkbox) checkbox.checked = true;
      });
    } else if (rule.schedule_type === 'monthly') {
      setValue('task-recurring-day', config.day || '');
    } else if (['quarterly', 'semiannual', 'annual'].includes(rule.schedule_type)) {
      setValue('task-recurring-month', config.month || '');
      setValue('task-recurring-month-day', config.day || '');
    }
  }

  function clearWeekdays() {
    document.querySelectorAll('#task-recurring-weekdays input[type="checkbox"]').forEach(cb => {
      cb.checked = false;
    });
  }

  function buildScheduleConfig(type) {
    if (type === 'weekly') {
      const days = Array.from(document.querySelectorAll('#task-recurring-weekdays input:checked'))
        .map(cb => parseInt(cb.value, 10))
        .filter(n => !Number.isNaN(n));
      return { weekdays: days };
    }
    if (type === 'monthly') {
      const day = parseInt(document.getElementById('task-recurring-day')?.value || '0', 10);
      return { day };
    }
    if (['quarterly', 'semiannual', 'annual'].includes(type)) {
      const month = parseInt(document.getElementById('task-recurring-month')?.value || '0', 10);
      const day = parseInt(document.getElementById('task-recurring-month-day')?.value || '0', 10);
      return { month, day };
    }
    return {};
  }

  function describeSchedule(rule) {
    const type = rule.schedule_type || '';
    const config = parseConfig(rule.schedule_config);
    const label = t(`tasks.recurring.types.${type}`) || type;
    if (type === 'weekly') {
      const days = Array.isArray(config.weekdays) ? config.weekdays : [];
      const labels = days.map(d => weekdayLabel(d)).filter(Boolean);
      return `${label}: ${labels.join(', ')}`;
    }
    if (type === 'monthly') {
      return `${label}: ${config.day || ''}`;
    }
    if (['quarterly', 'semiannual', 'annual'].includes(type)) {
      return `${label}: ${config.month || ''}/${config.day || ''}`;
    }
    return label;
  }

  function weekdayLabel(day) {
    switch (day) {
      case 0: return t('common.sun');
      case 1: return t('common.mon');
      case 2: return t('common.tue');
      case 3: return t('common.wed');
      case 4: return t('common.thu');
      case 5: return t('common.fri');
      case 6: return t('common.sat');
      default: return '';
    }
  }

  function parseConfig(raw) {
    try {
      if (!raw) return {};
      if (typeof raw === 'string') return JSON.parse(raw);
      return raw;
    } catch (_) {
      return {};
    }
  }

  async function saveRule() {
    if (!hasPermission('tasks.recurring.manage')) return;
    hideAlert('task-recurring-manage-alert');
    const templateId = parseInt(document.getElementById('task-recurring-template')?.value || '0', 10);
    const type = document.getElementById('task-recurring-type')?.value || 'daily';
    const timeOfDay = (document.getElementById('task-recurring-time')?.value || '').trim();
    const isActive = document.getElementById('task-recurring-active')?.checked !== false;
    if (!templateId) {
      showAlert('task-recurring-manage-alert', t('tasks.templateRequired'));
      return;
    }
    if (!timeOfDay) {
      showAlert('task-recurring-manage-alert', t('tasks.recurring.timeRequired'));
      return;
    }
    const scheduleConfig = buildScheduleConfig(type);
    const payload = {
      template_id: templateId,
      schedule_type: type,
      schedule_config: scheduleConfig,
      time_of_day: timeOfDay,
      is_active: isActive
    };
    try {
      if (currentRuleId) {
        await Api.put(`/api/tasks/recurring/${currentRuleId}`, payload);
      } else {
        await Api.post('/api/tasks/recurring', payload);
      }
      await openRecurring(currentRuleId);
    } catch (err) {
      showAlert('task-recurring-manage-alert', resolveErrorMessage(err, 'common.error'));
    }
  }

  async function toggleRule(rule) {
    if (!hasPermission('tasks.recurring.manage')) return;
    try {
      await Api.post(`/api/tasks/recurring/${rule.id}/toggle`, { is_active: !rule.is_active });
      await openRecurring(currentRuleId);
    } catch (err) {
      showAlert('task-recurring-alert', resolveErrorMessage(err, 'common.error'));
    }
  }

  async function runRule(id) {
    if (!hasPermission('tasks.recurring.run')) return;
    try {
      const task = await Api.post(`/api/tasks/recurring/${id}/run-now`, {});
      if (task && task.board_id === state.boardId) {
        state.tasks.push(task);
        state.taskMap.set(task.id, task);
        if (TasksPage.renderBoard) TasksPage.renderBoard();
      }
      await openRecurring(currentRuleId);
    } catch (err) {
      showAlert('task-recurring-alert', resolveErrorMessage(err, 'common.error'));
    }
  }

  function runNow() {
    if (currentRuleId) {
      runRule(currentRuleId);
    }
  }

  function describeRecurringRule(id) {
    if (!id) return '';
    const rule = state.recurringRules.find(r => `${r.id}` === `${id}`);
    if (!rule) return `#${id}`;
    return describeSchedule(rule);
  }

  function setValue(id, val) {
    const el = document.getElementById(id);
    if (el !== null && el !== undefined) el.value = val;
  }

  TasksPage.initRecurring = initRecurring;
  TasksPage.openRecurring = openRecurring;
  TasksPage.describeRecurringRule = describeRecurringRule;
})();
