import {
  fetchSnapshot,
  fetchPrompt,
  postAvatar,
  postDrawing,
  postGuess,
  postStartGame,
  postVote
} from "./player_api.js";
import { applyBrushColor, clearCanvas, setupCanvas } from "./player_canvas.js";
import { updateFromSnapshot } from "./player_view.js";
import { applyHTMLMessage } from "./ws_html.js";

const ctx = {
  els: {
    meta: document.getElementById("playerMeta"),
    joinCode: document.getElementById("joinCode"),
    gameStatus: document.getElementById("gameStatus"),
    playerList: document.getElementById("playerList"),
    playerName: document.getElementById("playerName"),
    playerError: document.getElementById("playerError"),
    scoreboardSection: document.getElementById("scoreboardSection"),
    scoreboardStatus: document.getElementById("scoreboardStatus"),
    scoreboardList: document.getElementById("scoreboardList"),
    drawSection: document.getElementById("drawSection"),
    avatarSection: document.getElementById("avatarSection"),
    avatarCanvas: document.getElementById("avatarCanvas"),
    saveAvatar: document.getElementById("saveAvatar"),
    promptText: document.getElementById("promptText"),
    canvas: document.getElementById("drawCanvas"),
    saveCanvas: document.getElementById("saveCanvas"),
    guessSection: document.getElementById("guessSection"),
    guessStatus: document.getElementById("guessStatus"),
    guessImage: document.getElementById("guessImage"),
    guessForm: document.getElementById("guessForm"),
    guessInput: document.getElementById("guessInput"),
    voteSection: document.getElementById("voteSection"),
    voteStatus: document.getElementById("voteStatus"),
    voteImage: document.getElementById("voteImage"),
    voteForm: document.getElementById("voteForm"),
    voteOptions: document.getElementById("voteOptions"),
    resultsSection: document.getElementById("resultsSection"),
    resultsScores: document.getElementById("resultsScores"),
    resultsList: document.getElementById("resultsList"),
    revealSection: document.getElementById("revealSection"),
    hostSection: document.getElementById("hostSection"),
    hostStartGame: document.getElementById("hostStartGame"),
    hostHelp: document.getElementById("hostHelp")
  },
  state: {
    pollTimer: null,
    assignedPrompt: "",
    currentRound: 0,
    hostId: 0,
    drawingSubmitted: false,
    lastPhase: "",
    showScoreboard: false,
    lastVoteKey: "",
    lastGuessKey: "",
    lastResultsKey: "",
    brushColor: "#1a1a1a",
    canvasCtx: null,
    canvasWidth: 800,
    canvasHeight: 600,
    avatarCtx: null
  },
  actions: {}
};

ctx.actions.applyBrushColor = () => applyBrushColor(ctx);
ctx.actions.applyAvatarColor = () => {
  if (avatarCanvasState && avatarCanvasState.canvasCtx) {
    avatarCanvasState.canvasCtx.strokeStyle = ctx.state.brushColor;
  }
};
ctx.actions.fetchPrompt = () => fetchPromptForPlayer();
ctx.actions.clearCanvas = () => clearCanvas(ctx);

async function loadPlayerView() {
  if (!ctx.els.meta) return;
  const gameId = ctx.els.meta.dataset.gameId;
  const { res, data } = await fetchSnapshot(gameId);
  if (!res.ok) {
    ctx.els.joinCode.textContent = "Unavailable";
    ctx.els.gameStatus.textContent = "Unknown";
    if (ctx.els.playerError) {
      ctx.els.playerError.textContent = data.error || "Unable to load game status.";
    }
    return;
  }
  if (ctx.els.playerError) {
    ctx.els.playerError.textContent = "";
  }
  updateFromSnapshot(ctx, data);
}

function startPolling() {
  if (ctx.state.pollTimer) return;
  ctx.state.pollTimer = setInterval(loadPlayerView, 3000);
}

function connectWS() {
  if (!ctx.els.meta) return;
  const gameId = ctx.els.meta.dataset.gameId;
  const protocol = window.location.protocol === "https:" ? "wss" : "ws";
  const socket = new WebSocket(`${protocol}://${window.location.host}/ws/games/${encodeURIComponent(gameId)}`);

  socket.addEventListener("message", (event) => {
    const htmlResult = applyHTMLMessage(event.data);
    if (htmlResult) {
      return;
    }
    try {
      const data = JSON.parse(event.data);
      updateFromSnapshot(ctx, data);
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

async function fetchPromptForPlayer() {
  if (!ctx.els.meta) return;
  if (ctx.state.assignedPrompt) {
    return;
  }
  const gameId = ctx.els.meta.dataset.gameId;
  const playerId = ctx.els.meta.dataset.playerId;
  const { res, data } = await fetchPrompt(gameId, playerId);
  if (!res.ok) {
    if (ctx.els.playerError) {
      ctx.els.playerError.textContent = data.error || "Unable to load prompt.";
    }
    return;
  }
  ctx.state.assignedPrompt = data.prompt || "";
  if (!ctx.state.assignedPrompt) {
    if (ctx.els.playerError) {
      ctx.els.playerError.textContent = "No prompts assigned.";
    }
    return;
  }
  if (ctx.els.promptText) {
    ctx.els.promptText.textContent = ctx.state.assignedPrompt;
  }
}

let avatarCanvasState = null;
if (ctx.els.avatarCanvas) {
  avatarCanvasState = {
    canvasCtx: null,
    canvasWidth: 800,
    canvasHeight: 600,
    brushColor: "#1a1a1a"
  };
  setupCanvas(
    {
      els: { canvas: ctx.els.avatarCanvas },
      state: avatarCanvasState
    },
    () => {}
  );
}

if (ctx.els.saveAvatar) {
  ctx.els.saveAvatar.addEventListener("click", async () => {
    if (!ctx.els.meta || !ctx.els.avatarCanvas) return;
    const gameId = ctx.els.meta.dataset.gameId;
    const playerId = Number(ctx.els.meta.dataset.playerId);
    const avatarData = ctx.els.avatarCanvas.toDataURL("image/png");
    const { res, data } = await postAvatar(gameId, playerId, avatarData);
    if (!res.ok) {
      if (ctx.els.playerError) {
        ctx.els.playerError.textContent = data.error || "Unable to save avatar.";
      }
      return;
    }
    if (ctx.els.playerError) {
      ctx.els.playerError.textContent = "";
    }
    updateFromSnapshot(ctx, data);
  });
}

if (ctx.els.hostStartGame) {
  ctx.els.hostStartGame.addEventListener("click", async () => {
    if (!ctx.els.meta) return;
    const gameId = ctx.els.meta.dataset.gameId;
    const playerId = Number(ctx.els.meta.dataset.playerId);
    const { res, data } = await postStartGame(gameId, playerId);
    if (!res.ok) {
      if (ctx.els.playerError) {
        ctx.els.playerError.textContent = data.error || "Unable to start game.";
      }
      return;
    }
    if (ctx.els.playerError) {
      ctx.els.playerError.textContent = "";
    }
    updateFromSnapshot(ctx, data);
  });
}

if (ctx.els.guessForm) {
  ctx.els.guessForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    if (!ctx.els.meta || !ctx.els.guessInput) return;
    const guess = ctx.els.guessInput.value.trim();
    if (!guess) {
      return;
    }
    const gameId = ctx.els.meta.dataset.gameId;
    const playerId = Number(ctx.els.meta.dataset.playerId);
    const { res, data } = await postGuess(gameId, playerId, guess);
    if (!res.ok) {
      if (ctx.els.playerError) {
        ctx.els.playerError.textContent = data.error || "Unable to submit guess.";
      }
      return;
    }
    if (ctx.els.playerError) {
      ctx.els.playerError.textContent = "";
    }
    updateFromSnapshot(ctx, data);
  });
}

if (ctx.els.voteForm) {
  ctx.els.voteForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    if (!ctx.els.meta || !ctx.els.voteOptions) return;
    const selected = ctx.els.voteOptions.querySelector("input[name='voteOption']:checked");
    if (!selected) {
      if (ctx.els.playerError) {
        ctx.els.playerError.textContent = "Choose an option before voting.";
      }
      return;
    }
    const gameId = ctx.els.meta.dataset.gameId;
    const playerId = Number(ctx.els.meta.dataset.playerId);
    const { res, data } = await postVote(gameId, playerId, selected.value);
    if (!res.ok) {
      if (ctx.els.playerError) {
        ctx.els.playerError.textContent = data.error || "Unable to submit vote.";
      }
      return;
    }
    if (ctx.els.playerError) {
      ctx.els.playerError.textContent = "";
    }
    updateFromSnapshot(ctx, data);
  });
}

setupCanvas(ctx, async (dataUrl) => {
  if (!ctx.els.meta) return;
  const gameId = ctx.els.meta.dataset.gameId;
  const playerId = Number(ctx.els.meta.dataset.playerId);
  const { res, data } = await postDrawing(gameId, playerId, dataUrl, ctx.state.assignedPrompt);
  if (!res.ok) {
    if (ctx.els.playerError) {
      ctx.els.playerError.textContent = data.error || "Unable to submit drawing.";
    }
    return;
  }
  if (ctx.els.playerError) {
    ctx.els.playerError.textContent = "";
  }
  ctx.state.drawingSubmitted = true;
  if (ctx.els.drawSection) {
    ctx.els.drawSection.style.display = "none";
  }
  updateFromSnapshot(ctx, data);
});

loadPlayerView();
connectWS();
