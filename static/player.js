import {
  fetchSnapshot,
  fetchPrompt,
  postAdvance,
  postAvatar,
  postEndGame,
  postDrawing,
  postGuess,
	postLike,
  postKick,
  postSettings,
  postStartGame,
  postVote
} from "./player_api.js";
import { applyBrushColor, clearCanvas, setupCanvas } from "./player_canvas.js";
import { createPhaseTimer, createPolling, createReconnect, formatTime } from "./realtime.js";
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
    avatarCanvasWrap: document.getElementById("avatarCanvasWrap"),
    avatarCanvas: document.getElementById("avatarCanvas"),
    avatarLockedHint: document.getElementById("avatarLockedHint"),
    saveAvatar: document.getElementById("saveAvatar"),
    avatarSavedSound: document.getElementById("avatarSavedSound"),
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
    hostAdvanceGame: document.getElementById("hostAdvanceGame"),
    hostEndGame: document.getElementById("hostEndGame"),
    hostHelp: document.getElementById("hostHelp"),
    hostLobbyStatus: document.getElementById("hostLobbyStatus"),
    hostSettingsForm: document.getElementById("hostSettingsForm"),
    hostRoundsInput: document.getElementById("hostRoundsInput"),
    hostLobbyLocked: document.getElementById("hostLobbyLocked"),
		hostAvatarsEnabled: document.getElementById("hostAvatarsEnabled"),
		hostAudienceEnabled: document.getElementById("hostAudienceEnabled"),
		hostJokesEnabled: document.getElementById("hostJokesEnabled"),
		hostPublicReplay: document.getElementById("hostPublicReplay"),
    hostSettingsStatus: document.getElementById("hostSettingsStatus"),
    hostPlayerActions: document.getElementById("hostPlayerActions"),
    phaseTimer: document.getElementById("phaseTimer")
  },
  state: {
    assignedPrompt: "",
    currentRound: 0,
    hostId: 0,
    drawingSubmitted: false,
    lastPhase: "",
    lastVoteKey: "",
    lastGuessKey: "",
    lastResultsKey: "",
    brushColor: "#1a1a1a",
    brushSize: 4,
    canvasCtx: null,
    canvasWidth: 800,
    canvasHeight: 600,
    avatarCtx: null,
    timerEndsAt: 0,
    authToken: "",
    avatarLocked: false,
    wsConn: null,
    unloading: false,
    gameMissing: false
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
const phaseTimer = createPhaseTimer((endsAt) => {
  ctx.state.timerEndsAt = endsAt;
  renderTimer();
});
const polling = createPolling(() => {
  loadPlayerView();
}, 3000);
const wsReconnect = createReconnect(() => {
  connectWS();
}, {
  baseDelayMs: 1000,
  exponential: true,
  maxDelayMs: 15000,
  maxExponent: 5
});

function playAudio(audio) {
  if (!audio) return;
  const playPromise = audio.play();
  if (playPromise && typeof playPromise.catch === "function") {
    playPromise.catch(() => {
      // Ignore autoplay failures.
    });
  }
}

function renderTimer() {
  if (!ctx.els.phaseTimer) return;
  if (!ctx.state.timerEndsAt) {
    ctx.els.phaseTimer.textContent = "--:--";
    return;
  }
  const remaining = Math.max(0, Math.round((ctx.state.timerEndsAt - Date.now()) / 1000));
  ctx.els.phaseTimer.textContent = formatTime(remaining);
}

function syncTimer(data) {
  phaseTimer.setEndsAt(data.phase_ends_at || "");
}

function markGameMissing() {
  if (ctx.state.gameMissing) {
    return;
  }
  ctx.state.gameMissing = true;
  polling.stop();
  wsReconnect.clear();
  if (ctx.state.wsConn) {
    const socket = ctx.state.wsConn;
    ctx.state.wsConn = null;
    socket.close();
  }
  if (ctx.els.joinCode) {
    ctx.els.joinCode.textContent = "Unavailable";
  }
  if (ctx.els.gameStatus) {
    ctx.els.gameStatus.textContent = "game not found";
  }
  if (ctx.els.playerError) {
    ctx.els.playerError.textContent = "game not found";
  }
}

async function loadPlayerView() {
  if (!ctx.els.meta) return;
  if (ctx.state.gameMissing) return;
  const gameId = ctx.els.meta.dataset.gameId;
  const playerId = ctx.els.meta.dataset.playerId;
  if (!ctx.state.authToken) {
    ctx.state.authToken = localStorage.getItem(`pt_auth_${gameId}_${playerId}`) || "";
  }
	const { res, data } = await fetchSnapshot(gameId, playerId);
  if (!res.ok) {
    if (res.status === 404) {
      markGameMissing();
      return;
    }
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
  syncTimer(data);
}

async function handleWSDisconnect(socket) {
  if (ctx.state.wsConn === socket) {
    ctx.state.wsConn = null;
  }
  if (ctx.state.unloading || ctx.state.gameMissing) {
    return;
  }
  await loadPlayerView();
  if (ctx.state.gameMissing) {
    return;
  }
  polling.start();
  wsReconnect.schedule(() => !ctx.state.unloading && !ctx.state.gameMissing && Boolean(ctx.els.meta));
}

function connectWS() {
  if (!ctx.els.meta || ctx.state.gameMissing) return;
  const existing = ctx.state.wsConn;
  if (existing && (existing.readyState === WebSocket.OPEN || existing.readyState === WebSocket.CONNECTING)) {
    return;
  }
  const gameId = ctx.els.meta.dataset.gameId;
  const protocol = window.location.protocol === "https:" ? "wss" : "ws";
  const socket = new WebSocket(`${protocol}://${window.location.host}/ws/games/${encodeURIComponent(gameId)}`);
  ctx.state.wsConn = socket;

  socket.addEventListener("open", () => {
    if (ctx.state.wsConn !== socket) {
      return;
    }
    wsReconnect.reset();
    polling.stop();
    loadPlayerView();
  });

	socket.addEventListener("message", (event) => {
    const htmlResult = applyHTMLMessage(event.data);
    if (htmlResult) {
      return;
    }
		try {
			const data = JSON.parse(event.data);
			if (data.type === "state_changed") {
				loadPlayerView();
				return;
			}
      updateFromSnapshot(ctx, data);
      syncTimer(data);
    } catch {
      // ignore invalid payloads
    }
  });

  socket.addEventListener("close", () => {
    handleWSDisconnect(socket);
  });

  socket.addEventListener("error", () => {
    handleWSDisconnect(socket);
  });
}

async function fetchPromptForPlayer() {
  if (!ctx.els.meta) return;
  if (ctx.state.gameMissing) return;
  if (ctx.state.assignedPrompt) {
    return;
  }
  const gameId = ctx.els.meta.dataset.gameId;
  const playerId = ctx.els.meta.dataset.playerId;
  const { res, data } = await fetchPrompt(gameId, playerId);
  if (!res.ok) {
    if (res.status === 404) {
      markGameMissing();
      return;
    }
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
    brushColor: "#1a1a1a",
    brushSize: 12
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
    if (ctx.state.avatarLocked) {
      return;
    }
    if (!ctx.els.meta || !ctx.els.avatarCanvas) return;
    const gameId = ctx.els.meta.dataset.gameId;
    const playerId = Number(ctx.els.meta.dataset.playerId);
    const avatarData = ctx.els.avatarCanvas.toDataURL("image/png");
    const { res, data } = await postAvatar(gameId, playerId, avatarData);
    if (!res.ok) {
      const avatarLocked = res.status === 409 && typeof data.error === "string" && data.error.includes("locked");
      if (avatarLocked) {
        ctx.state.avatarLocked = true;
        if (ctx.els.saveAvatar) {
          ctx.els.saveAvatar.style.display = "none";
          ctx.els.saveAvatar.disabled = true;
        }
      }
      if (ctx.els.playerError) {
        ctx.els.playerError.textContent = data.error || "Unable to save avatar.";
      }
      return;
    }
    if (ctx.els.playerError) {
      ctx.els.playerError.textContent = "";
    }
    ctx.state.avatarLocked = true;
    if (ctx.els.saveAvatar) {
      ctx.els.saveAvatar.style.display = "none";
      ctx.els.saveAvatar.disabled = true;
    }
    playAudio(ctx.els.avatarSavedSound);
    updateFromSnapshot(ctx, data);
  });
}

if (ctx.els.hostStartGame) {
  ctx.els.hostStartGame.addEventListener("click", async () => {
    if (!ctx.els.meta) return;
    const gameId = ctx.els.meta.dataset.gameId;
    const playerId = Number(ctx.els.meta.dataset.playerId);
    const { res, data } = await postStartGame(gameId, playerId, ctx.state.authToken);
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

if (ctx.els.hostAdvanceGame) {
  ctx.els.hostAdvanceGame.addEventListener("click", async () => {
    if (!ctx.els.meta) return;
    const gameId = ctx.els.meta.dataset.gameId;
    const playerId = Number(ctx.els.meta.dataset.playerId);
    const { res, data } = await postAdvance(gameId, playerId, ctx.state.authToken);
    if (!res.ok) {
      if (ctx.els.playerError) {
        ctx.els.playerError.textContent = data.error || "Unable to advance game.";
      }
      return;
    }
    if (ctx.els.playerError) {
      ctx.els.playerError.textContent = "";
    }
    updateFromSnapshot(ctx, data);
  });
}

if (ctx.els.hostEndGame) {
  ctx.els.hostEndGame.addEventListener("click", async () => {
    if (!ctx.els.meta) return;
    const gameId = ctx.els.meta.dataset.gameId;
    const playerId = Number(ctx.els.meta.dataset.playerId);
    const { res, data } = await postEndGame(gameId, playerId, ctx.state.authToken);
    if (!res.ok) {
      if (ctx.els.playerError) {
        ctx.els.playerError.textContent = data.error || "Unable to end game.";
      }
      return;
    }
    if (ctx.els.playerError) {
      ctx.els.playerError.textContent = "";
    }
    updateFromSnapshot(ctx, data);
  });
}

if (ctx.els.hostSettingsForm) {
  ctx.els.hostSettingsForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    if (!ctx.els.meta) return;
    const gameId = ctx.els.meta.dataset.gameId;
    const playerId = Number(ctx.els.meta.dataset.playerId);
    const rounds = Number(ctx.els.hostRoundsInput?.value || 0);
    const locked = Boolean(ctx.els.hostLobbyLocked?.checked);
    if (ctx.els.hostSettingsStatus) {
      ctx.els.hostSettingsStatus.textContent = "Saving...";
    }
    const { res, data } = await postSettings(gameId, {
      player_id: playerId,
      auth_token: ctx.state.authToken,
      rounds,
			lobby_locked: locked,
			avatars_enabled: Boolean(ctx.els.hostAvatarsEnabled?.checked),
			audience_enabled: Boolean(ctx.els.hostAudienceEnabled?.checked),
			jokes_enabled: Boolean(ctx.els.hostJokesEnabled?.checked),
			public_replay: Boolean(ctx.els.hostPublicReplay?.checked)
    });
    if (!res.ok) {
      if (ctx.els.hostSettingsStatus) {
        ctx.els.hostSettingsStatus.textContent = data.error || "Unable to save settings.";
      }
      return;
    }
    if (ctx.els.hostSettingsStatus) {
      ctx.els.hostSettingsStatus.textContent = "Settings saved.";
    }
    if (ctx.els.playerError) {
      ctx.els.playerError.textContent = "";
    }
    updateFromSnapshot(ctx, data);
  });
}

if (ctx.els.hostPlayerActions) {
  ctx.els.hostPlayerActions.addEventListener("click", async (event) => {
    const target = event.target;
    if (!target || target.tagName !== "BUTTON") {
      return;
    }
    if (target.disabled || !ctx.els.meta) {
      return;
    }
    const targetID = Number(target.dataset.playerId || 0);
    if (!targetID) {
      return;
    }
    const gameId = ctx.els.meta.dataset.gameId;
    const playerId = Number(ctx.els.meta.dataset.playerId);
    const { res, data } = await postKick(gameId, {
      player_id: playerId,
      auth_token: ctx.state.authToken,
      target_id: targetID
    });
    if (!res.ok) {
      if (ctx.els.playerError) {
        ctx.els.playerError.textContent = data.error || "Unable to remove player.";
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
	ctx.els.voteOptions?.addEventListener("click", async (event) => {
		const button = event.target.closest?.("button[data-like-choice]");
		if (!button || !ctx.els.meta) return;
		button.disabled = true;
		const gameId = ctx.els.meta.dataset.gameId;
		const playerId = Number(ctx.els.meta.dataset.playerId);
		const { res, data } = await postLike(gameId, playerId, Number(button.dataset.drawingIndex), button.dataset.likeChoice);
		if (!res.ok && ctx.els.playerError) ctx.els.playerError.textContent = data.error || "Unable to like title.";
	});
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
    const { res, data } = await postVote(gameId, playerId, {
      choice_id: selected.value,
      choice: selected.dataset.choiceText || ""
    });
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

document.addEventListener("visibilitychange", () => {
  if (ctx.state.gameMissing) {
    return;
  }
  if (document.visibilityState !== "visible") {
    return;
  }
  loadPlayerView();
  connectWS();
});

window.addEventListener("online", () => {
  if (ctx.state.gameMissing) {
    return;
  }
  loadPlayerView();
  connectWS();
});

window.addEventListener("beforeunload", () => {
  ctx.state.unloading = true;
  wsReconnect.clear();
  polling.stop();
  if (ctx.state.wsConn) {
    ctx.state.wsConn.close();
    ctx.state.wsConn = null;
  }
});

loadPlayerView();
connectWS();
