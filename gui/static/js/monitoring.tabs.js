(() => {
  const state = { activeTab: 'monitoring-tab-home' };
  const TAB_ROUTE = {
    'monitoring-tab-home': '/monitoring',
    'monitoring-tab-cert': '/monitoring/certs',
    'monitoring-tab-notify': '/monitoring/notifications',
    'monitoring-tab-settings': '/monitoring/settings',
  };
  let popstateBound = false;

  function bindTabs() {
    const tabs = document.querySelectorAll('#monitoring-tabs .tab-btn');
    tabs.forEach(btn => {
      const canCerts = MonitoringPage.hasPermission('monitoring.certs.view')
        || MonitoringPage.hasPermission('monitoring.certs.manage');
      if (btn.dataset.tab === 'monitoring-tab-cert' && !canCerts) {
        btn.hidden = true;
        btn.disabled = true;
      }
      const canNotifications = MonitoringPage.hasPermission('monitoring.notifications.view')
        || MonitoringPage.hasPermission('monitoring.notifications.manage');
      if (btn.dataset.tab === 'monitoring-tab-notify' && !canNotifications) {
        btn.hidden = true;
        btn.disabled = true;
      }
      const canSettings = MonitoringPage.hasPermission('monitoring.settings.manage')
        || MonitoringPage.hasPermission('monitoring.maintenance.view')
        || MonitoringPage.hasPermission('monitoring.maintenance.manage');
      if (btn.dataset.tab === 'monitoring-tab-settings' && !canSettings) {
        btn.hidden = true;
        btn.disabled = true;
      }
      btn.addEventListener('click', () => {
        if (btn.disabled || btn.hidden) return;
        switchTab(btn.dataset.tab, true);
      });
    });
    syncRouteTab();
    if (!popstateBound) {
      popstateBound = true;
      window.addEventListener('popstate', () => {
        syncRouteTab();
      });
    }
  }

  function switchTab(tabId, updatePath = false) {
    if (!tabId) return;
    state.activeTab = tabId;
    document.querySelectorAll('#monitoring-tabs .tab-btn').forEach(btn => {
      btn.classList.toggle('active', btn.dataset.tab === tabId);
    });
    document.querySelectorAll('#monitoring-page .tab-panel').forEach(panel => {
      panel.hidden = panel.dataset.tab !== tabId;
    });
    if (updatePath) {
      const nextPath = TAB_ROUTE[tabId] || '/monitoring';
      if (window.location.pathname !== nextPath) {
        window.history.pushState({}, '', nextPath);
      }
    }
  }

  function routeToTab() {
    const path = window.location.pathname.replace(/\/+$/, '');
    if (path === '/monitoring/certs') return 'monitoring-tab-cert';
    if (path === '/monitoring/notifications') return 'monitoring-tab-notify';
    if (path === '/monitoring/settings') return 'monitoring-tab-settings';
    return 'monitoring-tab-home';
  }

  function firstVisibleTab() {
    const btn = Array.from(document.querySelectorAll('#monitoring-tabs .tab-btn'))
      .find(item => !item.hidden && !item.disabled);
    return btn?.dataset.tab || 'monitoring-tab-home';
  }

  function syncRouteTab() {
    const tab = routeToTab();
    const btn = document.querySelector(`#monitoring-tabs .tab-btn[data-tab="${tab}"]`);
    if (btn && !btn.hidden && !btn.disabled) {
      switchTab(tab, false);
      return;
    }
    switchTab(firstVisibleTab(), false);
  }

  if (typeof MonitoringPage !== 'undefined') {
    MonitoringPage.bindTabs = bindTabs;
    MonitoringPage.switchTab = switchTab;
    MonitoringPage.syncRouteTab = syncRouteTab;
  }
})();
