(() => {
  const state = IncidentsPage.state;
  const { t, showError, escapeHtml } = IncidentsPage;

  function buildCreateAttachmentsHtml() {
    return `
      <div class="card incident-attachments-card">
        <div class="card-header">
          <h4>${t('incidents.create.attachmentsTitle')}</h4>
          <button type="button" class="btn ghost" id="incident-attachments-add">${t('incidents.create.attachmentsButton')}</button>
        </div>
        <div class="card-body">
          <div class="dropzone" id="incident-attachments-dropzone">
            <div class="dropzone-title">${t('incidents.create.attachmentsDrop')}</div>
            <div class="dropzone-subtitle">${t('incidents.create.attachmentsHint')}</div>
          </div>
          <div class="attachments-list" id="incident-attachments-list"></div>
        </div>
      </div>`;
  }

  function bindCreateAttachments(tabId) {
    const tab = state.tabs.find(t => t.id === tabId);
    const panel = document.querySelector(`#incidents-panels [data-tab="${tabId}"]`);
    if (!tab || !panel) return;
    ensureAttachmentsDraft(tab);
    const dropzone = panel.querySelector('#incident-attachments-dropzone');
    const addBtn = panel.querySelector('#incident-attachments-add');
    const list = panel.querySelector('#incident-attachments-list');
    if (!dropzone || !addBtn || !list) return;

    const fileInput = document.createElement('input');
    fileInput.type = 'file';
    fileInput.multiple = true;
    fileInput.hidden = true;
    fileInput.addEventListener('change', () => {
      if (fileInput.files && fileInput.files.length) {
        addFiles(tab, Array.from(fileInput.files));
      }
      fileInput.value = '';
    });
    panel.appendChild(fileInput);

    addBtn.addEventListener('click', () => fileInput.click());

    dropzone.addEventListener('dragover', (e) => {
      e.preventDefault();
      dropzone.classList.add('dragover');
    });
    dropzone.addEventListener('dragleave', () => dropzone.classList.remove('dragover'));
    dropzone.addEventListener('drop', (e) => {
      e.preventDefault();
      dropzone.classList.remove('dragover');
      const files = Array.from(e.dataTransfer?.files || []);
      if (files.length) addFiles(tab, files);
    });

    renderAttachmentsList(tab, list);
  }

  function ensureAttachmentsDraft(tab) {
    if (!tab.draft) tab.draft = { dirty: false };
    if (!Array.isArray(tab.draft.attachments)) tab.draft.attachments = [];
  }

  function addFiles(tab, files) {
    ensureAttachmentsDraft(tab);
    files.forEach(file => {
      if (!file) return;
      tab.draft.attachments.push(file);
    });
    const panel = document.querySelector(`#incidents-panels [data-tab="${tab.id}"]`);
    const list = panel?.querySelector('#incident-attachments-list');
    if (list) renderAttachmentsList(tab, list);
    tab.draft.dirty = true;
  }

  function renderAttachmentsList(tab, list) {
    ensureAttachmentsDraft(tab);
    list.innerHTML = '';
    if (!tab.draft.attachments.length) {
      list.innerHTML = `<div class="attachments-empty">${escapeHtml(t('incidents.create.attachmentsEmpty'))}</div>`;
      return;
    }
    tab.draft.attachments.forEach((file, idx) => {
      const item = document.createElement('div');
      item.className = 'attachment-item';
      item.innerHTML = `
        <div class="attachment-meta">
          <div class="attachment-name">${escapeHtml(file.name)}</div>
          <div class="attachment-size">${escapeHtml(formatBytes(file.size))}</div>
        </div>
        <button type="button" class="btn ghost" data-index="${idx}">${t('common.delete') || 'Delete'}</button>`;
      item.querySelector('button')?.addEventListener('click', () => {
        tab.draft.attachments.splice(idx, 1);
        renderAttachmentsList(tab, list);
      });
      list.appendChild(item);
    });
  }

  function formatBytes(bytes) {
    if (!bytes) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB'];
    let value = bytes;
    let unit = 0;
    while (value >= 1024 && unit < units.length - 1) {
      value /= 1024;
      unit += 1;
    }
    return `${value.toFixed(value >= 10 || unit === 0 ? 0 : 1)} ${units[unit]}`;
  }

  async function uploadCreateAttachment(incidentId, file) {
    try {
      const fd = new FormData();
      fd.append('file', file);
      await Api.upload(`/api/incidents/${encodeURIComponent(incidentId)}/attachments/upload`, fd);
    } catch (err) {
      showError(err, 'incidents.attachments.uploadFailed');
      throw err;
    }
  }

  function getCreateAttachments(tabId) {
    const tab = state.tabs.find(t => t.id === tabId);
    if (!tab || !tab.draft || !Array.isArray(tab.draft.attachments)) return [];
    return tab.draft.attachments.slice();
  }

  IncidentsPage.buildCreateAttachmentsHtml = buildCreateAttachmentsHtml;
  IncidentsPage.bindCreateAttachments = bindCreateAttachments;
  IncidentsPage.uploadCreateAttachment = uploadCreateAttachment;
  IncidentsPage.getCreateAttachments = getCreateAttachments;
})();
