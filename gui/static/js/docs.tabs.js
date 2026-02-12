(() => {
  if (typeof DocsPage === 'undefined') return;
  const state = DocsPage.state;

  function hasPermission(perm) {
    if (DocsPage.hasPermission) return DocsPage.hasPermission(perm);
    return true;
  }

  function bindTabs() {
    if (!state.tabs || !state.tabs.length) {
      state.tabs = [{ id: 'list', type: 'list', titleKey: 'docs.tabs.list', closable: false }];
    }
    if (!state.activeTabId) state.activeTabId = 'list';
    renderTabBar();
    switchTab(state.activeTabId);
  }

  function renderTabBar() {
    const bar = document.getElementById('docs-tabs');
    if (!bar) return;
    bar.innerHTML = '';
    state.tabs.forEach(tab => {
      const btn = document.createElement('button');
      btn.type = 'button';
      btn.className = 'tab-btn';
      if (tab.id === state.activeTabId) btn.classList.add('active');
      btn.dataset.tab = tab.id;
      const title = document.createElement('span');
      title.className = 'tab-title';
      title.textContent = tab.titleKey ? BerkutI18n.t(tab.titleKey) : (tab.title || tab.id);
      btn.appendChild(title);
      if (tab.closable) {
        const close = document.createElement('span');
        close.className = 'tab-close';
        close.textContent = 'x';
        close.setAttribute('role', 'button');
        close.setAttribute('aria-label', BerkutI18n.t('common.close') || 'Close');
        close.addEventListener('click', (e) => {
          e.stopPropagation();
          requestCloseTab(tab.id);
        });
        btn.appendChild(close);
      }
      btn.addEventListener('click', () => switchTab(tab.id));
      bar.appendChild(btn);
    });
  }

  async function switchTab(tabId) {
    const target = state.tabs.find(t => t.id === tabId);
    if (!target) return;
    if (state.activeTabId && state.activeTabId !== tabId) {
      const ok = await confirmLeaveTab(state.activeTabId);
      if (!ok) return;
    }
    state.activeTabId = tabId;
    renderTabBar();
    const panels = document.querySelectorAll('#docs-panels .docs-panel');
    panels.forEach(panel => {
      panel.hidden = panel.dataset.tab !== tabId;
    });
    if (target.type === 'doc') {
      openDocPanel(target);
      DocsPage.updateDocsPath(target.docId, target.mode || 'view');
    } else {
      DocsPage.updateDocsPath(null, 'list');
      DocEditor.close({ silent: true });
      if (DocsPage.loadDocs && hasPermission('docs.view')) {
        DocsPage.loadDocs();
      }
    }
  }

  async function confirmLeaveTab(tabId) {
    const tab = state.tabs.find(t => t.id === tabId);
    if (!tab || tab.type !== 'doc') return true;
    if (typeof DocEditor === 'undefined' || !DocEditor.isDirty) return true;
    if (!DocEditor.isDirty()) return true;
    if (DocsPage.confirmAction) {
      return await DocsPage.confirmAction({
        title: BerkutI18n.t('common.confirm'),
        message: BerkutI18n.t('docs.unsavedWarning'),
      });
    }
    return window.confirm(BerkutI18n.t('docs.unsavedWarning'));
  }

  function openDocTab(docId, mode = 'view') {
    if (!docId) return;
    const tabId = `doc-${docId}`;
    let tab = state.tabs.find(t => t.id === tabId);
    if (!tab) {
      tab = {
        id: tabId,
        type: 'doc',
        docId,
        mode,
        title: docTitle(docId),
        closable: true
      };
      state.tabs.push(tab);
    } else if (mode && tab.mode !== mode) {
      tab.mode = mode;
    }
    switchTab(tabId);
  }

  function docTitle(docId) {
    const doc = (state.docs || []).find(d => d.id === docId);
    if (!doc) return `#${docId}`;
    const reg = doc.reg_number ? ` (${doc.reg_number})` : '';
    return `${doc.title || ''}${reg}`.trim() || `#${docId}`;
  }

  function openDocPanel(tab) {
    if (!tab) return;
    const panels = document.getElementById('docs-panels');
    if (!panels) return;
    let panel = panels.querySelector(`[data-tab="${tab.id}"]`);
    if (!panel) {
      panel = document.createElement('div');
      panel.className = 'tab-panel docs-panel docs-doc-panel';
      panel.dataset.tab = tab.id;
      panels.appendChild(panel);
    }
    if (DocEditor && DocEditor.mount) {
      DocEditor.mount(panel);
    } else if (DocEditor && DocEditor.panel) {
      panel.appendChild(DocEditor.panel);
    } else {
      panel.appendChild(document.getElementById('doc-editor'));
    }
    if (DocEditor && DocEditor.open) {
      DocEditor.open(tab.docId, { mode: tab.mode || 'view' }).then(meta => {
        if (meta && meta.title) {
          tab.title = `${meta.title || ''}${meta.reg_number ? ` (${meta.reg_number})` : ''}`.trim();
          renderTabBar();
        }
      });
    }
    panel.hidden = false;
  }

  function updateActiveDocMode(docId, mode) {
    const tab = state.tabs.find(t => t.type === 'doc' && t.docId === docId);
    if (!tab) return;
    tab.mode = mode || tab.mode;
    if (tab.id === state.activeTabId) {
      DocsPage.updateDocsPath(docId, tab.mode || 'view');
    }
  }

  async function requestCloseTab(tabId) {
    const tab = state.tabs.find(t => t.id === tabId);
    if (!tab) return;
    if (tab.type === 'doc') {
      if (tabId === state.activeTabId) {
        const ok = await confirmLeaveTab(tabId);
        if (!ok) return;
      }
    }
    removeTab(tabId);
  }

  function removeTab(tabId) {
    const panel = document.querySelector(`#docs-panels [data-tab="${tabId}"]`);
    if (panel) panel.remove();
    state.tabs = state.tabs.filter(t => t.id !== tabId);
    if (state.activeTabId === tabId) {
      switchTab('list');
    } else {
      renderTabBar();
    }
  }

  DocsPage.bindTabs = bindTabs;
  DocsPage.switchTab = switchTab;
  DocsPage.openDocTab = openDocTab;
  DocsPage.requestCloseTab = requestCloseTab;
  DocsPage.removeTab = removeTab;
  DocsPage.updateActiveDocMode = updateActiveDocMode;
})();
