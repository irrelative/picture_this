async function requestJSON(url, options) {
  const res = await fetch(url, options);
  const data = await res.json().catch(() => ({}));
  return { res, data };
}

export async function fetchSnapshot(gameId) {
  return requestJSON(`/api/games/${encodeURIComponent(gameId)}`);
}

export async function fetchPrompt(gameId, playerId) {
  return requestJSON(`/api/games/${encodeURIComponent(gameId)}/players/${encodeURIComponent(playerId)}/prompt`);
}

export async function postStartGame(gameId, playerId) {
  return requestJSON(`/api/games/${encodeURIComponent(gameId)}/start`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ player_id: playerId })
  });
}

export async function postAvatar(gameId, playerId, avatarData) {
  return requestJSON(`/api/games/${encodeURIComponent(gameId)}/avatar`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ player_id: playerId, avatar_data: avatarData })
  });
}

export async function postDrawing(gameId, playerId, imageData, prompt) {
  return requestJSON(`/api/games/${encodeURIComponent(gameId)}/drawings`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      player_id: playerId,
      image_data: imageData,
      prompt
    })
  });
}

export async function postGuess(gameId, playerId, guess) {
  return requestJSON(`/api/games/${encodeURIComponent(gameId)}/guesses`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ player_id: playerId, guess })
  });
}

export async function postVote(gameId, playerId, choice) {
  return requestJSON(`/api/games/${encodeURIComponent(gameId)}/votes`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ player_id: playerId, choice })
  });
}
