const joinForm = document.getElementById("joinForm");
const joinResult = document.getElementById("joinResult");

if (joinForm) {
  joinForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    joinResult.textContent = "Joining game...";
    const code = joinForm.elements.code.value.trim();
    const name = joinForm.elements.name.value.trim();
    const res = await fetch(`/api/games/${encodeURIComponent(code)}/join`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name })
    });
    const data = await res.json();
    if (!res.ok) {
      joinResult.textContent = data.error || "Failed to join game.";
      return;
    }
    joinResult.textContent = `Joined game ${data.game_id} as ${data.player}. Waiting for the host...`;
  });
}
