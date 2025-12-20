const meta = document.getElementById("playerMeta");
const joinCode = document.getElementById("joinCode");
const gameStatus = document.getElementById("gameStatus");
const playerList = document.getElementById("playerList");
const playerName = document.getElementById("playerName");
const playerError = document.getElementById("playerError");
const drawSection = document.getElementById("drawSection");
const promptText = document.getElementById("promptText");
const promptChoices = document.getElementById("promptChoices");
const canvas = document.getElementById("drawCanvas");
const clearCanvas = document.getElementById("clearCanvas");
const saveCanvas = document.getElementById("saveCanvas");

const CANVAS_WIDTH = 800;
const CANVAS_HEIGHT = 600;
let pollTimer = null;
let assignedPrompts = [];
let selectedPrompt = "";

async function loadPlayerView() {
  if (!meta) return;
  const gameId = meta.dataset.gameId;
  const name = meta.dataset.playerName;

  if (playerName && name) {
    playerName.textContent = `Signed in as ${name}. Waiting for the host to begin.`;
  }

  await fetchSnapshot(gameId);
}

async function fetchSnapshot(gameId) {
  const res = await fetch(`/api/games/${encodeURIComponent(gameId)}`);
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    joinCode.textContent = "Unavailable";
    gameStatus.textContent = "Unknown";
    if (playerError) {
      playerError.textContent = data.error || "Unable to load game status.";
    }
    return;
  }
  if (playerError) {
    playerError.textContent = "";
  }
  updateFromSnapshot(data);
}

function updateFromSnapshot(data) {
  joinCode.textContent = data.join_code || "Unavailable";
  gameStatus.textContent = data.phase || "Unknown";
  playerList.innerHTML = "";
  const players = Array.isArray(data.players) ? data.players : [];
  if (players.length === 0) {
    const item = document.createElement("li");
    item.textContent = "No players yet";
    playerList.appendChild(item);
    return;
  }
  players.forEach((player) => {
    const item = document.createElement("li");
    item.textContent = player;
    playerList.appendChild(item);
  });

  if (drawSection) {
    drawSection.style.display = data.phase === "drawings" ? "grid" : "none";
    if (data.phase === "drawings") {
      fetchPrompt();
    }
  }
}

function startPolling() {
  if (pollTimer) return;
  pollTimer = setInterval(loadPlayerView, 3000);
}

function connectWS() {
  if (!meta) return;
  const gameId = meta.dataset.gameId;
  const protocol = window.location.protocol === "https:" ? "wss" : "ws";
  const socket = new WebSocket(`${protocol}://${window.location.host}/ws/games/${encodeURIComponent(gameId)}`);

  socket.addEventListener("message", (event) => {
    try {
      const data = JSON.parse(event.data);
      updateFromSnapshot(data);
    } catch {
      // ignore invalid payloads
    }
  });

  socket.addEventListener("close", () => {
    startPolling();
  });

  socket.addEventListener("error", () => {
    startPolling();
  });
}

loadPlayerView();
connectWS();

function setupCanvas() {
  if (!canvas) return;
  const ctx = canvas.getContext("2d");
  if (!ctx) return;

  ctx.lineCap = "round";
  ctx.lineJoin = "round";
  ctx.lineWidth = 4;
  ctx.strokeStyle = "#1a1a1a";
  ctx.fillStyle = "#ffffff";
  ctx.fillRect(0, 0, CANVAS_WIDTH, CANVAS_HEIGHT);

  let drawing = false;
  let lastPoint = null;

  function getPoint(event) {
    const rect = canvas.getBoundingClientRect();
    const clientX = event.clientX ?? (event.touches && event.touches[0]?.clientX);
    const clientY = event.clientY ?? (event.touches && event.touches[0]?.clientY);
    if (clientX == null || clientY == null) {
      return null;
    }
    const x = (clientX - rect.left) * (canvas.width / rect.width);
    const y = (clientY - rect.top) * (canvas.height / rect.height);
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
    ctx.beginPath();
    ctx.moveTo(lastPoint.x, lastPoint.y);
    ctx.lineTo(point.x, point.y);
    ctx.stroke();
    lastPoint = point;
  }

  function endDraw() {
    drawing = false;
    lastPoint = null;
  }

  canvas.addEventListener("pointerdown", (event) => {
    event.preventDefault();
    canvas.setPointerCapture(event.pointerId);
    startDraw(event);
  });

  canvas.addEventListener("pointermove", (event) => {
    event.preventDefault();
    moveDraw(event);
  });

  canvas.addEventListener("pointerup", (event) => {
    event.preventDefault();
    endDraw();
    canvas.releasePointerCapture(event.pointerId);
  });

  canvas.addEventListener("pointerleave", endDraw);
  canvas.addEventListener("pointercancel", endDraw);

  if (clearCanvas) {
    clearCanvas.addEventListener("click", () => {
      ctx.clearRect(0, 0, CANVAS_WIDTH, CANVAS_HEIGHT);
      ctx.fillStyle = "#ffffff";
      ctx.fillRect(0, 0, CANVAS_WIDTH, CANVAS_HEIGHT);
    });
  }

  if (saveCanvas) {
    saveCanvas.addEventListener("click", async () => {
      const dataUrl = canvas.toDataURL("image/png");
      if (!meta) return;
      const gameId = meta.dataset.gameId;
      const playerId = meta.dataset.playerId;
      const res = await fetch(`/api/games/${encodeURIComponent(gameId)}/drawings`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          player_id: Number(playerId),
          image_data: dataUrl,
          prompt: selectedPrompt
        })
      });
      const payload = await res.json().catch(() => ({}));
      if (!res.ok) {
        if (playerError) {
          playerError.textContent = payload.error || "Unable to submit drawing.";
        }
        return;
      }
      if (playerError) {
        playerError.textContent = "";
      }
    });
  }
}

setupCanvas();

async function fetchPrompt() {
  if (!meta) return;
  if (assignedPrompts.length > 0) {
    return;
  }
  const gameId = meta.dataset.gameId;
  const playerId = meta.dataset.playerId;
  const res = await fetch(`/api/games/${encodeURIComponent(gameId)}/players/${encodeURIComponent(playerId)}/prompt`);
  const payload = await res.json().catch(() => ({}));
  if (!res.ok) {
    if (playerError) {
      playerError.textContent = payload.error || "Unable to load prompt.";
    }
    return;
  }
  assignedPrompts = Array.isArray(payload.prompts) ? payload.prompts : [];
  if (assignedPrompts.length === 0) {
    if (playerError) {
      playerError.textContent = "No prompts assigned.";
    }
    return;
  }
  selectedPrompt = assignedPrompts[0];
  renderPromptChoices();
}

function renderPromptChoices() {
  if (promptText) {
    promptText.textContent = selectedPrompt || "Select a prompt";
  }
  if (!promptChoices) return;
  promptChoices.innerHTML = "";
  if (assignedPrompts.length <= 1) {
    return;
  }
  assignedPrompts.forEach((prompt) => {
    const label = document.createElement("label");
    label.className = "prompt-option";
    const radio = document.createElement("input");
    radio.type = "radio";
    radio.name = "promptChoice";
    radio.value = prompt;
    radio.checked = prompt === selectedPrompt;
    radio.addEventListener("change", () => {
      selectedPrompt = prompt;
      if (promptText) {
        promptText.textContent = selectedPrompt;
      }
    });
    const text = document.createElement("span");
    text.textContent = prompt;
    label.appendChild(radio);
    label.appendChild(text);
    promptChoices.appendChild(label);
  });
}
