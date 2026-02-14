async function requestJSON(url, options) {
  const res = await fetch(url, options);
  const data = await res.json().catch(() => ({}));
  return { res, data };
}

export async function fetchSnapshot(gameId) {
  return requestJSON(`/api/games/${encodeURIComponent(gameId)}`);
}

export async function fetchPrompt(gameId, playerId) {
  const authToken = localStorage.getItem(`pt_auth_${gameId}_${playerId}`) || "";
  const query = authToken ? `?auth_token=${encodeURIComponent(authToken)}` : "";
  return requestJSON(`/api/games/${encodeURIComponent(gameId)}/players/${encodeURIComponent(playerId)}/prompt${query}`);
}

export async function postStartGame(gameId, playerId, authToken) {
  return requestJSON(`/api/games/${encodeURIComponent(gameId)}/start`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ player_id: playerId, auth_token: authToken || "" })
  });
}

export async function postAvatar(gameId, playerId, avatarData) {
  const authToken = localStorage.getItem(`pt_auth_${gameId}_${playerId}`) || "";
  return requestJSON(`/api/games/${encodeURIComponent(gameId)}/avatar`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ player_id: playerId, avatar_data: avatarData, auth_token: authToken })
  });
}

export async function postDrawing(gameId, playerId, imageData, prompt) {
  const authToken = localStorage.getItem(`pt_auth_${gameId}_${playerId}`) || "";
  return requestJSON(`/api/games/${encodeURIComponent(gameId)}/drawings`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      player_id: playerId,
      image_data: imageData,
      prompt,
      auth_token: authToken
    })
  });
}

export async function postGuess(gameId, playerId, guess) {
  const authToken = localStorage.getItem(`pt_auth_${gameId}_${playerId}`) || "";
  return requestJSON(`/api/games/${encodeURIComponent(gameId)}/guesses`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ player_id: playerId, guess, auth_token: authToken })
  });
}

export async function postVote(gameId, playerId, payload) {
  const authToken = localStorage.getItem(`pt_auth_${gameId}_${playerId}`) || "";
  return requestJSON(`/api/games/${encodeURIComponent(gameId)}/votes`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      player_id: playerId,
      choice_id: payload?.choice_id || "",
      choice: payload?.choice || "",
      auth_token: authToken
    })
  });
}

export async function postAdvance(gameId, playerId, authToken) {
  return requestJSON(`/api/games/${encodeURIComponent(gameId)}/advance`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ player_id: playerId, auth_token: authToken || "" })
  });
}

export async function postEndGame(gameId, playerId, authToken) {
  return requestJSON(`/api/games/${encodeURIComponent(gameId)}/end`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ player_id: playerId, auth_token: authToken || "" })
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
