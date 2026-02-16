(() => {
  const state = IncidentsPage.state;
  const { showError } = IncidentsPage;

  async function loadCurrentUser() {
    try {
      const res = await Api.get('/api/auth/me');
      state.currentUser = res.user || null;
    } catch (err) {
      state.currentUser = null;
    }
  }

  async function loadIncidents() {
    try {
      const res = await Api.get('/api/incidents');
      state.incidents = (res.items || []).slice();
      if (IncidentsPage.renderHome) IncidentsPage.renderHome();
      if (IncidentsPage.renderTableRows) IncidentsPage.renderTableRows();
    } catch (err) {
      showError(err, 'incidents.forbidden');
    }
  }

  IncidentsPage.loadCurrentUser = loadCurrentUser;
  IncidentsPage.loadIncidents = loadIncidents;
})();
