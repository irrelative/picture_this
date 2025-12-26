import { applyHTMLMessage } from "./ws_html.js";

const createBtn = document.getElementById("createGame");
const createResult = document.getElementById("createResult");
const joinForm = document.getElementById("joinForm");
const joinResult = document.getElementById("joinResult");
const activeGames = document.getElementById("activeGames");

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
    applyHTMLMessage(event.data);
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
