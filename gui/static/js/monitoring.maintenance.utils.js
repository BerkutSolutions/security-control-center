(() => {
  function buildMonthDayList(container) {
    if (!container || container.children.length) return;
    for (let i = 1; i <= 31; i += 1) {
      const label = document.createElement('label');
      label.className = 'checkbox maintenance-day';
      label.innerHTML = `<span>${i}</span><input type="checkbox" value="${i}">`;
      container.appendChild(label);
    }
  }

  function markDays(root, days) {
    const set = new Set((days || []).map((n) => Number(n)));
    root?.querySelectorAll('input[type="checkbox"]').forEach((input) => {
      input.checked = set.has(Number(input.value));
    });
  }

  function getCheckedInts(root) {
    if (!root) return [];
    return Array.from(root.querySelectorAll('input[type="checkbox"]:checked'))
      .map((node) => Number(node.value))
      .filter((v) => Number.isInteger(v) && v > 0);
  }

  function monitorsLabel(item, monitors) {
    const ids = Array.isArray(item.monitor_ids) ? item.monitor_ids : [];
    if (!ids.length) return '-';
    const names = ids.map((id) => {
      const mon = (monitors || []).find((m) => Number(m.id) === Number(id));
      return mon?.name || `#${id}`;
    });
    return names.join(', ');
  }

  function strategyLabel(strategy, t) {
    const map = {
      single: t('monitoring.maintenance.strategy.single'),
      cron: t('monitoring.maintenance.strategy.cron'),
      interval: t('monitoring.maintenance.strategy.interval'),
      weekday: t('monitoring.maintenance.strategy.weekday'),
      monthday: t('monitoring.maintenance.strategy.monthday'),
    };
    return map[(strategy || '').toLowerCase()] || strategy || '-';
  }

  function windowLabel(item, formatDate) {
    if ((item.strategy || '').toLowerCase() === 'single') {
      return `${formatDate(item.starts_at)} - ${formatDate(item.ends_at)}`;
    }
    const schedule = item.schedule || {};
    if ((item.strategy || '').toLowerCase() === 'cron') {
      return `${schedule.cron_expression || '-'} / ${schedule.duration_min || 0}m`;
    }
    if (schedule.window_start && schedule.window_end) {
      return `${schedule.window_start} - ${schedule.window_end}`;
    }
    return '-';
  }

  function defaultTimezone() {
    if (typeof AppTime !== 'undefined' && AppTime.getTimeZone) return AppTime.getTimeZone();
    return 'UTC';
  }

  function toInputValue(value) {
    if (!value) return '';
    const dt = new Date(value);
    if (Number.isNaN(dt.getTime())) return '';
    const pad = (num) => `${num}`.padStart(2, '0');
    return `${dt.getFullYear()}-${pad(dt.getMonth() + 1)}-${pad(dt.getDate())}T${pad(dt.getHours())}:${pad(dt.getMinutes())}`;
  }

  function fromInputValue(value) {
    if (!value) return '';
    const raw = String(value).trim();
    const dt = new Date(raw);
    if (!Number.isNaN(dt.getTime())) return dt.toISOString();
    const m = raw.match(/^(\d{2})\.(\d{2})\.(\d{4})[ T](\d{2}):(\d{2})$/);
    if (!m) return '';
    const parsed = new Date(Number(m[3]), Number(m[2]) - 1, Number(m[1]), Number(m[4]), Number(m[5]), 0, 0);
    if (Number.isNaN(parsed.getTime())) return '';
    return parsed.toISOString();
  }

  function escapeHtml(str) {
    return (str || '').toString().replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  }

  if (typeof window !== 'undefined') {
    window.MonitoringMaintenanceUtils = {
      buildMonthDayList,
      markDays,
      getCheckedInts,
      monitorsLabel,
      strategyLabel,
      windowLabel,
      defaultTimezone,
      toInputValue,
      fromInputValue,
      escapeHtml,
    };
  }
})();
