(() => {
  if (typeof window === 'undefined') return;
  if (window.DocsPage && window.DocsPage.init) return;
  const scripts = [
    '/static/js/docs.core.js',
    '/static/js/docs.ui.js',
    '/static/js/docs.tabs.js',
    '/static/js/docs.folders.js',
    '/static/js/docs.users.js',
    '/static/js/docs.list.js',
    '/static/js/docs.onlyoffice.js',
    '/static/js/docs.editor.js',
    '/static/js/docs.templates.js',
    '/static/js/approvals.workflow.js',
    '/static/js/docs.viewer.js'
  ];
  let loaded = 0;
  const onLoad = () => {
    loaded += 1;
    if (loaded !== scripts.length) return;
    if (!window.DocsPage || !window.DocsPage.init) return;
  };
  scripts.forEach(src => {
    const script = document.createElement('script');
    script.src = src;
    script.onload = onLoad;
    script.onerror = onLoad;
    document.head.appendChild(script);
  });
})();
