import {
  fetchCategories,
  fetchSnapshot,
  postEndGame,
  postKick,
  postSettings,
  postStartGame
} from "./game_api.js";
import { updateFromSnapshot } from "./game_view.js";

const ctx = {
  els: {
    meta: document.getElementById("gameMeta"),
    joinCode: document.getElementById("joinCode"),
    gameStatus: document.getElementById("gameStatus"),
    playerList: document.getElementById("playerList"),
    playerActions: document.getElementById("playerActions"),
    gameError: document.getElementById("gameError"),
    startGame: document.getElementById("startGame"),
    endGame: document.getElementById("endGame"),
    settingsForm: document.getElementById("settingsForm"),
    roundsInput: document.getElementById("roundsInput"),
    maxPlayersInput: document.getElementById("maxPlayersInput"),
    promptCategory: document.getElementById("promptCategory"),
    lobbyLocked: document.getElementById("lobbyLocked"),
    lobbyStatus: document.getElementById("lobbyStatus"),
    settingsStatus: document.getElementById("settingsStatus")
  },
  state: {
    pollTimer: null,
    hostId: 0,
    categoriesLoaded: false,
    currentPhase: ""
  }
};

const lobbyAudio = document.getElementById("lobbyAudio");

function syncLobbyAudio(phase) {
  if (!lobbyAudio) return;
  if (phase === "lobby") {
    if (lobbyAudio.paused) {
      const playPromise = lobbyAudio.play();
      if (playPromise && typeof playPromise.catch === "function") {
        playPromise.catch(() => {
          // Ignore autoplay failures until user interacts.
        });
      }
    }
  } else if (!lobbyAudio.paused) {
    lobbyAudio.pause();
    lobbyAudio.currentTime = 0;
  }
}

function enableAudioOnInteraction() {
  if (!lobbyAudio) return;
  syncLobbyAudio(ctx.state.currentPhase);
}

async function loadGame() {
  if (!ctx.els.meta) return;
  const gameId = ctx.els.meta.dataset.gameId;
  const { res, data } = await fetchSnapshot(gameId);
  if (!res.ok) {
    ctx.els.joinCode.textContent = "Unavailable";
    ctx.els.gameStatus.textContent = "Unknown";
    if (ctx.els.gameError) {
      ctx.els.gameError.textContent = data.error || "Unable to load game status.";
    }
    return;
  }
  if (ctx.els.gameError) {
    ctx.els.gameError.textContent = "";
  }
  ctx.state.currentPhase = data.phase || "";
  updateFromSnapshot(ctx, data);
  syncLobbyAudio(ctx.state.currentPhase);
}

function startPolling() {
  if (ctx.state.pollTimer) return;
  ctx.state.pollTimer = setInterval(loadGame, 3000);
}

function connectWS() {
  if (!ctx.els.meta) return;
  const gameId = ctx.els.meta.dataset.gameId;
  const protocol = window.location.protocol === "https:" ? "wss" : "ws";
  const socket = new WebSocket(`${protocol}://${window.location.host}/ws/games/${encodeURIComponent(gameId)}?role=host`);

  socket.addEventListener("message", (event) => {
    try {
      const data = JSON.parse(event.data);
      ctx.state.currentPhase = data.phase || "";
      updateFromSnapshot(ctx, data);
      syncLobbyAudio(ctx.state.currentPhase);
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

async function loadCategories() {
  if (ctx.state.categoriesLoaded || !ctx.els.promptCategory) return;
  ctx.state.categoriesLoaded = true;
  const { data } = await fetchCategories();
  const categories = Array.isArray(data.categories) ? data.categories : [];
  ctx.els.promptCategory.innerHTML = "";
  const allOption = document.createElement("option");
  allOption.value = "";
  allOption.textContent = "All prompts";
  ctx.els.promptCategory.appendChild(allOption);
  categories.forEach((category) => {
    const option = document.createElement("option");
    option.value = category;
    option.textContent = category;
    ctx.els.promptCategory.appendChild(option);
  });
}

if (ctx.els.startGame) {
  ctx.els.startGame.addEventListener("click", async () => {
    if (!ctx.els.meta) return;
    if (!ctx.state.hostId) {
      if (ctx.els.gameError) {
        ctx.els.gameError.textContent = "Host not ready yet. Refresh in a moment.";
      }
      return;
    }
    const gameId = ctx.els.meta.dataset.gameId;
    const { res, data } = await postStartGame(gameId, ctx.state.hostId || 0);
    if (!res.ok) {
      if (ctx.els.gameError) {
        ctx.els.gameError.textContent = data.error || "Unable to start game.";
      }
      return;
    }
    if (ctx.els.gameError) {
      ctx.els.gameError.textContent = "";
    }
    updateFromSnapshot(ctx, data);
  });
}

if (ctx.els.endGame) {
  ctx.els.endGame.addEventListener("click", async () => {
    if (!ctx.els.meta) return;
    const gameId = ctx.els.meta.dataset.gameId;
    const { res, data } = await postEndGame(gameId);
    if (!res.ok) {
      if (ctx.els.gameError) {
        ctx.els.gameError.textContent = data.error || "Unable to end game.";
      }
      return;
    }
    if (ctx.els.gameError) {
      ctx.els.gameError.textContent = "";
    }
    updateFromSnapshot(ctx, data);
  });
}

if (ctx.els.settingsForm) {
  ctx.els.settingsForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    if (!ctx.els.meta) return;
    if (!ctx.state.hostId) {
      if (ctx.els.settingsStatus) {
        ctx.els.settingsStatus.textContent = "Host not ready yet. Refresh in a moment.";
      }
      return;
    }
    const gameId = ctx.els.meta.dataset.gameId;
    const rounds = Number(ctx.els.roundsInput?.value || 0);
    const maxPlayers = Number(ctx.els.maxPlayersInput?.value || 0);
    const category = ctx.els.promptCategory ? ctx.els.promptCategory.value : "";
    const locked = Boolean(ctx.els.lobbyLocked?.checked);
    if (ctx.els.settingsStatus) {
      ctx.els.settingsStatus.textContent = "Saving...";
    }
    const { res, data } = await postSettings(gameId, {
      player_id: ctx.state.hostId || 0,
      rounds,
      max_players: maxPlayers,
      prompt_category: category,
      lobby_locked: locked
    });
    if (!res.ok) {
      if (ctx.els.settingsStatus) {
        ctx.els.settingsStatus.textContent = data.error || "Unable to save settings.";
      }
      return;
    }
    if (ctx.els.settingsStatus) {
      ctx.els.settingsStatus.textContent = "Settings saved.";
    }
    updateFromSnapshot(ctx, data);
  });
  loadCategories();
}

if (ctx.els.playerActions) {
  ctx.els.playerActions.addEventListener("click", async (event) => {
    const target = event.target;
    if (!target || target.tagName !== "BUTTON") {
      return;
    }
    if (target.disabled) {
      return;
    }
    if (!ctx.els.meta) return;
    const gameId = ctx.els.meta.dataset.gameId;
    const playerId = Number(target.dataset.playerId || 0);
    if (!playerId) {
      if (ctx.els.gameError) {
        ctx.els.gameError.textContent = "Unable to resolve player.";
      }
      return;
    }
    const { res, data } = await postKick(gameId, {
      player_id: ctx.state.hostId || 0,
      target_id: playerId
    });
    if (!res.ok) {
      if (ctx.els.gameError) {
        ctx.els.gameError.textContent = data.error || "Unable to remove player.";
      }
      return;
    }
    if (ctx.els.gameError) {
      ctx.els.gameError.textContent = "";
    }
    updateFromSnapshot(ctx, data);
  });
}

loadGame();
connectWS();
document.addEventListener("click", enableAudioOnInteraction, { once: true });
document.addEventListener("keydown", enableAudioOnInteraction, { once: true });
