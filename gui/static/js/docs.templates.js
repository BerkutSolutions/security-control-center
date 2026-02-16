(() => {
  const state = DocsPage.state;
  const selectedValues = (selector, root = document) => Array.from(root.querySelector(selector)?.selectedOptions || []).map(o => o.value);

  function bindTemplateForm() {
    const form = document.getElementById('template-form');
    const alertBox = document.getElementById('templates-alert');
    if (!form) return;
    form.onsubmit = async (e) => {
      e.preventDefault();
      DocsPage.hideAlert(alertBox);
      const tplId = form.dataset.templateId;
      const tpl = state.templates.find(t => `${t.id}` === `${tplId}`);
      if (!tpl || !tpl.content) {
        DocsPage.showAlert(alertBox, BerkutI18n.t('docs.templateNoContent'));
        return;
      }
      const data = DocsPage.formDataToObj(new FormData(form));
      const vars = {};
      form.querySelectorAll('[data-var-name]').forEach(inp => { vars[inp.dataset.varName] = inp.value; });
      const filledContent = DocsPage.renderTemplate(tpl.content, vars);
      const payload = {
        title: data.title,
        folder_id: DocsPage.parseNullableInt(data.folder_id),
        classification_level: data.classification_level || 'PUBLIC',
        classification_tags: selectedValues('#template-tags'),
        inherit_acl: true,
      };
      let doc;
      try {
        doc = await Api.post('/api/docs', payload);
      } catch (err) {
        DocsPage.showAlert(alertBox, err.message);
        return;
      }
      try {
        await Api.put(`/api/docs/${doc.id}/content`, { content: filledContent, format: tpl.format || 'md', reason: 'template' });
      } catch (err) {
        DocsPage.showAlert(alertBox, err.message);
        return;
      }
      DocsPage.closeModal('#templates-modal');
      await DocsPage.loadDocs();
      DocsPage.openEditor(doc.id);
    };
  }

  function bindTemplateManagement() {
    const form = document.getElementById('template-edit-form');
    const alertBox = document.getElementById('template-manage-alert');
    const addVarBtn = document.getElementById('template-add-var');
    const varsContainer = document.getElementById('template-edit-vars');
    const newBtn = document.getElementById('btn-new-template');
    const deleteBtn = document.getElementById('template-delete');
    if (addVarBtn && varsContainer) {
      addVarBtn.onclick = () => addTemplateVar(varsContainer, {});
    }
    if (newBtn) {
      newBtn.onclick = () => {
        resetTemplateEditForm();
        if (form) form.hidden = false;
        if (varsContainer) varsContainer.innerHTML = '';
      };
    }
    if (form) {
      form.onsubmit = async (e) => {
        e.preventDefault();
        DocsPage.hideAlert(alertBox);
        const varsList = varsContainer ? Array.from(varsContainer.querySelectorAll('.template-var-row')) : [];
        const vars = varsList.map(row => ({
          name: row.querySelector('.var-name').value,
          label: row.querySelector('.var-label').value,
          default: row.querySelector('.var-default').value,
        })).filter(v => v.name);
        const payload = {
          id: form.dataset.templateId ? parseInt(form.dataset.templateId, 10) : 0,
          name: form.querySelector('input[name="name"]').value,
          description: form.querySelector('input[name="description"]').value,
          format: form.querySelector('select[name="format"]').value || 'md',
          content: document.getElementById('template-edit-content').value,
          variables: vars,
          classification_level: document.getElementById('template-edit-classification').value,
          classification_tags: selectedValues('#template-edit-tags'),
        };
        try {
          await Api.post('/api/templates', payload);
          await openTemplates();
          DocsPage.showAlert(alertBox, BerkutI18n.t('docs.templateSaved'), true);
        } catch (err) {
          DocsPage.showAlert(alertBox, err.message || 'save failed');
        }
      };
    }
    if (deleteBtn) {
      deleteBtn.onclick = async () => {
        const id = form && form.dataset.templateId;
        if (!id) return;
        if (!confirm(BerkutI18n.t('docs.templateDeleteConfirm'))) return;
        try {
          await Api.del(`/api/templates/${id}`);
          await openTemplates();
        } catch (err) {
          DocsPage.showAlert(alertBox, err.message || 'delete failed');
        }
      };
    }
  }

  async function openTemplates() {
    DocUI.populateClassificationSelect(document.getElementById('template-classification'));
    DocUI.populateClassificationSelect(document.getElementById('template-edit-classification'));
    DocUI.renderTagCheckboxes('#template-tags', { name: 'tags', selected: [] });
    DocUI.renderTagCheckboxes('#template-edit-tags', { name: 'edit-tags', selected: [] });
    const alertBox = document.getElementById('templates-alert');
    DocsPage.hideAlert(alertBox);
    try {
      const res = await Api.get('/api/templates').catch(async () => Api.get('/api/docs/templates'));
      state.templates = res.templates || res.items || [];
      renderTemplatesList();
      resetTemplateEditForm();
      DocsPage.openModal('#templates-modal');
    } catch (err) {
      DocsPage.showAlert(alertBox, err.message || 'templates unavailable');
    }
  }

  function renderTemplatesList() {
    const container = document.getElementById('templates-list');
    const form = document.getElementById('template-form');
    if (!container || !form) return;
    container.innerHTML = '';
    if (!state.templates.length) {
      container.textContent = BerkutI18n.t('docs.templatesEmpty');
      form.hidden = true;
      return;
    }
    state.templates.forEach(tpl => {
      const card = document.createElement('div');
      card.className = 'template-card';
      card.innerHTML = `
        <div class="template-head">
          <div>
            <div class="template-name">${DocsPage.escapeHtml(tpl.name || '')}</div>
            <div class="muted">${DocsPage.escapeHtml(tpl.description || '')}</div>
          </div>
          <div class="template-actions">
            <button class="btn secondary" data-use="${tpl.id}" data-i18n="docs.useTemplate">${BerkutI18n.t('docs.useTemplate')}</button>
            <button class="btn ghost" data-edit="${tpl.id}" data-i18n="common.edit">${BerkutI18n.t('common.edit')}</button>
            <button class="btn ghost danger" data-delete="${tpl.id}" data-i18n="common.delete">${BerkutI18n.t('common.delete')}</button>
          </div>
        </div>`;
      card.querySelector('[data-use]').onclick = () => selectTemplate(tpl);
      card.querySelector('[data-edit]').onclick = () => editTemplate(tpl);
      card.querySelector('[data-delete]').onclick = () => deleteTemplate(tpl.id);
      container.appendChild(card);
    });
    form.hidden = true;
  }

  function selectTemplate(tpl) {
    const form = document.getElementById('template-form');
    const varsContainer = document.getElementById('template-vars');
    if (!form || !varsContainer) return;
    form.hidden = false;
    form.dataset.templateId = tpl.id;
    form.title.value = tpl.name || '';
    DocUI.renderTagCheckboxes('#template-tags', { name: 'tags', selected: tpl.classification_tags || [] });
    varsContainer.innerHTML = '';
    (tpl.variables || []).forEach(v => {
      const field = document.createElement('div');
      field.className = 'form-field';
      field.innerHTML = `
        <label>${DocsPage.escapeHtml(v.label || v.name)}</label>
        <input data-var-name="${v.name}" value="${DocsPage.escapeHtml(v.default || '')}">
      `;
      varsContainer.appendChild(field);
    });
  }

  function resetTemplateEditForm() {
    const form = document.getElementById('template-edit-form');
    const vars = document.getElementById('template-edit-vars');
    const deleteBtn = document.getElementById('template-delete');
    if (!form) return;
    form.hidden = true;
    form.dataset.templateId = '';
    form.reset();
    DocUI.renderTagCheckboxes('#template-edit-tags', { name: 'edit-tags', selected: [] });
    if (vars) vars.innerHTML = '';
    if (deleteBtn) deleteBtn.hidden = true;
  }

  function editTemplate(tpl) {
    const form = document.getElementById('template-edit-form');
    const vars = document.getElementById('template-edit-vars');
    if (!form || !vars) return;
    form.hidden = false;
    form.dataset.templateId = tpl.id;
    form.querySelector('input[name="name"]').value = tpl.name || '';
    form.querySelector('input[name="description"]').value = tpl.description || '';
    form.querySelector('select[name="format"]').value = tpl.format || 'md';
    const classSelect = document.getElementById('template-edit-classification');
    if (classSelect) classSelect.value = DocUI.levelCodeByIndex(tpl.classification_level || 0);
    DocUI.renderTagCheckboxes('#template-edit-tags', { name: 'edit-tags', selected: tpl.classification_tags || [] });
    vars.innerHTML = '';
    (tpl.variables || []).forEach(v => addTemplateVar(vars, v));
    const deleteBtn = document.getElementById('template-delete');
    if (deleteBtn) deleteBtn.hidden = false;
    const content = document.getElementById('template-edit-content');
    if (content) content.value = tpl.content || '';
  }

  function addTemplateVar(container, v = {}) {
    const row = document.createElement('div');
    row.className = 'template-var-row';
    row.innerHTML = `
      <input placeholder="name" class="var-name" value="${DocsPage.escapeHtml(v.name || '')}">
      <input placeholder="label" class="var-label" value="${DocsPage.escapeHtml(v.label || '')}">
      <input placeholder="default" class="var-default" value="${DocsPage.escapeHtml(v.default || '')}">
      <button class="btn ghost" type="button">${BerkutI18n.t('common.delete')}</button>
    `;
    row.querySelector('button').onclick = () => row.remove();
    container.appendChild(row);
  }

  async function deleteTemplate(id) {
    if (!confirm(BerkutI18n.t('docs.templateDeleteConfirm'))) return;
    try {
      await Api.del(`/api/templates/${id}`);
      await openTemplates();
    } catch (err) {
      DocsPage.showAlert(document.getElementById('templates-alert'), err.message);
    }
  }

  DocsPage.bindTemplateForm = bindTemplateForm;
  DocsPage.bindTemplateManagement = bindTemplateManagement;
  DocsPage.openTemplates = openTemplates;
  DocsPage.renderTemplatesList = renderTemplatesList;
  DocsPage.selectTemplate = selectTemplate;
  DocsPage.resetTemplateEditForm = resetTemplateEditForm;
  DocsPage.editTemplate = editTemplate;
  DocsPage.addTemplateVar = addTemplateVar;
  DocsPage.deleteTemplate = deleteTemplate;
})();
