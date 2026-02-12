(() => {
  if (typeof window === 'undefined') return;
  if (window.IncidentsPage && window.IncidentsPage.init) return;
  const scripts = [
    '/static/js/incidents.core.js',
    '/static/js/incidents.data.js',
    '/static/js/incidents.tabs.js',
    '/static/js/incidents.dashboard.js',
    '/static/js/incidents.list.js',
    '/static/js/incidents.create.attachments.js',
    '/static/js/incidents.create.js',
    '/static/js/incidents.detail.core.js',
    '/static/js/incidents.detail.decisions.js',
    '/static/js/incidents.detail.stage-blocks.js',
    '/static/js/incidents.detail.stages.js',
    '/static/js/incidents.detail.links.js',
    '/static/js/incidents.detail.attachments.js',
    '/static/js/incidents.detail.timeline.js',
    '/static/js/incidents.detail.export.js'
  ];
  let loaded = 0;
  const onLoad = () => {
    loaded += 1;
    if (loaded !== scripts.length) return;
    if (!window.IncidentsPage || !window.IncidentsPage.init) return;
    if (document.readyState === 'loading') {
      window.addEventListener('DOMContentLoaded', () => window.IncidentsPage.init());
      return;
    }
    if (document.getElementById('incidents-page')) {
      window.IncidentsPage.init();
    }
  };
  scripts.forEach(src => {
    const script = document.createElement('script');
    script.src = src;
    script.async = false;
    script.onload = onLoad;
    script.onerror = onLoad;
    document.head.appendChild(script);
  });
})();
