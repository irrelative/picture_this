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

export function setPlayerRecoveryCode(gameId, playerId, recoveryCode) {
  if (recoveryCode && gameId && playerId) {
    localStorage.setItem(`pt_recovery_${gameId}_${playerId}`, recoveryCode);
  }
}
