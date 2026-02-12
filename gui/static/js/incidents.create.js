(() => {
  const state = IncidentsPage.state;
  const { t, showError } = IncidentsPage;

  function openCreateTab() {
    const existing = state.tabs.find(t => t.type === 'create');
    if (existing) {
      IncidentsPage.switchTab(existing.id);
      return;
    }
    const tabId = `create-${Date.now()}`;
    state.tabs.push({
      id: tabId,
      type: 'create',
      titleKey: 'incidents.tabs.create',
      closable: true,
      draft: { dirty: false }
    });
    const panels = document.getElementById('incidents-panels');
    const panel = document.createElement('div');
    panel.className = 'tab-panel';
    panel.dataset.tab = tabId;
    panel.id = `panel-${tabId}`;
    panel.innerHTML = buildCreateFormHtml();
    panels.appendChild(panel);
    bindCreateForm(tabId);
    IncidentsPage.switchTab(tabId);
  }

  const FALLBACK_TAGS = [
    { code: 'COMMERCIAL_SECRET', label: 'Коммерческая тайна' },
    { code: 'PERSONAL_DATA', label: 'ПДн' },
    { code: 'CRITICAL_INFRASTRUCTURE', label: 'КИИ' },
    { code: 'FEDERAL_LAW_152', label: 'ФЗ 152' },
    { code: 'FEDERAL_LAW_149', label: 'ФЗ 149' },
    { code: 'FEDERAL_LAW_187', label: 'ФЗ 187' },
    { code: 'FEDERAL_LAW_63', label: 'ФЗ 63' },
    { code: 'PCI_DSS', label: 'PCI DSS' },
  ];

  function availableTags() {
    if (typeof TagDirectory !== 'undefined' && TagDirectory.all) {
      return TagDirectory.all();
    }
    return FALLBACK_TAGS;
  }

  function renderTagOptions(container, selected = []) {
    if (!container) return;
    if (typeof DocUI !== 'undefined' && DocUI.renderTagCheckboxes) {
      DocUI.renderTagCheckboxes(container, { selected });
      return;
    }
    const select = container.tagName === 'SELECT' ? container : container.querySelector('select');
    if (!select) return;
    const selectedSet = new Set((selected || []).map(v => (v || '').toUpperCase()));
    select.innerHTML = '';
    const tags = availableTags();
    tags.forEach(tag => {
      const code = tag.code || tag;
      const label = (typeof TagDirectory !== 'undefined' && TagDirectory.label)
        ? TagDirectory.label(code)
        : (tag.label || code);
      const opt = document.createElement('option');
      opt.value = code;
      opt.textContent = label;
      opt.dataset.label = opt.textContent;
      opt.selected = selectedSet.has((code || '').toUpperCase());
      select.appendChild(opt);
    });
  }

  function buildCreateFormHtml() {
    const now = new Date();
    const pad = (num) => `${num}`.padStart(2, '0');
    const dateValue = `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`;
    const timeValue = (typeof AppTime !== 'undefined' && AppTime.formatTime)
      ? AppTime.formatTime(now)
      : `${pad(now.getHours())}:${pad(now.getMinutes())}`;
    return `
      <div class="card form-card incident-form-card">
        <div class="card-header incident-create-header">
          <div>
            <h3>${t('incidents.createTitle')}</h3>
            <p>${t('incidents.createSubtitle')}</p>
          </div>
          <div class="create-actions">
            <button class="btn ghost" id="incident-title-generate">${t('incidents.create.generateTitle')}</button>
            <button class="btn primary" id="incident-form-save">${t('incidents.form.save')}</button>
            <button class="btn ghost" id="incident-form-cancel">${t('incidents.form.cancel')}</button>
          </div>
        </div>
        <div class="card-body">
          <form class="incident-create-form" id="incident-create-form">
            <div class="incident-create-grid">
              <div class="create-column">
                <div class="form-field required">
                  <label>${t('incidents.form.name')}</label>
                  <input name="title" id="incident-form-title" required>
                </div>
                <div class="form-row two-up">
                  <div class="form-field">
                    <label>${t('incidents.form.incidentType')}</label>
                    <select class="incident-type-select" id="incident-form-type"></select>
                  </div>
                  <div class="form-field">
                    <label>${t('incidents.form.detectionSource')}</label>
                    <select class="incident-source-select" id="incident-form-source"></select>
                  </div>
                </div>
              <div class="form-row two-up">
                <div class="form-field">
                  <label>${t('incidents.form.severity')}</label>
                  <select name="severity" id="incident-form-severity">
                    <option value="low">${t('incidents.severity.low')}</option>
                      <option value="medium">${t('incidents.severity.medium')}</option>
                      <option value="high">${t('incidents.severity.high')}</option>
                      <option value="critical">${t('incidents.severity.critical')}</option>
                    </select>
                  </div>
                  <div class="form-field">
                    <label>${t('incidents.form.sla')}</label>
                    <input id="incident-form-sla" value="${t('incidents.form.slaDefault')}" data-default-value="${t('incidents.form.slaDefault')}">
                  </div>
                </div>
                <div class="form-row two-up">
                  <div class="form-field">
                    <label>${t('incidents.form.slaDeadline')}</label>
                    <div class="datetime-field inline">
                      <input type="date" id="incident-form-deadline-date" class="input date-input" lang="ru" value="${dateValue}" data-default-value="${dateValue}">
                      <input type="text" id="incident-form-deadline-time" class="input time-input" value="${timeValue}" data-default-value="${timeValue}" placeholder="${t('common.timePlaceholder')}">
                    </div>
                  </div>
                  <div class="form-field">
                    <label>${t('incidents.form.assets')}</label>
                    <input id="incident-form-assets" placeholder="${t('incidents.form.assetsPlaceholder')}" data-default-value="">
                  </div>
                </div>
              <div class="form-row two-up people-row">
                <div class="form-field">
                  <label>${t('incidents.form.owner')}</label>
                  <select class="incident-user-select" id="incident-form-owner"></select>
                  <div class="selected-hint" id="incident-form-owner-hint"></div>
                </div>
                <div class="form-field">
                  <label>${t('incidents.form.performers')}</label>
                  <select multiple class="incident-user-select" id="incident-form-assignee"></select>
                  <div class="selected-hint" id="incident-form-assignee-hint"></div>
                </div>
                <div class="form-field">
                  <label>${t('incidents.form.watchers')}</label>
                  <select multiple class="incident-user-select" id="incident-form-participants"></select>
                  <div class="selected-hint" id="incident-form-participants-hint"></div>
                </div>
              </div>
                <div class="form-field">
                  <label>${t('incidents.form.tags')}</label>
                  <select id="incident-form-tags" multiple class="select"></select>
                  <div class="selected-hint" data-tag-hint="incident-form-tags"></div>
                </div>
              </div>
              <div class="create-column create-column-notes">
                <div class="form-field required">
                  <label>${t('incidents.form.description')}</label>
                  <textarea name="description" id="incident-form-description" placeholder="${t('incidents.form.descriptionPlaceholder')}"></textarea>
                </div>
                <div class="form-grid narrative-grid">
                  <div class="form-field">
                    <label>${t('incidents.form.whatHappened')}</label>
                    <textarea id="incident-form-what"></textarea>
                  </div>
                  <div class="form-field">
                    <label>${t('incidents.form.detectedAt')}</label>
                    <input id="incident-form-detected" placeholder="${t('incidents.form.detectedPlaceholder')}">
                  </div>
                  <div class="form-field">
                    <label>${t('incidents.form.affected')}</label>
                    <textarea id="incident-form-affected"></textarea>
                  </div>
                  <div class="form-field">
                    <label>${t('incidents.form.risks')}</label>
                    <textarea id="incident-form-risk" placeholder="${t('incidents.form.riskPlaceholder')}"></textarea>
                  </div>
                  <div class="form-field">
                    <label>${t('incidents.form.actions')}</label>
                    <textarea id="incident-form-actions"></textarea>
                  </div>
                </div>
              </div>
            </div>
          </form>
          ${IncidentsPage.buildCreateAttachmentsHtml ? IncidentsPage.buildCreateAttachmentsHtml() : ''}
          <div class="info-card incident-create-info">
            <h4>${t('incidents.create.afterCreateTitle')}</h4>
            <ul>
              <li>${t('incidents.create.afterCreatePoint1')}</li>
              <li>${t('incidents.create.afterCreatePoint2')}</li>
              <li>${t('incidents.create.afterCreatePoint3')}</li>
            </ul>
          </div>
        </div>
      </div>`;
  }

  function bindCreateForm(tabId) {
    const tab = state.tabs.find(t => t.id === tabId);
    const panel = document.querySelector(`#incidents-panels [data-tab="${tabId}"]`);
    if (!tab || !panel) return;
    panel.querySelectorAll('.time-input').forEach(input => {
      input.inputMode = 'numeric';
    });
    if (IncidentsPage.bindCreateAttachments) IncidentsPage.bindCreateAttachments(tabId);
    const saveBtn = panel.querySelector('#incident-form-save');
    const cancelBtn = panel.querySelector('#incident-form-cancel');
    const generateBtn = panel.querySelector('#incident-title-generate');
    const inputs = panel.querySelectorAll('input, textarea, select');
    const ownerSelect = panel.querySelector('#incident-form-owner');
    const ownerHint = panel.querySelector('#incident-form-owner-hint');
    const assigneeSelect = panel.querySelector('#incident-form-assignee');
    const participantsSelect = panel.querySelector('#incident-form-participants');
    const assigneeHint = panel.querySelector('#incident-form-assignee-hint');
    const participantsHint = panel.querySelector('#incident-form-participants-hint');
    const typeSelect = panel.querySelector('#incident-form-type');
    const sourceSelect = panel.querySelector('#incident-form-source');
    const slaInput = panel.querySelector('#incident-form-sla');
    const deadlineDateInput = panel.querySelector('#incident-form-deadline-date');
    const deadlineTimeInput = panel.querySelector('#incident-form-deadline-time');
    const assetsInput = panel.querySelector('#incident-form-assets');
    const tagsContainer = panel.querySelector('#incident-form-tags');
    const whatInput = panel.querySelector('#incident-form-what');
    const detectedInput = panel.querySelector('#incident-form-detected');
    const affectedInput = panel.querySelector('#incident-form-affected');
    const riskInput = panel.querySelector('#incident-form-risk');
    const actionsInput = panel.querySelector('#incident-form-actions');
    const titleInput = panel.querySelector('#incident-form-title');
    const descriptionInput = panel.querySelector('#incident-form-description');
    const markDirty = () => { tab.draft.dirty = hasFormData(panel); };
    if (IncidentsPage.populateSelectOptions) {
      IncidentsPage.populateSelectOptions(typeSelect, IncidentsPage.getIncidentTypes ? IncidentsPage.getIncidentTypes() : []);
      IncidentsPage.populateSelectOptions(sourceSelect, IncidentsPage.getDetectionSources ? IncidentsPage.getDetectionSources() : []);
    }
    if (typeSelect && !typeSelect.dataset.defaultValue) typeSelect.dataset.defaultValue = typeSelect.value;
    if (sourceSelect && !sourceSelect.dataset.defaultValue) sourceSelect.dataset.defaultValue = sourceSelect.value;
    if (IncidentsPage.populateUserSelect) {
      IncidentsPage.populateUserSelect(ownerSelect, state.currentUser?.username ? [state.currentUser.username] : []);
      IncidentsPage.populateUserSelect(assigneeSelect, state.currentUser?.username ? [state.currentUser.username] : []);
      IncidentsPage.populateUserSelect(participantsSelect, []);
      if (IncidentsPage.enforceSingleSelect) IncidentsPage.enforceSingleSelect(ownerSelect);
      if (IncidentsPage.enforceSingleSelect) IncidentsPage.enforceSingleSelect(assigneeSelect);
      if (IncidentsPage.renderSelectedHint) {
        IncidentsPage.renderSelectedHint(ownerSelect, ownerHint);
        IncidentsPage.renderSelectedHint(assigneeSelect, assigneeHint);
        IncidentsPage.renderSelectedHint(participantsSelect, participantsHint);
      }
    }
    inputs.forEach(input => {
      if (input.tagName === 'SELECT' && !input.dataset.defaultValue) {
        input.dataset.defaultValue = input.value;
      }
      if (input.type === 'checkbox') {
        input.dataset.defaultValue = input.checked ? '1' : '0';
      } else if ((input.tagName === 'INPUT' || input.tagName === 'TEXTAREA') && input.dataset.defaultValue === undefined) {
        input.dataset.defaultValue = input.value || '';
      }
      if (input.tagName === 'SELECT' && input.multiple && IncidentsPage.setDefaultSelectValues) {
        IncidentsPage.setDefaultSelectValues(input);
      }
      input.addEventListener('input', markDirty);
      input.addEventListener('change', markDirty);
    });
    [ownerSelect, assigneeSelect, participantsSelect].forEach(select => {
      if (!select) return;
      const hint = panel.querySelector(`#${select.id}-hint`);
      const updateHint = () => {
        if (IncidentsPage.renderSelectedHint) {
          IncidentsPage.renderSelectedHint(select, hint);
        }
      };
      select.addEventListener('change', updateHint);
      select.addEventListener('selectionrefresh', updateHint);
    });
    const renderTags = (selected = []) => {
      renderTagOptions(tagsContainer, selected);
    };
    renderTags();
    document.addEventListener('tags:changed', () => {
      const selected = Array.from(tagsContainer?.selectedOptions || []).map(i => i.value);
      renderTags(selected);
      markDirty();
    });
    if (generateBtn && titleInput) {
      generateBtn.addEventListener('click', (e) => {
        e.preventDefault();
        const type = typeSelect?.value || t('incidents.create.defaultType');
        const source = sourceSelect?.value || t('incidents.create.placeholderActivity');
        const rawDeadline = deadlineDateInput?.value || '';
        const dateLabel = rawDeadline
          ? rawDeadline.split('-').reverse().join('.')
          : (typeof AppTime !== 'undefined' && AppTime.formatDate ? AppTime.formatDate(new Date()) : (() => {
            const dt = new Date();
            const pad = (num) => `${num}`.padStart(2, '0');
            return `${pad(dt.getDate())}.${pad(dt.getMonth() + 1)}.${dt.getFullYear()}`;
          })());
        const generated = `[${type}] ${source || t('incidents.create.placeholderActivity')} - ${dateLabel}`;
        titleInput.value = generated;
        titleInput.dispatchEvent(new Event('input', { bubbles: true }));
      });
    }
    if (saveBtn) {
      saveBtn.addEventListener('click', async (e) => {
        e.preventDefault();
        const title = titleInput?.value.trim();
        if (!title) {
          showError(new Error(t('incidents.form.nameRequired')), 'incidents.form.nameRequired');
          return;
        }
        const severity = panel.querySelector('#incident-form-severity')?.value || 'medium';
        const description = descriptionInput?.value || '';
        const owner = IncidentsPage.getSelectedValues ? IncidentsPage.getSelectedValues(ownerSelect)[0] : '';
        const assignee = IncidentsPage.getSelectedValues ? IncidentsPage.getSelectedValues(assigneeSelect)[0] : '';
        const participants = IncidentsPage.getSelectedValues ? IncidentsPage.getSelectedValues(participantsSelect) : [];
        const tags = Array.from(tagsContainer?.selectedOptions || []).map(i => i.value);
          const meta = {
            incident_type: typeSelect?.value || '',
            detection_source: sourceSelect?.value || '',
            sla_response: slaInput?.value || '',
            first_response_deadline: IncidentsPage.toISODateTime
              ? IncidentsPage.toISODateTime(deadlineDateInput?.value || '', deadlineTimeInput?.value || '')
              : '',
            assets: assetsInput?.value || '',
            what_happened: whatInput?.value || '',
            detected_at: detectedInput?.value || '',
            affected_systems: affectedInput?.value || '',
            risk: riskInput?.value || '',
          actions_taken: actionsInput?.value || '',
          tags
        };
        try {
          const payload = { title, severity, description, participants, meta };
          if (owner) payload.owner = owner;
          if (assignee) payload.assignee = assignee;
          const res = await Api.post('/api/incidents', payload);
          const attachments = IncidentsPage.getCreateAttachments ? IncidentsPage.getCreateAttachments(tabId) : [];
          if (attachments.length && IncidentsPage.uploadCreateAttachment) {
            for (const file of attachments) {
              try {
                await IncidentsPage.uploadCreateAttachment(res.id, file);
              } catch (_) {
                // Errors are already surfaced; keep creating flow intact.
              }
            }
          }
          state.incidents.push(res);
          if (IncidentsPage.renderHome) IncidentsPage.renderHome();
          if (IncidentsPage.renderList) IncidentsPage.renderList();
          tab.draft.dirty = false;
          IncidentsPage.removeTab(tabId);
          IncidentsPage.openIncidentTab(res.id);
        } catch (err) {
          showError(err, 'incidents.regNoFailed');
        }
      });
    }
    if (cancelBtn) {
      cancelBtn.addEventListener('click', (e) => {
        e.preventDefault();
        IncidentsPage.requestCloseTab(tabId);
      });
    }
  }

  function hasFormData(panel) {
    const fields = panel.querySelectorAll('input, textarea, select');
    return Array.from(fields).some(field => {
      if (field.tagName === 'SELECT') {
        if (field.multiple) {
          const initial = field.dataset.defaultValues || '[]';
          const current = JSON.stringify(IncidentsPage.getSelectedValues ? IncidentsPage.getSelectedValues(field) : []);
          return current !== initial;
        }
        const initial = field.dataset.defaultValue || '';
        return field.value !== initial;
      }
      if (field.type === 'checkbox') {
        const initial = field.dataset.defaultValue || '0';
        return (field.checked ? '1' : '0') !== initial;
      }
      const initial = field.dataset.defaultValue;
      if (initial !== undefined) {
        return (field.value || '') !== initial;
      }
      return field.value && field.value.trim() !== '';
    });
  }

  IncidentsPage.openCreateTab = openCreateTab;
  IncidentsPage.buildCreateFormHtml = buildCreateFormHtml;
  IncidentsPage.bindCreateForm = bindCreateForm;
  IncidentsPage.hasFormData = hasFormData;
})();
