const meta = document.getElementById("playerMeta");
const joinCode = document.getElementById("joinCode");
const gameStatus = document.getElementById("gameStatus");
const playerList = document.getElementById("playerList");
const playerName = document.getElementById("playerName");

async function loadPlayerView() {
  if (!meta) return;
  const gameId = meta.dataset.gameId;
  const name = meta.dataset.playerName;

  if (playerName && name) {
    playerName.textContent = `Signed in as ${name}. Waiting for the host to begin.`;
  }

  const res = await fetch(`/api/games/${encodeURIComponent(gameId)}`);
  if (!res.ok) {
    joinCode.textContent = "Unavailable";
    gameStatus.textContent = "Unknown";
    return;
  }
  const data = await res.json();
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

loadPlayerView();
setInterval(loadPlayerView, 3000);
