const RegistryReports = (() => {
  let cachedPerms = null;
  let cachedPermsAt = 0;

  function t(key) {
    return (typeof BerkutI18n !== 'undefined' && BerkutI18n.t) ? BerkutI18n.t(key) : key;
  }

  function escapeMd(value) {
    return (value || '')
      .toString()
      .replace(/\|/g, '\\|')
      .replace(/\r?\n/g, ' ')
      .trim();
  }

  function tagLabel(code) {
    const c = (code || '').toString().trim().toUpperCase();
    if (!c) return '';
    if (typeof DocUI !== 'undefined' && DocUI.tagLabel) return DocUI.tagLabel(c);
    return c;
  }

  function tagsText(tags) {
    const arr = Array.isArray(tags) ? tags : [];
    if (!arr.length) return '-';
    const rendered = arr.map(tagLabel).filter(Boolean);
    return rendered.length ? rendered.join(', ') : '-';
  }

  function mdTable(headers, rows) {
    const head = `| ${headers.map(escapeMd).join(' | ')} |`;
    const sep = `|${headers.map(() => '---').join('|')}|`;
    const body = rows.map(r => `| ${r.map(escapeMd).join(' | ')} |`).join('\n');
    return `${head}\n${sep}\n${body}`.trim();
  }

  async function loadPerms() {
    const now = Date.now();
    if (cachedPerms && (now - cachedPermsAt) < 15000) return cachedPerms;
    try {
      const me = await Api.get('/api/auth/me');
      const perms = Array.isArray(me?.user?.permissions) ? me.user.permissions : [];
      cachedPerms = perms;
      cachedPermsAt = now;
      return perms;
    } catch (_) {
      cachedPerms = [];
      cachedPermsAt = now;
      return [];
    }
  }

  function hasPerm(perms, perm) {
    if (!perm) return true;
    if (!Array.isArray(perms) || !perms.length) return true;
    return perms.includes(perm);
  }

  function requireReportPerms(perms) {
    return hasPerm(perms, 'reports.view') && hasPerm(perms, 'reports.create') && hasPerm(perms, 'reports.edit');
  }

  async function createReportDoc(title, content) {
    const res = await Api.post('/api/reports', { title: (title || '').trim() });
    const doc = res?.doc || res?.document || res;
    if (!doc?.id) throw new Error(t('common.error'));
    await Api.put(`/api/reports/${doc.id}/content`, {
      content: content || '',
      reason: t('reports.quick.reason') || 'Registry snapshot'
    });
    window.__pendingReportOpen = doc.id;
    if (window.AppNav?.navigateTo) {
      await window.AppNav.navigateTo('reports');
    } else {
      window.location.href = `/reports/${doc.id}`;
    }
  }

  function titleFor(moduleKey, moduleTitle) {
    const prefix = t('reports.quick.registryPrefix') || 'Registry';
    return `${prefix}: ${moduleTitle || moduleKey}`;
  }

  async function buildAssetsMarkdown(items, filters) {
    const title = titleFor('assets', t('assets.title'));
    const subtitle = t('assets.subtitle');
    const lines = [
      `# ${escapeMd(title)}`,
      subtitle ? `_${escapeMd(subtitle)}_` : '',
      filters ? `_${escapeMd(filters)}_` : '',
      ''
    ].filter(Boolean);
    if (!Array.isArray(items) || !items.length) {
      lines.push(escapeMd(t('assets.empty') || '-'));
      return lines.join('\n');
    }
    const headers = [
      t('assets.table.name'),
      t('assets.table.type'),
      t('assets.table.criticality'),
      t('assets.table.env'),
      t('assets.table.status'),
      t('assets.table.ip'),
      t('assets.table.owner'),
      t('assets.table.admin'),
      t('assets.table.tags')
    ];
    const rows = items.map(a => ([
      a?.name || '',
      t(`assets.type.${(a?.type || 'other').toString().toLowerCase()}`),
      t(`assets.criticality.${(a?.criticality || 'medium').toString().toLowerCase()}`),
      t(`assets.env.${(a?.env || 'other').toString().toLowerCase()}`),
      t(`assets.status.${(a?.status || 'active').toString().toLowerCase()}`),
      Array.isArray(a?.ip_addresses) && a.ip_addresses.length ? a.ip_addresses.join(', ') : '-',
      a?.owner || '-',
      a?.administrator || '-',
      tagsText(a?.tags)
    ]));
    lines.push(mdTable(headers, rows));
    return lines.join('\n');
  }

  async function buildSoftwareMarkdown(items, filters) {
    const title = titleFor('software', t('software.title'));
    const subtitle = t('software.subtitle');
    const lines = [
      `# ${escapeMd(title)}`,
      subtitle ? `_${escapeMd(subtitle)}_` : '',
      filters ? `_${escapeMd(filters)}_` : '',
      ''
    ].filter(Boolean);
    if (!Array.isArray(items) || !items.length) {
      lines.push(escapeMd(t('software.empty') || '-'));
      return lines.join('\n');
    }
    const headers = [
      t('software.table.name'),
      t('software.table.vendor'),
      t('software.table.tags'),
      t('software.table.updated')
    ];
    const rows = items.map(p => ([
      p?.name || '',
      p?.vendor || '-',
      tagsText(p?.tags),
      p?.updated_at ? (typeof AppTime !== 'undefined' && AppTime.formatDateTime ? AppTime.formatDateTime(p.updated_at) : p.updated_at) : '-'
    ]));
    lines.push(mdTable(headers, rows));
    return lines.join('\n');
  }

  async function buildFindingsMarkdown(items, filters) {
    const title = titleFor('findings', t('findings.title'));
    const subtitle = t('findings.subtitle');
    const lines = [
      `# ${escapeMd(title)}`,
      subtitle ? `_${escapeMd(subtitle)}_` : '',
      filters ? `_${escapeMd(filters)}_` : '',
      ''
    ].filter(Boolean);
    if (!Array.isArray(items) || !items.length) {
      lines.push(escapeMd(t('findings.empty') || '-'));
      return lines.join('\n');
    }
    const headers = [
      t('findings.table.title'),
      t('findings.table.status'),
      t('findings.table.severity'),
      t('findings.table.type'),
      t('findings.table.owner'),
      t('findings.table.due'),
      t('findings.table.tags'),
      t('findings.table.updated')
    ];
    const rows = items.map(f => ([
      f?.title || '',
      t(`findings.status.${(f?.status || 'open').toString().toLowerCase()}`),
      t(`findings.severity.${(f?.severity || 'medium').toString().toLowerCase()}`),
      t(`findings.type.${(f?.finding_type || f?.type || 'other').toString().toLowerCase()}`),
      f?.owner || '-',
      f?.due_at ? (typeof AppTime !== 'undefined' && AppTime.formatDate ? AppTime.formatDate(f.due_at) : f.due_at) : '-',
      tagsText(f?.tags),
      f?.updated_at ? (typeof AppTime !== 'undefined' && AppTime.formatDateTime ? AppTime.formatDateTime(f.updated_at) : f.updated_at) : '-'
    ]));
    lines.push(mdTable(headers, rows));
    return lines.join('\n');
  }

  function filtersSummary(pairs) {
    const items = (pairs || []).filter(p => p && p[0] && p[1]);
    if (!items.length) return '';
    return items.map(([k, v]) => `${k}: ${v}`).join(' · ');
  }

  async function createAssetsReport() {
    const perms = await loadPerms();
    if (!requireReportPerms(perms)) return;
    const canManage = hasPerm(perms, 'assets.manage');
    const val = (id) => (document.getElementById(id)?.value || '').toString().trim();
    const includeDeleted = canManage && (val('assets-filter-include-deleted') === '1' || val('assets-filter-include-deleted') === 'true');
    const url = new URL('/api/assets', window.location.origin);
    if (val('assets-filter-q')) url.searchParams.set('q', val('assets-filter-q'));
    if (val('assets-filter-type')) url.searchParams.set('type', val('assets-filter-type'));
    if (val('assets-filter-criticality')) url.searchParams.set('criticality', val('assets-filter-criticality'));
    if (val('assets-filter-env')) url.searchParams.set('env', val('assets-filter-env'));
    if (val('assets-filter-status')) url.searchParams.set('status', val('assets-filter-status'));
    if (val('assets-filter-tag')) url.searchParams.set('tag', val('assets-filter-tag'));
    if (includeDeleted) url.searchParams.set('include_deleted', '1');
    url.searchParams.set('limit', '200');
    const data = await Api.get(url.pathname + url.search);
    const items = Array.isArray(data?.items) ? data.items : [];
    const filters = filtersSummary([
      [t('assets.filter.search'), val('assets-filter-q')],
      [t('assets.filter.type'), val('assets-filter-type')],
      [t('assets.filter.criticality'), val('assets-filter-criticality')],
      [t('assets.filter.env'), val('assets-filter-env')],
      [t('assets.filter.status'), val('assets-filter-status')],
      [t('assets.filter.tag'), val('assets-filter-tag')],
      [t('assets.filter.archived'), includeDeleted ? t('assets.filter.includeArchived') : '']
    ]);
    const content = await buildAssetsMarkdown(items, filters);
    await createReportDoc(titleFor('assets', t('assets.title')), content);
  }

  async function createSoftwareReport() {
    const perms = await loadPerms();
    if (!requireReportPerms(perms)) return;
    const canManage = hasPerm(perms, 'software.manage');
    const val = (id) => (document.getElementById(id)?.value || '').toString().trim();
    const includeDeleted = canManage && (val('software-filter-include-deleted') === '1' || val('software-filter-include-deleted') === 'true');
    const url = new URL('/api/software', window.location.origin);
    if (val('software-filter-q')) url.searchParams.set('q', val('software-filter-q'));
    if (val('software-filter-vendor')) url.searchParams.set('vendor', val('software-filter-vendor'));
    if (val('software-filter-tag')) url.searchParams.set('tag', val('software-filter-tag'));
    if (includeDeleted) url.searchParams.set('include_deleted', '1');
    url.searchParams.set('limit', '200');
    const data = await Api.get(url.pathname + url.search);
    const items = Array.isArray(data?.items) ? data.items : [];
    const filters = filtersSummary([
      [t('software.filter.search'), val('software-filter-q')],
      [t('software.filter.vendor'), val('software-filter-vendor')],
      [t('software.filter.tag'), val('software-filter-tag')],
      [t('software.filter.archived'), includeDeleted ? t('software.filter.includeArchived') : '']
    ]);
    const content = await buildSoftwareMarkdown(items, filters);
    await createReportDoc(titleFor('software', t('software.title')), content);
  }

  async function createFindingsReport() {
    const perms = await loadPerms();
    if (!requireReportPerms(perms)) return;
    const canManage = hasPerm(perms, 'findings.manage');
    const val = (id) => (document.getElementById(id)?.value || '').toString().trim();
    const includeDeleted = canManage && (val('findings-filter-include-deleted') === '1' || val('findings-filter-include-deleted') === 'true');
    const url = new URL('/api/findings', window.location.origin);
    if (val('findings-filter-q')) url.searchParams.set('q', val('findings-filter-q'));
    if (val('findings-filter-status')) url.searchParams.set('status', val('findings-filter-status'));
    if (val('findings-filter-severity')) url.searchParams.set('severity', val('findings-filter-severity'));
    if (val('findings-filter-type')) url.searchParams.set('type', val('findings-filter-type'));
    if (val('findings-filter-tag')) url.searchParams.set('tag', val('findings-filter-tag'));
    if (includeDeleted) url.searchParams.set('include_deleted', '1');
    url.searchParams.set('limit', '200');
    const data = await Api.get(url.pathname + url.search);
    const items = Array.isArray(data?.items) ? data.items : [];
    const filters = filtersSummary([
      [t('findings.filter.search'), val('findings-filter-q')],
      [t('findings.filter.status'), val('findings-filter-status')],
      [t('findings.filter.severity'), val('findings-filter-severity')],
      [t('findings.filter.type'), val('findings-filter-type')],
      [t('findings.filter.tag'), val('findings-filter-tag')],
      [t('findings.filter.archived'), includeDeleted ? t('findings.filter.includeArchived') : '']
    ]);
    const content = await buildFindingsMarkdown(items, filters);
    await createReportDoc(titleFor('findings', t('findings.title')), content);
  }

  async function bind(buttonId, moduleKey) {
    const btn = document.getElementById(buttonId);
    if (!btn) return;
    const perms = await loadPerms();
    const allowed = requireReportPerms(perms);
    btn.hidden = !allowed;
    btn.disabled = !allowed;
    if (!allowed) return;
    btn.onclick = async () => {
      try {
        btn.disabled = true;
        if (moduleKey === 'assets') await createAssetsReport();
        if (moduleKey === 'software') await createSoftwareReport();
        if (moduleKey === 'findings') await createFindingsReport();
      } catch (err) {
        alert((err && err.message) ? err.message : t('common.error'));
      } finally {
        btn.disabled = false;
      }
    };
  }

  return { bind };
})();

if (typeof window !== 'undefined') {
  window.RegistryReports = RegistryReports;
}

