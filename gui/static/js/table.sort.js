(() => {
  const TABLE_SELECTOR = 'table.data-table, table.table';
  const tableSortState = new Map();
  const collator = new Intl.Collator(['ru', 'en'], { sensitivity: 'base', numeric: true });
  let muteObserver = false;
  let rerenderScheduled = false;

  function getTableKey(table) {
    if (!table) return '';
    if (table.id) return `#${table.id}`;
    if (table.dataset.sortId) return table.dataset.sortId;
    const page = table.closest('.page')?.id || 'page';
    const index = Array.from(document.querySelectorAll(TABLE_SELECTOR)).indexOf(table);
    return `${page}:${index}`;
  }

  function isSortableHeader(th) {
    if (!th) return false;
    if (th.dataset.sort === 'none') return false;
    if (th.classList.contains('no-sort')) return false;
    if ((th.dataset.i18n || '').trim() === 'common.actions') return false;
    if (th.querySelector('input[type="checkbox"]')) return false;
    const plainText = String(th.textContent || '').trim();
    if (!plainText && !(th.dataset.i18n || '').trim()) return false;
    return true;
  }

  function decorateHeaders(table) {
    const headers = table?.querySelectorAll('thead th') || [];
    headers.forEach((th) => {
      if (isSortableHeader(th)) {
        th.classList.add('is-sortable');
        if (!th.hasAttribute('aria-sort')) th.setAttribute('aria-sort', 'none');
      } else {
        th.classList.remove('is-sortable');
        th.removeAttribute('aria-sort');
      }
    });
  }

  function parseDateValue(raw) {
    const value = String(raw || '').trim();
    if (!value) return null;
    const direct = Date.parse(value);
    if (Number.isFinite(direct)) return direct;
    const local = value.match(/^(\d{2})\.(\d{2})\.(\d{4})(?:[ T](\d{2}):(\d{2})(?::(\d{2}))?)?$/);
    if (!local) return null;
    const day = parseInt(local[1], 10);
    const month = parseInt(local[2], 10) - 1;
    const year = parseInt(local[3], 10);
    const hh = parseInt(local[4] || '0', 10);
    const mm = parseInt(local[5] || '0', 10);
    const ss = parseInt(local[6] || '0', 10);
    const dt = new Date(year, month, day, hh, mm, ss);
    const ts = dt.getTime();
    return Number.isFinite(ts) ? ts : null;
  }

  function parseNumberValue(raw) {
    const src = String(raw || '')
      .replace(/\u00a0/g, ' ')
      .replace(/\s+/g, '')
      .replace(',', '.');
    if (!/^[-+]?\d+(?:\.\d+)?$/.test(src)) return null;
    const num = Number(src);
    return Number.isFinite(num) ? num : null;
  }

  function cellComparableValue(cell) {
    const raw = String(cell?.dataset?.sortValue || cell?.textContent || '').trim();
    const numeric = parseNumberValue(raw);
    if (numeric !== null) return { type: 'number', value: numeric, text: raw };
    const date = parseDateValue(raw);
    if (date !== null) return { type: 'date', value: date, text: raw };
    return { type: 'string', value: raw.toLowerCase(), text: raw };
  }

  function compareCells(leftCell, rightCell) {
    const left = cellComparableValue(leftCell);
    const right = cellComparableValue(rightCell);
    if ((left.type === 'number' || left.type === 'date') && left.type === right.type) {
      return left.value - right.value;
    }
    return collator.compare(left.text, right.text);
  }

  function sortTable(table, columnIndex, direction) {
    if (!table || columnIndex < 0) return;
    const tbody = table.tBodies && table.tBodies[0];
    if (!tbody) return;
    const rows = Array.from(tbody.rows).filter((row) => {
      if (row.classList.contains('placeholder')) return false;
      if (row.cells.length === 1 && Number(row.cells[0].colSpan || 1) > 1) return false;
      return true;
    });
    if (rows.length < 2) return;
    const sorted = rows
      .map((row, idx) => ({ row, idx }))
      .sort((a, b) => {
        const cmp = compareCells(a.row.cells[columnIndex], b.row.cells[columnIndex]);
        if (cmp !== 0) return direction === 'desc' ? -cmp : cmp;
        return a.idx - b.idx;
      });
    muteObserver = true;
    try {
      sorted.forEach((entry) => tbody.appendChild(entry.row));
    } finally {
      muteObserver = false;
    }
  }

  function applyHeaderSortState(table, activeColumn = -1, direction = 'asc') {
    const headers = table?.querySelectorAll('thead th') || [];
    headers.forEach((th, idx) => {
      if (!isSortableHeader(th)) return;
      if (idx === activeColumn) th.setAttribute('aria-sort', direction === 'desc' ? 'descending' : 'ascending');
      else th.setAttribute('aria-sort', 'none');
    });
  }

  function apply(tableOrSelector) {
    const table = typeof tableOrSelector === 'string' ? document.querySelector(tableOrSelector) : tableOrSelector;
    if (!table) return;
    decorateHeaders(table);
    const key = getTableKey(table);
    if (!key) return;
    const state = tableSortState.get(key);
    if (!state) return;
    sortTable(table, state.columnIndex, state.direction);
    applyHeaderSortState(table, state.columnIndex, state.direction);
  }

  function handleHeaderClick(event) {
    const th = event.target.closest('th');
    if (!th) return;
    const table = th.closest(TABLE_SELECTOR);
    if (!table) return;
    if (!isSortableHeader(th)) return;
    const headerRow = th.parentElement;
    if (!headerRow) return;
    const headers = Array.from(headerRow.children);
    const columnIndex = headers.indexOf(th);
    if (columnIndex < 0) return;
    const key = getTableKey(table);
    const prev = tableSortState.get(key);
    const direction = prev && prev.columnIndex === columnIndex && prev.direction === 'asc' ? 'desc' : 'asc';
    tableSortState.set(key, { columnIndex, direction });
    sortTable(table, columnIndex, direction);
    applyHeaderSortState(table, columnIndex, direction);
  }

  function decorateAndReapplyAll() {
    document.querySelectorAll(TABLE_SELECTOR).forEach((table) => {
      decorateHeaders(table);
      const key = getTableKey(table);
      const state = key ? tableSortState.get(key) : null;
      if (!state) return;
      sortTable(table, state.columnIndex, state.direction);
      applyHeaderSortState(table, state.columnIndex, state.direction);
    });
  }

  function scheduleDecorateAndReapply() {
    if (rerenderScheduled) return;
    rerenderScheduled = true;
    requestAnimationFrame(() => {
      rerenderScheduled = false;
      decorateAndReapplyAll();
    });
  }

  document.addEventListener('click', handleHeaderClick);
  const observer = new MutationObserver(() => {
    if (muteObserver) return;
    scheduleDecorateAndReapply();
  });
  observer.observe(document.documentElement, { childList: true, subtree: true });
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', decorateAndReapplyAll, { once: true });
  } else {
    decorateAndReapplyAll();
  }

  window.TableSort = {
    apply,
    clear(tableOrSelector) {
      const table = typeof tableOrSelector === 'string' ? document.querySelector(tableOrSelector) : tableOrSelector;
      if (!table) return;
      const key = getTableKey(table);
      if (!key) return;
      tableSortState.delete(key);
      applyHeaderSortState(table);
    },
  };
})();
