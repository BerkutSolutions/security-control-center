(() => {
  const state = IncidentsPage.state;
  const { t } = IncidentsPage;

  function renderTabBar() {
    const bar = document.getElementById('incidents-tabs');
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
      title.textContent = tab.titleKey ? t(tab.titleKey) : (tab.title || tab.id);
      btn.appendChild(title);
      if (tab.closable) {
        const close = document.createElement('span');
        close.className = 'tab-close';
        close.textContent = 'x';
        close.setAttribute('role', 'button');
        close.setAttribute('aria-label', t('common.close') || 'Close');
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

  function switchTab(tabId) {
    const tab = state.tabs.find(t => t.id === tabId);
    if (!tab) return;
    state.activeTabId = tabId;
    renderTabBar();
    document.querySelectorAll('#incidents-panels .tab-panel').forEach(panel => {
      panel.hidden = panel.dataset.tab !== tabId;
    });
    if (tab.type === 'incident') {
      if (IncidentsPage.renderIncidentPanel) IncidentsPage.renderIncidentPanel(tab.incidentId);
      setIncidentUrl(tab.incidentId);
    } else if (tab.type === 'list') {
      if (IncidentsPage.renderTableRows) IncidentsPage.renderTableRows();
    } else {
      clearIncidentUrl();
    }
  }

  async function requestCloseTab(tabId) {
    const tab = state.tabs.find(t => t.id === tabId);
    if (!tab) return;
    if (tab.type === 'create' && tab.draft && tab.draft.dirty) {
      const ok = await IncidentsPage.confirmAction({
        title: t('common.confirm'),
        message: t('incidents.unsavedWarning'),
      });
      if (!ok) {
        return;
      }
    }
    if (tab.type === 'incident') {
      const detail = state.incidentDetails.get(tab.incidentId);
      const hasUnsaved = detail && IncidentsPage.isIncidentDirty && IncidentsPage.isIncidentDirty(detail);
      if (hasUnsaved) {
        if (IncidentsPage.promptUnsavedChanges) {
          const choice = await IncidentsPage.promptUnsavedChanges({
            title: t('common.confirm'),
            message: t('incidents.unsavedPrompt')
          });
          if (choice === 'cancel') return;
          if (choice === 'save' && IncidentsPage.saveIncidentChanges) {
            try {
              await IncidentsPage.saveIncidentChanges(tab.incidentId);
            } catch (_) {
              return;
            }
          }
        } else {
          const ok = await IncidentsPage.confirmAction({
            title: t('common.confirm'),
            message: t('incidents.unsavedWarning'),
          });
          if (!ok) {
            return;
          }
        }
      }
    }
    removeTab(tabId);
  }

  function removeTab(tabId) {
    const tab = state.tabs.find(t => t.id === tabId);
    state.tabs = state.tabs.filter(t => t.id !== tabId);
    const panel = document.querySelector(`#incidents-panels [data-tab="${tabId}"]`);
    if (panel) panel.remove();
    if (tab && tab.type === 'incident') {
      state.incidentDetails.delete(tab.incidentId);
      if (state.pendingStageIncidentId === tab.incidentId) {
        state.pendingStageIncidentId = null;
      }
    }
    if (state.activeTabId === tabId) {
      switchTab('list');
    } else {
      renderTabBar();
    }
  }

  function setIncidentUrl(incidentId) {
    const next = incidentId ? `/incidents/${incidentId}` : '/incidents';
    if (window.location.pathname !== next) {
      window.history.replaceState({}, '', next);
    }
  }

  function clearIncidentUrl() {
    if (window.location.pathname !== '/incidents') {
      window.history.replaceState({}, '', '/incidents');
    }
  }

  IncidentsPage.renderTabBar = renderTabBar;
  IncidentsPage.switchTab = switchTab;
  IncidentsPage.requestCloseTab = requestCloseTab;
  IncidentsPage.removeTab = removeTab;
})();
