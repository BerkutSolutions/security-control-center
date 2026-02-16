(() => {
  const state = IncidentsPage.state;
  const { t, showError, escapeHtml, formatDate } = IncidentsPage;
  const TIMELINE_TEMPLATES = {
    'incident.create': { type: 'incidents.timeline.event.incident.create', message: 'incidents.timeline.message.incident.create' },
    'status.change': { type: 'incidents.timeline.event.status.change', message: 'incidents.timeline.message.status.change' },
    'severity.change': { type: 'incidents.timeline.event.severity.change', message: 'incidents.timeline.message.severity.change' },
    'assignee.change': { type: 'incidents.timeline.event.assignee.change', message: 'incidents.timeline.message.assignee.change' },
    'classification.change': { type: 'incidents.timeline.event.classification.change', message: 'incidents.timeline.message.classification.change' },
    'owner.change': { type: 'incidents.timeline.event.owner.change', message: 'incidents.timeline.message.owner.change' },
    'incident.delete': { type: 'incidents.timeline.event.incident.delete', message: 'incidents.timeline.message.incident.delete' },
    'incident.restore': { type: 'incidents.timeline.event.incident.restore', message: 'incidents.timeline.message.incident.restore' },
    'incident.closed': { type: 'incidents.timeline.event.incident.closed', message: 'incidents.timeline.message.incident.closed' },
    'stage.add': { type: 'incidents.timeline.event.stage.add', message: 'incidents.timeline.message.stage.add' },
    'stage.rename': { type: 'incidents.timeline.event.stage.rename', message: 'incidents.timeline.message.stage.rename' },
    'stage.reorder': { type: 'incidents.timeline.event.stage.reorder', message: 'incidents.timeline.message.stage.reorder' },
    'stage.delete': { type: 'incidents.timeline.event.stage.delete', message: 'incidents.timeline.message.stage.delete' },
    'stage.completed': { type: 'incidents.timeline.event.stage.completed', message: 'incidents.timeline.message.stage.completed' },
    'stage.content.update': { type: 'incidents.timeline.event.stage.content.update', message: 'incidents.timeline.message.stage.content.update' },
    'link.add': { type: 'incidents.timeline.event.link.add', message: 'incidents.timeline.message.link.add' },
    'link.remove': { type: 'incidents.timeline.event.link.remove', message: 'incidents.timeline.message.link.remove' },
    'attachment.upload': { type: 'incidents.timeline.event.attachment.upload', message: 'incidents.timeline.message.attachment.upload' },
    'attachment.download': { type: 'incidents.timeline.event.attachment.download', message: 'incidents.timeline.message.attachment.download' },
    'attachment.delete': { type: 'incidents.timeline.event.attachment.delete', message: 'incidents.timeline.message.attachment.delete' },
    'artifact.file.upload': { type: 'incidents.timeline.event.artifact.file.upload', message: 'incidents.timeline.message.artifact.file.upload' },
    'artifact.file.download': { type: 'incidents.timeline.event.artifact.file.download', message: 'incidents.timeline.message.artifact.file.download' },
    'artifact.file.delete': { type: 'incidents.timeline.event.artifact.file.delete', message: 'incidents.timeline.message.artifact.file.delete' },
    'incident.export': { type: 'incidents.timeline.event.incident.export', message: 'incidents.timeline.message.incident.export' },
    'report.doc.create': { type: 'incidents.timeline.event.report.doc.create', message: 'incidents.timeline.message.report.doc.create' },
    'monitoring.auto_create': { type: 'incidents.timeline.event.monitoring.auto_create', message: 'incidents.timeline.message.monitoring.auto_create' },
    'monitoring.auto_close': { type: 'incidents.timeline.event.monitoring.auto_close', message: 'incidents.timeline.message.monitoring.auto_close' },
  };

  function bindTimelineControls(incidentId) {
    const tabId = `incident-${incidentId}`;
    const panel = document.querySelector(`#incidents-panels [data-tab="${tabId}"]`);
    if (!panel) return;
    const containers = panel.querySelectorAll('.incident-timeline');
    if (!containers.length) return;
    containers.forEach(container => {
      if (container.dataset.bound === '1') return;
      container.dataset.bound = '1';
      const filterSel = container.querySelector('.incident-timeline-filter');
      const refreshBtn = container.querySelector('.incident-timeline-refresh');
      const saveBtn = container.querySelector('.incident-timeline-save');
      const typeSel = container.querySelector('.incident-timeline-type');
      const customTypeInput = container.querySelector('.incident-timeline-type-custom');
      if (typeSel && customTypeInput) {
        typeSel.onchange = () => {
          if (typeSel.value === '__custom__') {
            customTypeInput.hidden = false;
            customTypeInput.focus();
          } else {
            customTypeInput.hidden = true;
            customTypeInput.value = '';
          }
        };
      }
      if (filterSel) {
        filterSel.onchange = () => {
          const detail = state.incidentDetails.get(incidentId);
          if (detail) {
            detail.timelineFilter = filterSel.value;
            ensureIncidentTimeline(incidentId, true);
          }
        };
      }
      if (refreshBtn) {
        refreshBtn.onclick = () => ensureIncidentTimeline(incidentId, true);
      }
      if (saveBtn) {
        saveBtn.onclick = async () => {
          const msgInput = container.querySelector('.incident-timeline-message');
          const message = msgInput?.value.trim();
          const eventType = collectTimelineType(container);
          if (!message) return;
          try {
            await Api.post(`/api/incidents/${incidentId}/timeline`, { message, event_type: eventType });
            if (msgInput) msgInput.value = '';
            if (typeSel) typeSel.value = '';
            if (customTypeInput) {
              customTypeInput.value = '';
              customTypeInput.hidden = true;
            }
            await ensureIncidentTimeline(incidentId, true);
          } catch (err) {
            showError(err, 'incidents.timeline.saveFailed');
          }
        };
      }
    });
  }

  async function ensureIncidentTimeline(incidentId, force) {
    const detail = state.incidentDetails.get(incidentId);
    if (!detail) return;
    if (detail.timelineLoaded && !force) {
      renderIncidentTimeline(incidentId);
      return;
    }
    detail.timelineLoading = true;
    try {
      const qs = detail.timelineFilter ? `?event_type=${encodeURIComponent(detail.timelineFilter)}` : '';
      const res = await Api.get(`/api/incidents/${incidentId}/timeline${qs}`);
      detail.timeline = res.items || [];
      detail.timelineLoaded = true;
    } catch (err) {
      detail.timeline = [];
      showError(err, 'incidents.timeline.loadFailed');
    } finally {
      detail.timelineLoading = false;
      renderIncidentTimeline(incidentId);
    }
  }

  function renderIncidentTimeline(incidentId) {
    const tabId = `incident-${incidentId}`;
    const panel = document.querySelector(`#incidents-panels [data-tab="${tabId}"]`);
    const detail = state.incidentDetails.get(incidentId);
    if (!panel || !detail) return;
    const containers = Array.from(panel.querySelectorAll('.incident-timeline'));
    const lists = containers.length
      ? containers.map(c => c.querySelector('.timeline-list')).filter(Boolean)
      : Array.from(panel.querySelectorAll('.timeline-list'));
    if (!lists.length) return;
    lists.forEach(list => { list.innerHTML = ''; });
    const events = detail.timeline || [];
    const types = Array.from(new Set(events.map(e => e.event_type).filter(Boolean)));
    const ensureFilterOptions = (container) => {
      const filterSel = container.querySelector('.incident-timeline-filter');
      if (!filterSel) return;
      filterSel.innerHTML = `<option value="">${t('incidents.timeline.filterAll')}</option>`;
      types.forEach(tp => {
        const opt = document.createElement('option');
        opt.value = tp;
        opt.textContent = translateTimelineType(tp);
        if (detail.timelineFilter === tp) opt.selected = true;
        filterSel.appendChild(opt);
      });
      if (detail.timelineFilter && !types.includes(detail.timelineFilter)) {
        const opt = document.createElement('option');
        opt.value = detail.timelineFilter;
        opt.textContent = translateTimelineType(detail.timelineFilter);
        opt.selected = true;
        filterSel.appendChild(opt);
      }
      filterSel.value = detail.timelineFilter || '';
    };
    const ensureTypeOptions = (container) => {
      const typeSelect = container.querySelector('.incident-timeline-type');
      if (!typeSelect) return;
      typeSelect.innerHTML = `<option value="">${t('incidents.timeline.typePlaceholder')}</option>`;
      types.forEach(tp => {
        const opt = document.createElement('option');
        opt.value = tp;
        opt.textContent = translateTimelineType(tp);
        typeSelect.appendChild(opt);
      });
      const customOpt = document.createElement('option');
      customOpt.value = '__custom__';
      customOpt.textContent = t('incidents.timeline.typeCustom');
      typeSelect.appendChild(customOpt);
    };
    if (containers.length) {
      containers.forEach(container => {
        ensureFilterOptions(container);
        ensureTypeOptions(container);
      });
    } else {
      lists.forEach(list => {
        const container = list.closest('.incident-timeline');
        if (container) {
          ensureFilterOptions(container);
          ensureTypeOptions(container);
        }
      });
    }
    const targetLists = lists.length ? lists : [];
    if (!events.length) {
      targetLists.forEach(list => {
        const empty = document.createElement('div');
        empty.className = 'empty-state';
        empty.textContent = t('incidents.timeline.empty');
        list.appendChild(empty);
      });
      return;
    }
    events.forEach(ev => {
      const row = document.createElement('div');
      row.className = 'timeline-item';
      row.innerHTML = `
        <div class="timeline-time">${escapeHtml(formatDate(ev.created_at))}</div>
        <div class="timeline-body">
          <div class="timeline-type">${escapeHtml(translateTimelineType(ev.event_type))}</div>
          <div class="timeline-msg">${escapeHtml(translateTimelineMessage(ev.event_type, ev.message))}</div>
        </div>`;
      targetLists.forEach(list => list.appendChild(row.cloneNode(true)));
    });
  }

  function translateTimelineType(eventType) {
    if (!eventType) return '';
    const key = TIMELINE_TEMPLATES[eventType]?.type;
    if (key) {
      const translated = t(key);
      if (translated && translated !== key) return translated;
    }
    return eventType;
  }

  function translateTimelineMessage(eventType, raw) {
    const tplKey = TIMELINE_TEMPLATES[eventType]?.message;
    let detail = cleanDetail(eventType, raw) || '';
    if (eventType === 'status.change') {
      detail = humanizeStatus(detail);
    }
    const fallback = detail || raw || '';
    if (tplKey) {
      const tpl = t(tplKey);
      if (tpl && tpl !== tplKey) {
        return tpl.replace('{detail}', fallback).replace('{value}', fallback);
      }
    }
    return fallback;
  }

  function humanizeStatus(detail) {
    if (!detail) return '';
    const parts = detail.split('->').map(p => p.trim()).filter(Boolean);
    if (parts.length === 2) {
      const [from, to] = parts;
      const fromLabel = t(`incidents.status.${from}`) || from;
      const toLabel = t(`incidents.status.${to}`) || to;
      return `${fromLabel} → ${toLabel}`;
    }
    return detail;
  }

  function cleanDetail(evType, raw) {
    if (!raw) return '';
    if (evType.startsWith('stage.')) {
      const idx = raw.indexOf(':');
      return idx >= 0 ? raw.slice(idx + 1).trim() : raw;
    }
    return raw;
  }

  function collectTimelineType(container) {
    const select = container.querySelector('.incident-timeline-type');
    const custom = container.querySelector('.incident-timeline-type-custom');
    if (!select) return '';
    if (select.value === '__custom__') {
      return custom?.value.trim() || '';
    }
    return select.value || '';
  }

  IncidentsPage.bindTimelineControls = bindTimelineControls;
  IncidentsPage.ensureIncidentTimeline = ensureIncidentTimeline;
  IncidentsPage.renderIncidentTimeline = renderIncidentTimeline;
})();
