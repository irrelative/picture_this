const meta = document.getElementById("audienceMeta");
const joinCode = document.getElementById("audienceJoinCode");
const status = document.getElementById("audienceStatus");
const audienceCount = document.getElementById("audienceCount");
const voteSection = document.getElementById("audienceVoteSection");
const voteList = document.getElementById("audienceVoteList");
const audienceError = document.getElementById("audienceError");

let pollTimer = null;
const submittedVotes = new Set();

async function loadAudience() {
  if (!meta) return;
  const gameId = meta.dataset.gameId;
  await fetchSnapshot(gameId);
}

async function fetchSnapshot(gameId) {
  const res = await fetch(`/api/games/${encodeURIComponent(gameId)}`);
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    if (audienceError) {
      audienceError.textContent = data.error || "Unable to load game status.";
    }
    return;
  }
  if (audienceError) {
    audienceError.textContent = "";
  }
  updateFromSnapshot(data);
}

function updateFromSnapshot(data) {
  joinCode.textContent = data.join_code || "Unavailable";
  status.textContent = data.phase || "Unknown";
  if (audienceCount) {
    audienceCount.textContent = `Audience members: ${data.audience_count || 0}`;
  }

  if (!voteSection || !voteList) return;
  if (data.phase !== "guesses-votes") {
    voteSection.style.display = "none";
    return;
  }
  voteSection.style.display = "grid";
  renderAudienceVotes(data.audience_options || []);
}

function renderAudienceVotes(entries) {
  voteList.innerHTML = "";
  if (!Array.isArray(entries) || entries.length === 0) {
    const note = document.createElement("p");
    note.className = "hint";
    note.textContent = "Waiting for drawings.";
    voteList.appendChild(note);
    return;
  }
  entries.forEach((entry) => {
    const card = document.createElement("div");
    card.className = "audience-card";

    const title = document.createElement("h3");
    title.textContent = `Drawing ${entry.drawing_index + 1}`;

    const image = document.createElement("img");
    image.className = "guess-image";
    image.alt = "Audience drawing";
    image.src = entry.drawing_image || "";

    const options = document.createElement("div");
    options.className = "vote-options";
    const optionName = `audience-${entry.drawing_index}`;
    (entry.options || []).forEach((option, index) => {
      const label = document.createElement("label");
      label.className = "vote-option";
      const input = document.createElement("input");
      input.type = "radio";
      input.name = optionName;
      input.value = option;
      if (index === 0) {
        input.checked = true;
      }
      const span = document.createElement("span");
      span.textContent = option;
      label.appendChild(input);
      label.appendChild(span);
      options.appendChild(label);
    });

    const button = document.createElement("button");
    button.type = "button";
    button.className = "secondary";
    button.textContent = "Submit vote";
    button.disabled = submittedVotes.has(entry.drawing_index);
    button.addEventListener("click", async () => {
      const selected = options.querySelector(`input[name="${optionName}"]:checked`);
      if (!selected) return;
      await submitAudienceVote(entry.drawing_index, selected.value);
    });

    card.appendChild(title);
    card.appendChild(image);
    card.appendChild(options);
    card.appendChild(button);
    voteList.appendChild(card);
  });
}

async function submitAudienceVote(drawingIndex, choice) {
  if (!meta) return;
  const gameId = meta.dataset.gameId;
  const audienceId = meta.dataset.audienceId;
  const res = await fetch(`/api/games/${encodeURIComponent(gameId)}/audience/votes`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      audience_id: Number(audienceId),
      drawing_index: drawingIndex,
      choice
    })
  });
  const payload = await res.json().catch(() => ({}));
  if (!res.ok) {
    if (audienceError) {
      audienceError.textContent = payload.error || "Unable to submit vote.";
    }
    return;
  }
  if (audienceError) {
    audienceError.textContent = "";
  }
  submittedVotes.add(drawingIndex);
  updateFromSnapshot(payload);
}

function startPolling() {
  if (pollTimer) return;
  pollTimer = setInterval(loadAudience, 3000);
}

function connectWS() {
  if (!meta) return;
  const gameId = meta.dataset.gameId;
  const protocol = window.location.protocol === "https:" ? "wss" : "ws";
  const socket = new WebSocket(`${protocol}://${window.location.host}/ws/games/${encodeURIComponent(gameId)}`);

  socket.addEventListener("message", (event) => {
    try {
      const data = JSON.parse(event.data);
      updateFromSnapshot(data);
    } catch {
      // ignore invalid payloads
    }
  });

  socket.addEventListener("close", () => {
    startPolling();
  });

  socket.addEventListener("error", () => {
    startPolling();
  });
}

loadAudience();
connectWS();
