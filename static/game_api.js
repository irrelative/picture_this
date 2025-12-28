async function requestJSON(url, options) {
  const res = await fetch(url, options);
  const data = await res.json().catch(() => ({}));
  return { res, data };
}

export async function fetchSnapshot(gameId) {
  return requestJSON(`/api/games/${encodeURIComponent(gameId)}`);
}

export async function postStartGame(gameId, playerId) {
  return requestJSON(`/api/games/${encodeURIComponent(gameId)}/start`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ player_id: playerId || 0 })
  });
}

export async function postEndGame(gameId) {
  return requestJSON(`/api/games/${encodeURIComponent(gameId)}/end`, {
    method: "POST"
  });
}

export async function postSettings(gameId, payload) {
  return requestJSON(`/api/games/${encodeURIComponent(gameId)}/settings`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload)
  });
}

export async function postKick(gameId, payload) {
  return requestJSON(`/api/games/${encodeURIComponent(gameId)}/kick`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload)
  });
}
