(() => {
  const state = DocsPage.state;

  function exportDoc(docId) {
    const fmt = prompt(BerkutI18n.t('docs.exportPrompt'), 'pdf') || 'pdf';
    window.open(`/api/docs/${docId}/export?format=${encodeURIComponent(fmt)}`, '_blank');
  }

  async function openViewer(docId) {
    const legacyModal = document.getElementById('doc-view-modal');
    if (legacyModal) legacyModal.hidden = true;
    if (DocsPage.openDocTab) {
      DocsPage.openDocTab(docId, 'view');
      return;
    }
    const alertBox = document.getElementById('doc-view-alert');
    DocsPage.hideAlert(alertBox);
    const titleEl = document.getElementById('doc-view-title');
    const metaEl = document.getElementById('doc-view-meta');
    const frameWrap = document.getElementById('doc-viewer-frame-wrap');
    const frame = document.getElementById('doc-viewer-frame');
    const mdPane = document.getElementById('doc-viewer-md');
    const frameHint = document.getElementById('doc-viewer-frame-hint');
    if (mdPane) mdPane.innerHTML = '';
    if (frame) {
      frame.src = 'about:blank';
      frame.hidden = false;
    }
    if (frameHint) frameHint.textContent = '';
    if (frameWrap) frameWrap.hidden = true;
    if (mdPane) mdPane.hidden = true;
    state.viewerDoc = null;
    state.viewerContent = '';
    state.viewerFormat = 'md';
    try {
      if (DocsPage.updateDocsPath) {
        DocsPage.updateDocsPath(docId, 'view');
      }
      const meta = await Api.get(`/api/docs/${docId}`);
      state.viewerDoc = meta;
      if (titleEl) titleEl.textContent = `${meta.title || ''} (${meta.reg_number || ''})`;
      if (metaEl) metaEl.textContent = `${DocUI.levelName(meta.classification_level)} | ${DocUI.statusLabel(meta.status)} | ${DocsPage.formatDate(meta.updated_at || meta.created_at)}`;
      await renderDocInfo(meta, docId);
      let cont;
      try {
        cont = await Api.get(`/api/docs/${docId}/content`);
      } catch (err) {
        const msg = (err && err.message ? err.message : '').toLowerCase();
        if (msg.includes('not found')) {
          cont = { format: meta.format || 'md', content: '' };
        } else {
          throw err;
        }
      }
      const fmt = (cont.format || meta.format || 'md').toLowerCase();
      state.viewerFormat = fmt;
      if (fmt === 'md' || fmt === 'txt') {
        state.viewerContent = cont.content || '';
        renderViewerMarkdown(state.viewerContent);
      } else {
        const isPdf = fmt === 'pdf';
        const isDocx = fmt === 'docx';
        const converters = state.converterStatus || {};
        const pdfUrl = `/api/docs/${docId}/export?format=pdf&inline=1`;
        const docxUrl = `/api/docs/${docId}/export?format=docx&inline=1`;
        const docxDownloadUrl = `/api/docs/${docId}/export?format=docx`;
        if (isPdf) {
          if (frame) {
            frame.src = pdfUrl;
            frame.hidden = false;
          }
          if (frameWrap) frameWrap.hidden = false;
          if (frameHint) {
            frameHint.textContent = BerkutI18n.t('docs.view.pdfHint') || '';
          }
        } else if (isDocx) {
          const rendered = await renderViewerDocx(docxUrl);
          if (!rendered) {
            if (frameWrap) frameWrap.hidden = false;
          }
          if (!rendered && frameHint) {
            const hint = BerkutI18n.t('docs.view.docxHint') || '';
            const linkLabel = BerkutI18n.t('docs.view.docxDownload') || 'Download';
            frameHint.innerHTML = `${hint} <a href="${docxDownloadUrl}" target="_blank" rel="noreferrer">${linkLabel}</a>`;
          }
        } else if (converters.enabled && converters.soffice_available) {
          if (frame) {
            frame.src = pdfUrl;
            frame.hidden = false;
          }
          if (frameWrap) frameWrap.hidden = false;
        }
      }
      applyZoom(100);
      resetSearch();
    } catch (err) {
      DocsPage.showAlert(alertBox, err.message || 'Failed to open');
    }
  }

  async function renderDocInfo(meta, docId) {
    const createdEl = document.getElementById('doc-view-created');
    const updatedEl = document.getElementById('doc-view-updated');
    if (createdEl) createdEl.textContent = DocsPage.formatDate(meta.created_at);
    if (updatedEl) updatedEl.textContent = DocsPage.formatDate(meta.updated_at || meta.created_at);
    const versionsBtn = document.getElementById('doc-view-versions-btn');
    if (versionsBtn) {
      versionsBtn.onclick = () => DocsPage.openVersions(docId);
      versionsBtn.textContent = BerkutI18n.t('docs.view');
    }
    try {
      const verRes = await Api.get(`/api/docs/${docId}/versions`);
      if (versionsBtn) versionsBtn.textContent = `${BerkutI18n.t('docs.versionsTitle')} (${(verRes.versions || []).length})`;
    } catch (err) {
      console.warn('versions info', err);
    }
    const aclBox = document.getElementById('doc-view-acl');
    if (aclBox) {
      aclBox.textContent = '';
      try {
        const aclRes = await Api.get(`/api/docs/${docId}/acl`);
        const acl = aclRes.acl || [];
        const users = acl.filter(a => a.subject_type === 'user').map(a => a.subject_id);
        const roles = acl.filter(a => a.subject_type === 'role').map(a => a.subject_id);
        const parts = [];
        const rolesLabel = BerkutI18n.t('accounts.roles') || 'Roles';
        const usersLabel = BerkutI18n.t('accounts.users') || 'Users';
        if (roles.length) parts.push(`${rolesLabel}: ${roles.join(', ')}`);
        if (users.length) parts.push(`${usersLabel}: ${users.join(', ')}`);
        aclBox.textContent = parts.join(' | ') || BerkutI18n.t('docs.aclEmpty') || '-';
      } catch (err) {
        aclBox.textContent = '-';
        console.warn('acl info', err);
      }
    }
    const linksBox = document.getElementById('doc-view-links');
    if (linksBox) {
      linksBox.textContent = '';
      try {
        const linksRes = await Api.get(`/api/docs/${docId}/links`);
        if ((linksRes.links || []).length === 0) {
          linksBox.textContent = BerkutI18n.t('docs.linksEmpty') || '-';
        } else {
          linksRes.links.forEach(l => {
            const span = document.createElement('span');
            span.className = 'tag';
            span.textContent = `${l.target_type} #${l.target_id}`;
            linksBox.appendChild(span);
          });
        }
      } catch (err) {
        linksBox.textContent = '-';
        console.warn('links info', err);
      }
    }
    const controlsBox = document.getElementById('doc-view-controls');
    if (controlsBox) {
      controlsBox.textContent = '';
      try {
        const res = await Api.get(`/api/docs/${docId}/control-links`);
        const items = res.items || [];
        if (!items.length) {
          controlsBox.textContent = BerkutI18n.t('docs.controlsEmpty') || '-';
        } else {
          items.forEach(item => {
            const link = document.createElement('a');
            link.className = 'tag';
            link.href = `/controls?control=${item.control_id}`;
            link.textContent = `${item.code} - ${item.title}`;
            controlsBox.appendChild(link);
          });
        }
      } catch (err) {
        controlsBox.textContent = '-';
        console.warn('controls links info', err);
      }
    }
  }

  function renderViewerMarkdown(content) {
    const mdPane = document.getElementById('doc-viewer-md');
    if (!mdPane) return;
    mdPane.hidden = false;
    const rendered = renderMarkdown(content || '');
    mdPane.innerHTML = rendered.html;
    mdPane.querySelectorAll('[data-code-idx]').forEach(btn => {
      btn.onclick = async () => {
        const idx = parseInt(btn.getAttribute('data-code-idx'), 10);
        const code = (rendered.codeBlocks[idx] && rendered.codeBlocks[idx].code) || '';
        try {
          await navigator.clipboard.writeText(code);
          btn.textContent = 'Copied';
          setTimeout(() => { btn.textContent = 'Copy'; }, 1200);
        } catch (_) {
          btn.textContent = 'Error';
          setTimeout(() => { btn.textContent = 'Copy'; }, 1200);
        }
      };
    });
  }

  async function renderViewerDocx(docxUrl) {
    const mdPane = document.getElementById('doc-viewer-md');
    const frameWrap = document.getElementById('doc-viewer-frame-wrap');
    const frame = document.getElementById('doc-viewer-frame');
    if (!mdPane) return false;
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
      if (frame) {
        frame.src = 'about:blank';
        frame.hidden = true;
      }
      if (frameWrap) frameWrap.hidden = true;
      mdPane.hidden = false;
      mdPane.innerHTML = `<div class="docx-view">${result.value || ''}</div>`;
      if (result.messages && result.messages.length) {
        console.warn('docx render warnings', result.messages);
      }
      return true;
    } catch (err) {
      console.warn('docx render failed', err);
      return false;
    }
  }

  function renderMarkdown(md) {
    const esc = (str) => (str || '').replace(/[&<>"]/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
    const codeBlocks = [];
    const mdWithoutCodes = (md || '').replace(/```(\w+)?\n([\s\S]*?)```/g, (_, lang, code) => {
      const idx = codeBlocks.length;
      codeBlocks.push({ code: code || '', lang: lang || '' });
      return `@@CODE${idx}@@`;
    });

    const blocks = mdWithoutCodes.split(/\n{2,}/).map(b => {
      const lines = b.trim().split('\n');
      if (lines.length >= 2 && lines.every(l => l.trim().startsWith('|'))) {
        const [header, separator, ...rows] = lines;
        const headers = header.split('|').filter(Boolean).map(c => esc(c.trim()));
        const body = rows.map(r => r.split('|').filter(Boolean).map(c => esc(c.trim())));
        const headHtml = `<tr>${headers.map(h => `<th>${h}</th>`).join('')}</tr>`;
        const bodyHtml = body.map(r => `<tr>${r.map(c => `<td>${c}</td>`).join('')}</tr>`).join('');
        return `<table class="md-table"><thead>${headHtml}</thead><tbody>${bodyHtml}</tbody></table>`;
      }
      return esc(b);
    }).join('\n\n');

    let html = blocks;
    html = html.replace(/^###### (.*)$/gm, '<h6>$1</h6>')
      .replace(/^##### (.*)$/gm, '<h5>$1</h5>')
      .replace(/^#### (.*)$/gm, '<h4>$1</h4>')
      .replace(/^### (.*)$/gm, '<h3>$1</h3>')
      .replace(/^## (.*)$/gm, '<h2>$1</h2>')
      .replace(/^# (.*)$/gm, '<h1>$1</h1>')
      .replace(/!\[([^\]]*)\]\(([^)]+)\)/g, '<img alt="$1" src="$2">')
      .replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" target="_blank" rel="noreferrer">$1</a>')
      .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
      .replace(/__(.+?)__/g, '<strong>$1</strong>')
      .replace(/\*(.+?)\*/g, '<em>$1</em>')
      .replace(/_(.+?)_/g, '<em>$1</em>')
      .replace(/~~(.+?)~~/g, '<del>$1</del>')
      .replace(/`([^`\n]+)`/g, '<code>$1</code>')
      .replace(/^> (.*)$/gm, '<blockquote><p>$1</p></blockquote>')
      .replace(/^- (.*)$/gm, '<li>$1</li>')
      .replace(/(\r?\n){2,}/g, '</p><p>')
      .replace(/\r?\n/g, '<br>');

    html = html.replace(/@@CODE(\d+)@@/g, (_, idx) => {
      const i = parseInt(idx, 10);
      const block = codeBlocks[i] || { code: '', lang: '' };
      const code = esc(block.code);
      return `<div class="code-block"><div class="code-block-bar"><span class="code-lang">${esc(block.lang || 'code')}</span><button type="button" class="btn ghost icon-btn copy-code-btn" data-code-idx="${i}">Copy</button></div><pre><code>${code}</code></pre></div>`;
    });
    return { html: `<div class="md-view"><p>${html}</p></div>`, codeBlocks };
  }

  function bindViewerControls() {
    const zoomInput = document.getElementById('view-zoom');
    const zoomLabel = document.getElementById('view-zoom-label');
    if (zoomInput) {
      zoomInput.oninput = () => applyZoom(parseInt(zoomInput.value, 10) || 100);
      if (zoomLabel) zoomLabel.textContent = `${zoomInput.value}%`;
    }
    const searchBtn = document.getElementById('view-search-btn');
    const searchInput = document.getElementById('view-search');
    if (searchBtn && searchInput) {
      searchBtn.onclick = () => runSearch(searchInput.value.trim());
      searchInput.onkeypress = (e) => {
        if (e.key === 'Enter') {
          e.preventDefault();
          runSearch(searchInput.value.trim());
        }
      };
    }
    const editBtn = document.getElementById('view-edit-btn');
    if (editBtn) editBtn.onclick = () => {
      if (state.viewerDoc?.id) {
        DocsPage.closeModal('#doc-view-modal');
        DocsPage.openEditor(state.viewerDoc.id);
      }
    };
  }

  function applyZoom(val) {
    const zoomInput = document.getElementById('view-zoom');
    const zoomLabel = document.getElementById('view-zoom-label');
    const mdPane = document.getElementById('doc-viewer-md');
    const frame = document.getElementById('doc-viewer-frame');
    const zoom = Math.min(200, Math.max(50, val || 100));
    if (zoomInput) zoomInput.value = zoom;
    if (zoomLabel) zoomLabel.textContent = `${zoom}%`;
    if (mdPane && !mdPane.hidden) mdPane.style.fontSize = `${zoom}%`;
    if (frame && !frame.hidden) {
      frame.style.transformOrigin = '0 0';
      frame.style.transform = `scale(${zoom / 100})`;
      frame.style.width = `${10000 / zoom}%`;
      frame.style.height = `${10000 / zoom}%`;
    }
  }

  function runSearch(term) {
    resetSearch();
    if (!term) return;
    if (state.viewerFormat !== 'md' && state.viewerFormat !== 'txt') return;
    const mdPane = document.getElementById('doc-viewer-md');
    if (!mdPane) return;
    const regex = new RegExp(`(${term.replace(/[.*+?^${}()|[\\]\\\\]/g, '\\\\$&')})`, 'gi');
    mdPane.innerHTML = renderMarkdown(state.viewerContent || '').replace(regex, '<mark>$1</mark>');
  }

  function resetSearch() {
    if (state.viewerFormat === 'md' || state.viewerFormat === 'txt') {
      renderViewerMarkdown(state.viewerContent || '');
    }
  }

  DocsPage.exportDoc = exportDoc;
  DocsPage.openViewer = openViewer;
  DocsPage.renderDocInfo = renderDocInfo;
  DocsPage.renderViewerMarkdown = renderViewerMarkdown;
  DocsPage.renderMarkdown = renderMarkdown;
  DocsPage.bindViewerControls = bindViewerControls;
  DocsPage.applyZoom = applyZoom;
  DocsPage.runSearch = runSearch;
  DocsPage.resetSearch = resetSearch;
})();
