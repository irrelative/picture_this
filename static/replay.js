const meta = document.getElementById("replayMeta");
const status = document.getElementById("replayStatus");
const error = document.getElementById("replayError");
const eventCard = document.getElementById("eventCard");
const prevEvent = document.getElementById("prevEvent");
const nextEvent = document.getElementById("nextEvent");
const roundSelect = document.getElementById("roundSelect");

let events = [];
let currentIndex = 0;
let roundMap = new Map();

async function loadReplay() {
  if (!meta) return;
  const gameId = meta.dataset.gameId;
  const res = await fetch(`/api/games/${encodeURIComponent(gameId)}/events`);
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    if (error) {
      error.textContent = data.error || "Unable to load events.";
    }
    return;
  }
  if (error) {
    error.textContent = "";
  }
  events = Array.isArray(data.events) ? data.events : [];
  status.textContent = `Loaded ${events.length} events`;
  buildRoundMap();
  renderRoundOptions();
  currentIndex = 0;
  renderEvent();
}

function buildRoundMap() {
  roundMap = new Map();
  events.forEach((event, index) => {
    const roundId = event.round_id || "lobby";
    if (!roundMap.has(roundId)) {
      roundMap.set(roundId, []);
    }
    roundMap.get(roundId).push(index);
  });
}

function renderRoundOptions() {
  if (!roundSelect) return;
  roundSelect.innerHTML = "";
  Array.from(roundMap.keys()).forEach((roundId) => {
    const option = document.createElement("option");
    option.value = roundId;
    option.textContent = roundId === "lobby" ? "Lobby" : `Round ${roundId}`;
    roundSelect.appendChild(option);
  });
}

function renderEvent() {
  if (!eventCard) return;
  if (!events.length) {
    eventCard.textContent = "No events recorded.";
    return;
  }
  const event = events[currentIndex];
  if (!event) return;
  eventCard.innerHTML = "";
  const title = document.createElement("h3");
  title.textContent = `${event.type}`;
  const metaLine = document.createElement("p");
  metaLine.className = "meta";
  const roundLabel = event.round_id ? `Round ${event.round_id}` : "Lobby";
  metaLine.textContent = `${roundLabel} • ${new Date(event.created_at).toLocaleString()}`;
  const payload = document.createElement("pre");
  payload.textContent = JSON.stringify(event.payload || {}, null, 2);
  eventCard.appendChild(title);
  eventCard.appendChild(metaLine);
  eventCard.appendChild(payload);
}

function moveEvent(delta) {
  if (!events.length) return;
  currentIndex = Math.max(0, Math.min(events.length - 1, currentIndex + delta));
  renderEvent();
}

if (prevEvent) {
  prevEvent.addEventListener("click", () => moveEvent(-1));
}

if (nextEvent) {
  nextEvent.addEventListener("click", () => moveEvent(1));
}

if (roundSelect) {
  roundSelect.addEventListener("change", () => {
    const roundId = roundSelect.value;
    const indices = roundMap.get(roundId);
    if (indices && indices.length) {
      currentIndex = indices[0];
      renderEvent();
    }
  });
}

loadReplay();
