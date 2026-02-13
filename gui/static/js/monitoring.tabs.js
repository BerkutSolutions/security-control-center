(() => {
  const state = { activeTab: 'monitoring-tab-home' };
  const TAB_ROUTE = {
    'monitoring-tab-home': '/monitoring',
    'monitoring-tab-events': '/monitoring/events',
    'monitoring-tab-cert': '/monitoring/certs',
    'monitoring-tab-notify': '/monitoring/notifications',
    'monitoring-tab-sla': '/monitoring/sla',
    'monitoring-tab-maintenance': '/monitoring/maintenance',
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
      if (btn.dataset.tab === 'monitoring-tab-events' && !MonitoringPage.hasPermission('monitoring.events.view')) {
        btn.hidden = true;
        btn.disabled = true;
      }
      const canNotifications = MonitoringPage.hasPermission('monitoring.notifications.view')
        || MonitoringPage.hasPermission('monitoring.notifications.manage');
      if (btn.dataset.tab === 'monitoring-tab-notify' && !canNotifications) {
        btn.hidden = true;
        btn.disabled = true;
      }
      if (btn.dataset.tab === 'monitoring-tab-sla' && !MonitoringPage.hasPermission('monitoring.view')) {
        btn.hidden = true;
        btn.disabled = true;
      }
      const canMaintenance = MonitoringPage.hasPermission('monitoring.maintenance.view')
        || MonitoringPage.hasPermission('monitoring.maintenance.manage');
      if (btn.dataset.tab === 'monitoring-tab-maintenance' && !canMaintenance) {
        btn.hidden = true;
        btn.disabled = true;
      }
      const canSettings = MonitoringPage.hasPermission('monitoring.settings.manage')
        || canMaintenance;
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
    onTabActivated(tabId);
    if (updatePath) {
      const nextPath = TAB_ROUTE[tabId] || '/monitoring';
      if (window.location.pathname !== nextPath) {
        window.history.pushState({}, '', nextPath);
      }
    }
  }

  function routeToTab() {
    const path = window.location.pathname.replace(/\/+$/, '');
    if (path === '/monitoring/events') return 'monitoring-tab-events';
    if (path === '/monitoring/certs') return 'monitoring-tab-cert';
    if (path === '/monitoring/notifications') return 'monitoring-tab-notify';
    if (path === '/monitoring/sla') return 'monitoring-tab-sla';
    if (path === '/monitoring/maintenance') return 'monitoring-tab-maintenance';
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

  function onTabActivated(tabId) {
    if (tabId === 'monitoring-tab-home') {
      MonitoringPage.loadMonitors?.();
      return;
    }
    if (tabId === 'monitoring-tab-events') {
      MonitoringPage.refreshEventsFilters?.();
      MonitoringPage.refreshEventsCenter?.();
      return;
    }
    if (tabId === 'monitoring-tab-sla') {
      MonitoringPage.refreshSLA?.();
      return;
    }
    if (tabId === 'monitoring-tab-maintenance') {
      MonitoringPage.refreshMaintenanceList?.();
      return;
    }
    if (tabId === 'monitoring-tab-cert') {
      MonitoringPage.refreshCerts?.();
    }
  }

  if (typeof MonitoringPage !== 'undefined') {
    MonitoringPage.bindTabs = bindTabs;
    MonitoringPage.switchTab = switchTab;
    MonitoringPage.syncRouteTab = syncRouteTab;
  }
})();
