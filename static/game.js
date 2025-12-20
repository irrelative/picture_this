const meta = document.getElementById("gameMeta");
const joinCode = document.getElementById("joinCode");
const gameStatus = document.getElementById("gameStatus");
const playerList = document.getElementById("playerList");
const gameError = document.getElementById("gameError");
const startGame = document.getElementById("startGame");
const endGame = document.getElementById("endGame");
let pollTimer = null;
let hostId = 0;

async function loadGame() {
  if (!meta) return;
  const gameId = meta.dataset.gameId;
  await fetchSnapshot(gameId);
}

async function fetchSnapshot(gameId) {
  const res = await fetch(`/api/games/${encodeURIComponent(gameId)}`);
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    joinCode.textContent = "Unavailable";
    gameStatus.textContent = "Unknown";
    if (gameError) {
      gameError.textContent = data.error || "Unable to load game status.";
    }
    return;
  }
  if (gameError) {
    gameError.textContent = "";
  }
  updateFromSnapshot(data);
}

function updateFromSnapshot(data) {
  joinCode.textContent = data.join_code || "Unavailable";
  gameStatus.textContent = data.phase || "Unknown";
  hostId = data.host_id || 0;
  if (startGame) {
    startGame.style.display = data.phase === "lobby" ? "inline-flex" : "none";
  }
  if (endGame) {
    endGame.style.display = data.phase !== "complete" ? "inline-flex" : "none";
  }

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
}

function startPolling() {
  if (pollTimer) return;
  pollTimer = setInterval(loadGame, 3000);
}

function connectWS() {
  if (!meta) return;
  const gameId = meta.dataset.gameId;
  const protocol = window.location.protocol === "https:" ? "wss" : "ws";
  const socket = new WebSocket(`${protocol}://${window.location.host}/ws/games/${encodeURIComponent(gameId)}?role=host`);

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

if (startGame) {
  startGame.addEventListener("click", async () => {
    if (!meta) return;
    const gameId = meta.dataset.gameId;
    const res = await fetch(`/api/games/${encodeURIComponent(gameId)}/start`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ player_id: hostId || 0 })
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      if (gameError) {
        gameError.textContent = data.error || "Unable to start game.";
      }
      return;
    }
    if (gameError) {
      gameError.textContent = "";
    }
    updateFromSnapshot(data);
  });
}

if (endGame) {
  endGame.addEventListener("click", async () => {
    if (!meta) return;
    const gameId = meta.dataset.gameId;
    const res = await fetch(`/api/games/${encodeURIComponent(gameId)}/end`, {
      method: "POST"
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      if (gameError) {
        gameError.textContent = data.error || "Unable to end game.";
      }
      return;
    }
    if (gameError) {
      gameError.textContent = "";
    }
    updateFromSnapshot(data);
  });
}

loadGame();
connectWS();
