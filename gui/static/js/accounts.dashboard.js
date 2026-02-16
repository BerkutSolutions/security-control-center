(() => {
  const globalObj = typeof window !== 'undefined' ? window : globalThis;
  const AccountsPage = globalObj.AccountsPage || (globalObj.AccountsPage = {});
  const { setText, escapeHtml, formatDate } = AccountsPage;

  async function loadDashboard() {
    try {
      const data = await Api.get('/api/accounts/dashboard');
      setText('metric-total', data.total);
      setText('metric-active', data.active);
      setText('metric-blocked', data.blocked);
      const online = data.online_count != null ? data.online_count : data.online;
      setText('metric-online', online);
      setText('metric-without-password', data.without_password);
      setText('metric-require-change', data.require_change);
      setText('metric-without-2fa', data.without_2fa);
      renderOnlineUsers(data.online_users || []);
    } catch (err) {
      console.error('dashboard', err);
    }
  }

  function renderOnlineUsers(list) {
    const tbody = document.querySelector('#online-users-table tbody');
    if (!tbody) return;
    tbody.innerHTML = '';
    if (!list || !list.length) {
      const tr = document.createElement('tr');
      tr.innerHTML = `<td colspan="4">${BerkutI18n.t('accounts.noActiveSessions') || '-'}</td>`;
      tbody.appendChild(tr);
      return;
    }
    list.forEach(u => {
      const tr = document.createElement('tr');
      tr.innerHTML = `
        <td>${escapeHtml(u.username || '')}</td>
        <td>${escapeHtml(u.full_name || '')}</td>
        <td>${escapeHtml(u.department || '')}</td>
        <td>${formatDate(u.last_seen)}</td>`;
      tbody.appendChild(tr);
    });
  }

  AccountsPage.loadDashboard = loadDashboard;
  AccountsPage.renderOnlineUsers = renderOnlineUsers;
})();
