const DocEditor = (() => {
  let currentDocId = null;
  let currentFormat = 'md';
  let meta = null;
  let currentMode = 'view';
  let initialContent = '';
  let dirty = false;
  let callbacks = {};
  const els = {};

  function init(opts = {}) {
    callbacks = opts;
    els.panel = document.getElementById('doc-editor');
    if (!els.panel) return;
    console.log('[editor] init: panel found, initial hidden=', els.panel.hidden);
    els.title = document.getElementById('editor-title');
    els.content = document.getElementById('editor-content');
    els.viewer = document.getElementById('editor-viewer');
    els.pdfFrame = document.getElementById('editor-pdf-frame');
    els.nonMdBlock = document.getElementById('editor-nonmd');
    els.nonMdHint = document.getElementById('editor-nonmd-hint');
    els.downloadBtn = document.getElementById('editor-download');
    els.convertBtn = document.getElementById('editor-convert');
    els.reason = document.getElementById('editor-reason');
    els.alert = document.getElementById('editor-alert');
    els.mdView = document.getElementById('editor-md-view');
    els.editToggle = document.getElementById('editor-edit-toggle');
    els.classification = document.getElementById('editor-classification');
    els.tags = document.getElementById('editor-tags');
    els.status = document.getElementById('editor-status');
    els.owner = document.getElementById('editor-owner');
    els.reg = document.getElementById('editor-reg');
    els.folder = document.getElementById('editor-folder');
    els.toolbar = document.getElementById('editor-toolbar');
    els.saveBtn = document.getElementById('editor-save');
    els.closeBtn = document.getElementById('editor-close');
    els.linkType = document.getElementById('link-type');
    els.linkId = document.getElementById('link-id');
    els.addLink = document.getElementById('add-link');
    els.links = document.getElementById('editor-links');
    els.aclRoles = document.getElementById('editor-acl-roles');
    els.aclUsers = document.getElementById('editor-acl-users');
    els.aclSave = document.getElementById('editor-acl-save');
    els.aclRefresh = document.getElementById('editor-acl-refresh');
    DocUI.populateClassificationSelect(els.classification);
    renderEditorTags([]);
    bindToolbar();
    bindButtons();
    bindDirtyTracking();
    document.addEventListener('tags:changed', () => {
      const current = Array.from(document.querySelectorAll('#editor-tags option:checked')).map(opt => opt.value);
      if (current.length) {
        renderEditorTags(current);
      } else if (meta && meta.classification_tags) {
        renderEditorTags(meta.classification_tags);
      } else {
        renderEditorTags([]);
      }
    });
  }

  function bindToolbar() {
    if (!els.toolbar) return;
    els.toolbar.querySelectorAll('button[data-action]').forEach(btn => {
      btn.onclick = () => applyFormatting(btn.dataset.action);
    });
  }

  function bindButtons() {
    if (els.saveBtn) els.saveBtn.onclick = () => save();
    if (els.closeBtn) els.closeBtn.onclick = () => {
      if (callbacks.onClose) {
        callbacks.onClose(currentDocId);
      } else {
        close();
      }
    };
    if (els.editToggle) {
      els.editToggle.onclick = () => {
        if (currentMode === 'view') {
          setMode('edit');
        } else {
          setMode('view');
        }
      };
    }
    if (els.convertBtn) els.convertBtn.onclick = () => convertToMarkdown();
    if (els.downloadBtn) els.downloadBtn.onclick = () => download();
    if (els.addLink) els.addLink.onclick = () => addLink();
    if (els.addLinkInline) els.addLinkInline.onclick = () => addLink();
    if (els.aclSave) els.aclSave.onclick = () => saveAcl();
    if (els.aclRefresh) els.aclRefresh.onclick = () => loadAcl();
  }

  async function open(docId, opts = {}) {
    if (!els.panel) return;
    currentDocId = docId;
    currentMode = opts.mode === 'edit' ? 'edit' : 'view';
    setDirty(false);
    showAlert('');
    if (els.reason) els.reason.value = '';
    console.log('[editor] open', { docId, panelHidden: els.panel.hidden });
    try {
      meta = await Api.get(`/api/docs/${docId}`);
      let cont;
      try {
        cont = await Api.get(`/api/docs/${docId}/content`);
      } catch (err) {
        const msg = (err && err.message ? err.message : '').toLowerCase();
        if (msg.includes('not found')) {
          cont = { format: (meta && meta.format) || 'md', content: '' };
        } else {
          throw err;
        }
      }
      currentFormat = cont.format || 'md';
      renderMeta(meta);
      await renderContent(cont);
      await loadLinks();
      await loadAclOptions();
      await loadAcl();
      els.panel.hidden = false;
      console.log('[editor] open success', { docId, format: currentFormat, hidden: els.panel.hidden });
      return meta;
    } catch (err) {
      console.error('[editor] open failed', { docId, err: err.message });
      showAlert(err.message || 'Failed to load document');
      return null;
    }
  }

  function renderMeta(doc) {
    els.title.textContent = `${doc.title || ''} (${doc.reg_number || ''})`;
    const code = DocUI.levelCodeByIndex(doc.classification_level);
    if (els.classification) els.classification.value = code;
    const tags = (doc.classification_tags || []).map(t => t.toUpperCase());
    renderEditorTags(tags);
    if (els.status) els.status.textContent = DocUI.statusLabel(doc.status) || '-';
    if (els.owner) els.owner.textContent = (UserDirectory ? UserDirectory.name(doc.created_by) : (doc.created_by || '')) || '-';
    if (els.reg) els.reg.textContent = doc.reg_number || '-';
    if (els.folder) {
      if (doc.folder_id) {
        const folder = (window.DocsPage && DocsPage.state && DocsPage.state.folders || []).find(f => f.id === doc.folder_id);
        els.folder.textContent = folder ? folder.name : `#${doc.folder_id}`;
      } else {
        els.folder.textContent = '-';
      }
    }
  }

  function renderEditorTags(selected = []) {
    DocUI.renderTagCheckboxes('#editor-tags', { className: 'editor-tag', selected });
  }

  async function renderContent(res) {
    const format = (res.format || '').toLowerCase();
    console.log('[editor] renderContent', { docId: currentDocId, format, viewerHidden: els.viewer.hidden, contentHidden: els.content.hidden });
    if (format === 'md' || format === 'txt') {
      currentFormat = format;
      els.panel.style.display = 'flex';
      initialContent = res.content || '';
      setDirty(false);
      if (currentMode === 'view') {
        els.content.hidden = true;
        els.viewer.hidden = true;
        if (els.mdView) {
          els.mdView.hidden = false;
          renderMarkdownView(initialContent);
        }
      } else {
        els.content.hidden = false;
        if (els.mdView) els.mdView.hidden = true;
        els.viewer.hidden = true;
        els.content.value = initialContent;
        els.content.focus();
      }
    } else if (format === 'pdf') {
      els.panel.style.display = 'flex';
      if (els.mdView) els.mdView.hidden = true;
      els.content.hidden = true;
      els.viewer.hidden = false;
      els.nonMdBlock.hidden = true;
      els.pdfFrame.hidden = false;
      els.pdfFrame.src = `/api/docs/${currentDocId}/export?format=pdf&inline=1`;
    } else if (format === 'docx') {
      currentFormat = format;
      els.panel.style.display = 'flex';
      if (els.content) els.content.hidden = true;
      if (els.viewer) els.viewer.hidden = true;
      if (els.pdfFrame) els.pdfFrame.hidden = true;
      if (els.nonMdBlock) els.nonMdBlock.hidden = true;
      if (els.mdView) {
        els.mdView.hidden = false;
        els.mdView.innerHTML = '';
      }
      const docxUrl = `/api/docs/${currentDocId}/export?format=docx&inline=1`;
      const rendered = await renderDocxView(docxUrl);
      if (!rendered) {
        if (els.mdView) els.mdView.hidden = true;
        if (els.viewer) els.viewer.hidden = false;
        if (els.nonMdBlock) els.nonMdBlock.hidden = false;
        if (els.nonMdHint) els.nonMdHint.textContent = BerkutI18n.t('docs.nonEditable');
      }
    } else {
      els.panel.style.display = 'flex';
      if (els.mdView) els.mdView.hidden = true;
      els.content.hidden = true;
      els.viewer.hidden = false;
      els.pdfFrame.hidden = true;
      els.nonMdBlock.hidden = false;
      els.nonMdHint.textContent = BerkutI18n.t('docs.nonEditable');
    }
    applyMode();
  }

  async function save() {
    if (!currentDocId) return;
    const reason = (els.reason && els.reason.value || '').trim();
    if (!reason) {
      showAlert(BerkutI18n.t('editor.reasonRequired'));
      return;
    }
    if (currentFormat !== 'md' && currentFormat !== 'txt') {
      showAlert(BerkutI18n.t('editor.readonly'));
      return;
    }
    try {
      const format = currentFormat === 'txt' ? 'txt' : 'md';
      await Api.put(`/api/docs/${currentDocId}/content`, { content: els.content.value, format, reason });
      await maybeUpdateClassification();
      initialContent = els.content.value;
      setDirty(false);
      showAlert(BerkutI18n.t('editor.saved'), true);
      if (callbacks.onSave) callbacks.onSave(currentDocId);
    } catch (err) {
      showAlert(err.message || 'save failed');
    }
  }

  async function maybeUpdateClassification() {
    if (!meta) return;
    const nextLevel = els.classification ? els.classification.value : null;
    const nextTags = Array.from(document.querySelectorAll('#editor-tags option:checked')).map(opt => opt.value);
    const currentLevelCode = DocUI.levelCodeByIndex(meta.classification_level);
    const changed = nextLevel !== currentLevelCode || JSON.stringify(nextTags.sort()) !== JSON.stringify((meta.classification_tags || []).map(t => t.toUpperCase()).sort());
    if (!changed) return;
    try {
      await Api.post(`/api/docs/${currentDocId}/classification`, {
        classification_level: nextLevel,
        classification_tags: nextTags,
        inherit_classification: meta.inherit_classification,
      });
    } catch (err) {
      showAlert(err.message || 'classification not updated');
    }
  }

  async function loadLinks() {
    if (!els.links) return;
    els.links.innerHTML = '';
    try {
      const res = await Api.get(`/api/docs/${currentDocId}/links`);
      (res.links || []).forEach(l => {
        const row = document.createElement('div');
        row.className = 'link-item';
        row.innerHTML = `<span>${escapeHtml(l.target_type)} #${escapeHtml(l.target_id)}</span><button class="btn ghost" data-id="${l.id}">×</button>`;
        row.querySelector('button').onclick = () => removeLink(l.id);
        els.links.appendChild(row);
      });
    } catch (err) {
      console.warn('load links', err);
    }
  }

  async function addLink() {
    if (!currentDocId) return;
    const type = els.linkType.value;
    const id = els.linkId.value.trim();
    if (!type || !id) return;
    await Api.post(`/api/docs/${currentDocId}/links`, { target_type: type, target_id: id });
    els.linkId.value = '';
    await loadLinks();
  }

  async function removeLink(id) {
    await Api.del(`/api/docs/${currentDocId}/links/${id}`);
    await loadLinks();
  }

  function enhanceSelectWithTicks(sel) {
    if (!sel || sel.dataset.enhanced) return;
    sel.dataset.enhanced = '1';
    Array.from(sel.options).forEach(opt => {
      opt.dataset.label = opt.textContent;
      if (opt.selected) opt.textContent = `${opt.dataset.label} ✓`;
    });
    const refresh = () => {
      Array.from(sel.options).forEach(opt => {
        const base = opt.dataset.label || opt.textContent.replace(/ ✓$/, '');
        opt.dataset.label = base;
        opt.textContent = opt.selected ? `${base} ✓` : base;
      });
    };
    sel.addEventListener('mousedown', (e) => {
      const opt = e.target.closest('option');
      if (!opt) return;
      e.preventDefault();
      opt.selected = !opt.selected;
      refresh();
    });
    sel.addEventListener('change', refresh);
    sel.addEventListener('dblclick', (e) => {
      const opt = e.target.closest('option');
      if (!opt) return;
      opt.selected = !opt.selected;
      refresh();
    });
  }

  async function loadAclOptions() {
    if (!els.aclRoles || !els.aclUsers) return;
    await UserDirectory.load();
    const roleOptions = ['superadmin', 'admin', 'security_officer', 'doc_admin', 'doc_editor', 'doc_reviewer', 'doc_viewer', 'auditor', 'manager', 'analyst'];
    els.aclRoles.innerHTML = '';
    roleOptions.forEach(r => {
      const opt = document.createElement('option');
      opt.value = r;
      opt.textContent = r;
      els.aclRoles.appendChild(opt);
    });
    els.aclUsers.innerHTML = '';
    UserDirectory.all().forEach(u => {
      const opt = document.createElement('option');
      opt.value = u.id;
      opt.textContent = u.full_name || u.username;
      els.aclUsers.appendChild(opt);
    });
    enhanceSelectWithTicks(els.aclRoles);
    enhanceSelectWithTicks(els.aclUsers);
  }

  async function loadAcl() {
    if (!currentDocId || !els.aclRoles || !els.aclUsers) return;
    try {
      const res = await Api.get(`/api/docs/${currentDocId}/acl`);
      const acl = res.acl || [];
      const roleSelected = new Set(acl.filter(a => a.subject_type === 'role').map(a => a.subject_id));
      const userSelected = new Set(acl.filter(a => a.subject_type === 'user').map(a => a.subject_id));
      Array.from(els.aclRoles.options).forEach(o => { o.selected = roleSelected.has(o.value); });
      Array.from(els.aclUsers.options).forEach(o => {
        const u = UserDirectory.get(parseInt(o.value, 10));
        o.selected = u && userSelected.has(u.username);
      });
      enhanceSelectWithTicks(els.aclRoles);
      enhanceSelectWithTicks(els.aclUsers);
    } catch (err) {
      console.warn('load acl', err);
    }
  }

  async function saveAcl() {
    if (!currentDocId || !els.aclRoles || !els.aclUsers) return;
    const rolesSel = Array.from(els.aclRoles.selectedOptions).map(o => o.value);
    const usersSel = Array.from(els.aclUsers.selectedOptions).map(o => parseInt(o.value, 10)).filter(Boolean);
    const acl = [];
    rolesSel.forEach(r => {
      ['view', 'edit', 'manage'].forEach(p => acl.push({ subject_type: 'role', subject_id: r, permission: p }));
    });
    usersSel.forEach(uid => {
      const u = UserDirectory.get(uid);
      if (u) {
        ['view', 'edit', 'manage'].forEach(p => acl.push({ subject_type: 'user', subject_id: u.username, permission: p }));
      }
    });
    try {
      await Api.put(`/api/docs/${currentDocId}/acl`, { acl });
    } catch (err) {
      showAlert(err.message || 'ACL save failed');
    }
  }

  async function convertToMarkdown() {
    if (!currentDocId) return;
    try {
      const res = await Api.post(`/api/docs/${currentDocId}/convert`, {});
      currentFormat = 'md';
      els.content.value = res.content || '';
      els.content.hidden = false;
      els.viewer.hidden = true;
      showAlert(BerkutI18n.t('docs.converted'), true);
    } catch (err) {
      showAlert(err.message || 'convert failed');
    }
  }

  function download() {
    if (!currentDocId) return;
    const fmt = currentFormat || 'pdf';
    window.open(`/api/docs/${currentDocId}/export?format=${fmt}`, '_blank');
  }

  function applyFormatting(action) {
    if (!els.content || els.content.hidden) return;
    const textarea = els.content;
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

  function close(opts = {}) {
    if (els.panel) {
      els.panel.hidden = true;
      els.panel.style.display = 'none';
    }
    console.log('[editor] close', { panelHidden: els.panel?.hidden });
    currentDocId = null;
    setDirty(false);
    if (!opts.silent && callbacks.onClose) callbacks.onClose();
  }

  function bindDirtyTracking() {
    if (!els.content) return;
    els.content.addEventListener('input', () => {
      if (currentMode !== 'edit') return;
      setDirty(els.content.value !== initialContent);
    });
  }

  function setDirty(next) {
    dirty = !!next;
  }

  function isDirty() {
    return dirty;
  }

  function setMode(mode) {
    currentMode = mode === 'edit' ? 'edit' : 'view';
    applyMode();
    if (callbacks.onModeChange && currentDocId) {
      callbacks.onModeChange(currentDocId, currentMode);
    }
    if (currentFormat === 'md' || currentFormat === 'txt') {
      if (currentMode === 'view') {
        if (els.mdView) {
          els.mdView.hidden = false;
          renderMarkdownView(els.content?.value || initialContent);
        }
        if (els.content) els.content.hidden = true;
        if (els.viewer) els.viewer.hidden = true;
      } else {
        if (els.mdView) els.mdView.hidden = true;
        if (els.content) {
          els.content.hidden = false;
          if (!els.content.value) els.content.value = initialContent;
          els.content.focus();
        }
      }
    }
  }

  function applyMode() {
    if (!els.panel) return;
    const viewOnly = currentMode !== 'edit';
    els.panel.classList.toggle('view-only', viewOnly);
    if (els.editToggle) {
      els.editToggle.textContent = viewOnly ? (BerkutI18n.t('editor.edit') || 'Edit') : (BerkutI18n.t('editor.view') || 'View');
    }
    const disableInputs = [
      els.classification,
      els.tags,
      els.linkType,
      els.linkId,
      els.aclRoles,
      els.aclUsers
    ];
    disableInputs.forEach(el => {
      if (!el) return;
      el.disabled = viewOnly;
    });
  }

  function renderMarkdownView(content) {
    if (!els.mdView) return;
    if (typeof DocsPage !== 'undefined' && DocsPage.renderMarkdown) {
      const rendered = DocsPage.renderMarkdown(content || '');
      els.mdView.innerHTML = rendered.html || '';
    } else {
      els.mdView.textContent = content || '';
    }
  }

  async function renderDocxView(docxUrl) {
    if (!els.mdView) return false;
    if (!window.mammoth || !window.mammoth.convertToHtml) {
      return false;
    }
    try {
      const res = await fetch(docxUrl, { credentials: 'include' });
      if (!res.ok) {
        throw new Error(await res.text());
      }
      const arrayBuffer = await res.arrayBuffer();
      const result = await window.mammoth.convertToHtml({ arrayBuffer });
      els.mdView.hidden = false;
      const sanitize = (typeof DocsPage !== 'undefined' && DocsPage.sanitizeHtmlFragment)
        ? DocsPage.sanitizeHtmlFragment
        : (html) => String(html || '').replace(/<[^>]*>/g, '');
      els.mdView.innerHTML = `<div class="docx-view">${sanitize(result.value || '')}</div>`;
      if (result.messages && result.messages.length) {
        console.warn('docx render warnings', result.messages);
      }
      return true;
    } catch (err) {
      console.warn('docx render failed', err);
      return false;
    }
  }

  function mount(container) {
    if (!els.panel || !container) return;
    container.appendChild(els.panel);
  }

  function showAlert(msg, success = false) {
    if (!els.alert) return;
    if (!msg) {
      els.alert.hidden = true;
      els.alert.classList.remove('success');
      return;
    }
    els.alert.textContent = msg;
    els.alert.hidden = false;
    els.alert.classList.toggle('success', !!success);
  }

  function escapeHtml(str) {
    return (str || '').toString().replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', '\'': '&#39;' }[c]));
  }

  return { init, open, close, mount, isDirty, setMode };
})();

if (typeof window !== 'undefined') {
  window.DocEditor = DocEditor;
}
