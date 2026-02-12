(() => {
  const globalObj = typeof window !== 'undefined' ? window : globalThis;
  const DashboardPage = globalObj.DashboardPage || (globalObj.DashboardPage = {});
  const state = DashboardPage.state;
  const GRID_SIZE = 20;
  const SNAP_DISTANCE = 14;
  const GAP = 16;
  const MIN_SIZE = 240;

  function bindDragResize(card, handle) {
    if (!card) return;
    card.addEventListener('mousedown', (e) => {
      if (!state.editMode) return;
      if (e.button !== 0) return;
      if (shouldIgnoreDrag(e.target)) return;
      e.preventDefault();
      startDrag(card, e);
    });
    if (handle) {
      handle.addEventListener('mousedown', (e) => {
        if (!state.editMode) return;
        if (e.button !== 0) return;
        e.preventDefault();
        startResize(card, e);
      });
    }
  }

  function shouldIgnoreDrag(target) {
    if (!target) return true;
    return !!target.closest('button, a, input, select, textarea, .frame-actions, .frame-resize-handle');
  }

  function startDrag(card, e) {
    const id = card.dataset.frameId;
    const rect = card.getBoundingClientRect();
    const board = state.grid;
    if (!board) return;
    const boardRect = board.getBoundingClientRect();
    const startX = e.clientX;
    const startY = e.clientY;
    const offsetX = rect.left - boardRect.left;
    const offsetY = rect.top - boardRect.top;
    const width = rect.width;
    const height = rect.height;
    state.draggingId = id;
    card.classList.add('dragging');
    const onMove = (evt) => {
      const dx = evt.clientX - startX;
      const dy = evt.clientY - startY;
      const next = clampFrame(snapFrame({
        x: offsetX + dx,
        y: offsetY + dy,
        w: width,
        h: height
      }, id));
      applyFrameBox(card, next);
    };
    const onUp = () => {
      card.classList.remove('dragging');
      state.draggingId = null;
      document.removeEventListener('mousemove', onMove);
      document.removeEventListener('mouseup', onUp);
      persistFrameBox(id, card);
    };
    document.addEventListener('mousemove', onMove);
    document.addEventListener('mouseup', onUp);
  }

  function startResize(card, e) {
    const id = card.dataset.frameId;
    const rect = card.getBoundingClientRect();
    const board = state.grid;
    if (!board) return;
    const boardRect = board.getBoundingClientRect();
    const startX = e.clientX;
    const startY = e.clientY;
    const offsetX = rect.left - boardRect.left;
    const offsetY = rect.top - boardRect.top;
    const startW = rect.width;
    const startH = rect.height;
    const onMove = (evt) => {
      const dx = evt.clientX - startX;
      const dy = evt.clientY - startY;
      const next = clampFrame(snapFrame({
        x: offsetX,
        y: offsetY,
        w: Math.max(MIN_SIZE, startW + dx),
        h: Math.max(MIN_SIZE, startH + dy)
      }, id));
      applyFrameBox(card, next);
    };
    const onUp = () => {
      document.removeEventListener('mousemove', onMove);
      document.removeEventListener('mouseup', onUp);
      persistFrameBox(id, card);
    };
    document.addEventListener('mousemove', onMove);
    document.addEventListener('mouseup', onUp);
  }

  function positionFrame(card, id, idx) {
    const box = resolveCollisions(ensureFrameBox(id, idx), id);
    applyFrameBox(card, box);
  }

  function ensureFrameBox(id, idx) {
    const settings = getFrameSettings(id);
    if (settings && isFinite(settings.x) && isFinite(settings.y) && isFinite(settings.w)) {
      return normalizeBox(settings);
    }
    const defaults = defaultFrameSize(id);
    const boardWidth = state.grid ? state.grid.clientWidth : 1600;
    const visibleCount = visibleFramesCount();
    const targetWidth = Math.floor((boardWidth - GAP * (visibleCount - 1)) / visibleCount);
    const fitInRow = targetWidth >= MIN_SIZE;
    const size = fitInRow ? targetWidth : defaults.w;
    const height = fitInRow ? defaults.h : defaults.h;
    const maxPerRow = fitInRow
      ? visibleCount
      : Math.max(1, Math.floor((boardWidth + GAP) / (size + GAP)));
    const col = idx % maxPerRow;
    const row = Math.floor(idx / maxPerRow);
    const box = { x: col * (size + GAP), y: row * (height + GAP), w: size, h: height };
    setFrameBox(id, box);
    return box;
  }

  function defaultFrameSize(id) {
    switch (id) {
      case 'summary':
        return { w: 300, h: 300 };
      case 'todo':
      case 'documents':
      case 'incidents':
        return { w: 300, h: 300 };
      case 'incident_chart':
      case 'activity':
        return { w: 300, h: 300 };
      default:
        return { w: 300, h: 300 };
    }
  }

  function normalizeBox(box) {
    const w = Math.max(MIN_SIZE, parseInt(box.w, 10) || 400);
    const h = Math.max(MIN_SIZE, parseInt(box.h, 10) || w);
    return {
      x: Math.max(0, parseInt(box.x, 10) || 0),
      y: Math.max(0, parseInt(box.y, 10) || 0),
      w,
      h
    };
  }

  function applyFrameBox(card, box) {
    card.style.left = `${box.x}px`;
    card.style.top = `${box.y}px`;
    card.style.width = `${box.w}px`;
    card.style.height = `${box.h}px`;
    applyBoardHeight();
  }

  function clampFrame(box) {
    const board = state.grid;
    if (!board) return box;
    const width = board.clientWidth;
    const maxX = Math.max(0, width - box.w);
    return {
      x: Math.max(0, Math.min(box.x, maxX)),
      y: Math.max(0, box.y),
      w: box.w,
      h: box.h
    };
  }

  function persistFrameBox(id, card) {
    const board = state.grid;
    if (!board || !id) return;
    const boardRect = board.getBoundingClientRect();
    const rect = card.getBoundingClientRect();
    let box = {
      x: rect.left - boardRect.left,
      y: rect.top - boardRect.top,
      w: rect.width,
      h: rect.height
    };
    box = snapFrame(box, id);
    if (overlapsAny(box, getOtherBoxes(id))) {
      const prev = getFrameSettings(id);
      if (prev && isFinite(prev.x) && isFinite(prev.y) && isFinite(prev.w)) {
        const fallback = normalizeBox(prev);
        applyFrameBox(card, fallback);
        setFrameBox(id, fallback);
        return;
      }
    }
    setFrameBox(id, box);
    applyFrameBox(card, box);
    if (DashboardPage.setDirty) DashboardPage.setDirty();
    applyBoardHeight();
  }

  function visibleFramesCount() {
    const order = Array.isArray(state.layout?.order) ? state.layout.order : [];
    const hidden = new Set(state.layout?.hidden || []);
    const visible = order.filter(id => !hidden.has(id));
    return Math.max(1, visible.length);
  }

  function resolveCollisions(box, id) {
    const others = getOtherBoxes(id);
    if (!others.length) return box;
    let next = { ...box };
    let safety = 0;
    while (safety < 40 && overlapsAny(next, others)) {
      const hit = findFirstOverlap(next, others);
      if (!hit) break;
      next.y = Math.max(next.y, hit.y + hit.h + GAP);
      next = snapFrame(next, id);
      safety++;
    }
    return next;
  }

  function overlapsAny(box, others) {
    return others.some(other => isOverlap(box, other));
  }

  function findFirstOverlap(box, others) {
    return others.find(other => isOverlap(box, other)) || null;
  }

  function isOverlap(a, b) {
    if (!a || !b) return false;
    return a.x < b.x + b.w &&
      a.x + a.w > b.x &&
      a.y < b.y + b.h &&
      a.y + a.h > b.y;
  }

  function snapFrame(box, id) {
    let next = { ...box };
    next.x = snapToGrid(next.x);
    next.y = snapToGrid(next.y);
    next.w = snapToGrid(next.w, true);
    next.h = snapToGrid(next.h, true);
    const others = getOtherBoxes(id);
    others.forEach(other => {
      next = snapToNeighbor(next, other);
    });
    return next;
  }

  function snapToGrid(value, sizeMode) {
    const snap = GRID_SIZE;
    const snapped = Math.round(value / snap) * snap;
    if (sizeMode && snapped < MIN_SIZE) return MIN_SIZE;
    return Math.max(0, snapped);
  }

  function snapToNeighbor(box, other) {
    if (!other) return box;
    const candidates = [
      { axis: 'x', value: other.x, target: 'x' },
      { axis: 'x', value: other.x + other.w + GAP, target: 'x' },
      { axis: 'x', value: other.x + other.w - box.w, target: 'x' },
      { axis: 'y', value: other.y, target: 'y' },
      { axis: 'y', value: other.y + other.h + GAP, target: 'y' },
      { axis: 'y', value: other.y + other.h - box.h, target: 'y' }
    ];
    let next = { ...box };
    candidates.forEach(c => {
      const current = c.target === 'x' ? next.x : next.y;
      if (Math.abs(current - c.value) <= SNAP_DISTANCE) {
        if (c.target === 'x') next.x = c.value;
        if (c.target === 'y') next.y = c.value;
      }
    });
    return next;
  }

  function getOtherBoxes(skipId) {
    const board = state.grid;
    if (!board) return [];
    const nodes = board.querySelectorAll('.dashboard-frame');
    const res = [];
    nodes.forEach(node => {
      const id = node.dataset.frameId;
      if (!id || id === skipId) return;
      const box = getFrameBoxFromNode(node);
      if (box) res.push(box);
    });
    return res;
  }

  function getFrameBoxFromNode(node) {
    if (!node) return null;
    const rect = node.getBoundingClientRect();
    const board = state.grid;
    if (!board) return null;
    const boardRect = board.getBoundingClientRect();
    return {
      x: rect.left - boardRect.left,
      y: rect.top - boardRect.top,
      w: rect.width,
      h: rect.height
    };
  }

  function getFrameSettings(id) {
    if (!state.layout.settings) state.layout.settings = {};
    if (!state.layout.settings[id]) state.layout.settings[id] = {};
    return state.layout.settings[id];
  }

  function setFrameBox(id, box) {
    const settings = getFrameSettings(id);
    settings.x = box.x;
    settings.y = box.y;
    settings.w = box.w;
    settings.h = box.h;
  }

  function applyBoardHeight() {
    const board = state.grid;
    if (!board) return;
    let max = 0;
    const nodes = board.querySelectorAll('.dashboard-frame');
    nodes.forEach((node) => {
      const rect = node.getBoundingClientRect();
      const boardRect = board.getBoundingClientRect();
      const bottom = rect.top - boardRect.top + rect.height;
      if (bottom > max) max = bottom;
    });
    const next = Math.max(460, Math.ceil(max + GAP));
    board.style.minHeight = `${next}px`;
  }

  DashboardPage.bindDragResize = bindDragResize;
  DashboardPage.positionFrame = positionFrame;
  DashboardPage.ensureFrameBox = ensureFrameBox;
  DashboardPage.normalizeBox = normalizeBox;
  DashboardPage.applyFrameBox = applyFrameBox;
  DashboardPage.clampFrame = clampFrame;
  DashboardPage.persistFrameBox = persistFrameBox;
  DashboardPage.applyBoardHeight = applyBoardHeight;
  DashboardPage.getFrameSettings = getFrameSettings;
  DashboardPage.setFrameBox = setFrameBox;
})();
