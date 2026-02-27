(() => {
  const state = IncidentsPage.state;
  const { t, showError, escapeHtml } = IncidentsPage;

  async function ensureLinkOptions(incidentId) {
    const detail = state.incidentDetails.get(incidentId);
    if (!detail) return { docs: [], incidents: [], assets: [], software: [] };
    if (detail.linkOptions && detail.linkOptionsLoaded) return detail.linkOptions;
    detail.linkOptions = { docs: [], incidents: [], assets: [], software: [] };
    try {
      const [docsRes, incRes, assetsRes, softwareRes] = await Promise.all([
        Api.get('/api/docs/list?limit=200').catch(() => ({ items: [] })),
        Api.get('/api/incidents/list?limit=200').catch(() => ({ items: [] })),
        Api.get('/api/assets/list?limit=200').catch(() => ({ items: [] })),
        Api.get('/api/software/list?limit=200').catch(() => ({ items: [] }))
      ]);
      detail.linkOptions = {
        docs: docsRes.items || [],
        incidents: incRes.items || [],
        assets: assetsRes.items || [],
        software: softwareRes.items || []
      };
    } catch (err) {
      showError(err, 'incidents.links.loadFailed');
    } finally {
      detail.linkOptionsLoaded = true;
    }
    return detail.linkOptions;
  }

  function renderTargets(incidentId) {
    const tabId = `incident-${incidentId}`;
    const panel = document.querySelector(`#incidents-panels [data-tab="${tabId}"]`);
    const typeSelect = panel?.querySelector('.incident-link-type');
    const targetSelect = panel?.querySelector('.incident-link-target');
    const commentInput = panel?.querySelector('.incident-link-comment');
    if (!typeSelect || !targetSelect) return;
    const type = typeSelect.value;
    targetSelect.innerHTML = '';
    if (type === 'other') {
      targetSelect.disabled = true;
      targetSelect.value = '';
      if (commentInput) commentInput.required = true;
      return;
    }
    targetSelect.disabled = false;
    if (commentInput) commentInput.required = false;
    const placeholder = document.createElement('option');
    placeholder.value = '';
    placeholder.textContent = t('incidents.links.targetPlaceholder');
    targetSelect.appendChild(placeholder);
    ensureLinkOptions(incidentId).then((opts) => {
      let items = [];
      if (type === 'doc') items = opts.docs || [];
      if (type === 'incident') items = opts.incidents || [];
      if (type === 'report') items = (opts.docs || []).filter(d => (d.type || '').toLowerCase() === 'report');
      if (type === 'asset') items = opts.assets || [];
      if (type === 'software') items = opts.software || [];
      items.forEach(item => {
        const opt = document.createElement('option');
        opt.value = item.id;
        opt.textContent = linkOptionLabel(type, item);
        targetSelect.appendChild(opt);
      });
    });
  }

  function bindLinkControls(incidentId) {
    const tabId = `incident-${incidentId}`;
    const panel = document.querySelector(`#incidents-panels [data-tab="${tabId}"]`);
    if (!panel) return;
    const addBtn = panel.querySelector('.incident-link-add');
    const typeSelect = panel.querySelector('.incident-link-type');
    const targetSelect = panel.querySelector('.incident-link-target');
    const commentInput = panel.querySelector('.incident-link-comment');
    if (typeSelect) {
      typeSelect.onchange = () => renderTargets(incidentId);
      renderTargets(incidentId);
    }
    if (addBtn) {
      addBtn.onclick = async () => {
        const type = typeSelect?.value;
        const target = targetSelect?.value || '';
        const comment = commentInput?.value.trim() || '';
        if (!type) return;
        if (type !== 'other' && !target) return;
        if (type === 'other' && !comment) {
          showError(new Error(t('incidents.links.commentRequired')), 'incidents.links.commentRequired');
          return;
        }
        const payload = { target_type: type, target_id: target, comment };
        try {
          await Api.post(`/api/incidents/${incidentId}/links`, payload);
          await ensureIncidentLinks(incidentId, true);
          if (targetSelect) targetSelect.value = '';
          if (commentInput) commentInput.value = '';
        } catch (err) {
          showError(err, 'incidents.links.addFailed');
        }
      };
    }
  }

  async function ensureIncidentLinks(incidentId, force) {
    const detail = state.incidentDetails.get(incidentId);
    if (!detail) return;
    if (detail.linksLoaded && !force) {
      renderIncidentLinks(incidentId);
      renderAssetsMetaLinks(incidentId);
      return;
    }
    detail.linksLoading = true;
    try {
      const res = await Api.get(`/api/incidents/${incidentId}/links`);
      detail.links = res.items || [];
      detail.linksLoaded = true;
    } catch (err) {
      detail.links = [];
      showError(err, 'incidents.links.loadFailed');
    } finally {
      detail.linksLoading = false;
      renderIncidentLinks(incidentId);
      renderAssetsMetaLinks(incidentId);
    }
  }

  function renderAssetsMetaLinks(incidentId) {
    const tabId = `incident-${incidentId}`;
    const panel = document.querySelector(`#incidents-panels [data-tab="${tabId}"]`);
    const detail = state.incidentDetails.get(incidentId);
    if (!panel || !detail) return;
    const box = panel.querySelector(`#incident-assets-links-${incidentId}`);
    if (!box) return;
    box.innerHTML = '';
    const assets = (detail.links || []).filter(l => (l.entity_type || '').toLowerCase() === 'asset');
    if (!assets.length) {
      box.hidden = true;
      return;
    }
    assets.forEach((l) => {
      const a = document.createElement('a');
      a.className = 'tag';
      a.href = `/assets?asset=${encodeURIComponent(l.entity_id || '')}`;
      a.textContent = l.title ? `${l.title}` : `#${l.entity_id || ''}`;
      box.appendChild(a);
    });
    box.hidden = false;
  }

  function renderIncidentLinks(incidentId) {
    const tabId = `incident-${incidentId}`;
    const panel = document.querySelector(`#incidents-panels [data-tab="${tabId}"]`);
    const detail = state.incidentDetails.get(incidentId);
    if (!panel || !detail) return;
    const tbody = panel.querySelector('.incident-links-body');
    if (!tbody) return;
    tbody.innerHTML = '';
    if (!detail.links || !detail.links.length) {
      const tr = document.createElement('tr');
      tr.innerHTML = `<td colspan="5">${escapeHtml(t('incidents.links.empty'))}</td>`;
      tbody.appendChild(tr);
      return;
    }
    detail.links.forEach(link => {
      const tr = document.createElement('tr');
      const status = link.unverified ? t('incidents.links.unverified') : t('incidents.links.verified');
      const comment = link.comment || '';
      const title = formatLinkTitle(link);
      tr.innerHTML = `
        <td>${escapeHtml(formatLinkType(link.entity_type))}</td>
        <td>${title}</td>
        <td>${comment ? escapeHtml(comment) : '<span class="meta-empty">-</span>'}</td>
        <td>${escapeHtml(status)}</td>
        <td class="actions">
          ${link.entity_type === 'doc' || link.entity_type === 'report' ? `<button class="btn ghost link-open" data-id="${link.entity_id}">${t('incidents.links.openDoc')}</button>` : ''}
          ${link.entity_type === 'asset' ? `<button class="btn ghost link-open-asset" data-id="${link.entity_id}">${t('incidents.links.openAsset')}</button>` : ''}
          ${link.entity_type === 'software' ? `<button class="btn ghost link-open-software" data-id="${link.entity_id}">${t('incidents.links.openSoftware')}</button>` : ''}
          <button class="btn ghost link-remove" data-id="${link.id}">${t('incidents.links.remove')}</button>
        </td>`;
      tbody.appendChild(tr);
      const openBtn = tr.querySelector('.link-open');
      if (openBtn) {
        openBtn.onclick = () => IncidentsPage.openDocInDocs(link.entity_id);
      }
      const openAssetBtn = tr.querySelector('.link-open-asset');
      if (openAssetBtn) {
        openAssetBtn.onclick = () => IncidentsPage.openAssetInAssets(link.entity_id);
      }
      const openSoftwareBtn = tr.querySelector('.link-open-software');
      if (openSoftwareBtn) {
        openSoftwareBtn.onclick = () => {
          const id = link.entity_id || '';
          if (!id) return;
          window.history.pushState({}, '', `/software?software=${encodeURIComponent(id)}`);
          window.dispatchEvent(new PopStateEvent('popstate'));
        };
      }
      const removeBtn = tr.querySelector('.link-remove');
      if (removeBtn) {
        removeBtn.onclick = async () => {
          if (!confirm(t('incidents.links.removeConfirm'))) return;
          try {
            await Api.del(`/api/incidents/${incidentId}/links/${link.id}`);
            await ensureIncidentLinks(incidentId, true);
          } catch (err) {
            showError(err, 'incidents.links.removeFailed');
          }
        };
      }
    });
  }

  function formatLinkType(type) {
    return t(`incidents.links.type.${type}`) || type;
  }

  function linkOptionLabel(type, item) {
    if (type === 'incident') {
      return `#${item.id} ${item.reg_no || ''} ${item.title || ''}`.trim();
    }
    if (type === 'asset') {
      const name = item.name || '';
      const tt = item.type ? `(${item.type})` : '';
      return `${name} ${tt}`.trim();
    }
    if (type === 'software') {
      const vendor = item.vendor ? `(${item.vendor})` : '';
      return `${item.name || ''} ${vendor}`.trim();
    }
    const reg = item.reg_no ? `(${item.reg_no})` : '';
    return `${reg} ${item.title || ''}`.trim();
  }

  function formatLinkTitle(link) {
    if (!link) return '';
    if (link.title) return escapeHtml(link.title);
    if (link.entity_type === 'doc' || link.entity_type === 'report') {
      return `<span class="link-doc-ref">#${escapeHtml(link.entity_id || '')}</span>`;
    }
    return escapeHtml(link.entity_id || '');
  }

  IncidentsPage.bindLinkControls = bindLinkControls;
  IncidentsPage.ensureIncidentLinks = ensureIncidentLinks;
  IncidentsPage.renderIncidentLinks = renderIncidentLinks;
  IncidentsPage.ensureLinkOptions = ensureLinkOptions;
  IncidentsPage.linkOptionLabel = linkOptionLabel;
})();
