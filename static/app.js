const createBtn = document.getElementById("createGame");
const createResult = document.getElementById("createResult");
const joinForm = document.getElementById("joinForm");
const joinResult = document.getElementById("joinResult");

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
    window.location.href = "/games/" + encodeURIComponent(data.game_id);
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
