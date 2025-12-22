const createBtn = document.getElementById("createGame");
const createResult = document.getElementById("createResult");
const joinForm = document.getElementById("joinForm");
const joinResult = document.getElementById("joinResult");
const activeGames = document.getElementById("activeGames");
const gameList = document.getElementById("gameList");
const noGames = document.getElementById("noGames");

function renderActiveGames(games) {
  if (!gameList || !noGames) return;
  gameList.innerHTML = "";
  if (!Array.isArray(games) || games.length === 0) {
    noGames.style.display = "block";
    return;
  }
  noGames.style.display = "none";
  games.forEach((game) => {
    const card = document.createElement("div");
    card.className = "game-card";

    const info = document.createElement("div");
    const title = document.createElement("h3");
    title.textContent = game.id;
    const meta = document.createElement("p");
    meta.className = "meta";
    meta.textContent = `State: ${game.phase} · Players: ${game.players}`;
    info.appendChild(title);
    info.appendChild(meta);

    const actions = document.createElement("div");
    actions.className = "game-actions";
    const safeId = String(game.id || "").replace(/[^a-zA-Z0-9_-]/g, "");
    const resultId = `joinResult-${safeId || "game"}`;
    if (game.phase === "lobby") {
      const joinBtn = document.createElement("button");
      joinBtn.type = "button";
      joinBtn.className = "secondary join-active";
      joinBtn.dataset.joinCode = game.join_code || "";
      joinBtn.dataset.gameId = game.id || "";
      joinBtn.dataset.resultId = resultId;
      joinBtn.textContent = "Join lobby";
      actions.appendChild(joinBtn);
    } else {
      const status = document.createElement("span");
      status.className = "status-pill";
      status.textContent = "In progress";
      actions.appendChild(status);
    }
    const result = document.createElement("p");
    result.id = resultId;
    result.className = "result";
    actions.appendChild(result);

    card.appendChild(info);
    card.appendChild(actions);
    gameList.appendChild(card);
  });
}

if (createBtn) {
  createBtn.addEventListener("click", async () => {
    createResult.textContent = "Creating game...";
      const res = await fetch("/api/games", { method: "POST" });
      const data = await res.json();
      if (!res.ok) {
        createResult.textContent = data.error || "Failed to create game.";
        return;
      }
    createResult.textContent = "Game created. Join code: " + data.join_code;
    window.location.href = "/display/" + encodeURIComponent(data.game_id);
  });
}

if (joinForm) {
  joinForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    joinResult.textContent = "Joining game...";
    const code = joinForm.elements.code.value.trim();
    const name = joinForm.elements.name.value.trim();
      const res = await fetch("/api/games/" + encodeURIComponent(code) + "/join", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name })
      });
      const data = await res.json();
      if (!res.ok) {
        joinResult.textContent = data.error || "Failed to join game.";
        return;
      }
      window.location.href = "/play/" + encodeURIComponent(data.game_id) + "/" + encodeURIComponent(data.player_id);
    });
}


if (activeGames) {
  const protocol = window.location.protocol === "https:" ? "wss" : "ws";
  const ws = new WebSocket(`${protocol}://${window.location.host}/ws/home`);
  ws.addEventListener("message", (event) => {
    try {
      const data = JSON.parse(event.data);
      renderActiveGames(data.games);
    } catch {
      // ignore invalid payloads
    }
  });

  activeGames.addEventListener("click", async (event) => {
    const target = event.target;
    if (!target || !target.classList.contains("join-active")) {
      return;
    }
    const joinCode = target.dataset.joinCode || "";
    const gameId = target.dataset.gameId || "";
    const resultId = target.dataset.resultId || "";
    const resultEl = resultId ? document.getElementById(resultId) : null;
    const playerName = activeGames.dataset.playerName || "";
    if (!joinCode) {
      return;
    }
    if (!playerName) {
      window.location.href = "/join/" + encodeURIComponent(joinCode);
      return;
    }
    if (resultEl) {
      resultEl.textContent = "Joining game...";
    }
    const res = await fetch("/api/games/" + encodeURIComponent(joinCode) + "/join", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name: playerName })
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      if (resultEl) {
        resultEl.textContent = data.error || "Failed to join game.";
      }
      return;
    }
    const targetGame = data.game_id || gameId;
    window.location.href = "/play/" + encodeURIComponent(targetGame) + "/" + encodeURIComponent(data.player_id);
  });
}
