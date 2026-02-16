const ReportsPage = (() => {
  const state = {
    reports: [],
    templates: [],
    settings: null,
    currentUser: null,
    converters: null,
    sections: [],
    sectionsMeta: null,
    charts: [],
    chartsSnapshotId: 0,
    filters: {
      search: '',
      status: '',
      classification: '',
      tags: [],
      mine: false,
      periodFrom: '',
      periodTo: ''
    },
    editor: {
      id: null,
      meta: null,
      content: ''
    },
    activeTabId: 'reports-tab-home'
  };

  function hasPermission(perm) {
    if (!perm) return true;
    const perms = Array.isArray(state.currentUser?.permissions) ? state.currentUser.permissions : [];
    if (!perms.length) return true;
    return perms.includes(perm);
  }

  async function init() {
    const page = document.getElementById('reports-page');
    if (!page) return;
    try {
      const me = await Api.get('/api/auth/me');
      state.currentUser = me.user;
    } catch (err) {
      console.warn('reports me', err);
    }
    if (typeof UserDirectory !== 'undefined' && UserDirectory.load) {
      await UserDirectory.load();
    }
    applyAccessControls();
    bindTabs();
    if (ReportsPage.bindList) ReportsPage.bindList();
    if (ReportsPage.bindBuilder) ReportsPage.bindBuilder();
    if (ReportsPage.bindSections) ReportsPage.bindSections();
    if (ReportsPage.bindCharts) ReportsPage.bindCharts();
    if (ReportsPage.bindTemplates) ReportsPage.bindTemplates();
    if (ReportsPage.bindSettings) ReportsPage.bindSettings();
    bindModalClose();
    document.querySelectorAll('#reports-page input[type="date"]').forEach(input => {
      input.lang = 'ru';
    });
    populateClassificationSelects();
    populateTagFilters();
    document.addEventListener('tags:changed', populateTagFilters);
    if (typeof DocsPage !== 'undefined' && DocsPage.bindApprovalForm) {
      DocsPage.bindApprovalForm();
    }
    await ReportsPage.loadTemplates();
    await ReportsPage.loadSettings();
    await ReportsPage.loadReports();
    await handleReportsRoute();
    if (window.__pendingReportOpen) {
      const id = window.__pendingReportOpen;
      window.__pendingReportOpen = null;
      if (ReportsPage.openViewer) {
        ReportsPage.openViewer(id);
      }
    }
  }

  function bindTabs() {
    const tabs = document.querySelectorAll('#reports-tabs .tab-btn');
    tabs.forEach(btn => {
      btn.addEventListener('click', () => {
        const target = btn.dataset.tab;
        if (target) switchTab(target);
      });
    });
  }

  function applyAccessControls() {
    toggleTab('reports-tab-templates', ReportsPage.hasPermission('reports.templates.view'));
    toggleTab('reports-tab-settings', ReportsPage.hasPermission('reports.templates.manage'));
    const newBtn = document.getElementById('reports-new-btn');
    if (newBtn) newBtn.hidden = !ReportsPage.hasPermission('reports.create');
  }

  function toggleTab(tabId, allowed) {
    const btn = document.querySelector(`#reports-tabs .tab-btn[data-tab="${tabId}"]`);
    if (btn) {
      btn.hidden = !allowed;
      btn.disabled = !allowed;
    }
    const panel = document.querySelector(`.reports-panel[data-tab="${tabId}"]`);
    if (panel && !allowed) panel.hidden = true;
  }

  function switchTab(tabId, opts = {}) {
    const tabs = document.querySelectorAll('#reports-tabs .tab-btn');
    const panels = document.querySelectorAll('.reports-panel');
    tabs.forEach(btn => btn.classList.toggle('active', btn.dataset.tab === tabId));
    panels.forEach(panel => {
      panel.hidden = panel.dataset.tab !== tabId;
    });
    state.activeTabId = tabId;
    if (!opts.skipRoute) {
      updateReportsPath(null, tabId);
    }
  }

  function updateReportsPath(reportId, modeOrTab) {
    let next = '/reports';
    if (reportId) {
      next = modeOrTab === 'edit' ? `/reports/${reportId}/edit` : `/reports/${reportId}`;
    } else {
      const tabMap = {
        'reports-tab-templates': 'templates',
        'reports-tab-settings': 'settings',
        'reports-tab-home': ''
      };
      const slug = tabMap[modeOrTab] ?? tabMap[state.activeTabId] ?? '';
      if (slug) next = `/reports/${slug}`;
    }
    if (window.location.pathname !== next) {
      window.history.replaceState({}, '', next);
    }
  }

  function parseReportsRoute() {
    const parts = window.location.pathname.split('/').filter(Boolean);
    if (parts[0] !== 'reports') return null;
    if (!parts[1]) return { tab: 'reports-tab-home' };
    if (parts[1] === 'builder') return { create: true };
    if (parts[1] === 'templates') return { tab: 'reports-tab-templates' };
    if (parts[1] === 'settings') return { tab: 'reports-tab-settings' };
    const id = parseInt(parts[1], 10);
    if (Number.isFinite(id)) {
      return { reportId: id, mode: parts[2] === 'edit' ? 'edit' : 'view' };
    }
    return { tab: 'reports-tab-home' };
  }

  async function handleReportsRoute() {
    const route = parseReportsRoute();
    if (!route) return;
    if (route.tab) {
      switchTab(route.tab, { skipRoute: true });
      updateReportsPath(null, route.tab);
      return;
    }
    if (route.create) {
      switchTab('reports-tab-home', { skipRoute: true });
      if (ReportsPage.openCreateModal) ReportsPage.openCreateModal();
      updateReportsPath(null, 'reports-tab-home');
      return;
    }
    if (route.reportId) {
      switchTab('reports-tab-home', { skipRoute: true });
      if (route.mode === 'edit' && ReportsPage.openEditor) {
        await ReportsPage.openEditor(route.reportId, { mode: 'edit' });
        updateReportsPath(route.reportId, 'edit');
      } else if (ReportsPage.openViewer) {
        await ReportsPage.openViewer(route.reportId);
        updateReportsPath(route.reportId, 'view');
      }
    }
  }

  function populateClassificationSelects() {
    const selectors = [
      'reports-filter-classification',
      'report-classification',
      'report-editor-classification',
      'reports-settings-classification',
      'reports-settings-watermark'
    ];
    selectors.forEach(id => {
      const sel = document.getElementById(id);
      if (!sel) return;
      DocUI.populateClassificationSelect(sel);
      if (id === 'reports-filter-classification') {
        const opt = document.createElement('option');
        opt.value = '';
        opt.textContent = BerkutI18n.t('common.all');
        sel.insertBefore(opt, sel.firstChild);
      }
      if (id === 'reports-settings-watermark') {
        const opt = document.createElement('option');
        opt.value = '';
        opt.textContent = BerkutI18n.t('reports.settings.watermarkNone');
        sel.insertBefore(opt, sel.firstChild);
      }
    });
  }

  function bindModalClose() {
    document.querySelectorAll('[data-close]').forEach(btn => {
      btn.onclick = () => {
        const sel = btn.getAttribute('data-close');
        if (!sel) return;
        const el = document.querySelector(sel);
        if (el) el.hidden = true;
      };
    });
  }

  function populateTagFilters() {
    const tagsSelect = document.getElementById('reports-filter-tags');
    if (!tagsSelect) return;
    tagsSelect.innerHTML = '';
    const available = DocUI.availableTags ? DocUI.availableTags() : [];
    available.forEach(tag => {
      const code = tag.code || tag;
      const opt = document.createElement('option');
      opt.value = code;
      opt.textContent = DocUI.tagLabel ? DocUI.tagLabel(code) : (tag.label || code);
      opt.dataset.label = opt.textContent;
      tagsSelect.appendChild(opt);
    });
    if (DocsPage?.enhanceMultiSelects) {
      DocsPage.enhanceMultiSelects([tagsSelect.id]);
    }
    if (DocUI.bindTagHint) {
      DocUI.bindTagHint(tagsSelect, document.querySelector('[data-tag-hint="reports-filter-tags"]'));
    }
  }

  function formatDate(val) {
    if (!val) return '-';
    if (typeof AppTime !== 'undefined' && AppTime.formatDateTime) {
      return AppTime.formatDateTime(val);
    }
    const dt = new Date(val);
    if (Number.isNaN(dt.getTime())) return val;
    const pad = (num) => `${num}`.padStart(2, '0');
    return `${pad(dt.getDate())}.${pad(dt.getMonth() + 1)}.${dt.getFullYear()} ${pad(dt.getHours())}:${pad(dt.getMinutes())}`;
  }

  function formatDateLabel(val) {
    if (!val) return '';
    if (typeof AppTime !== 'undefined' && AppTime.formatDate) {
      const formatted = AppTime.formatDate(val);
      return formatted === '-' ? '' : formatted;
    }
    const dt = new Date(val);
    if (Number.isNaN(dt.getTime())) return '';
    const pad = (num) => `${num}`.padStart(2, '0');
    return `${pad(dt.getDate())}.${pad(dt.getMonth() + 1)}.${dt.getFullYear()}`;
  }

  function formatDateInput(val) {
    if (!val) return '';
    if (typeof AppTime !== 'undefined' && AppTime.toISODate) {
      const parsed = AppTime.toISODate(val);
      if (parsed) return parsed;
    }
    const dt = new Date(val);
    if (Number.isNaN(dt.getTime())) return '';
    const pad = (num) => `${num}`.padStart(2, '0');
    return `${dt.getFullYear()}-${pad(dt.getMonth() + 1)}-${pad(dt.getDate())}`;
  }

  function toISODateInput(value) {
    if (!value) return '';
    if (typeof AppTime !== 'undefined' && AppTime.toISODate) {
      return AppTime.toISODate(value);
    }
    return value;
  }

  function formatPeriod(meta) {
    if (!meta) return '-';
    const from = meta.period_from ? formatDateLabel(meta.period_from) : '';
    const to = meta.period_to ? formatDateLabel(meta.period_to) : '';
    if (from && to) return `${from} - ${to}`;
    return from || to || '-';
  }

  function showAlert(targetId, message, success = false) {
    const box = document.getElementById(targetId);
    if (!box) return;
    if (!message) {
      box.hidden = true;
      box.classList.remove('success');
      return;
    }
    box.textContent = message;
    box.hidden = false;
    box.classList.toggle('success', !!success);
  }

  return {
    state,
    init,
    hasPermission,
    switchTab,
    updateReportsPath,
    formatDate,
    formatDateLabel,
    formatDateInput,
    toISODateInput,
    formatPeriod,
    showAlert,
    populateTagFilters
  };
})();

if (typeof window !== 'undefined') {
  window.ReportsPage = ReportsPage;
}
