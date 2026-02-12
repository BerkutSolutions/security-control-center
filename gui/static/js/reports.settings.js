(() => {
  const state = ReportsPage.state;

  function bindSettings() {
    const form = document.getElementById('reports-settings-form');
    if (form) {
      form.onsubmit = async (e) => {
        e.preventDefault();
        await saveSettings();
      };
    }
  }

  async function loadSettings() {
    try {
      const res = await Api.get('/api/reports/settings');
      state.settings = res;
      renderSettings();
      if (ReportsPage.applySettingsToBuilder) {
        ReportsPage.applySettingsToBuilder(state.settings);
      }
    } catch (err) {
      console.warn('load settings', err);
    }
  }

  function renderSettings() {
    const s = state.settings || {};
    const cls = document.getElementById('reports-settings-classification');
    if (cls) cls.value = s.default_classification || 'INTERNAL';
    const tpl = document.getElementById('reports-settings-template');
    if (tpl) tpl.value = s.default_template_id || '';
    const headerEnabled = document.getElementById('reports-settings-header-enabled');
    if (headerEnabled) headerEnabled.checked = !!s.header_enabled;
    const logo = document.getElementById('reports-settings-logo');
    if (logo) logo.value = s.header_logo_path || '/gui/static/logo.png';
    const title = document.getElementById('reports-settings-title');
    if (title) title.value = s.header_title || '';
    const watermark = document.getElementById('reports-settings-watermark');
    if (watermark) watermark.value = s.watermark_threshold || '';
  }

  async function saveSettings() {
    ReportsPage.showAlert('reports-settings-alert', '');
    const payload = {
      default_classification: document.getElementById('reports-settings-classification')?.value || '',
      default_template_id: parseInt(document.getElementById('reports-settings-template')?.value || '0', 10) || 0,
      header_enabled: document.getElementById('reports-settings-header-enabled')?.checked || false,
      header_logo_path: document.getElementById('reports-settings-logo')?.value || '',
      header_title: document.getElementById('reports-settings-title')?.value || '',
      watermark_threshold: document.getElementById('reports-settings-watermark')?.value || ''
    };
    if (payload.default_template_id === 0) {
      payload.default_template_id = null;
    }
    try {
      const res = await Api.put('/api/reports/settings', payload);
      state.settings = res;
      ReportsPage.showAlert('reports-settings-alert', BerkutI18n.t('common.saved'), true);
    } catch (err) {
      ReportsPage.showAlert('reports-settings-alert', err.message || BerkutI18n.t('common.error'));
    }
  }

  ReportsPage.bindSettings = bindSettings;
  ReportsPage.loadSettings = loadSettings;
})();
