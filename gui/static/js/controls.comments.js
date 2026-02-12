(() => {
  const state = {
    controlId: null,
    comments: [],
    editingId: null
  };

  function init() {
    if (!document.getElementById('controls-page')) return;
    bindActions();
    document.addEventListener('controls:detailOpened', (e) => {
      const controlId = e?.detail?.controlId;
      if (!controlId) return;
      state.controlId = controlId;
      state.editingId = null;
      loadComments();
      updateEditorMode();
    });
  }

  function bindActions() {
    const submitBtn = document.getElementById('control-comment-submit');
    if (submitBtn) {
      submitBtn.addEventListener('click', () => submitComment());
    }
    const cancelBtn = document.getElementById('control-comment-cancel');
    if (cancelBtn) {
      cancelBtn.addEventListener('click', () => cancelEdit());
    }
    const list = document.getElementById('controls-comments-list');
    if (list) {
      list.addEventListener('click', (e) => {
        const editBtn = e.target.closest('[data-comment-edit]');
        if (editBtn) {
          startEdit(parseInt(editBtn.dataset.commentEdit, 10));
          return;
        }
        const delBtn = e.target.closest('[data-comment-delete]');
        if (delBtn) {
          deleteComment(parseInt(delBtn.dataset.commentDelete, 10));
          return;
        }
        const fileDelBtn = e.target.closest('[data-comment-file-delete]');
        if (fileDelBtn) {
          deleteFile(parseInt(fileDelBtn.dataset.commentId, 10), fileDelBtn.dataset.commentFileDelete);
        }
      });
    }
  }

  function currentUserId() {
    return ControlsPage?.state?.currentUser?.id || null;
  }

  function hasPerm(perm) {
    const perms = ControlsPage?.state?.permissions || [];
    if (!perm) return true;
    return Array.isArray(perms) ? perms.includes(perm) : false;
  }

  function canManageComment(comment) {
    if (!comment) return false;
    if (comment.author_id && comment.author_id === currentUserId()) return true;
    return hasPerm('controls.manage');
  }

  function updateEditorMode() {
    const editor = document.getElementById('control-comment-editor');
    const cancelBtn = document.getElementById('control-comment-cancel');
    const submitBtn = document.getElementById('control-comment-submit');
    const filesEl = document.getElementById('control-comment-files');
    if (!editor) return;
    const canCreate = hasPerm('controls.manage');
    editor.hidden = !canCreate;
    if (cancelBtn) cancelBtn.hidden = !state.editingId;
    if (submitBtn) {
      submitBtn.textContent = state.editingId ? t('controls.comments.update') : t('controls.comments.add');
    }
    if (filesEl) filesEl.disabled = !!state.editingId;
  }

  async function loadComments() {
    if (!state.controlId) return;
    try {
      const res = await Api.get(`/api/controls/${state.controlId}/comments`);
      state.comments = res.items || [];
    } catch (err) {
      state.comments = [];
      console.warn('control comments load', err);
    }
    renderComments();
  }

  function renderComments() {
    const list = document.getElementById('controls-comments-list');
    const empty = document.getElementById('controls-comments-empty');
    if (!list) return;
    list.innerHTML = '';
    if (!state.comments.length) {
      if (empty) empty.hidden = false;
      return;
    }
    if (empty) empty.hidden = true;
    state.comments.forEach(comment => {
      const wrapper = document.createElement('div');
      wrapper.className = 'control-comment';
      const meta = document.createElement('div');
      meta.className = 'control-comment-meta';
      const author = authorName(comment.author_id);
      const stamp = formatDate(comment.created_at);
      meta.innerHTML = `<span>${escapeHtml(author)}</span><span>${escapeHtml(stamp)}</span>`;
      wrapper.appendChild(meta);
      if (comment.content) {
        const body = document.createElement('div');
        body.className = 'control-comment-body';
        body.textContent = comment.content;
        wrapper.appendChild(body);
      }
      if (comment.attachments && comment.attachments.length) {
        const files = document.createElement('div');
        files.className = 'control-comment-files';
        comment.attachments.forEach(file => {
          const row = document.createElement('div');
          row.className = 'control-comment-file';
          const link = document.createElement('a');
          link.href = file.url || '#';
          link.textContent = `${file.name || t('controls.comments.file')} (${formatSize(file.size)})`;
          link.setAttribute('download', file.name || 'file');
          row.appendChild(link);
          if (canManageComment(comment)) {
            const btn = document.createElement('button');
            btn.type = 'button';
            btn.className = 'btn btn-sm ghost';
            btn.textContent = t('common.delete');
            btn.dataset.commentFileDelete = file.id;
            btn.dataset.commentId = comment.id;
            row.appendChild(btn);
          }
          files.appendChild(row);
        });
        wrapper.appendChild(files);
      }
      if (canManageComment(comment)) {
        const actions = document.createElement('div');
        actions.className = 'control-comment-actions';
        const editBtn = document.createElement('button');
        editBtn.type = 'button';
        editBtn.className = 'btn btn-sm ghost';
        editBtn.textContent = t('controls.comments.edit');
        editBtn.dataset.commentEdit = comment.id;
        actions.appendChild(editBtn);
        const delBtn = document.createElement('button');
        delBtn.type = 'button';
        delBtn.className = 'btn btn-sm danger';
        delBtn.textContent = t('controls.comments.delete');
        delBtn.dataset.commentDelete = comment.id;
        actions.appendChild(delBtn);
        wrapper.appendChild(actions);
      }
      list.appendChild(wrapper);
    });
  }

  async function submitComment() {
    if (!state.controlId) return;
    if (!hasPerm('controls.manage')) return;
    const textEl = document.getElementById('control-comment-text');
    const filesEl = document.getElementById('control-comment-files');
    const content = (textEl?.value || '').trim();
    const files = filesEl?.files || [];
    if (!state.editingId && !content && files.length === 0) {
      alert(t('controls.comments.required'));
      return;
    }
    if (state.editingId) {
      await updateComment(state.editingId, content);
      return;
    }
    try {
      if (files.length) {
        const form = new FormData();
        form.append('content', content);
        Array.from(files).forEach(file => form.append('files', file));
        await Api.upload(`/api/controls/${state.controlId}/comments`, form);
      } else {
        await Api.post(`/api/controls/${state.controlId}/comments`, { content });
      }
      if (textEl) textEl.value = '';
      if (filesEl) filesEl.value = '';
      await loadComments();
    } catch (err) {
      alert(err.message || t('common.error'));
    }
  }

  async function updateComment(commentId, content) {
    const comment = state.comments.find(item => item.id === commentId);
    if (!comment) return;
    if (!content && (!comment.attachments || !comment.attachments.length)) {
      alert(t('controls.comments.required'));
      return;
    }
    try {
      await Api.put(`/api/controls/${state.controlId}/comments/${commentId}`, { content });
      cancelEdit();
      await loadComments();
    } catch (err) {
      alert(err.message || t('common.error'));
    }
  }

  function startEdit(commentId) {
    const comment = state.comments.find(item => item.id === commentId);
    if (!comment) return;
    const textEl = document.getElementById('control-comment-text');
    if (textEl) textEl.value = comment.content || '';
    state.editingId = commentId;
    updateEditorMode();
  }

  function cancelEdit() {
    const textEl = document.getElementById('control-comment-text');
    const filesEl = document.getElementById('control-comment-files');
    if (textEl) textEl.value = '';
    if (filesEl) filesEl.value = '';
    state.editingId = null;
    updateEditorMode();
  }

  async function deleteComment(commentId) {
    if (!commentId || !state.controlId) return;
    if (!window.confirm(t('controls.comments.deleteConfirm'))) return;
    try {
      await Api.del(`/api/controls/${state.controlId}/comments/${commentId}`);
      await loadComments();
    } catch (err) {
      alert(err.message || t('common.error'));
    }
  }

  async function deleteFile(commentId, fileId) {
    if (!commentId || !fileId || !state.controlId) return;
    if (!window.confirm(t('controls.comments.fileDeleteConfirm'))) return;
    try {
      const res = await Api.del(`/api/controls/${state.controlId}/comments/${commentId}/files/${fileId}`);
      if (res && res.status === 'deleted') {
        state.comments = state.comments.filter(c => c.id !== commentId);
        renderComments();
        return;
      }
      await loadComments();
    } catch (err) {
      alert(err.message || t('common.error'));
    }
  }

  function authorName(authorId) {
    if (!authorId) return '-';
    if (typeof UserDirectory !== 'undefined' && UserDirectory.name) {
      return UserDirectory.name(authorId);
    }
    return `#${authorId}`;
  }

  function formatDate(val) {
    if (!val) return '-';
    if (typeof AppTime !== 'undefined' && AppTime.formatDateTime) {
      return AppTime.formatDateTime(val);
    }
    const dt = new Date(val);
    return Number.isNaN(dt.getTime()) ? '-' : dt.toLocaleString();
  }

  function formatSize(bytes) {
    if (!bytes) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB'];
    let size = bytes;
    let idx = 0;
    while (size >= 1024 && idx < units.length - 1) {
      size /= 1024;
      idx += 1;
    }
    return `${size.toFixed(size >= 10 || idx === 0 ? 0 : 1)} ${units[idx]}`;
  }

  function escapeHtml(str) {
    return (str || '').toString().replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  }

  function t(key) {
    const val = BerkutI18n.t(key);
    return val === key ? key : val;
  }

  if (typeof window !== 'undefined') {
    window.addEventListener('DOMContentLoaded', init);
  }
})();
