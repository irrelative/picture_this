import { applyHTMLMessage } from "./ws_html.js";

let displayContent = document.getElementById("displayContent");
const displayEventFx = document.getElementById("displayEventFx");
const displayShell = document.querySelector(".display-shell");
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
const interludeVoiceAudio = document.getElementById("interludeVoiceAudio");
const jokeNarrationAudio = document.getElementById("jokeNarrationAudio");

// Interlude voice-over catalog (text + generated asset path).
const interludeCues = {
  phase_drawings: {
    text: "Time to draw!",
    src: "/static/audio/interludes/phase_drawings.mp3"
  },
  phase_guesses: {
    text: "Pass it on. Write a fake prompt.",
    src: "/static/audio/interludes/phase_guesses.mp3"
  },
  phase_votes: {
    text: "Vote time. Pick the real prompt.",
    src: "/static/audio/interludes/phase_votes.mp3"
  },
  phase_results: {
    text: "Let's reveal the answers.",
    src: "/static/audio/interludes/phase_results.mp3"
  },
  reveal_guesses: {
    text: "Here come the lies.",
    src: "/static/audio/interludes/reveal_guesses.mp3"
  },
  reveal_votes: {
    text: "And now, the votes.",
    src: "/static/audio/interludes/reveal_votes.mp3"
  },
  phase_complete: {
    text: "Game over. Final scores!",
    src: "/static/audio/interludes/phase_complete.mp3"
  }
};

const state = {
  phase: "",
  phaseEndsAt: 0,
  revealStage: "",
  revealJokeAudio: "",
  revealDrawingIndex: -1,
  drawingSubmittedCount: 0,
  drawingRequiredCount: 0,
  guessSubmittedCount: 0,
  guessRequiredCount: 0,
  voteSubmittedCount: 0,
  voteRequiredCount: 0,
  timerHandle: null,
  playerCount: null,
  round: null,
  lastPhase: "",
  lastMusicPhase: "",
  lastJokeNarrationKey: "",
  interludeQueue: [],
  interludePlaying: false,
  currentInterludeKey: "",
  initialized: false,
  jokeNarrationPlaying: false,
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

function playSfx(audio, rate = 1, volume = 1) {
  if (!audio) return;
  const clip = audio.cloneNode(true);
  clip.playbackRate = rate;
  clip.volume = Math.max(0, Math.min(1, volume));
  const playPromise = clip.play();
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
  if (!state.phaseEndsAt) {
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
  if (state.jokeNarrationPlaying || state.interludePlaying) {
    return;
  }
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

function enqueueInterlude(key) {
  if (!interludeVoiceAudio || !key || !interludeCues[key]) return;
  const lastQueued = state.interludeQueue[state.interludeQueue.length - 1];
  if (lastQueued === key) {
    return;
  }
  state.interludeQueue.push(key);
  playNextInterlude();
}

function playNextInterlude() {
  if (!interludeVoiceAudio || state.interludePlaying || state.jokeNarrationPlaying) {
    return;
  }
  const key = state.interludeQueue.shift();
  if (!key || !interludeCues[key]) {
    return;
  }
  const cue = interludeCues[key];
  state.interludePlaying = true;
  state.currentInterludeKey = key;
  stopAllMusic();
  interludeVoiceAudio.src = cue.src;
  interludeVoiceAudio.currentTime = 0;
  const playPromise = interludeVoiceAudio.play();
  if (playPromise && typeof playPromise.catch === "function") {
    playPromise.catch(() => {
      state.interludePlaying = false;
      state.currentInterludeKey = "";
      if (state.connected) {
        syncPhaseAudio(state.phase);
      }
    });
  }
}

function queueTransitionInterludes(nextPhase, revealStage, revealDrawingIndex) {
  if (!state.initialized) {
    return;
  }
  if (nextPhase !== state.phase) {
    if (nextPhase === "drawings") enqueueInterlude("phase_drawings");
    if (nextPhase === "guesses") enqueueInterlude("phase_guesses");
    if (nextPhase === "guesses-votes") enqueueInterlude("phase_votes");
    if (nextPhase === "results") enqueueInterlude("phase_results");
    if (nextPhase === "complete") enqueueInterlude("phase_complete");
    return;
  }
  if (nextPhase !== "results") {
    return;
  }
  if (revealDrawingIndex !== state.revealDrawingIndex || revealStage !== state.revealStage) {
    if (revealStage === "guesses") enqueueInterlude("reveal_guesses");
    if (revealStage === "votes") enqueueInterlude("reveal_votes");
  }
}

function pulseDisplay(tone) {
  if (!displayShell) return;
  displayShell.classList.remove("impact-join", "impact-progress", "impact-all", "impact-phase");
  void displayShell.offsetWidth;
  displayShell.classList.add(tone);
}

function showEventToast(text, toneClass) {
  if (!displayEventFx) return;
  const toast = document.createElement("div");
  toast.className = `display-event-toast ${toneClass}`;
  toast.textContent = text;
  displayEventFx.appendChild(toast);
  requestAnimationFrame(() => {
    toast.classList.add("is-in");
  });
  setTimeout(() => {
    toast.classList.remove("is-in");
    toast.classList.add("is-out");
  }, 1500);
  setTimeout(() => {
    toast.remove();
  }, 2100);
}

function triggerDisplayEvent(text, toneClass, sound, rate = 1, volume = 1) {
  showEventToast(text, toneClass);
  pulseDisplay(toneClass);
  playSfx(sound, rate, volume);
}

function detectProgressEvents(next) {
  if (!state.initialized) {
    return;
  }

  if (next.playerCount > (state.playerCount || 0)) {
    triggerDisplayEvent("Player joined", "impact-join", joinSound, 1.0, 0.95);
  }
  if (state.playerCount !== null && next.playerCount < state.playerCount) {
    triggerDisplayEvent("Player left", "impact-phase", timerEndSound, 1.0, 0.75);
  }

  if (
    state.phase === "drawings" &&
    next.phase === "drawings" &&
    next.drawingSubmittedCount > state.drawingSubmittedCount
  ) {
    triggerDisplayEvent("Drawing submitted", "impact-progress", roundStartSound, 1.15, 0.65);
  }

  if (state.phase === "drawings" && next.phase === "guesses") {
    const timedOut = Boolean(state.phaseEndsAt) && state.phaseEndsAt <= Date.now();
    const text = timedOut ? "Drawing time ended" : "All drawings in";
    triggerDisplayEvent(text, "impact-phase", timerEndSound, 0.95, 0.9);
  }

  if (
    state.phase === "guesses" &&
    next.phase === "guesses" &&
    next.guessSubmittedCount > state.guessSubmittedCount
  ) {
    triggerDisplayEvent("Guess submitted", "impact-progress", joinSound, 1.35, 0.65);
  }

  const allGuessesReached =
    next.phase === "guesses" &&
    next.guessRequiredCount > 0 &&
    next.guessSubmittedCount >= next.guessRequiredCount &&
    state.guessSubmittedCount < state.guessRequiredCount;
  if ((state.phase === "guesses" && next.phase === "guesses-votes") || allGuessesReached) {
    triggerDisplayEvent("All guesses submitted", "impact-all", votingStartSound, 1.0, 0.95);
  }

  if (
    state.phase === "guesses-votes" &&
    next.phase === "guesses-votes" &&
    next.voteSubmittedCount > state.voteSubmittedCount
  ) {
    triggerDisplayEvent("Vote submitted", "impact-progress", joinSound, 0.85, 0.65);
  }

  const allVotesReached =
    next.phase === "guesses-votes" &&
    next.voteRequiredCount > 0 &&
    next.voteSubmittedCount >= next.voteRequiredCount &&
    state.voteSubmittedCount < state.voteRequiredCount;
  if ((state.phase === "guesses-votes" && next.phase === "results") || allVotesReached) {
    triggerDisplayEvent("All votes submitted", "impact-all", roundStartSound, 0.95, 0.9);
  }
}

function handleJokeNarration() {
  if (!jokeNarrationAudio) return;
  if (state.phase !== "results" || state.revealStage !== "joke" || !state.revealJokeAudio) {
    return;
  }
  const key = `${state.revealDrawingIndex}:${state.revealJokeAudio}`;
  if (state.lastJokeNarrationKey === key) {
    return;
  }
  state.lastJokeNarrationKey = key;
  stopAllMusic();
  jokeNarrationAudio.src = state.revealJokeAudio;
  jokeNarrationAudio.currentTime = 0;
  state.jokeNarrationPlaying = true;
  const playPromise = jokeNarrationAudio.play();
  if (playPromise && typeof playPromise.catch === "function") {
    playPromise.catch(() => {
      state.jokeNarrationPlaying = false;
      if (state.connected) {
        syncPhaseAudio(state.phase);
      }
    });
  }
}

function syncFromContent() {
  displayContent = document.getElementById("displayContent");
  if (!displayContent) return;
  const nextPhase = displayContent.dataset.phase || "";
  const revealStage = displayContent.dataset.revealStage || "";
  const revealJokeAudio = displayContent.dataset.revealJokeAudio || "";
  const revealDrawingIndex = Number(displayContent.dataset.revealDrawingIndex || -1);
  const drawingSubmittedCount = Number(displayContent.dataset.drawingSubmittedCount || 0);
  const drawingRequiredCount = Number(displayContent.dataset.drawingRequiredCount || 0);
  const guessSubmittedCount = Number(displayContent.dataset.guessSubmittedCount || 0);
  const guessRequiredCount = Number(displayContent.dataset.guessRequiredCount || 0);
  const voteSubmittedCount = Number(displayContent.dataset.voteSubmittedCount || 0);
  const voteRequiredCount = Number(displayContent.dataset.voteRequiredCount || 0);
  const endsAt = displayContent.dataset.phaseEndsAt ? Date.parse(displayContent.dataset.phaseEndsAt) : 0;
  state.phaseEndsAt = Number.isNaN(endsAt) ? 0 : endsAt;
  const countValue = Number(displayContent.dataset.playerCount || 0);
  const roundValue = Number(displayContent.dataset.round || 0);
  detectProgressEvents({
    phase: nextPhase,
    playerCount: Number.isNaN(countValue) ? state.playerCount || 0 : countValue,
    drawingSubmittedCount: Number.isNaN(drawingSubmittedCount) ? 0 : drawingSubmittedCount,
    drawingRequiredCount: Number.isNaN(drawingRequiredCount) ? 0 : drawingRequiredCount,
    guessSubmittedCount: Number.isNaN(guessSubmittedCount) ? 0 : guessSubmittedCount,
    guessRequiredCount: Number.isNaN(guessRequiredCount) ? 0 : guessRequiredCount,
    voteSubmittedCount: Number.isNaN(voteSubmittedCount) ? 0 : voteSubmittedCount,
    voteRequiredCount: Number.isNaN(voteRequiredCount) ? 0 : voteRequiredCount
  });
  queueTransitionInterludes(nextPhase, revealStage, revealDrawingIndex);
  if (
    roundStartSound &&
    nextPhase === "drawings" &&
    (state.round === null || roundValue > state.round) &&
    roundValue > 0
  ) {
    playAudio(roundStartSound);
  }
  state.phase = nextPhase;
  state.revealStage = revealStage;
  state.revealJokeAudio = revealJokeAudio;
  state.revealDrawingIndex = Number.isNaN(revealDrawingIndex) ? -1 : revealDrawingIndex;
  state.drawingSubmittedCount = Number.isNaN(drawingSubmittedCount) ? 0 : drawingSubmittedCount;
  state.drawingRequiredCount = Number.isNaN(drawingRequiredCount) ? 0 : drawingRequiredCount;
  state.guessSubmittedCount = Number.isNaN(guessSubmittedCount) ? 0 : guessSubmittedCount;
  state.guessRequiredCount = Number.isNaN(guessRequiredCount) ? 0 : guessRequiredCount;
  state.voteSubmittedCount = Number.isNaN(voteSubmittedCount) ? 0 : voteSubmittedCount;
  state.voteRequiredCount = Number.isNaN(voteRequiredCount) ? 0 : voteRequiredCount;
  state.playerCount = Number.isNaN(countValue) ? state.playerCount : countValue;
  state.round = Number.isNaN(roundValue) ? state.round : roundValue;
  state.lastPhase = nextPhase;
  if (!state.initialized) {
    state.initialized = true;
  }
  if (state.connected) {
    syncPhaseAudio(state.phase);
    handleJokeNarration();
    playNextInterlude();
  }
  renderTimer();
  if (!state.timerHandle) {
    state.timerHandle = setInterval(renderTimer, 1000);
  }
}

if (jokeNarrationAudio) {
  jokeNarrationAudio.addEventListener("ended", () => {
    state.jokeNarrationPlaying = false;
    if (state.connected) {
      syncPhaseAudio(state.phase);
    }
  });
  jokeNarrationAudio.addEventListener("error", () => {
    state.jokeNarrationPlaying = false;
    if (state.connected) {
      syncPhaseAudio(state.phase);
    }
  });
}

if (interludeVoiceAudio) {
  interludeVoiceAudio.addEventListener("ended", () => {
    state.interludePlaying = false;
    state.currentInterludeKey = "";
    if (state.connected) {
      syncPhaseAudio(state.phase);
    }
    playNextInterlude();
  });
  interludeVoiceAudio.addEventListener("error", () => {
    state.interludePlaying = false;
    state.currentInterludeKey = "";
    if (state.connected) {
      syncPhaseAudio(state.phase);
    }
    playNextInterlude();
  });
}

document.addEventListener(
  "click",
  () => {
    syncPhaseAudio(state.phase);
    playNextInterlude();
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
    if (jokeNarrationAudio) {
      jokeNarrationAudio.pause();
      jokeNarrationAudio.currentTime = 0;
    }
    if (interludeVoiceAudio) {
      interludeVoiceAudio.pause();
      interludeVoiceAudio.currentTime = 0;
    }
    state.interludeQueue = [];
    state.interludePlaying = false;
    state.currentInterludeKey = "";
    state.jokeNarrationPlaying = false;
    stopAllMusic();
    setTimeout(connectWS, 2000);
  });

  socket.addEventListener("error", () => {
    state.connected = false;
    if (jokeNarrationAudio) {
      jokeNarrationAudio.pause();
      jokeNarrationAudio.currentTime = 0;
    }
    if (interludeVoiceAudio) {
      interludeVoiceAudio.pause();
      interludeVoiceAudio.currentTime = 0;
    }
    state.interludeQueue = [];
    state.interludePlaying = false;
    state.currentInterludeKey = "";
    state.jokeNarrationPlaying = false;
    stopAllMusic();
    socket.close();
  });
}

syncFromContent();
connectWS();
