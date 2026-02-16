const BackupsPlan = (() => {
  const DEFAULT_TIME = '02:00';

  function init() {
    bindActions();
  }

  function bindActions() {
    const saveBtn = document.getElementById('backups-plan-save');
    const refreshBtn = document.getElementById('backups-plan-refresh');
    const schedule = document.getElementById('backups-plan-schedule-type');
    if (saveBtn) saveBtn.addEventListener('click', save);
    if (refreshBtn) refreshBtn.addEventListener('click', () => load(true));
    if (schedule) schedule.addEventListener('change', syncScheduleVisibility);
  }

  async function load(showSpinner = false) {
    toggleBusy(showSpinner);
    try {
      const res = await BackupsPage.apiGet('/api/backups/plan');
      render(res.item || {});
    } catch (err) {
      const e = BackupsPage.parseError(err);
      BackupsPage.setAlert('error', e.i18nKey, BackupsPage.t('common.serverError'));
    } finally {
      toggleBusy(false);
    }
  }

  async function save() {
    toggleBusy(true);
    BackupsPage.setAlert('', '', '');
    try {
      const payload = {
        enabled: !!document.getElementById('backups-plan-enabled')?.checked,
        cron_expression: '',
        schedule_type: (document.getElementById('backups-plan-schedule-type')?.value || 'daily').trim(),
        schedule_weekday: parseInt(document.getElementById('backups-plan-weekday')?.value || '0', 10),
        schedule_month_anchor: monthAnchorFromType((document.getElementById('backups-plan-schedule-type')?.value || 'daily').trim()),
        schedule_hour: parseTime().hour,
        schedule_minute: parseTime().minute,
        retention_days: parseInt(document.getElementById('backups-plan-retention-days')?.value || '30', 10),
        keep_last_successful: parseInt(document.getElementById('backups-plan-keep-last')?.value || '5', 10),
        include_files: !!document.getElementById('backups-plan-include-files')?.checked,
      };
      const res = await BackupsPage.apiPut('/api/backups/plan', payload);
      render(res.item || {});
      BackupsPage.setAlert('success', payload.enabled ? 'backups.plan.enabled' : 'backups.plan.disabled', '');
    } catch (err) {
      const e = BackupsPage.parseError(err);
      BackupsPage.setAlert('error', e.i18nKey, BackupsPage.t('common.serverError'));
    } finally {
      toggleBusy(false);
    }
  }

  function render(item) {
    setValue('backups-plan-schedule-type', item.schedule_type || 'daily');
    setValue('backups-plan-weekday', `${typeof item.schedule_weekday === 'number' ? item.schedule_weekday : 0}`);
    setValue('backups-plan-time', `${pad2(item.schedule_hour ?? 2)}:${pad2(item.schedule_minute ?? 0)}`);
    setValue('backups-plan-retention-days', `${item.retention_days || 30}`);
    setValue('backups-plan-keep-last', `${item.keep_last_successful || 5}`);
    setChecked('backups-plan-enabled', !!item.enabled);
    setChecked('backups-plan-include-files', !!item.include_files);
    syncScheduleVisibility();
  }

  function setValue(id, value) {
    const el = document.getElementById(id);
    if (el) el.value = value;
  }

  function setChecked(id, checked) {
    const el = document.getElementById(id);
    if (el) el.checked = !!checked;
  }

  function toggleBusy(disabled) {
    const ids = ['backups-plan-save', 'backups-plan-refresh', 'backups-plan-enabled', 'backups-plan-schedule-type', 'backups-plan-weekday', 'backups-plan-time', 'backups-plan-retention-days', 'backups-plan-keep-last', 'backups-plan-include-files'];
    ids.forEach((id) => {
      const el = document.getElementById(id);
      if (el) el.disabled = disabled;
    });
  }

  function syncScheduleVisibility() {
    const type = (document.getElementById('backups-plan-schedule-type')?.value || 'daily').trim();
    const weekdayWrap = document.getElementById('backups-plan-weekday-wrap');
    if (weekdayWrap) weekdayWrap.hidden = type !== 'weekly';
  }

  function parseTime() {
    const raw = (document.getElementById('backups-plan-time')?.value || DEFAULT_TIME).trim();
    const parts = raw.split(':');
    const hour = clampInt(parts[0], 2, 0, 23);
    const minute = clampInt(parts[1], 0, 0, 59);
    return { hour, minute };
  }

  function monthAnchorFromType(type) {
    return type === 'monthly_end' ? 'end' : 'start';
  }

  function clampInt(raw, fallback, min, max) {
    const value = parseInt(`${raw}`, 10);
    if (Number.isNaN(value)) return fallback;
    if (value < min) return min;
    if (value > max) return max;
    return value;
  }

  function pad2(v) {
    const n = clampInt(v, 0, 0, 99);
    return n < 10 ? `0${n}` : `${n}`;
  }

  return { init, load };
})();
