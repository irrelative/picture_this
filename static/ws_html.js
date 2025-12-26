export function applyHTMLMessage(payload) {
  let message = payload;
  if (typeof payload === "string") {
    try {
      message = JSON.parse(payload);
    } catch {
      return null;
    }
  }
  if (Array.isArray(message)) {
    let lastResult = null;
    message.forEach((item) => {
      const result = applySingleHTMLMessage(item);
      if (result) {
        lastResult = result;
      }
    });
    return lastResult;
  }
  return applySingleHTMLMessage(message);
}

function applySingleHTMLMessage(message) {
  if (!message || message.type !== "html") {
    return null;
  }
  const target = document.querySelector(message.target);
  if (!target) {
    return null;
  }
  const html = String(message.html || "").trim();
  if (!html) {
    return null;
  }
  const wrapper = document.createElement("div");
  wrapper.innerHTML = html;
  if (message.swap === "outer") {
    const next = wrapper.firstElementChild;
    if (!next) return null;
    target.replaceWith(next);
    return { target: next, swap: "outer" };
  }
  target.innerHTML = wrapper.innerHTML;
  return { target, swap: "inner" };
}
