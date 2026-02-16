export function formatTime(seconds) {
  const minutes = Math.floor(seconds / 60);
  const remainder = seconds % 60;
  return `${String(minutes).padStart(2, "0")}:${String(remainder).padStart(2, "0")}`;
}

function parseEndsAt(value) {
  if (!value) {
    return 0;
  }
  if (typeof value === "number") {
    return Number.isFinite(value) ? value : 0;
  }
  const parsed = Date.parse(value);
  return Number.isNaN(parsed) ? 0 : parsed;
}

export function createPhaseTimer(onTick) {
  let handle = null;
  let endsAt = 0;

  const tick = () => {
    onTick(endsAt);
  };

  const setEndsAt = (value) => {
    endsAt = parseEndsAt(value);
    tick();
    if (!handle) {
      handle = setInterval(tick, 1000);
    }
  };

  const clear = () => {
    if (handle) {
      clearInterval(handle);
      handle = null;
    }
    endsAt = 0;
    tick();
  };

  return {
    clear,
    getEndsAt: () => endsAt,
    setEndsAt
  };
}

export function createPolling(loadFn, intervalMs = 3000) {
  let handle = null;

  const start = () => {
    if (handle) {
      return;
    }
    handle = setInterval(loadFn, intervalMs);
  };

  const stop = () => {
    if (!handle) {
      return;
    }
    clearInterval(handle);
    handle = null;
  };

  return {
    isActive: () => Boolean(handle),
    start,
    stop
  };
}

export function createReconnect(connectFn, options = {}) {
  const baseDelayMs = options.baseDelayMs ?? 2000;
  const exponential = Boolean(options.exponential);
  const maxDelayMs = options.maxDelayMs ?? baseDelayMs;
  const maxExponent = options.maxExponent ?? 5;

  let handle = null;
  let attempts = 0;

  const nextDelayMs = () => {
    if (!exponential) {
      return baseDelayMs;
    }
    const exp = Math.min(attempts, maxExponent);
    return Math.min(maxDelayMs, baseDelayMs * 2 ** exp);
  };

  const clear = () => {
    if (!handle) {
      return;
    }
    clearTimeout(handle);
    handle = null;
  };

  const reset = () => {
    attempts = 0;
    clear();
  };

  const schedule = (shouldSchedule = () => true) => {
    if (handle || !shouldSchedule()) {
      return;
    }
    const delayMs = nextDelayMs();
    handle = setTimeout(() => {
      handle = null;
      attempts += 1;
      connectFn();
    }, delayMs);
  };

  return {
    attempts: () => attempts,
    clear,
    isScheduled: () => Boolean(handle),
    reset,
    schedule
  };
}
