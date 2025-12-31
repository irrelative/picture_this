import { setupCanvas } from "./player_canvas.js";

const joinForm = document.getElementById("joinForm");
const joinResult = document.getElementById("joinResult");
const avatarCanvas = document.getElementById("avatarCanvas");
const joinAs = document.getElementById("joinAs");
const joinAsButtons = document.getElementById("joinAsButtons");

const avatarState = {
  canvasCtx: null,
  canvasWidth: 800,
  canvasHeight: 600,
  brushColor: "#1a1a1a"
};

if (avatarCanvas) {
  setupCanvas(
    {
      els: { canvas: avatarCanvas },
      state: avatarState
    },
    () => {}
  );
}

async function submitJoin(nameOverride) {
  if (!joinForm) return;
  joinResult.textContent = "Joining game...";
  const code = joinForm.elements.code.value.trim();
  const name = (nameOverride || joinForm.elements.name.value || "").trim();
  const avatarData = avatarCanvas ? avatarCanvas.toDataURL("image/png") : "";
  const res = await fetch(`/api/games/${encodeURIComponent(code)}/join`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name, avatar_data: avatarData })
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    joinResult.textContent = data.error || "Failed to join game.";
    return;
  }
  window.location.href = `/play/${encodeURIComponent(data.game_id)}/${encodeURIComponent(data.player_id)}`;
}

function renderJoinAs(players) {
  if (!joinAs || !joinAsButtons) return;
  joinAsButtons.innerHTML = "";
  if (!Array.isArray(players) || players.length === 0) {
    joinAs.hidden = true;
    return;
  }
  players.forEach((player) => {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "secondary";
    button.textContent = `Join as ${player}`;
    button.addEventListener("click", () => {
      if (joinForm) {
        joinForm.elements.name.value = player;
      }
      submitJoin(player);
    });
    joinAsButtons.appendChild(button);
  });
  joinAs.hidden = false;
}

async function loadJoinAs() {
  if (!joinForm) return;
  const code = joinForm.elements.code.value.trim();
  if (!code) return;
  const res = await fetch(`/api/games/${encodeURIComponent(code)}`);
  if (!res.ok) return;
  const data = await res.json().catch(() => ({}));
  if (!data.paused) {
    if (joinAs) joinAs.hidden = true;
    return;
  }
  const players = Array.isArray(data.players) ? data.players : [];
  renderJoinAs(players);
}

if (joinForm) {
  joinForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    submitJoin();
  });
  loadJoinAs();
}
