(() => {
  const state = IncidentsPage.state;
  const { t, showError, escapeHtml, formatDate } = IncidentsPage;

  function bindAttachmentControls(incidentId) {
    const tabId = `incident-${incidentId}`;
    const panel = document.querySelector(`#incidents-panels [data-tab="${tabId}"]`);
    if (!panel) return;
    const uploadBtn = panel.querySelector('.incident-attachment-upload');
    const fileInput = panel.querySelector('.incident-attachment-file');
    const filter = panel.querySelector('.incident-attachment-filter');
    if (uploadBtn && fileInput) {
      uploadBtn.onclick = () => fileInput.click();
      fileInput.onchange = async () => {
        const file = fileInput.files && fileInput.files[0];
        if (!file) return;
        try {
          const fd = new FormData();
          fd.append('file', file);
          await Api.upload(`/api/incidents/${incidentId}/attachments/upload`, fd);
          fileInput.value = '';
          await ensureIncidentAttachments(incidentId, true);
        } catch (err) {
          showError(err, 'incidents.attachments.uploadFailed');
        }
      };
    }
    if (filter) {
      filter.onchange = () => {
        const detail = state.incidentDetails.get(incidentId);
        if (detail) {
          detail.attachmentFilter = filter.value;
          renderIncidentAttachments(incidentId);
        }
      };
    }
  }

  async function ensureIncidentAttachments(incidentId, force) {
    const detail = state.incidentDetails.get(incidentId);
    if (!detail) return;
    if (detail.attachmentsLoaded && !force) {
      renderIncidentAttachments(incidentId);
      return;
    }
    detail.attachmentsLoading = true;
    try {
      const res = await Api.get(`/api/incidents/${incidentId}/attachments`);
      detail.attachments = res.items || [];
      detail.attachmentsLoaded = true;
    } catch (err) {
      detail.attachments = [];
      showError(err, 'incidents.attachments.loadFailed');
    } finally {
      detail.attachmentsLoading = false;
      renderIncidentAttachments(incidentId);
    }
  }

  function renderIncidentAttachments(incidentId) {
    const tabId = `incident-${incidentId}`;
    const panel = document.querySelector(`#incidents-panels [data-tab="${tabId}"]`);
    const detail = state.incidentDetails.get(incidentId);
    if (!panel || !detail) return;
    const tbody = panel.querySelector('.incident-attachments-body');
    if (!tbody) return;
    tbody.innerHTML = '';
    const filter = detail.attachmentFilter || 'all';
    const list = (detail.attachments || []).filter(att => matchAttachmentFilter(att, filter));
    if (!list.length) {
      const tr = document.createElement('tr');
      tr.innerHTML = `<td colspan="5">${escapeHtml(t('incidents.attachments.empty'))}</td>`;
      tbody.appendChild(tr);
      return;
    }
    list.forEach(att => {
      const tr = document.createElement('tr');
      tr.innerHTML = `
        <td>${escapeHtml(att.filename)}</td>
        <td>${escapeHtml(formatBytes(att.size_bytes || 0))}</td>
        <td>${escapeHtml(formatDate(att.uploaded_at))}</td>
        <td>${escapeHtml(att.uploaded_by_name || att.uploaded_by || '')}</td>
        <td class="actions">
          <button class="btn ghost att-download" data-id="${att.id}">${t('incidents.attachments.download')}</button>
          <button class="btn ghost att-delete" data-id="${att.id}">${t('incidents.attachments.delete')}</button>
        </td>`;
      tbody.appendChild(tr);
      tr.querySelector('.att-download').onclick = () => {
        window.open(`/api/incidents/${incidentId}/attachments/${att.id}/download`, '_blank');
      };
      tr.querySelector('.att-delete').onclick = async () => {
        if (!confirm(t('incidents.attachments.deleteConfirm'))) return;
        try {
          await Api.del(`/api/incidents/${incidentId}/attachments/${att.id}`);
          await ensureIncidentAttachments(incidentId, true);
        } catch (err) {
          showError(err, 'incidents.attachments.deleteFailed');
        }
      };
    });
  }

  function matchAttachmentFilter(att, filter) {
    if (!att) return false;
    if (filter === 'all') return true;
    const name = (att.filename || '').toLowerCase();
    const ct = (att.content_type || '').toLowerCase();
    if (filter === 'images') {
      return ct.startsWith('image/') || /\.(png|jpe?g|gif|bmp|webp)$/i.test(name);
    }
    if (filter === 'archives') {
      return /\.(zip|rar|7z|tar|gz)$/i.test(name);
    }
    if (filter === 'docs') {
      return /\.(pdf|docx?|xlsx?|pptx?|txt|md)$/i.test(name);
    }
    return true;
  }

  function formatBytes(bytes) {
    if (!bytes) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB'];
    let idx = 0;
    let val = bytes;
    while (val >= 1024 && idx < units.length - 1) {
      val /= 1024;
      idx++;
    }
    return `${val.toFixed(val >= 10 || idx === 0 ? 0 : 1)} ${units[idx]}`;
  }

  IncidentsPage.bindAttachmentControls = bindAttachmentControls;
  IncidentsPage.ensureIncidentAttachments = ensureIncidentAttachments;
  IncidentsPage.renderIncidentAttachments = renderIncidentAttachments;
  IncidentsPage.matchAttachmentFilter = matchAttachmentFilter;
  IncidentsPage.formatBytes = formatBytes;
})();
