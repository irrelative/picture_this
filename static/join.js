import { setupCanvas } from "./player_canvas.js";

const joinForm = document.getElementById("joinForm");
const joinResult = document.getElementById("joinResult");
const avatarCanvas = document.getElementById("avatarCanvas");

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

if (joinForm) {
  joinForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    joinResult.textContent = "Joining game...";
    const code = joinForm.elements.code.value.trim();
    const name = joinForm.elements.name.value.trim();
    const avatarData = avatarCanvas ? avatarCanvas.toDataURL("image/png") : "";
    const res = await fetch(`/api/games/${encodeURIComponent(code)}/join`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name, avatar_data: avatarData })
    });
    const data = await res.json();
    if (!res.ok) {
      joinResult.textContent = data.error || "Failed to join game.";
      return;
    }
    window.location.href = `/play/${encodeURIComponent(data.game_id)}/${encodeURIComponent(data.player_id)}`;
  });
}
