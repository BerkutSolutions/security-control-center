(() => {
  function renderApprovalStages(stages = [{}]) {
    const container = document.getElementById('approval-stages');
    if (!container) return;
    if (!stages.length) stages = [{}];
    container.innerHTML = '';
    stages.forEach((stage, idx) => {
      const placeholderName = `${BerkutI18n.t('docs.stageNamePlaceholder') || 'Stage'} ${idx + 1}`;
      const block = document.createElement('div');
      block.className = 'approval-stage';
      block.dataset.index = `${idx}`;
      block.innerHTML = `
        <div class="stage-top">
          <div class="stage-title">
            <span class="stage-pill">${BerkutI18n.t('docs.approvalStage') || 'Stage'} ${idx + 1}</span>
            <input type="text" class="stage-name" value="${DocsPage.escapeHtml(stage.name || '')}" placeholder="${DocsPage.escapeHtml(placeholderName)}">
          </div>
          ${idx > 0 ? '<button class="btn ghost danger" type="button" data-remove-stage>&times;</button>' : ''}
        </div>
        <div class="stage-body stage-grid">
          <div class="stage-col">
            <div class="form-field required">
              <label>${BerkutI18n.t('docs.field.approvers')}</label>
              <select multiple class="stage-approvers"></select>
              <div class="selected-hint selected-approvers"></div>
            </div>
          </div>
          <div class="stage-col">
            <div class="form-field">
              <label>${BerkutI18n.t('docs.field.observers')}</label>
              <select multiple class="stage-observers"></select>
              <div class="selected-hint selected-observers"></div>
            </div>
          </div>
          <div class="stage-col">
            <div class="form-field">
              <label>${BerkutI18n.t('docs.field.message')}</label>
              <textarea class="stage-message" rows="3" placeholder="${DocsPage.escapeHtml(BerkutI18n.t('docs.stageMessagePlaceholder') || '')}">${DocsPage.escapeHtml(stage.message || '')}</textarea>
            </div>
          </div>
        </div>
      `;
      const approversSel = block.querySelector('.stage-approvers');
      if (approversSel) {
        if (!approversSel.id) approversSel.id = `approval-approvers-${idx}`;
        approversSel.innerHTML = '';
        UserDirectory.all().forEach(u => {
          const opt = document.createElement('option');
          opt.value = u.id;
          opt.textContent = u.full_name || u.username;
          opt.dataset.label = opt.textContent;
          approversSel.appendChild(opt);
        });
        (stage.approvers || []).forEach(id => {
          const opt = Array.from(approversSel.options).find(o => parseInt(o.value, 10) === id);
          if (opt) opt.selected = true;
        });
        DocsPage.enhanceMultiSelects([approversSel.id]);
        const rerender = () => renderStageSelections(block);
        approversSel.addEventListener('change', rerender);
        approversSel.addEventListener('selectionrefresh', rerender);
      }
      const observersSel = block.querySelector('.stage-observers');
      if (observersSel) {
        if (!observersSel.id) observersSel.id = `approval-observers-${idx}`;
        observersSel.innerHTML = '';
        UserDirectory.all().forEach(u => {
          const opt = document.createElement('option');
          opt.value = u.id;
          opt.textContent = u.full_name || u.username;
          opt.dataset.label = opt.textContent;
          observersSel.appendChild(opt);
        });
        (stage.observers || []).forEach(id => {
          const opt = Array.from(observersSel.options).find(o => parseInt(o.value, 10) === id);
          if (opt) opt.selected = true;
        });
        DocsPage.enhanceMultiSelects([observersSel.id]);
        const rerenderObs = () => renderStageSelections(block);
        observersSel.addEventListener('change', rerenderObs);
        observersSel.addEventListener('selectionrefresh', rerenderObs);
      }
      const removeBtn = block.querySelector('[data-remove-stage]');
      if (removeBtn) {
        removeBtn.onclick = () => {
          block.remove();
          renumberStages();
        };
      }
      container.appendChild(block);
      renderStageSelections(block);
    });
    renumberStages();
  }

  function renumberStages() {
    const stages = Array.from(document.querySelectorAll('#approval-stages .approval-stage'));
    stages.forEach((block, idx) => {
      block.dataset.index = `${idx}`;
      const header = block.querySelector('.stage-pill');
      const placeholderName = `${BerkutI18n.t('docs.approvalStage') || 'Stage'} ${idx + 1}`;
      if (header) header.textContent = placeholderName;
      const nameInput = block.querySelector('.stage-name');
      if (nameInput && !nameInput.value) {
        nameInput.placeholder = `${BerkutI18n.t('docs.stageNamePlaceholder') || 'Stage'} ${idx + 1}`;
      }
      const removeBtn = block.querySelector('[data-remove-stage]');
      if (removeBtn) removeBtn.hidden = idx === 0;
      renderStageSelections(block);
    });
  }

  function collectStages(includeEmpty = false) {
    const stages = [];
    document.querySelectorAll('#approval-stages .approval-stage').forEach(block => {
      const name = block.querySelector('.stage-name')?.value || '';
      const approvers = Array.from(block.querySelectorAll('.stage-approvers option'))
        .filter(o => o.selected)
        .map(o => parseInt(o.value, 10))
        .filter(Boolean);
      const observers = Array.from(block.querySelectorAll('.stage-observers option'))
        .filter(o => o.selected)
        .map(o => parseInt(o.value, 10))
        .filter(Boolean);
      const message = (block.querySelector('.stage-message')?.value || '').trim();
      if (approvers.length || includeEmpty) {
        stages.push({ name, approvers, observers, message });
      }
    });
    return stages;
  }

  function renderStageSelections(block) {
    if (!block) return;
    const fill = (sel, target) => {
      if (!sel || !target) return;
      const names = Array.from(sel.options)
        .filter(o => o.selected)
        .map(o => o.dataset.label || o.textContent);
      target.innerHTML = '';
      if (!names.length) {
        target.textContent = BerkutI18n.t('docs.stageEmptySelection') || '';
        return;
      }
      names.forEach(n => {
        const badge = document.createElement('span');
        badge.className = 'tag';
        badge.textContent = n;
        target.appendChild(badge);
      });
    };
    fill(block.querySelector('.stage-approvers'), block.querySelector('.selected-approvers'));
    fill(block.querySelector('.stage-observers'), block.querySelector('.selected-observers'));
  }

  function bindApprovalForm() {
    const form = document.getElementById('approval-form');
    const alertBox = document.getElementById('approval-alert');
    const addStageBtn = document.getElementById('approval-add-stage');
    if (!form) return;
    if (addStageBtn) {
      addStageBtn.onclick = (e) => {
        e.preventDefault();
        const existing = collectStages(true);
        existing.push({ name: '', approvers: [], observers: [], message: '' });
        renderApprovalStages(existing);
      };
    }
    form.onsubmit = async (e) => {
      e.preventDefault();
      DocsPage.hideAlert(alertBox);
      const docId = form.dataset.docId;
      const stages = collectStages(false);
      if (!stages.length) {
        DocsPage.showAlert(alertBox, BerkutI18n.t('docs.approvalNeedApprover'));
        return;
      }
      const payload = { stages };
      try {
        await Api.post(`/api/docs/${docId}/approval/start`, payload);
        DocsPage.closeModal('#start-approval-modal');
      } catch (err) {
        const msg = err.message || BerkutI18n.t('docs.approvalForbidden');
        DocsPage.showAlert(alertBox, msg);
      }
    };
  }

  async function openApprovalModal(docId) {
    const form = document.getElementById('approval-form');
    if (!form) return;
    form.dataset.docId = docId;
    await UserDirectory.load();
    renderApprovalStages([{ approvers: [], observers: [], message: '' }]);
    DocsPage.openModal('#start-approval-modal');
  }

  DocsPage.renderApprovalStages = renderApprovalStages;
  DocsPage.renumberStages = renumberStages;
  DocsPage.collectStages = collectStages;
  DocsPage.renderStageSelections = renderStageSelections;
  DocsPage.bindApprovalForm = bindApprovalForm;
  DocsPage.openApprovalModal = openApprovalModal;
})();

