const els = {
  meta: document.getElementById("audienceMeta"),
  joinCode: document.getElementById("audienceJoinCode"),
  phase: document.getElementById("audiencePhase"),
  timer: document.getElementById("audienceTimer"),
  joinPanel: document.getElementById("audienceJoinPanel"),
  joinForm: document.getElementById("audienceJoinForm"),
  nameInput: document.getElementById("audienceName"),
  votePanel: document.getElementById("audienceVotePanel"),
  voteForm: document.getElementById("audienceVoteForm"),
  status: document.getElementById("audienceStatus"),
  image: document.getElementById("audienceImage"),
  options: document.getElementById("audienceOptions"),
  result: document.getElementById("audienceResult"),
  error: document.getElementById("audienceError")
};

const state = {
  pollTimer: null,
  timerHandle: null,
  timerEndsAt: 0,
  reconnectHandle: null,
  socket: null,
  audience: null,
  snapshot: null
};

function audienceStorageKey(gameId) {
  return `pt_audience_${gameId}`;
}

function loadAudience(gameId) {
  const raw = localStorage.getItem(audienceStorageKey(gameId));
  if (!raw) return null;
  try {
    const parsed = JSON.parse(raw);
    if (!parsed || !parsed.audience_id || !parsed.token) {
      return null;
    }
    parsed.submitted = parsed.submitted || {};
    return parsed;
  } catch {
    return null;
  }
}

function saveAudience(gameId) {
  if (!state.audience) return;
  localStorage.setItem(audienceStorageKey(gameId), JSON.stringify(state.audience));
}

function clearAudience(gameId) {
  state.audience = null;
  localStorage.removeItem(audienceStorageKey(gameId));
}

async function requestJSON(url, options) {
  const res = await fetch(url, options);
  const data = await res.json().catch(() => ({}));
  return { res, data };
}

function formatTime(seconds) {
  const minutes = Math.floor(seconds / 60);
  const remainder = seconds % 60;
  return `${String(minutes).padStart(2, "0")}:${String(remainder).padStart(2, "0")}`;
}

function renderTimer() {
  if (!els.timer) return;
  if (!state.timerEndsAt) {
    els.timer.textContent = "--:--";
    return;
  }
  const remaining = Math.max(0, Math.round((state.timerEndsAt - Date.now()) / 1000));
  els.timer.textContent = formatTime(remaining);
}

function syncTimer(snapshot) {
  const endsAt = snapshot.phase_ends_at ? Date.parse(snapshot.phase_ends_at) : 0;
  state.timerEndsAt = Number.isNaN(endsAt) ? 0 : endsAt;
  renderTimer();
  if (!state.timerHandle) {
    state.timerHandle = setInterval(renderTimer, 1000);
  }
}

function voteKey(snapshot, drawingIndex) {
  return `${snapshot.current_round || 0}:${drawingIndex}`;
}

function hasSubmitted(snapshot, drawingIndex) {
  if (!state.audience || drawingIndex < 0) return false;
  return Boolean(state.audience.submitted[voteKey(snapshot, drawingIndex)]);
}

function markSubmitted(snapshot, drawingIndex) {
  if (!state.audience || drawingIndex < 0) return;
  state.audience.submitted[voteKey(snapshot, drawingIndex)] = true;
  const gameId = els.meta?.dataset.gameId || "";
  if (gameId) {
    saveAudience(gameId);
  }
}

function renderOptions(options, disabled) {
  if (!els.options) return;
  els.options.innerHTML = "";
  if (!Array.isArray(options) || options.length === 0) {
    const note = document.createElement("p");
    note.className = "hint";
    note.textContent = "Waiting for options.";
    els.options.appendChild(note);
    return;
  }
  options.forEach((option) => {
    const choice = option && typeof option === "object" ? option : { id: option, text: option };
    const label = document.createElement("label");
    label.className = "vote-option card-surface";
    const input = document.createElement("input");
    input.type = "radio";
    input.name = "audienceOption";
    input.value = choice.id || choice.text || "";
    input.dataset.choiceText = choice.text || "";
    input.disabled = disabled;
    const span = document.createElement("span");
    span.textContent = choice.text || "";
    label.appendChild(input);
    label.appendChild(span);
    els.options.appendChild(label);
  });
}

function renderSnapshot(snapshot) {
  state.snapshot = snapshot;
  syncTimer(snapshot);
  if (els.joinCode) {
    els.joinCode.textContent = snapshot.join_code || "Unavailable";
  }
  if (els.phase) {
    els.phase.textContent = snapshot.phase || "unknown";
  }
  if (els.error) {
    els.error.textContent = "";
  }

  if (els.joinPanel) {
    els.joinPanel.style.display = state.audience ? "none" : "grid";
  }
  if (els.votePanel) {
    els.votePanel.style.display = state.audience ? "grid" : "none";
  }

  if (!state.audience) {
    if (els.status) {
      els.status.textContent = "Join the audience to vote.";
    }
    return;
  }

  if (snapshot.phase !== "guesses-votes") {
    if (els.status) {
      els.status.textContent = "Waiting for the voting phase.";
    }
    if (els.image) {
      els.image.style.display = "none";
    }
    renderOptions([], true);
    if (els.voteForm) {
      els.voteForm.style.display = "none";
    }
    return;
  }

  const voteFocus = snapshot.vote_focus || null;
  if (!voteFocus) {
    if (els.status) {
      els.status.textContent = "Waiting for the next drawing.";
    }
    if (els.image) {
      els.image.style.display = "none";
    }
    renderOptions([], true);
    if (els.voteForm) {
      els.voteForm.style.display = "none";
    }
    return;
  }

  const drawingIndex = Number(voteFocus.drawing_index || -1);
  const submitted = hasSubmitted(snapshot, drawingIndex);
  if (els.status) {
    els.status.textContent = submitted
      ? "Vote submitted for this drawing. Waiting for the next one."
      : "Pick the prompt you think is real.";
  }
  if (els.image) {
    els.image.src = voteFocus.drawing_image || "";
    els.image.style.display = voteFocus.drawing_image ? "block" : "none";
  }
  if (els.voteForm) {
    els.voteForm.style.display = "grid";
    const submitButton = els.voteForm.querySelector("button");
    if (submitButton) {
      submitButton.disabled = submitted;
    }
  }
  renderOptions(voteFocus.options || [], submitted);
}

async function loadSnapshot() {
  const gameId = els.meta?.dataset.gameId || "";
  if (!gameId) return;
  const { res, data } = await requestJSON(`/api/games/${encodeURIComponent(gameId)}`);
  if (!res.ok) {
    if (els.error) {
      els.error.textContent = data.error || "Unable to load game status.";
    }
    return;
  }
  renderSnapshot(data);
}

function stopPolling() {
  if (!state.pollTimer) return;
  clearInterval(state.pollTimer);
  state.pollTimer = null;
}

function startPolling() {
  if (state.pollTimer) return;
  state.pollTimer = setInterval(loadSnapshot, 3000);
}

function scheduleReconnect() {
  if (state.reconnectHandle) return;
  state.reconnectHandle = setTimeout(() => {
    state.reconnectHandle = null;
    connectWS();
  }, 2000);
}

function connectWS() {
  const gameId = els.meta?.dataset.gameId || "";
  if (!gameId || typeof WebSocket === "undefined") {
    startPolling();
    return;
  }
  const protocol = window.location.protocol === "https:" ? "wss" : "ws";
  const socket = new WebSocket(
    `${protocol}://${window.location.host}/ws/games/${encodeURIComponent(gameId)}?role=audience`
  );
  state.socket = socket;

  socket.addEventListener("open", () => {
    stopPolling();
    if (state.reconnectHandle) {
      clearTimeout(state.reconnectHandle);
      state.reconnectHandle = null;
    }
    loadSnapshot();
  });

  socket.addEventListener("message", (event) => {
    try {
      const payload = JSON.parse(event.data);
      if (payload && typeof payload === "object" && payload.phase) {
        renderSnapshot(payload);
      }
    } catch {
      // Ignore non-JSON and HTML-targeted payloads.
    }
  });

  socket.addEventListener("close", () => {
    startPolling();
    scheduleReconnect();
  });

  socket.addEventListener("error", () => {
    socket.close();
  });
}

if (els.joinForm) {
  els.joinForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    const gameId = els.meta?.dataset.gameId || "";
    if (!gameId) return;
    const name = (els.nameInput?.value || "").trim();
    if (!name) {
      if (els.error) {
        els.error.textContent = "Display name is required.";
      }
      return;
    }
    if (els.result) {
      els.result.textContent = "Joining audience...";
    }
    const existing = state.audience || loadAudience(gameId);
    const { res, data } = await requestJSON(`/api/games/${encodeURIComponent(gameId)}/audience`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name, token: existing?.token || "" })
    });
    if (!res.ok) {
      if (els.result) {
        els.result.textContent = "";
      }
      if (els.error) {
        els.error.textContent = data.error || "Unable to join audience.";
      }
      return;
    }
    const previousSubmitted =
      existing && existing.token && existing.token === data.token ? existing.submitted || {} : {};
    state.audience = {
      audience_id: data.audience_id,
      token: data.token,
      name: data.audience_name || name,
      submitted: previousSubmitted
    };
    saveAudience(gameId);
    if (els.result) {
      els.result.textContent = `Joined as ${state.audience.name}.`;
    }
    if (els.error) {
      els.error.textContent = "";
    }
    loadSnapshot();
  });
}

if (els.voteForm) {
  els.voteForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    if (!state.audience || !state.snapshot) {
      return;
    }
    const selected = els.options?.querySelector("input[name='audienceOption']:checked");
    if (!selected) {
      if (els.error) {
        els.error.textContent = "Choose an option before voting.";
      }
      return;
    }
    const voteFocus = state.snapshot.vote_focus || null;
    if (!voteFocus) {
      return;
    }
    const drawingIndex = Number(voteFocus.drawing_index || -1);
    const gameId = els.meta?.dataset.gameId || "";
    const payload = {
      audience_id: state.audience.audience_id,
      token: state.audience.token,
      drawing_index: drawingIndex,
      choice_id: selected.value,
      choice: selected.dataset.choiceText || ""
    };
    const { res, data } = await requestJSON(`/api/games/${encodeURIComponent(gameId)}/audience/votes`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload)
    });
    if (!res.ok) {
      const alreadySubmitted =
        res.status === 409 && typeof data.error === "string" && data.error.includes("already submitted");
      if (alreadySubmitted) {
        markSubmitted(state.snapshot, drawingIndex);
        if (els.result) {
          els.result.textContent = "Vote already recorded for this drawing.";
        }
        if (els.error) {
          els.error.textContent = "";
        }
        loadSnapshot();
        return;
      }
      const authExpired =
        res.status === 409 &&
        typeof data.error === "string" &&
        (data.error.includes("audience member not found") || data.error.includes("invalid audience authentication"));
      if (authExpired) {
        clearAudience(gameId);
        if (els.result) {
          els.result.textContent = "";
        }
        if (els.error) {
          els.error.textContent = "Audience session expired. Please join again.";
        }
        if (state.snapshot) {
          renderSnapshot(state.snapshot);
        }
        return;
      }
      if (els.error) {
        els.error.textContent = data.error || "Unable to submit vote.";
      }
      return;
    }
    markSubmitted(state.snapshot, drawingIndex);
    if (els.result) {
      els.result.textContent = "Audience vote submitted.";
    }
    if (els.error) {
      els.error.textContent = "";
    }
    if (data && typeof data === "object") {
      renderSnapshot(data);
    } else {
      loadSnapshot();
    }
  });
}

const gameId = els.meta?.dataset.gameId || "";
if (gameId) {
  state.audience = loadAudience(gameId);
}
loadSnapshot();
connectWS();
