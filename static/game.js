import {
  fetchCategories,
  fetchSnapshot,
  postEndGame,
  postKick,
  postSettings,
  postStartGame
} from "./game_api.js";
import { updateFromSnapshot } from "./game_view.js";
import { applyHTMLMessage } from "./ws_html.js";

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
    displayTimer: document.getElementById("displayTimer"),
    displayRound: document.getElementById("displayRound"),
    displayStageTitle: document.getElementById("displayStageTitle"),
    displayStageStatus: document.getElementById("displayStageStatus"),
    displayStageImage: document.getElementById("displayStageImage"),
    displayOptions: document.getElementById("displayOptions"),
    displayScoreboard: document.getElementById("displayScoreboard"),
    displayScoreTitle: document.getElementById("displayScoreTitle"),
    displayScoreStatus: document.getElementById("displayScoreStatus"),
    displayScoreList: document.getElementById("displayScoreList"),
    displayFinalScores: document.getElementById("displayFinalScores"),
    displayFinalList: document.getElementById("displayFinalList"),
    hostPanel: document.getElementById("hostPanel"),
    hostStatus: document.getElementById("hostStatus"),
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
    currentPhase: "",
    lastMusicPhase: "",
    timerEndsAt: 0,
    timerPhase: "",
    timerHandle: null
  }
};

const lobbyAudio = document.getElementById("lobbyAudio");
const drawingAudio = document.getElementById("drawingAudio");
const writeLieAudio = document.getElementById("writeLieAudio");
const chooseLieAudio = document.getElementById("chooseLieAudio");
const questionAudio = document.getElementById("questionAudio");
const creditsAudio = document.getElementById("creditsAudio");

const phaseMusic = new Map([
  ["lobby", lobbyAudio],
  ["drawings", drawingAudio],
  ["guesses", writeLieAudio],
  ["guesses-votes", chooseLieAudio],
  ["results", questionAudio],
  ["complete", creditsAudio]
]);

function playAudio(audio) {
  if (!audio) return;
  if (!audio.paused) return;
  const playPromise = audio.play();
  if (playPromise && typeof playPromise.catch === "function") {
    playPromise.catch(() => {
      // Ignore autoplay failures until user interacts.
    });
  }
}

function stopAudio(audio) {
  if (!audio || audio.paused) return;
  audio.pause();
  audio.currentTime = 0;
}

function syncPhaseAudio(phase) {
  const targetAudio = phaseMusic.get(phase) || null;
  if (ctx.state.lastMusicPhase !== phase) {
    phaseMusic.forEach((audio, key) => {
      if (key !== phase) {
        stopAudio(audio);
      }
    });
    if (targetAudio) {
      targetAudio.currentTime = 0;
    }
    ctx.state.lastMusicPhase = phase;
  }
  playAudio(targetAudio);
}

function enableAudioOnInteraction() {
  syncPhaseAudio(ctx.state.currentPhase);
}

function formatTime(seconds) {
  const minutes = Math.floor(seconds / 60);
  const remainder = seconds % 60;
  return `${String(minutes).padStart(2, "0")}:${String(remainder).padStart(2, "0")}`;
}

function renderTimer() {
  if (!ctx.els.displayTimer) return;
  if (ctx.state.timerPhase !== "drawings" || !ctx.state.timerEndsAt) {
    ctx.els.displayTimer.textContent = "--:--";
    return;
  }
  const remaining = Math.max(0, Math.round((ctx.state.timerEndsAt - Date.now()) / 1000));
  ctx.els.displayTimer.textContent = formatTime(remaining);
}

function syncTimer(data) {
  const endsAt = data.phase_ends_at ? Date.parse(data.phase_ends_at) : 0;
  ctx.state.timerEndsAt = Number.isNaN(endsAt) ? 0 : endsAt;
  ctx.state.timerPhase = data.phase || "";
  renderTimer();
  if (!ctx.state.timerHandle) {
    ctx.state.timerHandle = setInterval(renderTimer, 1000);
  }
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
  syncPhaseAudio(ctx.state.currentPhase);
  syncTimer(data);
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
    const htmlResult = applyHTMLMessage(event.data);
    if (htmlResult) {
      return;
    }
    try {
      const data = JSON.parse(event.data);
      ctx.state.currentPhase = data.phase || "";
      updateFromSnapshot(ctx, data);
      syncPhaseAudio(ctx.state.currentPhase);
      syncTimer(data);
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
