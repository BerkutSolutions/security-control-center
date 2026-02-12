(() => {
  if (typeof window === 'undefined') return;
  const scripts = [
    '/static/js/incidents.detail.core.js',
    '/static/js/incidents.detail.decisions.js',
    '/static/js/incidents.detail.stage-blocks.js',
    '/static/js/incidents.detail.stages.js',
    '/static/js/incidents.detail.links.js',
    '/static/js/incidents.detail.attachments.js',
    '/static/js/incidents.detail.timeline.js',
    '/static/js/incidents.detail.export.js'
  ];
  scripts.forEach(src => {
    const script = document.createElement('script');
    script.src = src;
    script.async = false;
    document.head.appendChild(script);
  });
})();
