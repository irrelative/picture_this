import { applyHTMLMessage } from "./ws_html.js";

let displayContent = document.getElementById("displayContent");
const lobbyAudio = document.getElementById("lobbyAudio");
const drawingAudio = document.getElementById("drawingAudio");
const writeLieAudio = document.getElementById("writeLieAudio");
const chooseLieAudio = document.getElementById("chooseLieAudio");
const questionAudio = document.getElementById("questionAudio");
const creditsAudio = document.getElementById("creditsAudio");
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
  lastMusicPhase: "",
  timerEndedKey: "",
  connected: false
};

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

function stopAllMusic() {
  phaseMusic.forEach((audio) => {
    stopAudio(audio);
  });
  state.lastMusicPhase = "";
}

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
      playAudio(timerEndSound);
    }
  }
}

function syncPhaseAudio(phase) {
  const targetAudio = phaseMusic.get(phase) || null;
  if (state.lastMusicPhase !== phase) {
    phaseMusic.forEach((audio, key) => {
      if (key !== phase) {
        stopAudio(audio);
      }
    });
    if (targetAudio) {
      targetAudio.currentTime = 0;
    }
    state.lastMusicPhase = phase;
  }
  playAudio(targetAudio);
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
      playAudio(joinSound);
    }
  }
  if (
    roundStartSound &&
    nextPhase === "drawings" &&
    (state.round === null || roundValue > state.round) &&
    roundValue > 0
  ) {
    playAudio(roundStartSound);
  }
  if (votingStartSound && nextPhase === "guesses-votes" && state.lastPhase !== "guesses-votes") {
    playAudio(votingStartSound);
  }
  state.phase = nextPhase;
  state.playerCount = Number.isNaN(countValue) ? state.playerCount : countValue;
  state.round = Number.isNaN(roundValue) ? state.round : roundValue;
  state.lastPhase = nextPhase;
  if (state.connected) {
    syncPhaseAudio(state.phase);
  }
  renderTimer();
  if (!state.timerHandle) {
    state.timerHandle = setInterval(renderTimer, 1000);
  }
}

document.addEventListener(
  "click",
  () => {
    syncPhaseAudio(state.phase);
  },
  { once: true }
);

function connectWS() {
  if (!displayContent) return;
  const gameId = displayContent.dataset.gameId;
  if (!gameId) return;
  const protocol = window.location.protocol === "https:" ? "wss" : "ws";
  const socket = new WebSocket(`${protocol}://${window.location.host}/ws/games/${encodeURIComponent(gameId)}?role=display`);

  socket.addEventListener("open", () => {
    state.connected = true;
    syncFromContent();
  });

  socket.addEventListener("message", (event) => {
    const result = applyHTMLMessage(event.data);
    if (result && result.target) {
      displayContent = result.target;
      syncFromContent();
    }
  });

  socket.addEventListener("close", () => {
    state.connected = false;
    stopAllMusic();
    setTimeout(connectWS, 2000);
  });

  socket.addEventListener("error", () => {
    state.connected = false;
    stopAllMusic();
    socket.close();
  });
}

syncFromContent();
connectWS();
