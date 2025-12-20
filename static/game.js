const meta = document.getElementById("gameMeta");
const joinCode = document.getElementById("joinCode");
const gameStatus = document.getElementById("gameStatus");
const playerList = document.getElementById("playerList");
const gameError = document.getElementById("gameError");

async function loadGame() {
  if (!meta) return;
  const gameId = meta.dataset.gameId;
  const res = await fetch(`/api/games/${encodeURIComponent(gameId)}`);
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    joinCode.textContent = "Unavailable";
    gameStatus.textContent = "Unknown";
    if (gameError) {
      gameError.textContent = data.error || "Unable to load game status.";
    }
    return;
  }
  if (gameError) {
    gameError.textContent = "";
  }
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
}

loadGame();
setInterval(loadGame, 3000);
