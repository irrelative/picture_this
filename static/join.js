import { gameAPIPath, getPlayerRecoveryCredentials, postJSON, requestJSON, setPlayerAuthToken, setPlayerRecoveryCode } from "./api_client.js";

const joinForm = document.getElementById("joinForm");
const joinResult = document.getElementById("joinResult");
const joinAs = document.getElementById("joinAs");
const joinAsButtons = document.getElementById("joinAsButtons");
const recoverForm = document.getElementById("recoverForm");
const localRecovery = document.getElementById("localRecovery");
const localRecoveryHint = document.getElementById("localRecoveryHint");
const localRecoveryButton = document.getElementById("localRecoveryButton");
let savedRecovery = null;

async function submitJoin(nameOverride) {
  if (!joinForm) return;
  joinResult.textContent = "Joining game...";
  const code = joinForm.elements.code.value.trim();
  const name = (nameOverride || joinForm.elements.name.value || "").trim();
  const { res, data } = await postJSON(gameAPIPath(code, "/join"), { name });
  if (!res.ok) {
    joinResult.textContent = data.error || "Failed to join game.";
    return;
  }
  setPlayerAuthToken(data.game_id, data.player_id, data.auth_token);
  setPlayerRecoveryCode(data.game_id, data.player_id, data.recovery_code, data.player || name);
  window.location.href = `/play/${encodeURIComponent(data.game_id)}/${encodeURIComponent(data.player_id)}`;
}

recoverForm?.addEventListener("submit", async (event) => {
  event.preventDefault();
  joinResult.textContent = "Recovering your seat...";
  const code = joinForm.elements.code.value.trim();
  const { res, data } = await postJSON(gameAPIPath(code, "/players/recover"), {
    name: recoverForm.elements.name.value.trim(),
    recovery_code: recoverForm.elements.recovery_code.value.trim()
  });
  if (!res.ok) {
    joinResult.textContent = data.error || "Seat recovery failed.";
    return;
  }
  setPlayerAuthToken(data.game_id, data.player_id, data.auth_token);
  setPlayerRecoveryCode(data.game_id, data.player_id, data.recovery_code, recoverForm.elements.name.value.trim());
  window.location.href = `/play/${encodeURIComponent(data.game_id)}/${encodeURIComponent(data.player_id)}`;
});

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
  const { res, data } = await requestJSON(gameAPIPath(code));
  if (!res.ok) return;
  savedRecovery = getPlayerRecoveryCredentials(data.game_id);
  if (savedRecovery && localRecovery && localRecoveryButton) {
    localRecovery.hidden = false;
    if (localRecoveryHint) localRecoveryHint.textContent = savedRecovery.player_name ? `Saved seat found for ${savedRecovery.player_name}.` : "A saved seat was found on this device.";
  }
  if (!data.paused) {
    if (joinAs) joinAs.hidden = true;
    return;
  }
  const players = Array.isArray(data.players) ? data.players : [];
  renderJoinAs(players);
}

localRecoveryButton?.addEventListener("click", async () => {
  if (!savedRecovery) return;
  joinResult.textContent = "Recovering your saved seat...";
  const code = joinForm.elements.code.value.trim();
  const { res, data } = await postJSON(gameAPIPath(code, "/players/recover"), {
    name: savedRecovery.player_name || joinForm.elements.name.value.trim(),
    recovery_code: savedRecovery.recovery_code
  });
  if (!res.ok) {
    joinResult.textContent = data.error || "Seat recovery failed.";
    return;
  }
  setPlayerAuthToken(data.game_id, data.player_id, data.auth_token);
  setPlayerRecoveryCode(data.game_id, data.player_id, data.recovery_code, savedRecovery.player_name);
  window.location.href = `/play/${encodeURIComponent(data.game_id)}/${encodeURIComponent(data.player_id)}`;
});

if (joinForm) {
  joinForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    submitJoin();
  });
  loadJoinAs();
}
