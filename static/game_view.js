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
  if (els.advanceGame) {
    const canAdvance = phase !== "lobby" && phase !== "complete";
    els.advanceGame.style.display = canAdvance ? "inline-flex" : "none";
    els.advanceGame.disabled = !canAdvance;
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
      row.className = "player-action-row card-surface";
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
      els.hostStatus.textContent = "Use Advance to move to the next stage when everyone is ready.";
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
  const guessAssignments = Array.isArray(data.guess_assignments) ? data.guess_assignments : [];
  const voteAssignments = Array.isArray(data.vote_assignments) ? data.vote_assignments : [];
  const guessTurn = data.guess_focus || (guessAssignments.length > 0 ? guessAssignments[0] : null);
  const voteTurn = data.vote_focus || (voteAssignments.length > 0 ? voteAssignments[0] : null);
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
    const required = Number(data.guess_required_count || 0);
    const submitted = Number(data.guess_submitted_count || 0);
    let status = ownerName
      ? `Collecting guesses for ${ownerName}'s drawing.`
      : "Collecting guesses.";
    if (required > 0) {
      status += ` (${submitted}/${required} submitted)`;
    }
    if (guessAssignments.length > 0) {
      const pendingForOne = Array.isArray(guessTurn.pending_for_one) ? guessTurn.pending_for_one.length : 0;
      if (pendingForOne > 0) {
        status += ` ${pendingForOne} left on this drawing.`;
      }
    }
    setStage(els, "Guessing prompts", status, drawingImage);
    setOptions(els, []);
    return;
  }

  if (phase === "guesses-votes") {
    const drawingImage = voteTurn ? voteTurn.drawing_image : "";
    const ownerName = voteTurn ? names[voteTurn.drawing_owner] : "";
    const required = Number(data.vote_required_count || 0);
    const submitted = Number(data.vote_submitted_count || 0);
    let status = ownerName ? `Vote on the real prompt for ${ownerName}'s drawing.` : "Voting on prompts.";
    if (required > 0) {
      status += ` (${submitted}/${required} submitted)`;
    }
    if (voteAssignments.length > 0) {
      const pendingForOne = Array.isArray(voteTurn.pending_for_one) ? voteTurn.pending_for_one.length : 0;
      if (pendingForOne > 0) {
        status += ` ${pendingForOne} left on this drawing.`;
      }
    }
    setStage(els, "Vote for the real prompt", status, drawingImage);
    setOptions(els, voteTurn ? voteTurn.options : []);
    return;
  }

  if (phase === "results") {
    const reveal = data.reveal || null;
    let status = "Reviewing answers and votes.";
    if (reveal && reveal.stage === "guesses") {
      status = "Revealing guesses.";
    } else if (reveal && reveal.stage === "votes") {
      status = "Revealing votes.";
    } else if (reveal && reveal.stage === "joke") {
      status = "Narrator is reading the joke.";
    }
    if (reveal && reveal.drawing_image) {
      setStage(els, "Drawing results", status, reveal.drawing_image);
    } else {
      setStage(els, "Drawing results", status, "");
    }
    setOptions(els, revealOptions(reveal));
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

  const showFinal = phase === "complete";
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
    item.className = "card-surface";
    const text = option && typeof option === "object" ? option.text : option;
    item.textContent = text || "";
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

function revealOptions(reveal) {
  if (!reveal) return [];
  const lines = [];
  if (reveal.stage === "guesses") {
    const guesses = Array.isArray(reveal.guesses) ? reveal.guesses : [];
    guesses.forEach((guess) => {
      lines.push(`${guess.player_name || "Player"}: ${guess.text || ""}`);
    });
  } else {
    const options = Array.isArray(reveal.options) ? reveal.options : [];
    if (options.length > 0) {
      options.forEach((option) => {
        const type = option.type || "";
        const text = option.text || "";
        const owner = option.owner_name || "Player";
        if (type === "prompt") {
          lines.push(`Prompt: ${text}`);
        } else {
          lines.push(`${owner} wrote: ${text}`);
        }
        const playerVotes = Array.isArray(option.player_votes) ? option.player_votes : [];
        if (playerVotes.length > 0) {
          const voters = playerVotes.map((vote) => vote.player_name || "Player");
          lines.push(`Picked by: ${voters.join(", ")}`);
        }
        const audienceCount = Number(option.audience_count || 0);
        if (audienceCount > 0) {
          lines.push(`Audience picks: ${audienceCount}`);
        }
      });
    } else {
      if (reveal.prompt) {
        lines.push(`Prompt: ${reveal.prompt}`);
      }
      const votes = Array.isArray(reveal.votes) ? reveal.votes : [];
      votes.forEach((vote) => {
        lines.push(`${vote.player_name || "Player"}: ${vote.text || ""}`);
      });
      const audienceVotes = Array.isArray(reveal.audience_votes) ? reveal.audience_votes : [];
      audienceVotes.forEach((vote) => {
        lines.push(`Audience: ${vote.text || ""} (${vote.count || 0})`);
      });
    }
    if (reveal.stage === "joke" && reveal.joke) {
      lines.push(`Joke: ${reveal.joke}`);
    }
  }
  const deltas = Array.isArray(reveal.score_deltas) ? reveal.score_deltas : [];
  if (deltas.length > 0) {
    lines.push("Score changes:");
    deltas.forEach((entry) => {
      lines.push(`${entry.player_name || "Player"}: +${entry.delta || 0}`);
    });
  }
  return lines;
}
