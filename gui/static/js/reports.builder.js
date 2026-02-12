(() => {
  const state = ReportsPage.state;

  function bindBuilder() {
    const form = document.getElementById('report-create-form');
    const modeRadios = document.querySelectorAll('input[name="mode"]');
    const templateRow = document.getElementById('report-template-row');
    modeRadios.forEach(r => {
      r.addEventListener('change', () => {
        const selected = document.querySelector('input[name="mode"]:checked')?.value || 'empty';
        if (templateRow) templateRow.hidden = selected !== 'template';
      });
    });
    if (form) {
      form.onsubmit = async (e) => {
        e.preventDefault();
        await createReport();
      };
    }
    const createModal = document.getElementById('report-create-modal');
    if (createModal) createModal.hidden = true;
    const closeBtn = document.getElementById('report-editor-close');
    if (closeBtn) closeBtn.onclick = () => closeEditor();
    const saveBtn = document.getElementById('report-editor-save');
    if (saveBtn) saveBtn.onclick = () => saveContent();
    const metaSaveBtn = document.getElementById('report-editor-meta-save');
    if (metaSaveBtn) metaSaveBtn.onclick = () => saveMeta();
    const previewBtn = document.getElementById('report-editor-preview-btn');
    if (previewBtn) previewBtn.onclick = () => togglePreview();
    bindToolbar();
    populateTags();
    populateOwnerAndAcl();
  }

  async function createReport() {
    ReportsPage.showAlert('report-create-alert', '');
    const title = (document.getElementById('report-title')?.value || '').trim();
    if (!title) {
      ReportsPage.showAlert('report-create-alert', BerkutI18n.t('reports.error.titleRequired'));
      return;
    }
    const mode = document.querySelector('input[name="mode"]:checked')?.value || 'empty';
    const payload = collectPayload();
    try {
      let res;
      if (mode === 'template') {
        const tpl = document.getElementById('report-template-select')?.value;
        payload.template_id = parseInt(tpl, 10) || 0;
        if (!payload.template_id) {
          ReportsPage.showAlert('report-create-alert', BerkutI18n.t('reports.error.templateNotFound'));
          return;
        }
        res = await Api.post('/api/reports/from-template', payload);
      } else {
        res = await Api.post('/api/reports', payload);
      }
      const doc = res.doc || res.document || res;
      if (doc?.id) {
        await ReportsPage.loadReports();
        closeCreateModal();
        if (ReportsPage.openViewer) {
          ReportsPage.openViewer(doc.id);
        } else {
          ReportsPage.openEditor(doc.id);
        }
      }
    } catch (err) {
      ReportsPage.showAlert('report-create-alert', err.message || BerkutI18n.t('common.error'));
    }
  }

  function openCreateModal(opts = {}) {
    const modal = document.getElementById('report-create-modal');
    if (modal) modal.hidden = false;
    ReportsPage.showAlert('report-create-alert', '');
    if (!opts.preserveValues) resetCreateForm();
  }

  function closeCreateModal() {
    const modal = document.getElementById('report-create-modal');
    if (modal) modal.hidden = true;
  }

  function resetCreateForm() {
    const form = document.getElementById('report-create-form');
    if (form) form.reset();
    const templateRow = document.getElementById('report-template-row');
    if (templateRow) templateRow.hidden = true;
    const emptyMode = document.querySelector('input[name="mode"][value="empty"]');
    if (emptyMode) emptyMode.checked = true;
    const defaultOwner = document.getElementById('report-owner');
    if (defaultOwner && state.currentUser) defaultOwner.value = state.currentUser.id;
    const inheritAcl = document.getElementById('report-inherit-acl');
    if (inheritAcl) inheritAcl.checked = true;
    if (DocUI?.bindTagHint) {
      const tags = document.getElementById('report-tags');
      const hint = document.querySelector('[data-tag-hint="report-tags"]');
      if (tags && hint) DocUI.bindTagHint(tags, hint);
    }
  }

  function collectPayload() {
    const tags = Array.from(document.getElementById('report-tags')?.selectedOptions || []).map(o => o.value);
    const aclRoles = Array.from(document.getElementById('report-acl-roles')?.selectedOptions || []).map(o => o.value);
    const aclUsers = Array.from(document.getElementById('report-acl-users')?.selectedOptions || []).map(o => parseInt(o.value, 10)).filter(Boolean);
    const owner = parseInt(document.getElementById('report-owner')?.value || '0', 10);
    return {
      title: (document.getElementById('report-title')?.value || '').trim(),
      classification_level: document.getElementById('report-classification')?.value || '',
      classification_tags: tags,
      period_from: ReportsPage.toISODateInput(document.getElementById('report-period-from')?.value || ''),
      period_to: ReportsPage.toISODateInput(document.getElementById('report-period-to')?.value || ''),
      owner: owner || undefined,
      acl_roles: aclRoles,
      acl_users: aclUsers,
      inherit_acl: document.getElementById('report-inherit-acl')?.checked || false
    };
  }

  async function openEditor(id, opts = {}) {
    if (!id) return;
    ReportsPage.showAlert('report-editor-alert', '');
    try {
      const metaRes = await Api.get(`/api/reports/${id}`);
      const contentRes = await Api.get(`/api/reports/${id}/content`);
      state.editor.id = id;
      state.editor.meta = metaRes;
      state.editor.content = contentRes.content || '';
      const reason = document.getElementById('report-editor-reason');
      if (reason) reason.value = '';
      renderEditor(metaRes, contentRes.content || '');
      setEditorMode(opts.mode === 'view' ? 'view' : 'edit');
      document.getElementById('report-editor')?.removeAttribute('hidden');
      if (!opts.skipRoute && ReportsPage.updateReportsPath) {
        ReportsPage.updateReportsPath(id, 'edit');
      }
      if (ReportsPage.loadSections) await ReportsPage.loadSections(id);
      if (ReportsPage.loadCharts) await ReportsPage.loadCharts(id);
    } catch (err) {
      ReportsPage.showAlert('report-create-alert', err.message || BerkutI18n.t('common.error'));
    }
  }

  function renderEditor(metaRes, content) {
    const doc = metaRes.doc || metaRes.document || {};
    const meta = metaRes.meta || {};
    const title = document.getElementById('report-editor-title');
    if (title) title.textContent = `${doc.title || ''} (${doc.reg_number || ''})`;
    const contentEl = document.getElementById('report-editor-content');
    if (contentEl) contentEl.value = content || '';
    const metaTitle = document.getElementById('report-editor-title-input');
    if (metaTitle) metaTitle.value = doc.title || '';
    const status = document.getElementById('report-editor-status');
    if (status) status.value = meta.report_status || meta.status || 'draft';
    const pf = document.getElementById('report-editor-period-from');
    if (pf) pf.value = ReportsPage.formatDateInput(meta.period_from);
    const pt = document.getElementById('report-editor-period-to');
    if (pt) pt.value = ReportsPage.formatDateInput(meta.period_to);
    const cls = document.getElementById('report-editor-classification');
    if (cls) cls.value = DocUI.levelCodeByIndex(doc.classification_level);
    const tags = (doc.classification_tags || []).map(t => t.toUpperCase());
    DocUI.renderTagCheckboxes('#report-editor-tags', { className: 'editor-tag', selected: tags });
  }

  async function saveContent() {
    if (!state.editor.id) return;
    const reason = (document.getElementById('report-editor-reason')?.value || '').trim();
    if (!reason) {
      ReportsPage.showAlert('report-editor-alert', BerkutI18n.t('editor.reasonRequired'));
      return;
    }
    try {
      const content = document.getElementById('report-editor-content')?.value || '';
      await Api.put(`/api/reports/${state.editor.id}/content`, { content, reason });
      ReportsPage.showAlert('report-editor-alert', BerkutI18n.t('editor.saved'), true);
    } catch (err) {
      ReportsPage.showAlert('report-editor-alert', err.message || BerkutI18n.t('common.error'));
    }
  }

  async function saveMeta() {
    if (!state.editor.id) return;
    const tags = Array.from(document.getElementById('report-editor-tags')?.selectedOptions || []).map(o => o.value);
    const payload = {
      title: (document.getElementById('report-editor-title-input')?.value || '').trim(),
      status: document.getElementById('report-editor-status')?.value || 'draft',
      period_from: ReportsPage.toISODateInput(document.getElementById('report-editor-period-from')?.value || ''),
      period_to: ReportsPage.toISODateInput(document.getElementById('report-editor-period-to')?.value || ''),
      classification_level: document.getElementById('report-editor-classification')?.value || '',
      classification_tags: tags
    };
    try {
      await Api.put(`/api/reports/${state.editor.id}`, payload);
      await ReportsPage.loadReports();
      ReportsPage.showAlert('report-editor-alert', BerkutI18n.t('common.saved'), true);
    } catch (err) {
      ReportsPage.showAlert('report-editor-alert', err.message || BerkutI18n.t('common.error'));
    }
  }

  function togglePreview(force) {
    const preview = document.getElementById('report-editor-preview');
    const content = document.getElementById('report-editor-content')?.value || '';
    if (!preview) return;
    const next = typeof force === 'boolean' ? force : preview.hidden;
    if (next) {
      preview.innerHTML = renderMarkdown(content);
      preview.hidden = false;
    } else {
      preview.hidden = true;
    }
  }

  function setEditorMode(mode) {
    const panel = document.getElementById('report-editor');
    const textarea = document.getElementById('report-editor-content');
    const preview = document.getElementById('report-editor-preview');
    const toolbar = document.getElementById('report-editor-toolbar');
    const viewOnly = mode === 'view';
    if (panel) panel.classList.toggle('view-only', viewOnly);
    if (toolbar) toolbar.hidden = viewOnly;
    if (textarea) textarea.hidden = viewOnly;
    if (preview && viewOnly) {
      togglePreview(true);
    } else if (preview && !viewOnly) {
      preview.hidden = true;
    }
  }

  async function openViewer(id) {
    if (ReportsPage.switchTab) {
      ReportsPage.switchTab('reports-tab-home', { skipRoute: true });
    }
    await openEditor(id, { mode: 'view', skipRoute: true });
    if (ReportsPage.updateReportsPath) {
      ReportsPage.updateReportsPath(id, 'view');
    }
  }

  function renderMarkdown(md) {
    if (typeof DocsPage !== 'undefined' && DocsPage.renderMarkdown) {
      const rendered = DocsPage.renderMarkdown(md || '');
      return rendered.html || '';
    }
    return `<pre>${escapeHtml(md || '')}</pre>`;
  }

  function closeEditor() {
    const panel = document.getElementById('report-editor');
    if (panel) panel.hidden = true;
    state.editor.id = null;
    state.editor.meta = null;
    state.editor.content = '';
    if (ReportsPage.updateReportsPath) {
      ReportsPage.updateReportsPath(null, ReportsPage.state?.activeTabId || 'reports-tab-home');
    }
  }

  function bindToolbar() {
    const toolbar = document.getElementById('report-editor-toolbar');
    const textarea = document.getElementById('report-editor-content');
    if (!toolbar || !textarea) return;
    toolbar.querySelectorAll('button[data-action]').forEach(btn => {
      btn.onclick = () => applyFormatting(btn.dataset.action, textarea);
    });
  }

  function applyFormatting(action, textarea) {
    if (!textarea) return;
    const start = textarea.selectionStart;
    const end = textarea.selectionEnd;
    const selected = textarea.value.substring(start, end);
    let replacement = selected;
    switch (action) {
      case 'bold':
        replacement = `**${selected || BerkutI18n.t('editor.placeholder')}**`;
        break;
      case 'italic':
        replacement = `*${selected || BerkutI18n.t('editor.placeholder')}*`;
        break;
      case 'heading':
        replacement = `## ${selected || BerkutI18n.t('editor.placeholder')}`;
        break;
      case 'list':
        replacement = selected.split('\n').map(line => line ? `- ${line}` : '- ').join('\n');
        break;
      case 'quote':
        replacement = selected.split('\n').map(line => `> ${line || ''}`).join('\n');
        break;
      case 'code':
        replacement = `\`\`\`\n${selected || BerkutI18n.t('editor.placeholder')}\n\`\`\``;
        break;
      case 'link':
        replacement = `[${selected || BerkutI18n.t('editor.placeholder')}]()`;
        break;
      case 'table':
        replacement = `| Col1 | Col2 |\n| --- | --- |\n| ${selected || 'text'} |  |`;
        break;
    }
    textarea.setRangeText(replacement, start, end, 'end');
    textarea.focus();
  }

  function populateTags() {
    DocUI.renderTagCheckboxes('#report-tags', { className: 'report-tag' });
    DocUI.renderTagCheckboxes('#report-editor-tags', { className: 'editor-tag' });
  }

  async function populateOwnerAndAcl() {
    if (!UserDirectory) return;
    await UserDirectory.load();
    const owner = document.getElementById('report-owner');
    if (owner) {
      owner.innerHTML = '';
      UserDirectory.all().forEach(u => {
        const opt = document.createElement('option');
        opt.value = u.id;
        opt.textContent = u.full_name || u.username;
        owner.appendChild(opt);
      });
      if (state.currentUser) owner.value = state.currentUser.id;
    }
    const rolesSel = document.getElementById('report-acl-roles');
    const roleOptions = ['superadmin', 'admin', 'security_officer', 'doc_admin', 'doc_editor', 'doc_reviewer', 'doc_viewer', 'auditor', 'manager', 'analyst'];
    if (rolesSel) {
      rolesSel.innerHTML = '';
      roleOptions.forEach(r => {
        const opt = document.createElement('option');
        opt.value = r;
        opt.textContent = r;
        rolesSel.appendChild(opt);
      });
    }
    const usersSel = document.getElementById('report-acl-users');
    if (usersSel) {
      usersSel.innerHTML = '';
      UserDirectory.all().forEach(u => {
        const opt = document.createElement('option');
        opt.value = u.id;
        opt.textContent = u.full_name || u.username;
        usersSel.appendChild(opt);
      });
    }
    if (DocsPage?.enhanceMultiSelects) {
      DocsPage.enhanceMultiSelects(['report-acl-roles', 'report-acl-users']);
    }
    if (DocsPage?.attachSelectedPreview) {
      DocsPage.attachSelectedPreview(rolesSel);
      DocsPage.attachSelectedPreview(usersSel);
    }
  }

  function escapeHtml(str) {
    return (str || '').toString().replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  }

  ReportsPage.bindBuilder = bindBuilder;
  ReportsPage.openEditor = openEditor;
  ReportsPage.openViewer = openViewer;
  ReportsPage.openCreateModal = openCreateModal;
  ReportsPage.closeCreateModal = closeCreateModal;
  ReportsPage.applySettingsToBuilder = (settings) => {
    if (!settings) return;
    const cls = document.getElementById('report-classification');
    if (cls && settings.default_classification) {
      cls.value = settings.default_classification;
    }
    const tpl = document.getElementById('report-template-select');
    if (tpl && settings.default_template_id) {
      tpl.value = settings.default_template_id;
    }
  };
})();
