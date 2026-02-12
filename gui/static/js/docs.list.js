(() => {
  const state = DocsPage.state;

  async function loadDocs() {
    const params = new URLSearchParams();
    if (state.selectedFolder) params.set('folder_id', state.selectedFolder);
    if (state.filters.status) params.set('status', state.filters.status);
    if (state.filters.search) params.set('q', state.filters.search);
    if (state.filters.mine) params.set('mine', '1');
    if (state.filters.review) params.set('status_in', 'review');
    if (state.filters.secret) params.set('min_level', '4');
    if (state.filters.tags && state.filters.tags.length) params.set('tags', state.filters.tags.join(','));
    params.set('limit', '200');
    let res;
    try {
      res = await Api.get(`/api/docs?${params.toString()}`);
      console.log('[docs] loadDocs success', { count: res.items?.length || 0, folder: state.selectedFolder });
    } catch (err) {
      console.error('load docs', err);
      renderDocs([]);
      return;
    }
    state.converterStatus = res.converters || null;
    let items = res.items || [];
    if (!state.selectedFolder) {
      items = items.filter(d => !d.folder_id);
    }
    state.docs = items;
    renderConverterBanner();
    renderDocs(state.docs);
  }

  function renderConverterBanner() {
    const banner = document.getElementById('converter-banner');
    if (!banner) return;
    const st = state.converterStatus;
    if (!st || st.enabled) {
      banner.hidden = true;
      return;
    }
    const reason = st.message || BerkutI18n.t('docs.converterUnavailable');
    banner.textContent = `${BerkutI18n.t('docs.converterWarning')}: ${reason}`;
    banner.hidden = false;
  }

  function renderDocs(items) {
    const tbody = document.querySelector('#docs-table tbody');
    const count = document.getElementById('docs-count');
    if (count) count.textContent = `${items.length}`;
    if (!tbody) return;
    tbody.innerHTML = '';
    const childFolders = DocsPage.getChildFolders(state.selectedFolder);
    if (state.selectedFolder) {
      const current = state.folderMap[state.selectedFolder];
      const parentId = current ? (current.parent_id || null) : null;
      const up = document.createElement('tr');
      up.className = 'folder-row folder-up';
      up.dataset.type = 'folder-up';
      up.innerHTML = `
        <td></td>
        <td><span class="folder-name-cell">.. ${BerkutI18n.t('docs.folderUp')}</span></td>
        <td colspan="6"></td>
      `;
      up.onclick = () => DocsPage.selectFolder(parentId);
      tbody.appendChild(up);
    }
    childFolders.forEach(folder => {
      const tr = document.createElement('tr');
      tr.dataset.folderId = folder.id;
      tr.dataset.type = 'folder';
      tr.classList.add('folder-row');
      tr.innerHTML = `
        <td></td>
        <td><span class="folder-name-cell">[DIR] ${DocsPage.escapeHtml(folder.name || '')}</span></td>
        <td>${BerkutI18n.t('docs.folderLabel')}</td>
        <td>-</td>
        <td>${DocUI.levelName(folder.classification_level)}</td>
        <td>${DocUI.tagsText(folder.classification_tags)}</td>
        <td></td>
        <td></td>
      `;
      tr.onclick = () => DocsPage.selectFolder(folder.id);
      tbody.appendChild(tr);
    });
    if (!childFolders.length && !items.length) {
      const tr = document.createElement('tr');
      tr.innerHTML = `<td colspan="8">${BerkutI18n.t('docs.empty')}</td>`;
      tbody.appendChild(tr);
      return;
    }
    items.forEach(doc => {
      const tr = document.createElement('tr');
      tr.dataset.id = doc.id;
      tr.dataset.type = 'doc';
      const tags = DocUI.tagsText(doc.classification_tags);
      const owner = state.currentUser && doc.created_by === state.currentUser.id ? BerkutI18n.t('docs.owner.me') : UserDirectory.name(doc.created_by || '');
      tr.innerHTML = `
        <td>${DocsPage.escapeHtml(doc.reg_number || '')}</td>
        <td>${DocsPage.escapeHtml(doc.title || '')}</td>
        <td>${(doc.format || 'md').toUpperCase()}</td>
        <td><span class="badge status-${doc.status}">${DocUI.statusLabel(doc.status)}</span></td>
        <td>${DocUI.levelName(doc.classification_level)}</td>
        <td>${DocsPage.escapeHtml(tags || '-')}</td>
        <td>${DocsPage.escapeHtml(owner || '')}</td>
        <td>${DocsPage.formatDate(doc.updated_at || doc.created_at)}</td>
      `;
      tr.onclick = () => DocsPage.openDocTab(doc.id, 'view');
      tr.ondblclick = (e) => { e.stopPropagation(); DocsPage.openDocTab(doc.id, 'edit'); };
      tbody.appendChild(tr);
    });
  }

  DocsPage.loadDocs = loadDocs;
  DocsPage.renderConverterBanner = renderConverterBanner;
  DocsPage.renderDocs = renderDocs;
})();
