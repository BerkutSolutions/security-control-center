(() => {
  const state = DocsPage.state;
  const selectedValues = (selector) => Array.from(document.querySelector(selector)?.selectedOptions || []).map(o => o.value);

  async function loadFolders() {
    try {
      const res = await Api.get('/api/docs/folders');
      state.folders = res.folders || [];
      state.folderMap = {};
      state.folders.forEach(f => { state.folderMap[f.id] = f; });
      if (state.selectedFolder && !state.folderMap[state.selectedFolder]) {
        state.selectedFolder = null;
      }
      console.log('[docs] loadFolders', { count: state.folders.length });
      fillFolderSelects();
      DocsPage.renderDocs(state.docs);
    } catch (err) {
      console.error('load folders', err);
    }
  }

  function fillFolderSelects() {
    const selects = ['create-folder', 'import-folder', 'template-folder'];
    selects.forEach(id => {
      const sel = document.getElementById(id);
      if (!sel) return;
      sel.innerHTML = `<option value="">${BerkutI18n.t('docs.rootFolder')}</option>`;
      state.folders.forEach(f => {
        const opt = document.createElement('option');
        opt.value = f.id;
        opt.textContent = f.name;
        sel.appendChild(opt);
      });
      sel.value = state.selectedFolder || '';
    });
  }

  function getChildFolders(parentId) {
    const pid = parentId || null;
    return state.folders.filter(f => (f.parent_id || null) === pid);
  }

  function selectFolder(folderId) {
    state.selectedFolder = folderId || null;
    fillFolderSelects();
    DocsPage.loadDocs();
  }

  function openFolderModal(folder) {
    const form = document.getElementById('folder-form');
    const alertBox = document.getElementById('folder-alert');
    const title = document.getElementById('folder-modal-title');
    if (!form) return;
    DocsPage.hideAlert(alertBox);
    DocUI.populateClassificationSelect(document.getElementById('folder-classification'));
    form.reset();
    const idInput = document.getElementById('folder-id');
    const parentInput = document.getElementById('folder-parent');
    if (idInput) idInput.value = folder?.id || '';
    if (parentInput) parentInput.value = folder ? (folder.parent_id || '') : (state.selectedFolder || '');
    const nameInput = form.querySelector('input[name="name"]');
    if (nameInput) nameInput.value = folder?.name || '';
    const tags = (folder?.classification_tags || []).map(t => t.toUpperCase());
    DocUI.renderTagCheckboxes('#folder-tags', { name: 'folder_tags', selected: tags });
    const clsSel = document.getElementById('folder-classification');
    if (clsSel) {
      const levelVal = folder
        ? (typeof folder.classification_level === 'number' ? DocUI.levelCodeByIndex(folder.classification_level) : (folder.classification_level || DocUI.levelCodeByIndex(0)))
        : DocUI.levelCodeByIndex(0);
      clsSel.value = levelVal;
    }
    if (title) title.textContent = folder ? BerkutI18n.t('docs.folderEditTitle') : BerkutI18n.t('docs.folderCreateTitle');
    DocsPage.openModal('#folder-modal');
  }

  function bindFolderForm() {
    const form = document.getElementById('folder-form');
    const alertBox = document.getElementById('folder-alert');
    if (!form) return;
    form.onsubmit = async (e) => {
      e.preventDefault();
      DocsPage.hideAlert(alertBox);
      const data = DocsPage.formDataToObj(new FormData(form));
      const payload = {
        name: data.name,
        parent_id: DocsPage.parseNullableInt(data.parent_id),
        classification_level: data.classification_level || 'PUBLIC',
        classification_tags: selectedValues('#folder-tags'),
      };
      try {
        if (data.id) {
          await Api.put(`/api/docs/folders/${data.id}`, payload);
        } else {
          await Api.post('/api/docs/folders', payload);
        }
        DocsPage.closeModal('#folder-modal');
        await loadFolders();
        await DocsPage.loadDocs();
      } catch (err) {
        const msg = err.message || BerkutI18n.t('common.accessDenied') || 'Access denied';
        DocsPage.showAlert(alertBox, msg);
      }
    };
  }

  async function deleteFolder(folderId) {
    if (!folderId) return;
    if (!confirm(BerkutI18n.t('docs.folderDeleteConfirm'))) return;
    try {
      await Api.del(`/api/docs/folders/${folderId}`);
      if (state.selectedFolder === folderId) {
        state.selectedFolder = null;
      }
      await loadFolders();
      await DocsPage.loadDocs();
    } catch (err) {
      console.error('delete folder', err);
    }
  }

  function folderClassification(folderId) {
    if (!folderId) return null;
    const f = state.folderMap[folderId];
    if (!f) return null;
    const levelCode = DocUI.levelCodeByIndex(typeof f.classification_level === 'number' ? f.classification_level : (f.classification_level || 0));
    return { levelCode };
  }

  function syncFolderClassification(prefix) {
    const folderSel = document.getElementById(`${prefix}-folder`);
    const clsSel = document.getElementById(`${prefix}-classification`);
    if (!folderSel || !clsSel) return;
    const fId = DocsPage.parseNullableInt(folderSel.value);
    const f = folderClassification(fId);
    if (f) {
      clsSel.value = f.levelCode;
      clsSel.setAttribute('disabled', 'true');
    } else {
      clsSel.removeAttribute('disabled');
    }
  }

  DocsPage.loadFolders = loadFolders;
  DocsPage.fillFolderSelects = fillFolderSelects;
  DocsPage.getChildFolders = getChildFolders;
  DocsPage.selectFolder = selectFolder;
  DocsPage.openFolderModal = openFolderModal;
  DocsPage.bindFolderForm = bindFolderForm;
  DocsPage.deleteFolder = deleteFolder;
  DocsPage.folderClassification = folderClassification;
  DocsPage.syncFolderClassification = syncFolderClassification;
})();
