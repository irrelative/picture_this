function normalizePhase(phase) {
  if (phase === "votes") {
    return "guesses-votes";
  }
  return phase;
}

export function updateFromSnapshot(ctx, data) {
  const { els, state, actions } = ctx;
  const phase = normalizePhase(data.phase);
  const prevPhase = state.lastPhase || "";
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
  const playerIDs = Array.isArray(data.player_ids) ? data.player_ids : [];
  players.forEach((player, index) => {
    const item = document.createElement("li");
    const dot = document.createElement("span");
    dot.className = "player-dot";
    const colorKey = String(playerIDs[index] || "");
    if (colorMap && colorMap[colorKey]) {
      dot.style.backgroundColor = colorMap[colorKey];
    }
    const name = document.createElement("span");
    name.textContent = player;
    item.appendChild(dot);
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
  }

  if (phase === "drawings" && prevPhase === "guesses-votes" && roundNumber > 1) {
    state.showScoreboard = true;
  }
  if (phase !== "drawings") {
    state.showScoreboard = false;
  }
  if (state.drawingSubmitted) {
    state.showScoreboard = false;
  }
  state.lastPhase = phase;

  state.hostId = data.host_id || 0;
  const playerId = Number(els.meta?.dataset.playerId || 0);
  const playerNameValue = els.meta?.dataset.playerName || "";
  const isHost = playerId !== 0 && playerId === state.hostId;
  if (els.playerName && playerNameValue) {
    if (isHost) {
      els.playerName.textContent = `Signed in as ${playerNameValue}. You're the host — start the game when at least two players have joined.`;
    } else {
      els.playerName.textContent = `Signed in as ${playerNameValue}. Waiting for the host to begin.`;
    }
  }
  if (els.renameInput && playerNameValue && !els.renameInput.value) {
    els.renameInput.value = playerNameValue;
  }
  if (els.hostSection) {
    els.hostSection.style.display = "grid";
  }
  if (els.hostStartGame) {
    const enoughPlayers = players.length >= 2;
    const canStart = phase === "lobby" && isHost && enoughPlayers;
    els.hostStartGame.disabled = !canStart;
    els.hostStartGame.style.display = isHost ? "inline-flex" : "none";
    if (els.hostHelp) {
      if (phase !== "lobby") {
        els.hostHelp.textContent = "Game already started.";
      } else if (!isHost) {
        els.hostHelp.textContent = "Only the host can start the game.";
      } else if (!enoughPlayers) {
        els.hostHelp.textContent = "Waiting for at least two players to join.";
      } else {
        els.hostHelp.textContent = "All players are in. Start when ready.";
      }
    }
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
    }
  }

  updateScoreboard(ctx, data, phase);
  updateGuessPhase(ctx, data, phase);
  updateVotePhase(ctx, data, phase);
  updateResultsPhase(ctx, data, phase);
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
  const turn = data.guess_turn || null;
  const isTurn = turn && turn.guesser_id === playerId;
  const isOwnDrawing = turn && turn.drawing_owner === playerId;
  const drawingImage = turn ? turn.drawing_image : "";
  const guessKey = `${turn ? turn.guesser_id : "none"}-${turn ? turn.drawing_index : "none"}`;
  if (guessKey !== state.lastGuessKey) {
    if (els.guessInput) {
      els.guessInput.value = "";
    }
    state.lastGuessKey = guessKey;
  }
  if (els.guessStatus) {
    if (isOwnDrawing) {
      els.guessStatus.textContent = "This is your drawing, no guessing needed.";
    } else {
      els.guessStatus.textContent = isTurn ? "Your turn to guess." : "Waiting for the next guess.";
    }
  }
  if (els.guessImage) {
    els.guessImage.src = drawingImage || "";
    els.guessImage.style.display = drawingImage ? "block" : "none";
  }
  if (els.guessForm) {
    const showForm = isTurn && !isOwnDrawing;
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
  const turn = data.vote_turn || null;
  const isTurn = turn && turn.voter_id === playerId;
  const voteKey = `${turn ? turn.voter_id : "none"}-${turn ? turn.drawing_index : "none"}`;
  if (voteKey !== state.lastVoteKey) {
    renderVoteOptions(ctx, turn ? turn.options : []);
    state.lastVoteKey = voteKey;
  }
  if (els.voteStatus) {
    els.voteStatus.textContent = isTurn ? "Your turn to vote." : "Waiting for the next vote.";
  }
  if (els.voteImage) {
    const drawingImage = turn ? turn.drawing_image : "";
    els.voteImage.src = drawingImage || "";
    els.voteImage.style.display = drawingImage ? "block" : "none";
  }
  if (els.voteForm) {
    const submitButton = els.voteForm.querySelector("button");
    if (submitButton) {
      submitButton.disabled = !isTurn;
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
  const shouldShow = state.showScoreboard && phase === "drawings" && scores.length > 0;
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

function renderVoteOptions(ctx, options) {
  const { els } = ctx;
  if (!els.voteOptions) return;
  els.voteOptions.innerHTML = "";
  if (!Array.isArray(options) || options.length === 0) {
    const note = document.createElement("p");
    note.className = "hint";
    note.textContent = "Waiting for the next drawing.";
    els.voteOptions.appendChild(note);
    return;
  }
  options.forEach((option, index) => {
    const label = document.createElement("label");
    label.className = "vote-option";
    const input = document.createElement("input");
    input.type = "radio";
    input.name = "voteOption";
    input.value = option;
    if (index === 0) {
      input.checked = true;
    }
    const span = document.createElement("span");
    span.textContent = option;
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
    card.className = "result-card";
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
    card.className = "result-card";

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
    image.className = "guess-image";
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
    (entry.votes || []).forEach((vote) => {
      const item = document.createElement("li");
      item.textContent = `${vote.player_name || "Player"}: ${vote.text}`;
      votesList.appendChild(item);
    });
    votes.appendChild(votesList);

    card.appendChild(header);
    card.appendChild(image);
    card.appendChild(guesses);
    card.appendChild(votes);
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
  stage.textContent = reveal.stage === "guesses" ? "Guesses" : "Votes";
  header.appendChild(title);
  header.appendChild(stage);

  const image = document.createElement("img");
  image.className = "guess-image";
  image.alt = "Drawing reveal";
  image.src = reveal.drawing_image || "";

  const owner = document.createElement("p");
  owner.className = "meta";
  owner.textContent = `Artist: ${reveal.drawing_owner_name || "Unknown"}`;

  const list = document.createElement("ul");
  list.className = "reveal-list";
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
    els.revealSection.appendChild(prompt);
    (reveal.votes || []).forEach((vote) => {
      const item = document.createElement("li");
      item.textContent = `${vote.player_name || "Player"}: ${vote.text}`;
      list.appendChild(item);
    });
  }

  els.revealSection.appendChild(header);
  els.revealSection.appendChild(image);
  els.revealSection.appendChild(owner);
  els.revealSection.appendChild(list);
}
