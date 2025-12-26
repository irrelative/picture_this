import { applyHTMLMessage } from "./ws_html.js";

let displayContent = document.getElementById("displayContent");
const lobbyAudio = document.getElementById("lobbyAudio");
const joinSound = document.getElementById("joinSound");
const roundStartSound = document.getElementById("roundStartSound");
const timerEndSound = document.getElementById("timerEndSound");
const votingStartSound = document.getElementById("votingStartSound");

const state = {
  phase: "",
  phaseEndsAt: 0,
  timerHandle: null,
  playerCount: null,
  round: null,
  lastPhase: "",
  timerEndedKey: ""
};

function formatTime(seconds) {
  const minutes = Math.floor(seconds / 60);
  const remainder = seconds % 60;
  return `${String(minutes).padStart(2, "0")}:${String(remainder).padStart(2, "0")}`;
}

function renderTimer() {
  const timerEl = document.getElementById("displayTimer");
  if (!timerEl) return;
  if (state.phase !== "drawings" || !state.phaseEndsAt) {
    timerEl.textContent = "--:--";
    return;
  }
  const remaining = Math.max(0, Math.round((state.phaseEndsAt - Date.now()) / 1000));
  timerEl.textContent = formatTime(remaining);
  if (remaining === 0 && timerEndSound) {
    const key = `${state.phase}:${state.round || 0}`;
    if (state.timerEndedKey !== key) {
      state.timerEndedKey = key;
      const playPromise = timerEndSound.play();
      if (playPromise && typeof playPromise.catch === "function") {
        playPromise.catch(() => {
          // Ignore autoplay failures until user interacts.
        });
      }
    }
  }
}

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

function syncFromContent() {
  displayContent = document.getElementById("displayContent");
  if (!displayContent) return;
  const nextPhase = displayContent.dataset.phase || "";
  const endsAt = displayContent.dataset.phaseEndsAt ? Date.parse(displayContent.dataset.phaseEndsAt) : 0;
  state.phaseEndsAt = Number.isNaN(endsAt) ? 0 : endsAt;
  const countValue = Number(displayContent.dataset.playerCount || 0);
  const roundValue = Number(displayContent.dataset.round || 0);
  if (state.phase === "lobby" && state.playerCount !== null && countValue > state.playerCount) {
    if (joinSound) {
      const playPromise = joinSound.play();
      if (playPromise && typeof playPromise.catch === "function") {
        playPromise.catch(() => {
          // Ignore autoplay failures until user interacts.
        });
      }
    }
  }
  if (
    roundStartSound &&
    nextPhase === "drawings" &&
    (state.round === null || roundValue > state.round) &&
    roundValue > 0
  ) {
    const playPromise = roundStartSound.play();
    if (playPromise && typeof playPromise.catch === "function") {
      playPromise.catch(() => {
        // Ignore autoplay failures until user interacts.
      });
    }
  }
  if (votingStartSound && nextPhase === "guesses-votes" && state.lastPhase !== "guesses-votes") {
    const playPromise = votingStartSound.play();
    if (playPromise && typeof playPromise.catch === "function") {
      playPromise.catch(() => {
        // Ignore autoplay failures until user interacts.
      });
    }
  }
  state.phase = nextPhase;
  state.playerCount = Number.isNaN(countValue) ? state.playerCount : countValue;
  state.round = Number.isNaN(roundValue) ? state.round : roundValue;
  state.lastPhase = nextPhase;
  syncLobbyAudio(state.phase);
  renderTimer();
  if (!state.timerHandle) {
    state.timerHandle = setInterval(renderTimer, 1000);
  }
}

document.addEventListener(
  "click",
  () => {
    syncLobbyAudio(state.phase);
  },
  { once: true }
);

function connectWS() {
  if (!displayContent) return;
  const gameId = displayContent.dataset.gameId;
  if (!gameId) return;
  const protocol = window.location.protocol === "https:" ? "wss" : "ws";
  const socket = new WebSocket(`${protocol}://${window.location.host}/ws/games/${encodeURIComponent(gameId)}?role=display`);

  socket.addEventListener("message", (event) => {
    const result = applyHTMLMessage(event.data);
    if (result && result.target) {
      displayContent = result.target;
      syncFromContent();
    }
  });

  socket.addEventListener("close", () => {
    setTimeout(connectWS, 2000);
  });

  socket.addEventListener("error", () => {
    socket.close();
  });
}

syncFromContent();
connectWS();
