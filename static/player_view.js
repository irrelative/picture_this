function normalizePhase(phase) {
  if (phase === "votes") {
    return "guesses-votes";
  }
  return phase;
}

export function updateFromSnapshot(ctx, data) {
  const { els, state, actions } = ctx;
  const phase = normalizePhase(data.phase);
  els.joinCode.textContent = data.join_code || "Unavailable";
  els.gameStatus.textContent = phase || "Unknown";
  els.playerList.innerHTML = "";
  const players = Array.isArray(data.players) ? data.players : [];
  if (players.length === 0) {
    const item = document.createElement("li");
    item.textContent = "No players yet";
    els.playerList.appendChild(item);
    return;
  }
  const colorMap = data.player_colors || {};
  const avatarMap = data.player_avatars || {};
  const avatarLocks = data.player_avatar_locks || {};
  const playerIDs = Array.isArray(data.player_ids) ? data.player_ids : [];
  players.forEach((player, index) => {
    const item = document.createElement("li");
    item.className = "player-entry";
    const dot = document.createElement("span");
    dot.className = "player-dot";
    const colorKey = String(playerIDs[index] || "");
    if (colorMap && colorMap[colorKey]) {
      dot.style.backgroundColor = colorMap[colorKey];
    }
    const avatarSrc = avatarMap[colorKey] || "";
    if (avatarSrc) {
      const avatar = document.createElement("img");
      avatar.className = "player-avatar";
      avatar.alt = `${player} avatar`;
      avatar.src = avatarSrc;
      if (colorMap && colorMap[colorKey]) {
        avatar.style.borderColor = colorMap[colorKey];
      }
      item.appendChild(avatar);
    } else {
      item.appendChild(dot);
    }
    const name = document.createElement("span");
    name.textContent = player;
    item.appendChild(name);
    els.playerList.appendChild(item);
  });

  const roundNumber = data.current_round || 0;
  if (roundNumber && roundNumber !== state.currentRound) {
    state.currentRound = roundNumber;
    state.drawingSubmitted = false;
    state.assignedPrompt = "";
    if (els.promptText) {
      els.promptText.textContent = "Loading...";
    }
    if (actions.clearCanvas) {
      actions.clearCanvas();
    }
  }

  state.lastPhase = phase;

  state.hostId = data.host_id || 0;
  const playerId = Number(els.meta?.dataset.playerId || 0);
  const playerNameValue = els.meta?.dataset.playerName || "";
  const isHost = playerId !== 0 && playerId === state.hostId;
  if (els.playerName && playerNameValue) {
    if (isHost) {
      els.playerName.textContent = `Signed in as ${playerNameValue}. You're the host.`;
    } else {
      els.playerName.textContent = `Signed in as ${playerNameValue}. Waiting for the host to begin.`;
    }
  }
  if (els.hostSection) {
    els.hostSection.style.display = isHost ? "grid" : "none";
  }
  if (els.hostStartGame) {
    const enoughPlayers = players.length >= 2;
    const canStart = phase === "lobby" && isHost && enoughPlayers;
    els.hostStartGame.disabled = !canStart;
    els.hostStartGame.style.display = isHost ? "inline-flex" : "none";
    if (els.hostAdvanceGame) {
      const canAdvance = phase !== "lobby" && phase !== "complete" && isHost;
      els.hostAdvanceGame.disabled = !canAdvance;
      els.hostAdvanceGame.style.display = isHost ? "inline-flex" : "none";
    }
    if (els.hostHelp) {
      if (!isHost) {
        els.hostHelp.textContent = "Only the host can control game flow.";
      } else if (phase === "complete") {
        els.hostHelp.textContent = "Game complete.";
      } else if (phase !== "lobby") {
        els.hostHelp.textContent = "Use Advance when everyone is ready for the next step.";
      } else if (!enoughPlayers) {
        els.hostHelp.textContent = "Waiting for at least two players to join.";
      } else {
        els.hostHelp.textContent = "All players are in. Start when ready.";
      }
    }
  }
  if (els.hostEndGame) {
    const canEnd = isHost && phase !== "complete";
    els.hostEndGame.disabled = !canEnd;
    els.hostEndGame.style.display = isHost ? "inline-flex" : "none";
  }
  if (els.hostLobbyStatus) {
    const maxPlayers = data.max_players > 0 ? data.max_players : "∞";
    const lockedText = data.lobby_locked ? "Locked" : "Open";
    els.hostLobbyStatus.textContent = `Players: ${players.length}/${maxPlayers}. ${lockedText} lobby.`;
  }
  if (els.hostRoundsInput) {
    els.hostRoundsInput.value = data.total_rounds || data.prompts_per_player || 2;
  }
  if (els.hostMaxPlayersInput) {
    els.hostMaxPlayersInput.value = data.max_players || 0;
  }
  if (els.hostLobbyLocked) {
    els.hostLobbyLocked.checked = Boolean(data.lobby_locked);
  }
  if (els.hostSettingsForm) {
    const disabled = phase !== "lobby" || !isHost;
    Array.from(els.hostSettingsForm.elements).forEach((el) => {
      if (el.tagName === "BUTTON") return;
      el.disabled = disabled;
    });
    const submitButton = els.hostSettingsForm.querySelector("button");
    if (submitButton) {
      submitButton.disabled = disabled;
    }
  }
  if (els.hostPlayerActions) {
    renderHostPlayerActions(ctx, players, playerIDs, phase, isHost, state.hostId);
  }

  if (els.avatarSection) {
    els.avatarSection.style.display = phase === "lobby" ? "grid" : "none";
  }
  state.avatarLocked = Boolean(avatarLocks[String(playerId)] || avatarLocks[playerId]);
  if (els.avatarCanvasWrap) {
    const showCanvas = phase === "lobby" && !state.avatarLocked;
    els.avatarCanvasWrap.style.display = showCanvas ? "grid" : "none";
  }
  if (els.avatarLockedHint) {
    const showLockedHint = phase === "lobby" && state.avatarLocked;
    els.avatarLockedHint.style.display = showLockedHint ? "block" : "none";
  }
  if (els.saveAvatar) {
    const canSaveAvatar = phase === "lobby" && !state.avatarLocked;
    els.saveAvatar.style.display = canSaveAvatar ? "inline-flex" : "none";
    els.saveAvatar.disabled = !canSaveAvatar;
  }

  if (els.drawSection) {
    if (phase === "drawings" && !state.drawingSubmitted) {
      els.drawSection.style.display = "grid";
      if (actions.fetchPrompt) {
        actions.fetchPrompt();
      }
    } else {
      els.drawSection.style.display = "none";
    }
  }

  if (colorMap && els.meta) {
    const playerIdValue = els.meta.dataset.playerId;
    if (playerIdValue && colorMap[playerIdValue]) {
      state.brushColor = colorMap[playerIdValue];
      if (actions.applyBrushColor) {
        actions.applyBrushColor();
      }
      if (actions.applyAvatarColor) {
        actions.applyAvatarColor();
      }
    }
  }

  updateScoreboard(ctx, data, phase);
  updateGuessPhase(ctx, data, phase);
  updateVotePhase(ctx, data, phase);
  updateResultsPhase(ctx, data, phase);
}

function renderHostPlayerActions(ctx, players, playerIDs, phase, isHost, hostId) {
  const { els } = ctx;
  if (!els.hostPlayerActions) return;
  els.hostPlayerActions.innerHTML = "";
  if (!isHost) {
    return;
  }
  players.forEach((playerName, index) => {
    const playerID = Number(playerIDs[index] || 0);
    if (!playerID) {
      return;
    }
    const row = document.createElement("div");
    row.className = "player-action-row card-surface";
    const label = document.createElement("span");
    label.textContent = playerID === hostId ? `${playerName}*` : playerName;
    const kickButton = document.createElement("button");
    kickButton.type = "button";
    kickButton.className = "secondary";
    kickButton.textContent = "Remove";
    kickButton.dataset.playerId = String(playerID);
    if (playerID === hostId || phase !== "lobby") {
      kickButton.disabled = true;
    }
    row.appendChild(label);
    row.appendChild(kickButton);
    els.hostPlayerActions.appendChild(row);
  });
}

function updateGuessPhase(ctx, data, phase) {
  const { els, state } = ctx;
  if (!els.guessSection) return;
  if (phase !== "guesses") {
    els.guessSection.style.display = "none";
    return;
  }
  els.guessSection.style.display = "grid";
  const playerId = Number(els.meta?.dataset.playerId || 0);
  const assignments = Array.isArray(data.guess_assignments) ? data.guess_assignments : [];
  const assignment = assignments.find((entry) => Number(entry.player_id) === playerId) || null;
  const isOwnDrawing = assignment && Number(assignment.drawing_owner) === playerId;
  const canSubmit = Boolean(assignment) && !isOwnDrawing;
  const remainingMap = data.guess_remaining || {};
  const remaining = Number(remainingMap[String(playerId)] || 0);
  const hasSubmitted = !assignment || remaining === 0;
  const drawingImage = assignment ? assignment.drawing_image : "";
  const guessKey = `${assignment ? assignment.drawing_index : "none"}`;
  if (guessKey !== state.lastGuessKey) {
    if (els.guessInput) {
      els.guessInput.value = "";
    }
    state.lastGuessKey = guessKey;
  }
  if (els.guessStatus) {
    if (remaining === 0 && !assignment) {
      els.guessStatus.textContent = "Waiting for the next reveal.";
    } else if (canSubmit) {
      els.guessStatus.textContent = "Submit your fake prompt now.";
    } else if (isOwnDrawing) {
      els.guessStatus.textContent = "This is your drawing. Sit tight while others submit.";
    } else if (hasSubmitted) {
      els.guessStatus.textContent = "Guess submitted. Waiting for others.";
    } else {
      els.guessStatus.textContent = "Waiting for your assignment.";
    }
  }
  if (els.guessImage) {
    els.guessImage.src = drawingImage || "";
    els.guessImage.style.display = drawingImage ? "block" : "none";
  }
  if (els.guessForm) {
    const showForm = canSubmit;
    els.guessForm.style.display = showForm ? "grid" : "none";
    const submitButton = els.guessForm.querySelector("button");
    if (submitButton) {
      submitButton.disabled = !showForm;
    }
    if (els.guessInput) {
      els.guessInput.disabled = !showForm;
    }
  }
}

function updateVotePhase(ctx, data, phase) {
  const { els, state } = ctx;
  if (!els.voteSection) return;
  if (phase !== "guesses-votes") {
    els.voteSection.style.display = "none";
    return;
  }
  els.voteSection.style.display = "grid";
  const playerId = Number(els.meta?.dataset.playerId || 0);
  const assignments = Array.isArray(data.vote_assignments) ? data.vote_assignments : [];
  const assignment = assignments.find((entry) => Number(entry.player_id) === playerId) || null;
  const isOwnDrawing = assignment && Number(assignment.drawing_owner) === playerId;
  const remainingMap = data.vote_remaining || {};
  const remaining = Number(remainingMap[String(playerId)] || 0);
  const hasSubmitted = !assignment || remaining === 0;
  const canSubmit = Boolean(assignment) && !isOwnDrawing;
  const voteKey = `${assignment ? assignment.drawing_index : "none"}`;
  if (voteKey !== state.lastVoteKey) {
    const note = isOwnDrawing ? "This is your drawing. No vote needed." : "";
    renderVoteOptions(ctx, assignment ? assignment.options : [], note);
    state.lastVoteKey = voteKey;
  }
  if (els.voteStatus) {
    if (remaining === 0 && !assignment) {
      els.voteStatus.textContent = "Waiting for the next reveal.";
    } else if (canSubmit) {
      els.voteStatus.textContent = "Pick the real prompt and vote.";
    } else if (isOwnDrawing) {
      els.voteStatus.textContent = "This is your drawing. No vote needed.";
    } else if (hasSubmitted) {
      els.voteStatus.textContent = "Vote submitted. Waiting for others.";
    } else {
      els.voteStatus.textContent = "Waiting for your assignment.";
    }
  }
  if (els.voteImage) {
    const drawingImage = assignment ? assignment.drawing_image : "";
    els.voteImage.src = drawingImage || "";
    els.voteImage.style.display = drawingImage ? "block" : "none";
  }
  if (els.voteForm) {
    const showForm = canSubmit;
    els.voteForm.style.display = showForm ? "grid" : "none";
    const submitButton = els.voteForm.querySelector("button");
    if (submitButton) {
      submitButton.disabled = !showForm;
    }
  }
}

function updateResultsPhase(ctx, data, phase) {
  const { els, state } = ctx;
  if (!els.resultsSection) return;
  if (phase !== "results" && phase !== "complete") {
    els.resultsSection.style.display = "none";
    return;
  }
  els.resultsSection.style.display = "grid";
  const results = data.results || [];
  const scores = data.scores || [];
  const reveal = data.reveal || null;
  const resultsKey = JSON.stringify({ results, scores, reveal, phase });
  if (resultsKey !== state.lastResultsKey) {
    if (phase === "results") {
      renderReveal(ctx, reveal);
      renderResults(ctx, [], scores);
    } else {
      renderReveal(ctx, null);
      renderResults(ctx, results, scores);
    }
    state.lastResultsKey = resultsKey;
  }
}

function updateScoreboard(ctx, data, phase) {
  const { els, state } = ctx;
  if (!els.scoreboardSection || !els.scoreboardList) return;
  const scores = Array.isArray(data.scores) ? data.scores : [];
  const roundNumber = data.current_round || 0;
  const drawingsCount = data.counts ? data.counts.drawings || 0 : 0;
  const betweenRounds = phase === "drawings" && roundNumber > 1 && drawingsCount === 0;
  const shouldShow = betweenRounds && !state.drawingSubmitted && scores.length > 0;
  els.scoreboardSection.style.display = shouldShow ? "grid" : "none";
  if (!shouldShow) {
    return;
  }
  if (els.scoreboardStatus) {
    els.scoreboardStatus.textContent =
      roundNumber > 1
        ? `Round ${roundNumber} is starting. Here are the scores so far.`
        : "Here are the scores so far.";
  }
  renderScoreList(els.scoreboardList, scores);
}

function renderScoreList(container, scores) {
  container.innerHTML = "";
  if (!Array.isArray(scores) || scores.length === 0) {
    const note = document.createElement("p");
    note.className = "hint";
    note.textContent = "Scores will appear here once the round finishes.";
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

function renderVoteOptions(ctx, options, noteText) {
  const { els } = ctx;
  const playerId = Number(els.meta?.dataset.playerId || 0);
  if (!els.voteOptions) return;
  els.voteOptions.innerHTML = "";
  if (noteText) {
    const note = document.createElement("p");
    note.className = "hint";
    note.textContent = noteText;
    els.voteOptions.appendChild(note);
    return;
  }
  if (!Array.isArray(options) || options.length === 0) {
    const note = document.createElement("p");
    note.className = "hint";
    note.textContent = "Waiting for the next drawing.";
    els.voteOptions.appendChild(note);
    return;
  }
  options.forEach((option) => {
    const choice = option && typeof option === "object" ? option : { id: option, text: option, type: "guess" };
    const label = document.createElement("label");
    label.className = "vote-option card-surface";
    const input = document.createElement("input");
    input.type = "radio";
    input.name = "voteOption";
    input.value = choice.id || choice.text || "";
    input.dataset.choiceText = choice.text || "";
    if (choice.type === "guess" && Number(choice.owner_id || 0) === playerId) {
      input.disabled = true;
    }
    const span = document.createElement("span");
    span.textContent = choice.text || "";
    if (input.disabled) {
      span.textContent = `${choice.text || ""} (your lie)`;
    }
    if (choice.type === "prompt") {
      label.classList.add("vote-option-prompt");
    }
    label.appendChild(input);
    label.appendChild(span);
    els.voteOptions.appendChild(label);
  });
}

function renderResults(ctx, results, scores) {
  const { els } = ctx;
  if (els.resultsScores) {
    els.resultsScores.innerHTML = "";
  }
  if (els.resultsList) {
    els.resultsList.innerHTML = "";
  }

  if (Array.isArray(scores) && scores.length > 0 && els.resultsScores) {
    const card = document.createElement("div");
    card.className = "result-card card-surface";
    const title = document.createElement("h3");
    title.textContent = "Scoreboard";
    const list = document.createElement("ol");
    list.className = "score-list";
    scores.forEach((entry) => {
      const item = document.createElement("li");
      item.textContent = `${entry.player_name || "Player"}: ${entry.score}`;
      list.appendChild(item);
    });
    card.appendChild(title);
    card.appendChild(list);
    els.resultsScores.appendChild(card);
  }

  if (!Array.isArray(results) || results.length === 0) {
    return;
  }

  results.forEach((entry, index) => {
    const card = document.createElement("div");
    card.className = "result-card card-surface";

    const header = document.createElement("div");
    header.className = "result-block";
    const title = document.createElement("h3");
    title.textContent = `Drawing ${index + 1}`;
    const prompt = document.createElement("p");
    prompt.className = "prompt-text";
    prompt.textContent = entry.prompt || "";
    header.appendChild(title);
    header.appendChild(prompt);

    const image = document.createElement("img");
    image.className = "guess-image media-frame";
    image.alt = "Drawing"
    image.src = entry.drawing_image || "";

    const guesses = document.createElement("div");
    guesses.className = "result-block";
    const guessesTitle = document.createElement("h4");
    guessesTitle.textContent = "Guesses";
    guesses.appendChild(guessesTitle);
    const guessesList = document.createElement("ul");
    guessesList.className = "reveal-list";
    (entry.guesses || []).forEach((guess) => {
      const item = document.createElement("li");
      item.textContent = `${guess.player_name || "Player"}: ${guess.text}`;
      guessesList.appendChild(item);
    });
    guesses.appendChild(guessesList);

    const votes = document.createElement("div");
    votes.className = "result-block";
    const votesTitle = document.createElement("h4");
    votesTitle.textContent = "Votes";
    votes.appendChild(votesTitle);
    const votesList = document.createElement("ul");
    votesList.className = "reveal-list";
    const options = Array.isArray(entry.options) ? entry.options : [];
    if (options.length > 0) {
      options.forEach((option) => {
        const item = document.createElement("li");
        const optionType = option.type || "";
        const optionText = option.text || "";
        const ownerName = option.owner_name || "Player";
        let text = optionType === "prompt" ? `Prompt: ${optionText}` : `${ownerName} wrote: ${optionText}`;
        const playerVotes = Array.isArray(option.player_votes) ? option.player_votes : [];
        if (playerVotes.length > 0) {
          const voters = playerVotes.map((vote) => vote.player_name || "Player");
          text += ` | Picked by: ${voters.join(", ")}`;
        }
        const audienceCount = Number(option.audience_count || 0);
        if (audienceCount > 0) {
          text += ` | Audience: ${audienceCount}`;
        }
        item.textContent = text;
        votesList.appendChild(item);
      });
    } else {
      (entry.votes || []).forEach((vote) => {
        const item = document.createElement("li");
        item.textContent = `${vote.player_name || "Player"}: ${vote.text}`;
        votesList.appendChild(item);
      });
      if (Array.isArray(entry.audience_votes) && entry.audience_votes.length > 0) {
        const audience = document.createElement("p");
        audience.className = "meta";
        const parts = entry.audience_votes.map((vote) => `${vote.text} (${vote.count})`);
        audience.textContent = `Audience: ${parts.join(", ")}`;
        votes.appendChild(audience);
      }
    }
    votes.appendChild(votesList);

    card.appendChild(header);
    card.appendChild(image);
    card.appendChild(guesses);
    card.appendChild(votes);
    if (Array.isArray(entry.score_deltas) && entry.score_deltas.length > 0) {
      const deltaBlock = document.createElement("div");
      deltaBlock.className = "result-block";
      const deltaTitle = document.createElement("h4");
      deltaTitle.textContent = "Score changes";
      const deltaList = document.createElement("ul");
      deltaList.className = "reveal-list";
      entry.score_deltas.forEach((delta) => {
        const item = document.createElement("li");
        item.textContent = `${delta.player_name || "Player"}: +${delta.delta || 0}`;
        deltaList.appendChild(item);
      });
      deltaBlock.appendChild(deltaTitle);
      deltaBlock.appendChild(deltaList);
      card.appendChild(deltaBlock);
    }
    if (entry.joke) {
      const joke = document.createElement("p");
      joke.className = "result-joke";
      joke.textContent = entry.joke;
      card.appendChild(joke);
    }
    if (els.resultsList) {
      els.resultsList.appendChild(card);
    }
  });
}

function renderReveal(ctx, reveal) {
  const { els } = ctx;
  if (!els.revealSection) return;
  if (!reveal) {
    els.revealSection.style.display = "none";
    els.revealSection.innerHTML = "";
    return;
  }

  els.revealSection.style.display = "grid";
  els.revealSection.innerHTML = "";

  const header = document.createElement("div");
  header.className = "result-block";
  const title = document.createElement("h3");
  title.textContent = `Drawing ${reveal.drawing_index + 1}`;
  const stage = document.createElement("p");
  stage.className = "meta";
  if (reveal.stage === "guesses") {
    stage.textContent = "Guesses";
  } else if (reveal.stage === "joke") {
    stage.textContent = "Joke";
  } else {
    stage.textContent = "Votes";
  }
  header.appendChild(title);
  header.appendChild(stage);

  const image = document.createElement("img");
  image.className = "guess-image media-frame";
  image.alt = "Drawing reveal";
  image.src = reveal.drawing_image || "";

  const owner = document.createElement("p");
  owner.className = "meta";
  owner.textContent = `Artist: ${reveal.drawing_owner_name || "Unknown"}`;

  const list = document.createElement("ul");
  list.className = "reveal-list";
  let promptEl = null;
  if (reveal.stage === "guesses") {
    (reveal.guesses || []).forEach((guess) => {
      const item = document.createElement("li");
      item.textContent = `${guess.player_name || "Player"}: ${guess.text}`;
      list.appendChild(item);
    });
  } else {
    const prompt = document.createElement("p");
    prompt.className = "prompt-text";
    prompt.textContent = `Prompt: ${reveal.prompt || ""}`;
    promptEl = prompt;
    const options = Array.isArray(reveal.options) ? reveal.options : [];
    if (options.length > 0) {
      options.forEach((option) => {
        const item = document.createElement("li");
        const optionType = option.type || "";
        const optionText = option.text || "";
        const ownerName = option.owner_name || "Player";
        let text = optionType === "prompt" ? `Prompt: ${optionText}` : `${ownerName} wrote: ${optionText}`;
        const playerVotes = Array.isArray(option.player_votes) ? option.player_votes : [];
        if (playerVotes.length > 0) {
          const voters = playerVotes.map((vote) => vote.player_name || "Player");
          text += ` | Picked by: ${voters.join(", ")}`;
        }
        const audienceCount = Number(option.audience_count || 0);
        if (audienceCount > 0) {
          text += ` | Audience: ${audienceCount}`;
        }
        item.textContent = text;
        list.appendChild(item);
      });
    } else {
      const votes = Array.isArray(reveal.votes) ? reveal.votes : [];
      votes.forEach((vote) => {
        const item = document.createElement("li");
        item.textContent = `${vote.player_name || "Player"}: ${vote.text}`;
        list.appendChild(item);
      });
      const audienceVotes = Array.isArray(reveal.audience_votes) ? reveal.audience_votes : [];
      if (audienceVotes.length > 0) {
        const audience = document.createElement("p");
        audience.className = "meta";
        const parts = audienceVotes.map((vote) => `${vote.text} (${vote.count})`);
        audience.textContent = `Audience: ${parts.join(", ")}`;
        list.appendChild(audience);
      }
    }
  }

  els.revealSection.appendChild(header);
  els.revealSection.appendChild(image);
  els.revealSection.appendChild(owner);
  if (promptEl) {
    els.revealSection.appendChild(promptEl);
  }
  els.revealSection.appendChild(list);
  if (reveal.stage === "joke" && reveal.joke) {
    const joke = document.createElement("p");
    joke.className = "result-joke";
    joke.textContent = reveal.joke;
    els.revealSection.appendChild(joke);
  }
  if (Array.isArray(reveal.score_deltas) && reveal.score_deltas.length > 0) {
    const deltas = document.createElement("ul");
    deltas.className = "reveal-list";
    reveal.score_deltas.forEach((entry) => {
      const item = document.createElement("li");
      item.textContent = `${entry.player_name || "Player"}: +${entry.delta || 0}`;
      deltas.appendChild(item);
    });
    els.revealSection.appendChild(deltas);
  }
}
