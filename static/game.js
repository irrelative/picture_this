const meta = document.getElementById("gameMeta");
const joinCode = document.getElementById("joinCode");
const gameStatus = document.getElementById("gameStatus");
const playerList = document.getElementById("playerList");
const playerActions = document.getElementById("playerActions");
const gameError = document.getElementById("gameError");
const startGame = document.getElementById("startGame");
const endGame = document.getElementById("endGame");
const settingsForm = document.getElementById("settingsForm");
const roundsInput = document.getElementById("roundsInput");
const maxPlayersInput = document.getElementById("maxPlayersInput");
const promptCategory = document.getElementById("promptCategory");
const lobbyLocked = document.getElementById("lobbyLocked");
const lobbyStatus = document.getElementById("lobbyStatus");
const settingsStatus = document.getElementById("settingsStatus");
let pollTimer = null;
let hostId = 0;
let categoriesLoaded = false;

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
    startGame.disabled = data.phase === "lobby" ? (data.players?.length || 0) < 2 : true;
  }
  if (endGame) {
    endGame.style.display = data.phase !== "complete" ? "inline-flex" : "none";
  }

  playerList.innerHTML = "";
  if (playerActions) {
    playerActions.innerHTML = "";
  }
  const players = Array.isArray(data.players) ? data.players : [];
  if (players.length === 0) {
    const item = document.createElement("li");
    item.textContent = "No players yet";
    playerList.appendChild(item);
    return;
  }
  const playerIDs = Array.isArray(data.player_ids) ? data.player_ids : [];
  players.forEach((player, index) => {
    const item = document.createElement("li");
    item.textContent = player;
    playerList.appendChild(item);

    if (playerActions) {
      const row = document.createElement("div");
      row.className = "player-action-row";
      const label = document.createElement("span");
      label.textContent = player;
      const kickButton = document.createElement("button");
      kickButton.type = "button";
      kickButton.className = "secondary";
      kickButton.textContent = "Remove";
      kickButton.dataset.playerId = String(playerIDs[index] || 0);
      if (playerIDs[index] === hostId) {
        kickButton.disabled = true;
      }
      row.appendChild(label);
      row.appendChild(kickButton);
      playerActions.appendChild(row);
    }
  });

  if (roundsInput) {
    roundsInput.value = data.total_rounds || data.prompts_per_player || 2;
  }
  if (maxPlayersInput) {
    maxPlayersInput.value = data.max_players || 0;
  }
  if (promptCategory) {
    promptCategory.value = data.prompt_category || "";
  }
  if (lobbyLocked) {
    lobbyLocked.checked = Boolean(data.lobby_locked);
  }
  if (lobbyStatus) {
    const maxPlayers = data.max_players > 0 ? data.max_players : "∞";
    const lockedText = data.lobby_locked ? "Locked" : "Open";
    const audienceCount = data.audience_count != null ? data.audience_count : 0;
    lobbyStatus.textContent = `Players: ${players.length}/${maxPlayers}. ${lockedText} lobby. Audience: ${audienceCount}.`;
  }
  if (settingsForm) {
    const disabled = data.phase !== "lobby";
    Array.from(settingsForm.elements).forEach((el) => {
      if (el.tagName === "BUTTON") return;
      el.disabled = disabled;
    });
    if (settingsForm.querySelector("button")) {
      settingsForm.querySelector("button").disabled = disabled;
    }
  }


  if (playerActions) {
    const disabled = data.phase !== "lobby";
    Array.from(playerActions.querySelectorAll("button")).forEach((button) => {
      button.disabled = disabled;
    });
  }
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

async function loadCategories() {
  if (categoriesLoaded || !promptCategory) return;
  categoriesLoaded = true;
  const res = await fetch("/api/prompts/categories");
  const data = await res.json().catch(() => ({}));
  const categories = Array.isArray(data.categories) ? data.categories : [];
  promptCategory.innerHTML = "";
  const allOption = document.createElement("option");
  allOption.value = "";
  allOption.textContent = "All prompts";
  promptCategory.appendChild(allOption);
  categories.forEach((category) => {
    const option = document.createElement("option");
    option.value = category;
    option.textContent = category;
    promptCategory.appendChild(option);
  });
}

if (settingsForm) {
  settingsForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    if (!meta) return;
    const gameId = meta.dataset.gameId;
    const rounds = Number(roundsInput?.value || 0);
    const maxPlayers = Number(maxPlayersInput?.value || 0);
    const category = promptCategory ? promptCategory.value : "";
    const locked = Boolean(lobbyLocked?.checked);
    if (settingsStatus) {
      settingsStatus.textContent = "Saving...";
    }
    const res = await fetch(`/api/games/${encodeURIComponent(gameId)}/settings`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        player_id: hostId || 0,
        rounds,
        max_players: maxPlayers,
        prompt_category: category,
        lobby_locked: locked
      })
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      if (settingsStatus) {
        settingsStatus.textContent = data.error || "Unable to save settings.";
      }
      return;
    }
    if (settingsStatus) {
      settingsStatus.textContent = "Settings saved.";
    }
    updateFromSnapshot(data);
  });
  loadCategories();
}

if (playerActions) {
  playerActions.addEventListener("click", async (event) => {
    const target = event.target;
    if (!target || target.tagName !== "BUTTON") {
      return;
    }
    if (target.disabled) {
      return;
    }
    if (!meta) return;
    const gameId = meta.dataset.gameId;
    const playerId = Number(target.dataset.playerId || 0);
    if (!playerId) {
      if (gameError) {
        gameError.textContent = "Unable to resolve player.";
      }
      return;
    }
    const kickRes = await fetch(`/api/games/${encodeURIComponent(gameId)}/kick`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        player_id: hostId || 0,
        target_id: playerId
      })
    });
    const data = await kickRes.json().catch(() => ({}));
    if (!kickRes.ok) {
      if (gameError) {
        gameError.textContent = data.error || "Unable to remove player.";
      }
      return;
    }
    if (gameError) {
      gameError.textContent = "";
    }
    updateFromSnapshot(data);
  });
}
