export function updateFromSnapshot(ctx, data) {
  const { els, state } = ctx;
  const phase = data.phase || "unknown";
  els.joinCode.textContent = data.join_code || "Unavailable";
  els.gameStatus.textContent = phase;
  state.hostId = data.host_id || 0;
  if (els.startGame) {
    els.startGame.style.display = phase === "lobby" ? "inline-flex" : "none";
    els.startGame.disabled = phase === "lobby" ? (data.players?.length || 0) < 2 : true;
  }
  if (els.endGame) {
    els.endGame.style.display = phase !== "complete" ? "inline-flex" : "none";
  }
  if (els.hostPanel) {
    els.hostPanel.style.display = phase === "lobby" ? "grid" : "none";
  }

  els.playerList.innerHTML = "";
  if (els.playerActions) {
    els.playerActions.innerHTML = "";
  }
  const players = Array.isArray(data.players) ? data.players : [];
  if (players.length === 0) {
    const item = document.createElement("li");
    item.textContent = "No players yet";
    els.playerList.appendChild(item);
    return;
  }
  const playerIDs = Array.isArray(data.player_ids) ? data.player_ids : [];
  const avatarMap = data.player_avatars || {};
  players.forEach((player, index) => {
    const item = document.createElement("li");
    item.className = "player-entry";
    const playerName = player;
    const isHost = playerIDs[index] === state.hostId;
    const nameText = isHost ? `${playerName}*` : playerName;
    const avatarSrc = avatarMap[String(playerIDs[index] || "")] || "";
    if (avatarSrc) {
      const avatar = document.createElement("img");
      avatar.className = "player-avatar";
      avatar.alt = `${playerName} avatar`;
      avatar.src = avatarSrc;
      item.appendChild(avatar);
    }
    const name = document.createElement("span");
    name.textContent = nameText;
    item.appendChild(name);
    els.playerList.appendChild(item);

    if (els.playerActions) {
      const row = document.createElement("div");
      row.className = "player-action-row";
      const label = document.createElement("span");
      const isHost = playerIDs[index] === state.hostId;
      label.textContent = nameText;
      const kickButton = document.createElement("button");
      kickButton.type = "button";
      kickButton.className = "secondary";
      kickButton.textContent = "Remove";
      kickButton.dataset.playerId = String(playerIDs[index] || 0);
      if (playerIDs[index] === state.hostId) {
        kickButton.disabled = true;
      }
      row.appendChild(label);
      row.appendChild(kickButton);
      els.playerActions.appendChild(row);
    }
  });

  if (els.roundsInput) {
    els.roundsInput.value = data.total_rounds || data.prompts_per_player || 2;
  }
  if (els.maxPlayersInput) {
    els.maxPlayersInput.value = data.max_players || 0;
  }
  if (els.promptCategory) {
    els.promptCategory.value = data.prompt_category || "";
  }
  if (els.lobbyLocked) {
    els.lobbyLocked.checked = Boolean(data.lobby_locked);
  }
  if (els.lobbyStatus) {
    const maxPlayers = data.max_players > 0 ? data.max_players : "∞";
    const lockedText = data.lobby_locked ? "Locked" : "Open";
    els.lobbyStatus.textContent = `Players: ${players.length}/${maxPlayers}. ${lockedText} lobby.`;
  }
  if (els.hostStatus) {
    if (phase !== "lobby") {
      els.hostStatus.textContent = "Host controls are disabled once the game begins.";
    } else if (players.length < 2) {
      els.hostStatus.textContent = "Waiting for at least two players to join.";
    } else {
      els.hostStatus.textContent = "Ready to start when everyone is here.";
    }
  }
  if (els.settingsForm) {
    const disabled = phase !== "lobby" || !state.hostId;
    Array.from(els.settingsForm.elements).forEach((el) => {
      if (el.tagName === "BUTTON") return;
      el.disabled = disabled;
    });
    const submitButton = els.settingsForm.querySelector("button");
    if (submitButton) {
      submitButton.disabled = disabled;
    }
  }

  if (els.playerActions) {
    const disabled = phase !== "lobby";
    Array.from(els.playerActions.querySelectorAll("button")).forEach((button) => {
      button.disabled = disabled;
    });
  }

  updateDisplayHeader(ctx, data);
  updateDisplayStage(ctx, data, phase);
  updateDisplayScores(ctx, data, phase);
}

function updateDisplayHeader(ctx, data) {
  const { els } = ctx;
  if (els.displayRound) {
    const totalRounds = data.total_rounds || data.prompts_per_player || 0;
    const round = data.current_round || 0;
    if (round && totalRounds) {
      els.displayRound.textContent = `Round ${round} of ${totalRounds}`;
    } else {
      els.displayRound.textContent = "--";
    }
  }
}

function updateDisplayStage(ctx, data, phase) {
  const { els } = ctx;
  const guessTurn = data.guess_turn || null;
  const voteTurn = data.vote_turn || null;
  const names = buildNameMap(data);
  if (!els.displayStageTitle || !els.displayStageStatus) return;

  if (phase === "lobby") {
    setStage(els, "Waiting for players", "Share the join code so everyone can join.", "");
    setOptions(els, []);
    return;
  }

  if (phase === "drawings") {
    setStage(els, "Drawing round", "Players are drawing their prompts.", "");
    setOptions(els, []);
    return;
  }

  if (phase === "guesses") {
    const drawingImage = guessTurn ? guessTurn.drawing_image : "";
    const ownerName = guessTurn ? names[guessTurn.drawing_owner] : "";
    const guesserName = guessTurn ? names[guessTurn.guesser_id] : "";
    const status = ownerName
      ? `Guessing the prompt for ${ownerName}'s drawing.`
      : "Guessing in progress.";
    setStage(els, guesserName ? `Guessing: ${guesserName}` : "Guessing prompts", status, drawingImage);
    setOptions(els, []);
    return;
  }

  if (phase === "guesses-votes") {
    const drawingImage = voteTurn ? voteTurn.drawing_image : "";
    const ownerName = voteTurn ? names[voteTurn.drawing_owner] : "";
    const status = ownerName ? `Vote on the real prompt for ${ownerName}'s drawing.` : "Voting on prompts.";
    setStage(els, "Vote for the real prompt", status, drawingImage);
    setOptions(els, voteTurn ? voteTurn.options : []);
    return;
  }

  if (phase === "results") {
    const reveal = data.reveal || null;
    if (reveal && reveal.drawing_image) {
      setStage(els, "Round results", "Reviewing answers and votes.", reveal.drawing_image);
    } else {
      setStage(els, "Round results", "Reviewing answers and votes.", "");
    }
    setOptions(els, []);
    return;
  }

  if (phase === "complete") {
    setStage(els, "Game complete", "Thanks for playing!", "");
    setOptions(els, []);
  }
}

function updateDisplayScores(ctx, data, phase) {
  const { els } = ctx;
  const scores = Array.isArray(data.scores) ? data.scores : [];
  const drawingsCount = data.counts ? data.counts.drawings : 0;
  const betweenRounds = phase === "drawings" && (data.current_round || 0) > 1 && drawingsCount === 0;

  if (els.displayScoreboard) {
    els.displayScoreboard.style.display = betweenRounds ? "grid" : "none";
  }
  if (betweenRounds) {
    if (els.displayScoreTitle) {
      els.displayScoreTitle.textContent = "Scoreboard";
    }
    if (els.displayScoreStatus) {
      els.displayScoreStatus.textContent = "Current standings after the last round.";
    }
    if (els.displayScoreList) {
      renderScoreList(els.displayScoreList, scores);
    }
  }

  const showFinal = phase === "results" || phase === "complete";
  if (els.displayFinalScores) {
    els.displayFinalScores.style.display = showFinal ? "grid" : "none";
  }
  if (showFinal && els.displayFinalList) {
    renderScoreList(els.displayFinalList, scores);
  }
}

function setStage(els, title, status, imageUrl) {
  if (els.displayStageTitle) {
    els.displayStageTitle.textContent = title;
  }
  if (els.displayStageStatus) {
    els.displayStageStatus.textContent = status;
  }
  if (els.displayStageImage) {
    if (imageUrl) {
      els.displayStageImage.src = imageUrl;
      els.displayStageImage.style.display = "block";
    } else {
      els.displayStageImage.style.display = "none";
    }
  }
}

function setOptions(els, options) {
  if (!els.displayOptions) return;
  els.displayOptions.innerHTML = "";
  if (!Array.isArray(options) || options.length === 0) {
    return;
  }
  const list = document.createElement("ul");
  list.className = "display-option-list";
  options.forEach((option) => {
    const item = document.createElement("li");
    item.textContent = option;
    list.appendChild(item);
  });
  els.displayOptions.appendChild(list);
}

function renderScoreList(container, scores) {
  container.innerHTML = "";
  if (!Array.isArray(scores) || scores.length === 0) {
    const note = document.createElement("p");
    note.className = "hint";
    note.textContent = "Scores will appear here once results are available.";
    container.appendChild(note);
    return;
  }
  const list = document.createElement("ul");
  list.className = "score-list";
  scores.forEach((entry) => {
    const item = document.createElement("li");
    item.textContent = `${entry.player_name || "Player"}: ${entry.score}`;
    list.appendChild(item);
  });
  container.appendChild(list);
}

function buildNameMap(data) {
  const players = Array.isArray(data.players) ? data.players : [];
  const ids = Array.isArray(data.player_ids) ? data.player_ids : [];
  const map = {};
  players.forEach((player, index) => {
    map[ids[index]] = player;
  });
  return map;
}
