(() => {
  const globalObj = typeof window !== 'undefined' ? window : globalThis;
  const DashboardPage = globalObj.DashboardPage || (globalObj.DashboardPage = {});
  const state = DashboardPage.state || {
    root: null,
    grid: null,
    empty: null,
    layout: null,
    defaultLayout: null,
    frames: [],
    data: {},
    currentUser: null,
    tasksAvailable: false,
    editMode: false,
    dirty: false,
    baseline: null,
    draggingId: null,
    pendingNavigation: null,
    beforeUnloadBound: false
  };
  DashboardPage.state = state;

  function t(key) {
    return (globalObj.BerkutI18n && BerkutI18n.t(key)) || key;
  }

  function clone(obj) {
    try {
      return JSON.parse(JSON.stringify(obj));
    } catch (_) {
      return obj;
    }
  }

  function normalizeLayout(layout) {
    const safe = layout || {};
    return {
      order: Array.isArray(safe.order) ? safe.order.slice() : [],
      hidden: Array.isArray(safe.hidden) ? safe.hidden.slice() : [],
      settings: safe.settings && typeof safe.settings === 'object' ? clone(safe.settings) : {}
    };
  }

  function init() {
    state.root = document.getElementById('dashboard-page');
    if (!state.root) return;
    document.body.classList.add('dashboard-mode');
    state.grid = document.getElementById('dashboard-grid');
    state.empty = document.getElementById('dashboard-empty');
    bindToolbar();
    bindConfigModal();
    bindFrameSettingsModal();
    bindUnsavedModal();
    load();
    bindBeforeUnload();
  }

  async function load() {
    try {
      const [res, me] = await Promise.all([
        Api.get('/api/dashboard'),
        Api.get('/api/auth/me').catch(() => ({ user: null }))
      ]);
      state.layout = normalizeLayout(res.layout);
      state.defaultLayout = normalizeLayout(res.default_layout || res.layout);
      state.frames = Array.isArray(res.frames) ? res.frames : [];
      state.data = {
        summary: res.summary || {},
        todo: res.todo || {},
        tasks: res.tasks || {},
        documents: res.documents || {},
        incidents: res.incidents || {},
        activity: res.activity || {}
      };
      state.currentUser = me.user || null;
      state.tasksAvailable = !!res.tasks_available;
      state.editMode = false;
      state.dirty = false;
      state.baseline = clone(state.layout);
      render();
    } catch (err) {
      console.error('dashboard load', err);
      if (state.grid) {
        state.grid.innerHTML = `<div class="card"><div class="card-body">${t('dashboard.loadError')}</div></div>`;
      }
    }
  }

  function bindToolbar() {
    const editBtn = document.getElementById('dashboard-edit-btn');
    const saveBtn = document.getElementById('dashboard-save-btn');
    const cancelBtn = document.getElementById('dashboard-cancel-btn');
    const configBtn = document.getElementById('dashboard-config-btn');
    if (editBtn) editBtn.addEventListener('click', () => setEditMode(true));
    if (saveBtn) saveBtn.addEventListener('click', () => handleSave());
    if (cancelBtn) cancelBtn.addEventListener('click', () => handleCancel());
    if (configBtn) configBtn.addEventListener('click', () => openConfigModal());
  }

  function bindConfigModal() {
    const modal = document.getElementById('dashboard-config-modal');
    const closeBtn = document.getElementById('dashboard-config-close');
    const saveBtn = document.getElementById('dashboard-config-save');
    const resetBtn = document.getElementById('dashboard-reset-btn');
    if (closeBtn) closeBtn.addEventListener('click', () => closeModal(modal));
    if (saveBtn) saveBtn.addEventListener('click', () => closeModal(modal));
    if (resetBtn) resetBtn.addEventListener('click', () => resetToDefault());
  }

  function bindFrameSettingsModal() {
    const modal = document.getElementById('dashboard-frame-settings-modal');
    const closeBtn = document.getElementById('dashboard-frame-settings-close');
    const cancelBtn = document.getElementById('dashboard-frame-settings-cancel');
    const saveBtn = document.getElementById('dashboard-frame-settings-save');
    if (closeBtn) closeBtn.addEventListener('click', () => closeModal(modal));
    if (cancelBtn) cancelBtn.addEventListener('click', () => closeModal(modal));
    if (saveBtn) saveBtn.addEventListener('click', () => {
      if (DashboardPage.saveFrameSettings) {
        DashboardPage.saveFrameSettings();
      }
    });
  }

  function bindUnsavedModal() {
    const modal = document.getElementById('dashboard-unsaved-modal');
    const closeBtn = document.getElementById('dashboard-unsaved-close');
    const saveBtn = document.getElementById('dashboard-unsaved-save');
    const discardBtn = document.getElementById('dashboard-unsaved-discard');
    const cancelBtn = document.getElementById('dashboard-unsaved-cancel');
    const cancelHandler = () => {
      closeModal(modal);
      if (state.pendingNavigation) {
        state.pendingNavigation.resolve(false);
        state.pendingNavigation = null;
      }
    };
    if (closeBtn) closeBtn.addEventListener('click', cancelHandler);
    if (cancelBtn) cancelBtn.addEventListener('click', cancelHandler);
    if (saveBtn) saveBtn.addEventListener('click', async () => {
      const ok = await handleSave();
      if (ok) {
        modal.hidden = true;
        if (state.pendingNavigation) {
          state.pendingNavigation.resolve(true);
          state.pendingNavigation = null;
        }
      }
    });
    if (discardBtn) discardBtn.addEventListener('click', () => {
      state.layout = clone(state.baseline || state.layout);
      state.dirty = false;
      setEditMode(false);
      modal.hidden = true;
      if (state.pendingNavigation) {
        state.pendingNavigation.resolve(true);
        state.pendingNavigation = null;
      }
    });
    if (modal) {
      modal.addEventListener('click', (e) => {
        if (!e.target.classList.contains('modal-backdrop')) return;
        cancelHandler();
      });
    }
  }

  function render() {
    updateToolbar();
    if (DashboardPage.renderFrames) {
      DashboardPage.renderFrames();
    }
  }

  function updateToolbar() {
    const editBtn = document.getElementById('dashboard-edit-btn');
    const editActions = document.getElementById('dashboard-edit-actions');
    const configBtn = document.getElementById('dashboard-config-btn');
    const editHint = document.getElementById('dashboard-edit-hint');
    if (editBtn) editBtn.hidden = state.editMode;
    if (editActions) editActions.hidden = !state.editMode;
    if (configBtn) configBtn.hidden = !state.editMode;
    if (editHint) editHint.hidden = !state.editMode;
    if (state.root) {
      state.root.classList.toggle('dashboard-edit-mode', state.editMode);
    }
  }

  function openConfigModal() {
    const modal = document.getElementById('dashboard-config-modal');
    const list = document.getElementById('dashboard-config-list');
    if (!modal || !list) return;
    list.innerHTML = '';
    const hidden = new Set(state.layout.hidden || []);
    const frameMap = DashboardPage.frameTitleMap ? DashboardPage.frameTitleMap() : {};
    (state.frames || []).forEach((frame) => {
      const id = frame.id;
      const label = t(frameMap[id] || frame.title || id);
      const item = document.createElement('label');
      item.className = 'checkbox-row';
      const input = document.createElement('input');
      input.type = 'checkbox';
      input.checked = !hidden.has(id);
      input.dataset.frameId = id;
      input.addEventListener('change', () => toggleFrameVisibility(id, input.checked));
      const span = document.createElement('span');
      span.textContent = label;
      item.appendChild(input);
      item.appendChild(span);
      list.appendChild(item);
    });
    modal.hidden = false;
  }

  function toggleFrameVisibility(id, visible) {
    const hidden = new Set(state.layout.hidden || []);
    if (!visible) {
      hidden.add(id);
    } else {
      hidden.delete(id);
    }
    state.layout.hidden = Array.from(hidden);
    setDirty();
    if (DashboardPage.renderFrames) {
      DashboardPage.renderFrames();
    }
  }

  function resetToDefault() {
    if (!state.defaultLayout) return;
    state.layout = clone(state.defaultLayout);
    setDirty();
    render();
  }

  function closeModal(modal) {
    if (modal) modal.hidden = true;
  }

  function setEditMode(enabled) {
    state.editMode = !!enabled;
    updateToolbar();
    if (DashboardPage.renderFrames) {
      DashboardPage.renderFrames();
    }
  }

  function setDirty() {
    state.dirty = true;
  }

  async function handleSave() {
    if (!state.editMode) return true;
    try {
      await Api.post('/api/dashboard/layout', { layout: state.layout });
      state.baseline = clone(state.layout);
      state.dirty = false;
      setEditMode(false);
      return true;
    } catch (err) {
      console.error('dashboard save', err);
      return false;
    }
  }

  function handleCancel() {
    if (!state.editMode) return;
    state.layout = clone(state.baseline || state.layout);
    state.dirty = false;
    setEditMode(false);
  }

  function bindBeforeUnload() {
    if (state.beforeUnloadBound) return;
    state.beforeUnloadBound = true;
    window.addEventListener('beforeunload', (e) => {
      if (!state.editMode || !state.dirty) return;
      e.preventDefault();
      e.returnValue = '';
    });
  }

  async function confirmNavigation() {
    if (!state.editMode || !state.dirty) return true;
    const modal = document.getElementById('dashboard-unsaved-modal');
    if (!modal) {
      return window.confirm(t('dashboard.unsavedMessage'));
    }
    modal.hidden = false;
    return new Promise((resolve) => {
      state.pendingNavigation = { resolve };
    });
  }

  DashboardPage.init = init;
  DashboardPage.load = load;
  DashboardPage.render = render;
  DashboardPage.updateToolbar = updateToolbar;
  DashboardPage.openConfigModal = openConfigModal;
  DashboardPage.toggleFrameVisibility = toggleFrameVisibility;
  DashboardPage.resetToDefault = resetToDefault;
  DashboardPage.closeModal = closeModal;
  DashboardPage.setEditMode = setEditMode;
  DashboardPage.setDirty = setDirty;
  DashboardPage.handleSave = handleSave;
  DashboardPage.handleCancel = handleCancel;
  DashboardPage.confirmNavigation = confirmNavigation;
  DashboardPage.hasUnsavedChanges = () => state.editMode && state.dirty;
  DashboardPage.t = t;
  DashboardPage.clone = clone;
  DashboardPage.normalizeLayout = normalizeLayout;
})();
