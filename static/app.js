import { gameAPIPath, postJSON, setPlayerAuthToken, setPlayerRecoveryCode } from "./api_client.js";
import { applyHTMLMessage } from "./ws_html.js";

const createGameForm = document.getElementById("createGameForm");
const createResult = document.getElementById("createResult");
const joinForm = document.getElementById("joinForm");
const joinResult = document.getElementById("joinResult");
const activeGames = document.getElementById("activeGames");
const registerForm = document.getElementById("registerForm");
const registerResult = document.getElementById("registerResult");
const loginForm = document.getElementById("loginForm");
const loginResult = document.getElementById("loginResult");
const logoutButton = document.getElementById("logoutButton");

if (createGameForm) {
  createGameForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    if (!createResult) return;
    createResult.textContent = "Creating game...";
    const minPlayers = Number(createGameForm.elements.min_players?.value || 2);
    const maxPlayers = Number(createGameForm.elements.max_players?.value || 0);
    const { res, data } = await postJSON("/api/games", {
      min_players: minPlayers,
      max_players: maxPlayers
    });
    if (!res.ok) {
      createResult.textContent = data.error || "Failed to create game.";
      return;
    }
    createResult.textContent = "Game created. Join code: " + data.join_code;
    window.location.href = "/display/" + encodeURIComponent(data.game_id);
  });
}

if (registerForm) {
  registerForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    if (registerResult) {
      registerResult.textContent = "Creating account...";
    }
    const email = registerForm.elements.email.value.trim();
    const username = registerForm.elements.username.value.trim();
    const password = registerForm.elements.password.value;
    const { res, data } = await postJSON("/api/auth/register", {
      email,
      username,
      password
    });
    if (!res.ok) {
      if (registerResult) {
        registerResult.textContent = res.status === 409
          ? "That email is already registered. Sign in instead."
          : data.error || "Unable to register.";
      }
      if (res.status === 409 && loginForm) {
        loginForm.elements.email.value = email;
        loginForm.elements.password.focus();
      }
      return;
    }
    window.location.reload();
  });
}

if (loginForm) {
  loginForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    if (loginResult) {
      loginResult.textContent = "Logging in...";
    }
    const email = loginForm.elements.email.value.trim();
    const password = loginForm.elements.password.value;
    const { res, data } = await postJSON("/api/auth/login", {
      email,
      password
    });
    if (!res.ok) {
      if (loginResult) {
        loginResult.textContent = data.error || "Unable to log in.";
      }
      return;
    }
    window.location.reload();
  });
}

if (logoutButton) {
  logoutButton.addEventListener("click", async () => {
    await postJSON("/api/auth/logout", {});
    window.location.reload();
  });
}

if (joinForm) {
  joinForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    if (joinResult) {
      joinResult.textContent = "Joining game...";
    }
    const code = joinForm.elements.code.value.trim();
    const name = joinForm.elements.name.value.trim();
    const { res, data } = await postJSON(gameAPIPath(code, "/join"), {
      name
    });
    if (!res.ok) {
      if (joinResult) {
        joinResult.textContent = data.error || "Failed to join game.";
      }
      return;
    }
    setPlayerAuthToken(data.game_id, data.player_id, data.auth_token);
    setPlayerRecoveryCode(data.game_id, data.player_id, data.recovery_code);
    window.location.href = "/play/" + encodeURIComponent(data.game_id) + "/" + encodeURIComponent(data.player_id);
  });
}

if (activeGames) {
  const protocol = window.location.protocol === "https:" ? "wss" : "ws";
  const ws = new WebSocket(`${protocol}://${window.location.host}/ws/home`);
  ws.addEventListener("message", (event) => {
    applyHTMLMessage(event.data);
  });

  activeGames.addEventListener("click", async (event) => {
    const target = event.target;
    if (!target || !target.classList.contains("join-active")) {
      return;
    }
    const joinCode = target.dataset.joinCode || "";
    const gameId = target.dataset.gameId || "";
    const resultId = target.dataset.resultId || "";
    const resultEl = resultId ? document.getElementById(resultId) : null;
    const playerName = activeGames.dataset.playerName || "";
    if (!joinCode) {
      return;
    }
    if (!playerName) {
      window.location.href = "/join/" + encodeURIComponent(joinCode);
      return;
    }
    if (resultEl) {
      resultEl.textContent = "Joining game...";
    }
    const { res, data } = await postJSON(gameAPIPath(joinCode, "/join"), {
      name: playerName
    });
    if (!res.ok) {
      if (resultEl) {
        resultEl.textContent = data.error || "Failed to join game.";
      }
      return;
    }
    setPlayerAuthToken(data.game_id, data.player_id, data.auth_token);
    const targetGame = data.game_id || gameId;
    window.location.href = "/play/" + encodeURIComponent(targetGame) + "/" + encodeURIComponent(data.player_id);
  });
}
