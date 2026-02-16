(() => {
  const state = ReportsPage.state;

  function bindTemplates() {
    const newBtn = document.getElementById('reports-template-new');
    if (newBtn) newBtn.onclick = () => openTemplateForm();
    const form = document.getElementById('reports-template-form');
    if (form) {
      form.onsubmit = async (e) => {
        e.preventDefault();
        await saveTemplate();
      };
    }
    const delBtn = document.getElementById('reports-template-delete');
    if (delBtn) delBtn.onclick = () => deleteTemplate();
  }

  async function loadTemplates() {
    try {
      const res = await Api.get('/api/reports/templates');
      state.templates = res.templates || [];
      renderTemplates();
      populateTemplateSelects();
    } catch (err) {
      console.warn('load templates', err);
    }
  }

  function renderTemplates() {
    const list = document.getElementById('reports-templates-list');
    if (!list) return;
    list.innerHTML = '';
    state.templates.forEach(tpl => {
      const card = document.createElement('div');
      card.className = 'template-card';
      card.innerHTML = `
        <div class="template-head">
          <div>
            <div class="template-name">${escapeHtml(tpl.name)}</div>
            <p class="muted">${escapeHtml(tpl.description || '')}</p>
          </div>
          <div class="template-actions">
            <button class="btn ghost" data-action="use">${BerkutI18n.t('reports.template.use')}</button>
            <button class="btn secondary" data-action="edit">${BerkutI18n.t('common.edit')}</button>
          </div>
        </div>
        <pre class="template-preview">${escapeHtml(tpl.template_markdown || '')}</pre>
      `;
      card.querySelector('[data-action="use"]').onclick = () => useTemplate(tpl);
      card.querySelector('[data-action="edit"]').onclick = () => openTemplateForm(tpl);
      list.appendChild(card);
    });
  }

  function populateTemplateSelects() {
    const selects = ['report-template-select', 'reports-settings-template'];
    selects.forEach(id => {
      const sel = document.getElementById(id);
      if (!sel) return;
      sel.innerHTML = '';
      const empty = document.createElement('option');
      empty.value = '';
      empty.textContent = BerkutI18n.t('common.none');
      sel.appendChild(empty);
      state.templates.forEach(t => {
        const opt = document.createElement('option');
        opt.value = t.id;
        opt.textContent = t.name;
        sel.appendChild(opt);
      });
    });
  }

  function openTemplateForm(tpl) {
    const form = document.getElementById('reports-template-form');
    if (!form) return;
    const list = document.getElementById('reports-templates-list');
    if (list && form.previousElementSibling !== null) {
      list.parentNode.insertBefore(form, list);
    }
    form.hidden = false;
    document.getElementById('reports-template-id').value = tpl?.id || '';
    document.getElementById('reports-template-name').value = tpl?.name || '';
    document.getElementById('reports-template-description').value = tpl?.description || '';
    document.getElementById('reports-template-markdown').value = tpl?.template_markdown || '';
    const delBtn = document.getElementById('reports-template-delete');
    if (delBtn) delBtn.hidden = !tpl?.id;
    form.scrollIntoView({ behavior: 'smooth', block: 'start' });
  }

  async function saveTemplate() {
    const payload = {
      id: parseInt(document.getElementById('reports-template-id').value || '0', 10) || 0,
      name: document.getElementById('reports-template-name').value.trim(),
      description: document.getElementById('reports-template-description').value.trim(),
      template_markdown: document.getElementById('reports-template-markdown').value
    };
    if (!payload.name) {
      ReportsPage.showAlert('reports-templates-alert', BerkutI18n.t('reports.error.templateNameRequired'));
      return;
    }
    try {
      await Api.post('/api/reports/templates', payload);
      ReportsPage.showAlert('reports-templates-alert', BerkutI18n.t('common.saved'), true);
      await loadTemplates();
    } catch (err) {
      ReportsPage.showAlert('reports-templates-alert', err.message || BerkutI18n.t('common.error'));
    }
  }

  async function deleteTemplate() {
    const id = parseInt(document.getElementById('reports-template-id').value || '0', 10);
    if (!id) return;
    if (!window.confirm(BerkutI18n.t('reports.template.deleteConfirm'))) return;
    try {
      await Api.del(`/api/reports/templates/${id}`);
      ReportsPage.showAlert('reports-templates-alert', BerkutI18n.t('reports.template.deleted'), true);
      document.getElementById('reports-template-form').hidden = true;
      await loadTemplates();
    } catch (err) {
      ReportsPage.showAlert('reports-templates-alert', err.message || BerkutI18n.t('common.error'));
    }
  }

  function useTemplate(tpl) {
    const mode = document.querySelector('input[name="mode"][value="template"]');
    if (mode) mode.checked = true;
    const row = document.getElementById('report-template-row');
    if (row) row.hidden = false;
    const sel = document.getElementById('report-template-select');
    if (sel) sel.value = tpl.id;
    if (ReportsPage.openCreateModal) {
      ReportsPage.openCreateModal({ preserveValues: true });
    }
  }

  function escapeHtml(str) {
    return (str || '').toString().replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  }

  ReportsPage.bindTemplates = bindTemplates;
  ReportsPage.loadTemplates = loadTemplates;
})();
