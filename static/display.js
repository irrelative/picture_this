const lobbyAudio = document.getElementById("lobbyAudio");

const state = {
  phase: "",
  phaseEndsAt: 0,
  timerHandle: null
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
  const content = document.getElementById("displayContent");
  if (!content) return;
  state.phase = content.dataset.phase || "";
  const endsAt = content.dataset.phaseEndsAt ? Date.parse(content.dataset.phaseEndsAt) : 0;
  state.phaseEndsAt = Number.isNaN(endsAt) ? 0 : endsAt;
  syncLobbyAudio(state.phase);
  renderTimer();
  if (!state.timerHandle) {
    state.timerHandle = setInterval(renderTimer, 1000);
  }
}

document.body.addEventListener("htmx:afterSwap", (event) => {
  if (event.target && event.target.id === "displayContent") {
    syncFromContent();
  }
});

document.addEventListener(
  "click",
  () => {
    syncLobbyAudio(state.phase);
  },
  { once: true }
);

syncFromContent();
