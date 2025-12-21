const meta = document.getElementById("playerMeta");
const joinCode = document.getElementById("joinCode");
const gameStatus = document.getElementById("gameStatus");
const playerList = document.getElementById("playerList");
const playerName = document.getElementById("playerName");
const playerError = document.getElementById("playerError");
const drawSection = document.getElementById("drawSection");
const renameForm = document.getElementById("renameForm");
const renameInput = document.getElementById("renameInput");
const promptText = document.getElementById("promptText");
const canvas = document.getElementById("drawCanvas");
const saveCanvas = document.getElementById("saveCanvas");
const guessSection = document.getElementById("guessSection");
const guessStatus = document.getElementById("guessStatus");
const guessImage = document.getElementById("guessImage");
const guessForm = document.getElementById("guessForm");
const guessInput = document.getElementById("guessInput");
const voteSection = document.getElementById("voteSection");
const voteStatus = document.getElementById("voteStatus");
const voteImage = document.getElementById("voteImage");
const voteForm = document.getElementById("voteForm");
const voteOptions = document.getElementById("voteOptions");
const resultsSection = document.getElementById("resultsSection");
const resultsScores = document.getElementById("resultsScores");
const resultsList = document.getElementById("resultsList");
const revealSection = document.getElementById("revealSection");
const hostSection = document.getElementById("hostSection");
const hostStartGame = document.getElementById("hostStartGame");
const hostHelp = document.getElementById("hostHelp");

const CANVAS_WIDTH = 800;
const CANVAS_HEIGHT = 600;
let pollTimer = null;
let assignedPrompt = "";
let currentRound = 0;
let hostId = 0;
let drawingSubmitted = false;
let lastVoteKey = "";
let lastGuessKey = "";
let lastResultsKey = "";

async function loadPlayerView() {
  if (!meta) return;
  const gameId = meta.dataset.gameId;

  await fetchSnapshot(gameId);
}

async function fetchSnapshot(gameId) {
  const res = await fetch(`/api/games/${encodeURIComponent(gameId)}`);
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    joinCode.textContent = "Unavailable";
    gameStatus.textContent = "Unknown";
    if (playerError) {
      playerError.textContent = data.error || "Unable to load game status.";
    }
    return;
  }
  if (playerError) {
    playerError.textContent = "";
  }
  updateFromSnapshot(data);
}

function updateFromSnapshot(data) {
  joinCode.textContent = data.join_code || "Unavailable";
  gameStatus.textContent = data.phase || "Unknown";
  playerList.innerHTML = "";
  const players = Array.isArray(data.players) ? data.players : [];
  if (players.length === 0) {
    const item = document.createElement("li");
    item.textContent = "No players yet";
    playerList.appendChild(item);
    return;
  }
  players.forEach((player) => {
    const item = document.createElement("li");
    item.textContent = player;
    playerList.appendChild(item);
  });

  if (data.round_number && data.round_number !== currentRound) {
    currentRound = data.round_number;
    drawingSubmitted = false;
    assignedPrompt = "";
    if (promptText) {
      promptText.textContent = "Loading...";
    }
  }

  hostId = data.host_id || 0;
  const playerId = meta ? Number(meta.dataset.playerId) : 0;
  const playerNameValue = meta ? meta.dataset.playerName : "";
  const isHost = playerId !== 0 && playerId === hostId;
  if (playerName && playerNameValue) {
    if (isHost) {
      playerName.textContent = `Signed in as ${playerNameValue}. You're the host — start the game when at least two players have joined.`;
    } else {
      playerName.textContent = `Signed in as ${playerNameValue}. Waiting for the host to begin.`;
    }
  }
  if (renameInput && playerNameValue && !renameInput.value) {
    renameInput.value = playerNameValue;
  }
  if (hostSection) {
    hostSection.style.display = "grid";
  }
  if (hostStartGame) {
    const enoughPlayers = players.length >= 2;
    const canStart = data.phase === "lobby" && isHost && enoughPlayers;
    hostStartGame.disabled = !canStart;
    hostStartGame.style.display = isHost ? "inline-flex" : "none";
    if (hostHelp) {
      if (data.phase !== "lobby") {
        hostHelp.textContent = "Game already started.";
      } else if (!isHost) {
        hostHelp.textContent = "Only the host can start the game.";
      } else if (!enoughPlayers) {
        hostHelp.textContent = "Waiting for at least two players to join.";
      } else {
        hostHelp.textContent = "All players are in. Start when ready.";
      }
    }
  }

  if (drawSection) {
    if (data.phase === "drawings" && !drawingSubmitted) {
      drawSection.style.display = "grid";
      fetchPrompt();
    } else {
      drawSection.style.display = "none";
    }
  }

  updateGuessPhase(data);
  updateVotePhase(data);
  updateResultsPhase(data);
}

if (renameForm) {
  renameForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    if (!meta || !renameInput) return;
    const name = renameInput.value.trim();
    if (!name) return;
    const gameId = meta.dataset.gameId;
    const playerId = meta.dataset.playerId;
    const res = await fetch(`/api/games/${encodeURIComponent(gameId)}/rename`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        player_id: Number(playerId),
        name
      })
    });
    const payload = await res.json().catch(() => ({}));
    if (!res.ok) {
      if (playerError) {
        playerError.textContent = payload.error || "Unable to update name.";
      }
      return;
    }
    if (playerError) {
      playerError.textContent = "";
    }
    meta.dataset.playerName = name;
    updateFromSnapshot(payload);
  });
}

function startPolling() {
  if (pollTimer) return;
  pollTimer = setInterval(loadPlayerView, 3000);
}

function connectWS() {
  if (!meta) return;
  const gameId = meta.dataset.gameId;
  const protocol = window.location.protocol === "https:" ? "wss" : "ws";
  const socket = new WebSocket(`${protocol}://${window.location.host}/ws/games/${encodeURIComponent(gameId)}`);

  socket.addEventListener("message", (event) => {
    try {
      const data = JSON.parse(event.data);
      updateFromSnapshot(data);
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

loadPlayerView();
connectWS();

function updateGuessPhase(data) {
  if (!guessSection) return;
  if (data.phase !== "guesses") {
    guessSection.style.display = "none";
    return;
  }
  guessSection.style.display = "grid";
  const playerId = meta ? Number(meta.dataset.playerId) : 0;
  const turn = data.guess_turn || null;
  const isTurn = turn && turn.guesser_id === playerId;
  const drawingImage = turn ? turn.drawing_image : "";
  const guessKey = `${turn ? turn.guesser_id : "none"}-${turn ? turn.drawing_index : "none"}`;
  if (guessKey !== lastGuessKey) {
    if (guessInput) {
      guessInput.value = "";
    }
    lastGuessKey = guessKey;
  }
  if (guessStatus) {
    guessStatus.textContent = isTurn ? "Your turn to guess." : "Waiting for the next guess.";
  }
  if (guessImage) {
    guessImage.src = drawingImage || "";
    guessImage.style.display = drawingImage ? "block" : "none";
  }
  if (guessForm) {
    const submitButton = guessForm.querySelector("button");
    if (submitButton) {
      submitButton.disabled = !isTurn;
    }
    if (guessInput) {
      guessInput.disabled = !isTurn;
    }
  }
}

function updateVotePhase(data) {
  if (!voteSection) return;
  if (data.phase !== "guesses-votes") {
    voteSection.style.display = "none";
    return;
  }
  voteSection.style.display = "grid";
  const playerId = meta ? Number(meta.dataset.playerId) : 0;
  const turn = data.vote_turn || null;
  const isTurn = turn && turn.voter_id === playerId;
  const voteKey = `${turn ? turn.voter_id : "none"}-${turn ? turn.drawing_index : "none"}`;
  if (voteKey !== lastVoteKey) {
    renderVoteOptions(turn ? turn.options : []);
    lastVoteKey = voteKey;
  }
  if (voteStatus) {
    voteStatus.textContent = isTurn ? "Your turn to vote." : "Waiting for the next vote.";
  }
  if (voteImage) {
    const drawingImage = turn ? turn.drawing_image : "";
    voteImage.src = drawingImage || "";
    voteImage.style.display = drawingImage ? "block" : "none";
  }
  if (voteForm) {
    const submitButton = voteForm.querySelector("button");
    if (submitButton) {
      submitButton.disabled = !isTurn;
    }
  }
}

function updateResultsPhase(data) {
  if (!resultsSection) return;
  if (data.phase !== "results" && data.phase !== "complete") {
    resultsSection.style.display = "none";
    return;
  }
  resultsSection.style.display = "grid";
  const results = data.results || [];
  const scores = data.scores || [];
  const reveal = data.reveal || null;
  const resultsKey = JSON.stringify({ results, scores, reveal, phase: data.phase });
  if (resultsKey !== lastResultsKey) {
    if (data.phase === "results") {
      renderReveal(reveal);
      renderResults([], scores);
    } else {
      renderReveal(null);
      renderResults(results, scores);
    }
    lastResultsKey = resultsKey;
  }
}

function setupCanvas() {
  if (!canvas) return;
  const ctx = canvas.getContext("2d");
  if (!ctx) return;

  ctx.lineCap = "round";
  ctx.lineJoin = "round";
  ctx.lineWidth = 4;
  ctx.strokeStyle = "#1a1a1a";
  ctx.fillStyle = "#ffffff";
  ctx.fillRect(0, 0, CANVAS_WIDTH, CANVAS_HEIGHT);

  let drawing = false;
  let lastPoint = null;

  function getPoint(event) {
    const rect = canvas.getBoundingClientRect();
    const clientX = event.clientX ?? (event.touches && event.touches[0]?.clientX);
    const clientY = event.clientY ?? (event.touches && event.touches[0]?.clientY);
    if (clientX == null || clientY == null) {
      return null;
    }
    const x = (clientX - rect.left) * (canvas.width / rect.width);
    const y = (clientY - rect.top) * (canvas.height / rect.height);
    return { x, y };
  }

  function startDraw(event) {
    drawing = true;
    lastPoint = getPoint(event);
  }

  function moveDraw(event) {
    if (!drawing) return;
    const point = getPoint(event);
    if (!point || !lastPoint) return;
    ctx.beginPath();
    ctx.moveTo(lastPoint.x, lastPoint.y);
    ctx.lineTo(point.x, point.y);
    ctx.stroke();
    lastPoint = point;
  }

  function endDraw() {
    drawing = false;
    lastPoint = null;
  }

  canvas.addEventListener("pointerdown", (event) => {
    event.preventDefault();
    canvas.setPointerCapture(event.pointerId);
    startDraw(event);
  });

  canvas.addEventListener("pointermove", (event) => {
    event.preventDefault();
    moveDraw(event);
  });

  canvas.addEventListener("pointerup", (event) => {
    event.preventDefault();
    endDraw();
    canvas.releasePointerCapture(event.pointerId);
  });

  canvas.addEventListener("pointerleave", endDraw);
  canvas.addEventListener("pointercancel", endDraw);

  if (saveCanvas) {
    saveCanvas.addEventListener("click", async () => {
      const dataUrl = canvas.toDataURL("image/png");
      if (!meta) return;
      const gameId = meta.dataset.gameId;
      const playerId = meta.dataset.playerId;
      const res = await fetch(`/api/games/${encodeURIComponent(gameId)}/drawings`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          player_id: Number(playerId),
          image_data: dataUrl,
          prompt: assignedPrompt
        })
      });
      const payload = await res.json().catch(() => ({}));
      if (!res.ok) {
      if (playerError) {
        playerError.textContent = payload.error || "Unable to submit drawing.";
      }
      return;
    }
    if (playerError) {
      playerError.textContent = "";
    }
    drawingSubmitted = true;
    if (drawSection) {
      drawSection.style.display = "none";
    }
    });
  }
}

setupCanvas();

function renderVoteOptions(options) {
  if (!voteOptions) return;
  voteOptions.innerHTML = "";
  if (!Array.isArray(options) || options.length === 0) {
    const note = document.createElement("p");
    note.className = "hint";
    note.textContent = "No options yet.";
    voteOptions.appendChild(note);
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
    voteOptions.appendChild(label);
  });
}

function renderResults(results, scores) {
  if (resultsScores) {
    resultsScores.innerHTML = "";
    if (!Array.isArray(scores) || scores.length === 0) {
      const note = document.createElement("p");
      note.className = "hint";
      note.textContent = "Scores will appear once voting is complete.";
      resultsScores.appendChild(note);
    } else {
      const header = document.createElement("h3");
      header.textContent = "Scoreboard";
      resultsScores.appendChild(header);
      const list = document.createElement("ul");
      list.className = "score-list";
      scores.forEach((entry) => {
        const item = document.createElement("li");
        item.textContent = `${entry.player_name}: ${entry.score} pts`;
        list.appendChild(item);
      });
      resultsScores.appendChild(list);
    }
  }
  if (!resultsList) return;
  resultsList.innerHTML = "";
  if (!Array.isArray(results) || results.length === 0) {
    const note = document.createElement("p");
    note.className = "hint";
    note.textContent = "No results yet.";
    resultsList.appendChild(note);
    return;
  }
  results.forEach((entry) => {
    const card = document.createElement("div");
    card.className = "result-card";

    const header = document.createElement("div");
    const title = document.createElement("h3");
    title.textContent = `Drawing ${entry.drawing_index + 1}`;
    const meta = document.createElement("p");
    meta.className = "meta";
    meta.textContent = `Artist: ${entry.drawing_owner_name || "Unknown"}`;
    header.appendChild(title);
    header.appendChild(meta);

    const image = document.createElement("img");
    image.className = "guess-image";
    image.alt = "Drawing result";
    image.src = entry.drawing_image || "";

    const prompt = document.createElement("p");
    prompt.className = "prompt-text";
    prompt.textContent = `Prompt: ${entry.prompt || ""}`;

    const guesses = document.createElement("div");
    guesses.className = "result-block";
    const guessesTitle = document.createElement("h4");
    guessesTitle.textContent = "Guesses";
    guesses.appendChild(guessesTitle);
    const guessList = document.createElement("ul");
    (entry.guesses || []).forEach((guess) => {
      const item = document.createElement("li");
      item.textContent = `${guess.player_name || "Player"}: ${guess.text}`;
      guessList.appendChild(item);
    });
    guesses.appendChild(guessList);

    const votes = document.createElement("div");
    votes.className = "result-block";
    const votesTitle = document.createElement("h4");
    votesTitle.textContent = "Votes";
    votes.appendChild(votesTitle);
    const voteList = document.createElement("ul");
    (entry.votes || []).forEach((vote) => {
      const item = document.createElement("li");
      item.textContent = `${vote.player_name || "Player"}: ${vote.text}`;
      voteList.appendChild(item);
    });
    votes.appendChild(voteList);

    card.appendChild(header);
    card.appendChild(image);
    card.appendChild(prompt);
    card.appendChild(guesses);
    card.appendChild(votes);
    resultsList.appendChild(card);
  });
}

function renderReveal(reveal) {
  if (!revealSection) return;
  revealSection.innerHTML = "";
  if (!reveal) {
    revealSection.style.display = "none";
    return;
  }
  revealSection.style.display = "grid";
  const header = document.createElement("div");
  const title = document.createElement("h3");
  title.textContent = `Reveal ${reveal.index + 1} of ${reveal.total}`;
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
    revealSection.appendChild(prompt);
    (reveal.votes || []).forEach((vote) => {
      const item = document.createElement("li");
      item.textContent = `${vote.player_name || "Player"}: ${vote.text}`;
      list.appendChild(item);
    });
  }

  revealSection.appendChild(header);
  revealSection.appendChild(image);
  revealSection.appendChild(owner);
  revealSection.appendChild(list);
}

if (hostStartGame) {
  hostStartGame.addEventListener("click", async () => {
    if (!meta) return;
    const gameId = meta.dataset.gameId;
    const playerId = Number(meta.dataset.playerId);
    const res = await fetch(`/api/games/${encodeURIComponent(gameId)}/start`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ player_id: playerId })
    });
    const payload = await res.json().catch(() => ({}));
    if (!res.ok) {
      if (playerError) {
        playerError.textContent = payload.error || "Unable to start game.";
      }
      return;
    }
    if (playerError) {
      playerError.textContent = "";
    }
    updateFromSnapshot(payload);
  });
}

async function fetchPrompt() {
  if (!meta) return;
  if (assignedPrompt) {
    return;
  }
  const gameId = meta.dataset.gameId;
  const playerId = meta.dataset.playerId;
  const res = await fetch(`/api/games/${encodeURIComponent(gameId)}/players/${encodeURIComponent(playerId)}/prompt`);
  const payload = await res.json().catch(() => ({}));
  if (!res.ok) {
    if (playerError) {
      playerError.textContent = payload.error || "Unable to load prompt.";
    }
    return;
  }
  assignedPrompt = payload.prompt || "";
  if (!assignedPrompt) {
    if (playerError) {
      playerError.textContent = "No prompts assigned.";
    }
    return;
  }
  if (promptText) {
    promptText.textContent = assignedPrompt;
  }
}

if (guessForm) {
  guessForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    if (!meta || !guessInput) return;
    const guess = guessInput.value.trim();
    if (!guess) {
      return;
    }
    const gameId = meta.dataset.gameId;
    const playerId = meta.dataset.playerId;
    const res = await fetch(`/api/games/${encodeURIComponent(gameId)}/guesses`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        player_id: Number(playerId),
        guess
      })
    });
    const payload = await res.json().catch(() => ({}));
    if (!res.ok) {
      if (playerError) {
        playerError.textContent = payload.error || "Unable to submit guess.";
      }
      return;
    }
    if (playerError) {
      playerError.textContent = "";
    }
    updateFromSnapshot(payload);
  });
}

if (voteForm) {
  voteForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    if (!meta || !voteOptions) return;
    const selected = voteOptions.querySelector("input[name='voteOption']:checked");
    if (!selected) {
      if (playerError) {
        playerError.textContent = "Choose an option before voting.";
      }
      return;
    }
    const gameId = meta.dataset.gameId;
    const playerId = meta.dataset.playerId;
    const res = await fetch(`/api/games/${encodeURIComponent(gameId)}/votes`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        player_id: Number(playerId),
        choice: selected.value
      })
    });
    const payload = await res.json().catch(() => ({}));
    if (!res.ok) {
      if (playerError) {
        playerError.textContent = payload.error || "Unable to submit vote.";
      }
      return;
    }
    if (playerError) {
      playerError.textContent = "";
    }
    updateFromSnapshot(payload);
  });
}
