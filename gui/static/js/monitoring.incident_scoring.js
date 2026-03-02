(() => {
  function t(key) {
    return (typeof MonitoringPage !== 'undefined' && MonitoringPage.t) ? MonitoringPage.t(key) : key;
  }

  function clamp01(v) {
    const num = Number(v);
    if (!Number.isFinite(num)) return null;
    if (num < 0) return 0;
    if (num > 1) return 1;
    return num;
  }

  function formatScore(val) {
    const v = clamp01(val);
    if (v === null) return '-';
    return v.toFixed(2);
  }

  function scoreClass(val) {
    const v = clamp01(val);
    if (v === null) return 'score-unknown';
    if (v >= 0.85) return 'score-high';
    if (v >= 0.50) return 'score-medium';
    return 'score-low';
  }

  function reasonLabel(code) {
    const c = String(code || '').trim();
    if (!c) return '';
    const key = `monitoring.scoring.reason.${c}`;
    const translated = t(key);
    return (translated && translated !== key) ? translated : c;
  }

  function renderReasonChips(container, emptyEl, reasons) {
    if (!container) return;
    container.innerHTML = '';
    const list = Array.isArray(reasons) ? reasons.filter(Boolean) : [];
    if (!list.length) {
      if (emptyEl) emptyEl.hidden = false;
      return;
    }
    if (emptyEl) emptyEl.hidden = true;
    list.slice(0, 10).forEach((code) => {
      const chip = document.createElement('span');
      chip.className = 'monitor-item-tag';
      chip.textContent = reasonLabel(code);
      chip.title = String(code);
      container.appendChild(chip);
    });
  }

  if (typeof MonitoringPage !== 'undefined') {
    MonitoringPage.incidentScoring = {
      clamp01,
      formatScore,
      scoreClass,
      reasonLabel,
      renderReasonChips,
    };
  }
})();

