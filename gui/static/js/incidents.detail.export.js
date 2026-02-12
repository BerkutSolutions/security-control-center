(() => {
  const { t, showError } = IncidentsPage;

  function bindExportControls(incidentId) {
    const tabId = `incident-${incidentId}`;
    const panel = document.querySelector(`#incidents-panels [data-tab="${tabId}"]`);
    if (!panel) return;
    panel.querySelectorAll('.incident-export-btn').forEach(btn => {
      btn.onclick = () => {
        const format = btn.dataset.format || 'md';
        window.open(`/api/incidents/${incidentId}/export?format=${encodeURIComponent(format)}`, '_blank');
      };
    });
    const reportBtn = panel.querySelector('.incident-report-doc');
    if (reportBtn) {
      reportBtn.onclick = async () => {
        try {
          const res = await Api.post(`/api/incidents/${incidentId}/create-report-doc`, {});
          if (res && res.doc_id) {
            openDocInDocs(res.doc_id);
          }
        } catch (err) {
          showError(err, 'incidents.report.createFailed');
        }
      };
    }
    const reportCreateBtn = panel.querySelector('.incident-create-report');
    if (reportCreateBtn) {
      reportCreateBtn.onclick = async () => {
        try {
          const res = await Api.post('/api/reports/from-incident', { incident_id: incidentId });
          if (res && res.report_id) {
            openReportInReports(res.report_id);
          }
        } catch (err) {
          showError(err, 'incidents.report.createFailed');
        }
      };
    }
  }

  function openDocInDocs(docId) {
    const id = parseInt(docId, 10);
    if (!id) return;
    if (window.DocsPage && typeof window.DocsPage.openDocTab === 'function' && document.getElementById('docs-page')) {
      window.DocsPage.openDocTab(id, 'view');
    } else {
      window.__pendingDocOpen = id;
    }
    navigateToDocs();
  }

  function navigateToDocs() {
    if (window.location.pathname === '/docs') return;
    window.history.pushState({}, '', '/docs');
    window.dispatchEvent(new PopStateEvent('popstate'));
  }

  function openReportInReports(reportId) {
    const id = parseInt(reportId, 10);
    if (!id) return;
    if (window.ReportsPage && typeof window.ReportsPage.openViewer === 'function' && document.getElementById('reports-page')) {
      window.ReportsPage.openViewer(id);
    } else {
      window.__pendingReportOpen = id;
    }
    navigateToReports(id);
  }

  function navigateToReports(reportId) {
    const current = window.location.pathname;
    const next = reportId ? `/reports/${reportId}` : '/reports';
    if (current === next) return;
    window.history.pushState({}, '', next);
    window.dispatchEvent(new PopStateEvent('popstate'));
  }

  IncidentsPage.bindExportControls = bindExportControls;
  IncidentsPage.openDocInDocs = openDocInDocs;
  IncidentsPage.openReportInReports = openReportInReports;
})();
