export function updateFromSnapshot(ctx, data) {
  const { els, state } = ctx;
  els.joinCode.textContent = data.join_code || "Unavailable";
  els.gameStatus.textContent = data.phase || "Unknown";
  state.hostId = data.host_id || 0;
  if (els.startGame) {
    els.startGame.style.display = data.phase === "lobby" ? "inline-flex" : "none";
    els.startGame.disabled = data.phase === "lobby" ? (data.players?.length || 0) < 2 : true;
  }
  if (els.endGame) {
    els.endGame.style.display = data.phase !== "complete" ? "inline-flex" : "none";
  }

  els.playerList.innerHTML = "";
  if (els.playerActions) {
    els.playerActions.innerHTML = "";
  }
  const players = Array.isArray(data.players) ? data.players : [];
  if (players.length === 0) {
    const item = document.createElement("li");
    item.textContent = "No players yet";
    els.playerList.appendChild(item);
    return;
  }
  const playerIDs = Array.isArray(data.player_ids) ? data.player_ids : [];
  players.forEach((player, index) => {
    const item = document.createElement("li");
    item.textContent = player;
    els.playerList.appendChild(item);

    if (els.playerActions) {
      const row = document.createElement("div");
      row.className = "player-action-row";
      const label = document.createElement("span");
      label.textContent = player;
      const kickButton = document.createElement("button");
      kickButton.type = "button";
      kickButton.className = "secondary";
      kickButton.textContent = "Remove";
      kickButton.dataset.playerId = String(playerIDs[index] || 0);
      if (playerIDs[index] === state.hostId) {
        kickButton.disabled = true;
      }
      row.appendChild(label);
      row.appendChild(kickButton);
      els.playerActions.appendChild(row);
    }
  });

  if (els.roundsInput) {
    els.roundsInput.value = data.total_rounds || data.prompts_per_player || 2;
  }
  if (els.maxPlayersInput) {
    els.maxPlayersInput.value = data.max_players || 0;
  }
  if (els.promptCategory) {
    els.promptCategory.value = data.prompt_category || "";
  }
  if (els.lobbyLocked) {
    els.lobbyLocked.checked = Boolean(data.lobby_locked);
  }
  if (els.lobbyStatus) {
    const maxPlayers = data.max_players > 0 ? data.max_players : "∞";
    const lockedText = data.lobby_locked ? "Locked" : "Open";
    const audienceCount = data.audience_count != null ? data.audience_count : 0;
    els.lobbyStatus.textContent = `Players: ${players.length}/${maxPlayers}. ${lockedText} lobby. Audience: ${audienceCount}.`;
  }
  if (els.settingsForm) {
    const disabled = data.phase !== "lobby";
    Array.from(els.settingsForm.elements).forEach((el) => {
      if (el.tagName === "BUTTON") return;
      el.disabled = disabled;
    });
    const submitButton = els.settingsForm.querySelector("button");
    if (submitButton) {
      submitButton.disabled = disabled;
    }
  }

  if (els.playerActions) {
    const disabled = data.phase !== "lobby";
    Array.from(els.playerActions.querySelectorAll("button")).forEach((button) => {
      button.disabled = disabled;
    });
  }
}
