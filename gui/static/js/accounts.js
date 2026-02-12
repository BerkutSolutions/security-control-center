(() => {
  if (typeof window === 'undefined') return;
  if (window.AccountsPage && window.AccountsPage.init) return;
  const scripts = [
    '/static/js/accounts.core.js',
    '/static/js/accounts.dashboard.js',
    '/static/js/accounts.roles.js',
    '/static/js/accounts.groups.js',
    '/static/js/accounts.users.js',
    '/static/js/accounts.bulk.js',
    '/static/js/accounts.import.js'
  ];
  let loaded = 0;
  const onLoad = () => {
    loaded += 1;
    if (loaded !== scripts.length) return;
    if (!window.AccountsPage || !window.AccountsPage.init) return;
    if (document.readyState === 'loading') {
      window.addEventListener('DOMContentLoaded', () => window.AccountsPage.init());
      return;
    }
    if (document.getElementById('accounts-page')) {
      window.AccountsPage.init();
    }
  };
  scripts.forEach(src => {
    const script = document.createElement('script');
    script.src = src;
    script.onload = onLoad;
    script.onerror = onLoad;
    document.head.appendChild(script);
  });
})();
