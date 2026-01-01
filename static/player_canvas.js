export function setupCanvas(ctx, onSave) {
  const { els, state } = ctx;
  if (!els.canvas) return;
  const ctx2d = els.canvas.getContext("2d");
  if (!ctx2d) return;
  state.canvasCtx = ctx2d;

  ctx2d.lineCap = "round";
  ctx2d.lineJoin = "round";
  ctx2d.lineWidth = state.brushSize || 4;
  ctx2d.strokeStyle = state.brushColor;
  ctx2d.fillStyle = "#ffffff";
  ctx2d.fillRect(0, 0, state.canvasWidth, state.canvasHeight);

  let drawing = false;
  let lastPoint = null;

  function getPoint(event) {
    const rect = els.canvas.getBoundingClientRect();
    const clientX = event.clientX ?? (event.touches && event.touches[0]?.clientX);
    const clientY = event.clientY ?? (event.touches && event.touches[0]?.clientY);
    if (clientX == null || clientY == null) {
      return null;
    }
    const x = (clientX - rect.left) * (els.canvas.width / rect.width);
    const y = (clientY - rect.top) * (els.canvas.height / rect.height);
    return { x, y };
  }

  function startDraw(event) {
    drawing = true;
    lastPoint = getPoint(event);
  }

  function moveDraw(event) {
    if (!drawing) return;
    const point = getPoint(event);
    if (!point || !lastPoint) return;
    ctx2d.beginPath();
    ctx2d.moveTo(lastPoint.x, lastPoint.y);
    ctx2d.lineTo(point.x, point.y);
    ctx2d.stroke();
    lastPoint = point;
  }

  function endDraw() {
    drawing = false;
    lastPoint = null;
  }

  els.canvas.addEventListener("pointerdown", (event) => {
    event.preventDefault();
    els.canvas.setPointerCapture(event.pointerId);
    startDraw(event);
  });

  els.canvas.addEventListener("pointermove", (event) => {
    event.preventDefault();
    moveDraw(event);
  });

  els.canvas.addEventListener("pointerup", (event) => {
    event.preventDefault();
    endDraw();
    els.canvas.releasePointerCapture(event.pointerId);
  });

  els.canvas.addEventListener("pointerleave", endDraw);
  els.canvas.addEventListener("pointercancel", endDraw);

  if (els.saveCanvas) {
    els.saveCanvas.addEventListener("click", () => {
      const dataUrl = els.canvas.toDataURL("image/png");
      onSave(dataUrl);
    });
  }
}

export function applyBrushColor(ctx) {
  if (!ctx.state.canvasCtx) return;
  ctx.state.canvasCtx.strokeStyle = ctx.state.brushColor;
}

export function clearCanvas(ctx) {
  const ctx2d = ctx.state.canvasCtx;
  if (!ctx2d) return;
  ctx2d.fillStyle = "#ffffff";
  ctx2d.fillRect(0, 0, ctx.state.canvasWidth, ctx.state.canvasHeight);
}
