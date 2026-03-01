(() => {
  const els = {};

  function bindSettings() {
    els.alert = document.getElementById('monitoring-settings-alert');
    els.form = document.getElementById('monitoring-settings-form');
    els.defaultsForm = document.getElementById('monitoring-defaults-form');
    els.save = document.getElementById('monitoring-settings-save');
    els.engineStatsCard = document.getElementById('monitoring-engine-stats-card');
    els.engineStats = document.getElementById('monitoring-engine-stats');
    els.engineStatsAlert = document.getElementById('monitoring-engine-stats-alert');
    els.engineStatsRefresh = document.getElementById('monitoring-engine-stats-refresh');
    els.retention = document.getElementById('monitoring-retention');
    els.maxConcurrent = document.getElementById('monitoring-max-concurrent');
    els.defaultTimeout = document.getElementById('monitoring-default-timeout');
    els.defaultInterval = document.getElementById('monitoring-default-interval');
    els.defaultRetries = document.getElementById('monitoring-default-retries');
    els.defaultRetryInterval = document.getElementById('monitoring-default-retry-interval');
    els.defaultSla = document.getElementById('monitoring-default-sla');
    els.engineEnabled = document.getElementById('monitoring-engine-enabled');
    els.allowPrivate = document.getElementById('monitoring-allow-private');
    els.tlsRefresh = document.getElementById('monitoring-tls-refresh');
    els.tlsExpiring = document.getElementById('monitoring-tls-expiring');
    els.notifySuppress = document.getElementById('monitoring-notify-suppress');
    els.notifyRepeat = document.getElementById('monitoring-notify-repeat');
    els.notifyMaintenance = document.getElementById('monitoring-notify-maintenance');
    els.logDnsEvents = document.getElementById('monitoring-log-dns-events');
    els.autoTLSIncident = document.getElementById('monitoring-auto-tls-incident');
    els.autoTLSIncidentDays = document.getElementById('monitoring-auto-tls-incident-days');
    els.autoIncidentCloseOnUp = document.getElementById('monitoring-auto-incident-close-on-up');

    const canManage = MonitoringPage.hasPermission('monitoring.settings.manage');
    if (!canManage) {
      const card = els.form?.closest('.card');
      if (card) card.hidden = true;
      const defaultsCard = els.defaultsForm?.closest('.card');
      if (defaultsCard) defaultsCard.hidden = true;
    }
    if (canManage && els.save) {
      els.save.addEventListener('click', async () => {
        await saveSettings();
      });
    }
    if (canManage) loadSettings();

    bindEngineStats();
  }

  function bindEngineStats() {
    if (!MonitoringPage.hasPermission('monitoring.view')) {
      if (els.engineStatsCard) els.engineStatsCard.hidden = true;
      return;
    }
    if (els.engineStatsRefresh) {
      els.engineStatsRefresh.addEventListener('click', async () => {
        await loadEngineStats();
      });
    }
    loadEngineStats();
    setInterval(() => {
      loadEngineStats();
    }, 15000);
  }

  async function loadSettings() {
    try {
      const res = await Api.get('/api/monitoring/settings');
      MonitoringPage.state.settings = res;
      renderSettings(res);
    } catch (err) {
      console.error('monitoring settings', err);
    }
  }

  function renderSettings(settings) {
    if (!settings) return;
    if (els.retention) els.retention.value = settings.retention_days || 30;
    if (els.maxConcurrent) els.maxConcurrent.value = settings.max_concurrent_checks || 10;
    if (els.defaultTimeout) els.defaultTimeout.value = settings.default_timeout_sec || 5;
    if (els.defaultInterval) els.defaultInterval.value = settings.default_interval_sec || 60;
    if (els.defaultRetries) els.defaultRetries.value = settings.default_retries ?? 0;
    if (els.defaultRetryInterval) els.defaultRetryInterval.value = settings.default_retry_interval_sec || 5;
    if (els.defaultSla) els.defaultSla.value = settings.default_sla_target_pct || 90;
    if (els.engineEnabled) els.engineEnabled.checked = !!settings.engine_enabled;
    if (els.allowPrivate) els.allowPrivate.checked = !!settings.allow_private_networks;
    if (els.tlsRefresh) els.tlsRefresh.value = settings.tls_refresh_hours || 24;
    if (els.tlsExpiring) els.tlsExpiring.value = settings.tls_expiring_days || 30;
    if (els.notifySuppress) els.notifySuppress.value = settings.notify_suppress_minutes || 5;
    if (els.notifyRepeat) els.notifyRepeat.value = settings.notify_repeat_down_minutes || 30;
    if (els.notifyMaintenance) els.notifyMaintenance.checked = !!settings.notify_maintenance;
    if (els.logDnsEvents) els.logDnsEvents.checked = settings.log_dns_events !== false;
    if (els.autoTLSIncident) els.autoTLSIncident.checked = !!settings.auto_tls_incident;
    if (els.autoTLSIncidentDays) els.autoTLSIncidentDays.value = settings.auto_tls_incident_days || 14;
    if (els.autoIncidentCloseOnUp) els.autoIncidentCloseOnUp.checked = !!settings.auto_incident_close_on_up;
  }

  async function loadEngineStats() {
    if (!MonitoringPage.hasPermission('monitoring.view')) return;
    try {
      const stats = await Api.get('/api/monitoring/engine/stats');
      renderEngineStats(stats);
    } catch (err) {
      if (els.engineStatsAlert) {
        MonitoringPage.showAlert(els.engineStatsAlert, MonitoringPage.sanitizeErrorMessage(err.message || err), false);
      }
    }
  }

  function renderEngineStats(stats) {
    if (!els.engineStats) return;
    if (!stats) {
      els.engineStats.innerHTML = '';
      return;
    }
    MonitoringPage.hideAlert(els.engineStatsAlert);
    const tuning = stats.tuning || {};
    const jitter = (tuning.jitter_percent || 0) > 0
      ? `${tuning.jitter_percent}% (max ${tuning.jitter_max_seconds || 0}s)`
      : MonitoringPage.t('monitoring.engineStats.jitterDisabled');
    const waitQ = stats.check_wait_time_seconds_quantiles || {};
    const durQ = stats.attempt_duration_ms_quantiles || {};
    const errCounts = stats.error_class_counts || {};
    const errPairs = Object.keys(errCounts)
      .sort()
      .map(k => `${k}: ${errCounts[k]}`)
      .join(', ') || '-';
    const totalAttempts = stats.attempts_total ?? 0;
    const retryAttempts = stats.retry_attempts_total ?? 0;
    const retryShare = totalAttempts > 0 ? (retryAttempts / totalAttempts) : 0;
    els.engineStats.innerHTML = `
      <ul class="metric-list">
        <li><strong>${MonitoringPage.t('monitoring.engineStats.inflight')}</strong> ${stats.inflight_checks ?? 0} / ${stats.max_concurrent ?? 0}</li>
        <li><strong>${MonitoringPage.t('monitoring.engineStats.dueLast')}</strong> ${stats.due_count_last_tick ?? 0}</li>
        <li><strong>${MonitoringPage.t('monitoring.engineStats.startedLast')}</strong> ${stats.started_last_tick ?? 0}</li>
        <li><strong>${MonitoringPage.t('monitoring.engineStats.retryDueLast')}</strong> ${stats.retry_due_count_last_tick ?? 0}</li>
        <li><strong>${MonitoringPage.t('monitoring.engineStats.retryStartedLast')}</strong> ${stats.retry_started_last_tick ?? 0}</li>
        <li><strong>${MonitoringPage.t('monitoring.engineStats.retryBudgetLast')}</strong> ${stats.retry_budget_last_tick ?? 0}</li>
        <li><strong>${MonitoringPage.t('monitoring.engineStats.skippedSem')}</strong> ${stats.skipped_due_due_to_semaphore_last_tick ?? 0}</li>
        <li><strong>${MonitoringPage.t('monitoring.engineStats.skippedJitter')}</strong> ${stats.skipped_due_due_to_jitter_last_tick ?? 0}</li>
        <li><strong>${MonitoringPage.t('monitoring.engineStats.waitP95')}</strong> ${(waitQ.p95 ?? 0).toFixed(3)}s</li>
        <li><strong>${MonitoringPage.t('monitoring.engineStats.durP95')}</strong> ${(durQ.p95 ?? 0).toFixed(0)}ms</li>
        <li><strong>${MonitoringPage.t('monitoring.engineStats.jitter')}</strong> ${jitter}</li>
        <li><strong>${MonitoringPage.t('monitoring.engineStats.retryScheduled')}</strong> ${stats.retry_scheduled_total ?? 0}</li>
        <li><strong>${MonitoringPage.t('monitoring.engineStats.retryShare')}</strong> ${(retryShare * 100).toFixed(1)}%</li>
        <li><strong>${MonitoringPage.t('monitoring.engineStats.errors')}</strong> ${errPairs}</li>
      </ul>
    `;
  }

  async function saveSettings() {
    if (!MonitoringPage.hasPermission('monitoring.settings.manage')) return;
    MonitoringPage.hideAlert(els.alert);
    const payload = {
      retention_days: parseInt(els.retention.value, 10) || 0,
      max_concurrent_checks: parseInt(els.maxConcurrent.value, 10) || 0,
      default_timeout_sec: parseInt(els.defaultTimeout.value, 10) || 0,
      default_interval_sec: parseInt(els.defaultInterval.value, 10) || 0,
      default_retries: parseInt(els.defaultRetries?.value, 10) || 0,
      default_retry_interval_sec: parseInt(els.defaultRetryInterval?.value, 10) || 0,
      default_sla_target_pct: parseFloat(els.defaultSla?.value) || 0,
      engine_enabled: !!els.engineEnabled.checked,
      allow_private_networks: !!els.allowPrivate.checked,
      tls_refresh_hours: parseInt(els.tlsRefresh.value, 10) || 0,
      tls_expiring_days: parseInt(els.tlsExpiring.value, 10) || 0,
      notify_suppress_minutes: parseInt(els.notifySuppress.value, 10) || 0,
      notify_repeat_down_minutes: parseInt(els.notifyRepeat.value, 10) || 0,
      notify_maintenance: !!els.notifyMaintenance.checked,
      log_dns_events: !!els.logDnsEvents?.checked,
      auto_tls_incident: !!els.autoTLSIncident?.checked,
      auto_tls_incident_days: parseInt(els.autoTLSIncidentDays?.value, 10) || 0,
      auto_incident_close_on_up: !!els.autoIncidentCloseOnUp?.checked,
    };
    try {
      const res = await Api.put('/api/monitoring/settings', payload);
      MonitoringPage.state.settings = res;
      renderSettings(res);
      MonitoringPage.showAlert(els.alert, MonitoringPage.t('monitoring.settings.saved'), true);
    } catch (err) {
      MonitoringPage.showAlert(els.alert, MonitoringPage.sanitizeErrorMessage(err.message || err), false);
    }
  }

  if (typeof MonitoringPage !== 'undefined') {
    MonitoringPage.bindSettings = bindSettings;
  }
})();
