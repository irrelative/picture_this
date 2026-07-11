import { gameAPIPath, getPlayerAuthToken, requestJSON } from "./api_client.js";

export async function fetchSnapshot(gameId, playerId) {
	const authToken = getPlayerAuthToken(gameId, playerId);
	const query = authToken ? `?auth_token=${encodeURIComponent(authToken)}` : "";
	return requestJSON(gameAPIPath(gameId, `/players/${encodeURIComponent(playerId)}/state${query}`));
}

export async function fetchPrompt(gameId, playerId) {
  const authToken = getPlayerAuthToken(gameId, playerId);
  const query = authToken ? `?auth_token=${encodeURIComponent(authToken)}` : "";
  return requestJSON(gameAPIPath(gameId, `/players/${encodeURIComponent(playerId)}/prompt${query}`));
}

export async function postStartGame(gameId, playerId, authToken) {
  return requestJSON(gameAPIPath(gameId, "/start"), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ player_id: playerId, auth_token: authToken || "" })
  });
}

export async function postAvatar(gameId, playerId, avatarData) {
  const authToken = getPlayerAuthToken(gameId, playerId);
  return requestJSON(gameAPIPath(gameId, "/avatar"), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ player_id: playerId, avatar_data: avatarData, auth_token: authToken })
  });
}

export async function postDrawing(gameId, playerId, imageData, prompt) {
  const authToken = getPlayerAuthToken(gameId, playerId);
  return requestJSON(gameAPIPath(gameId, "/drawings"), {
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
  const authToken = getPlayerAuthToken(gameId, playerId);
  return requestJSON(gameAPIPath(gameId, "/guesses"), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ player_id: playerId, guess, auth_token: authToken })
  });
}

export async function postVote(gameId, playerId, payload) {
  const authToken = getPlayerAuthToken(gameId, playerId);
  return requestJSON(gameAPIPath(gameId, "/votes"), {
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

export async function postLike(gameId, playerId, drawingIndex, choiceId) {
  const authToken = getPlayerAuthToken(gameId, playerId);
  return requestJSON(gameAPIPath(gameId, "/likes"), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ player_id: playerId, drawing_index: drawingIndex, choice_id: choiceId, auth_token: authToken })
  });
}

export async function postAdvance(gameId, playerId, authToken) {
  return requestJSON(gameAPIPath(gameId, "/advance"), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ player_id: playerId, auth_token: authToken || "" })
  });
}

export async function postEndGame(gameId, playerId, authToken) {
  return requestJSON(gameAPIPath(gameId, "/end"), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ player_id: playerId, auth_token: authToken || "" })
  });
}

export async function postSettings(gameId, payload) {
  return requestJSON(gameAPIPath(gameId, "/settings"), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload)
  });
}

export async function postKick(gameId, payload) {
  return requestJSON(gameAPIPath(gameId, "/kick"), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload)
  });
}
