export async function requestJSON(url, options) {
  const res = await fetch(url, options);
  const data = await res.json().catch(() => ({}));
  return { res, data };
}

export async function postJSON(url, payload) {
  return requestJSON(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload)
  });
}

export function gameAPIPath(gameId, suffix = "") {
  return `/api/games/${encodeURIComponent(gameId)}${suffix}`;
}

function playerAuthTokenKey(gameId, playerId) {
  return `pt_auth_${gameId}_${playerId}`;
}

export function getPlayerAuthToken(gameId, playerId) {
  return localStorage.getItem(playerAuthTokenKey(gameId, playerId)) || "";
}

export function setPlayerAuthToken(gameId, playerId, authToken) {
  if (!authToken || !gameId || !playerId) {
    return;
  }
  localStorage.setItem(playerAuthTokenKey(gameId, playerId), authToken);
}

export function setPlayerRecoveryCode(gameId, playerId, recoveryCode, playerName = "") {
  if (recoveryCode && gameId && playerId) {
    localStorage.setItem(`pt_recovery_${gameId}_${playerId}`, JSON.stringify({
      game_id: gameId,
      player_id: Number(playerId),
      player_name: playerName,
      recovery_code: recoveryCode
    }));
  }
}

export function getPlayerRecoveryCredentials(gameId, playerId = 0) {
  const prefix = `pt_recovery_${gameId}_`;
  for (let index = 0; index < localStorage.length; index += 1) {
    const key = localStorage.key(index) || "";
    if (!key.startsWith(prefix)) continue;
    const storedPlayerId = Number(key.slice(prefix.length));
    if (playerId && storedPlayerId !== Number(playerId)) continue;
    const raw = localStorage.getItem(key) || "";
    try {
      const parsed = JSON.parse(raw);
      if (parsed?.recovery_code) return { ...parsed, player_id: storedPlayerId };
    } catch {
      if (raw) return { game_id: gameId, player_id: storedPlayerId, player_name: "", recovery_code: raw };
    }
  }
  return null;
}
