const els = {
  title: document.getElementById("app-title"),
  local: document.getElementById("btn-local"),
  remote: document.getElementById("btn-remote"),
  key: document.getElementById("tun-key"),
  generate: document.getElementById("btn-generate"),
  copy: document.getElementById("btn-copy"),
  paste: document.getElementById("btn-paste"),
  start: document.getElementById("btn-start"),
  startText: document.getElementById("start-text"),
  spinner: document.getElementById("start-spinner"),
  logs: document.getElementById("log-list"),
  nat: document.getElementById("nat-hint"),
  upgrade: document.getElementById("upgrade-box"),
  upgradeVer: document.getElementById("upgrade-ver"),
  overlay: document.getElementById("exit-overlay"),
};

let workType = "Local";
let initialized = false;
let currentKind = "initializing";
let connectedOnce = false;

function setWorkType(type) {
  workType = type;
  els.local.classList.toggle("active", type === "Local");
  els.remote.classList.toggle("active", type === "Remote");
}

function setControlsEnabled(enabled) {
  els.local.disabled = !enabled;
  els.remote.disabled = !enabled;
  els.key.disabled = !enabled;
  els.generate.disabled = !enabled;
  els.paste.disabled = !enabled;
}

function applyState(s) {
  if (!s) return;
  els.title.textContent = "Goodlink  v" + (s.version || "");
  currentKind = s.button.kind;

  if (!initialized || !s.button.othersEnabled) {
    setWorkType(s.workType || "Local");
    if (document.activeElement !== els.key) {
      els.key.value = s.tunKey || "";
    }
    initialized = true;
  }

  setControlsEnabled(!!s.button.othersEnabled);

  els.startText.textContent = s.button.text || "";
  els.start.disabled = !s.button.enabled;
  els.start.className = "start-btn " + (s.button.importance || "high");
  els.spinner.classList.toggle("hidden", !s.button.activity);

  if (s.nat) {
    els.nat.textContent = s.nat.text || "";
    els.nat.className = "nat " + (s.nat.ready ? (s.nat.isNAT4 ? "warn" : "ok") : "detecting");
  }

  if (s.upgrade && s.upgrade.need) {
    els.upgrade.classList.remove("hidden");
    els.upgradeVer.textContent = "v" + s.upgrade.latest;
  } else {
    els.upgrade.classList.add("hidden");
  }
}

function renderLogs(lines) {
  els.logs.textContent = (lines || []).join("\n");
  els.logs.scrollTop = els.logs.scrollHeight;
}

function appendLog(line) {
  if (els.logs.textContent) {
    els.logs.textContent += "\n" + line;
  } else {
    els.logs.textContent = line;
  }
  els.logs.scrollTop = els.logs.scrollHeight;
}

els.local.addEventListener("click", () => setWorkType("Local"));
els.remote.addEventListener("click", () => setWorkType("Remote"));

els.generate.addEventListener("click", async () => {
  const resp = await fetch("/api/key/generate", { method: "POST" });
  if (!resp.ok) return;
  const data = await resp.json();
  if (data.tunKey) els.key.value = data.tunKey;
});

els.copy.addEventListener("click", async () => {
  try {
    await navigator.clipboard.writeText(els.key.value || "");
  } catch (_) { }
});

els.paste.addEventListener("click", async () => {
  try {
    const text = await navigator.clipboard.readText();
    if (text) els.key.value = text;
  } catch (_) { }
});

els.start.addEventListener("click", async () => {
  if (els.start.disabled) return;
  if (currentKind === "idle") {
    await fetch("/api/start", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ workType, tunKey: els.key.value }),
    });
    return;
  }
  await fetch("/api/stop", { method: "POST" });
});

function hideExited() {
  els.overlay.classList.add("hidden");
}

function showExited() {
  els.overlay.classList.remove("hidden");
  setControlsEnabled(false);
  els.start.disabled = true;
}

async function checkAlive() {
  try {
    const resp = await fetch("/api/state", { cache: "no-store" });
    if (resp.ok) return;
  } catch (_) { }
  showExited();
}

function connectEvents() {
  const es = new EventSource("/api/events");
  es.addEventListener("snapshot", (e) => {
    connectedOnce = true;
    hideExited();
    const s = JSON.parse(e.data);
    applyState(s);
    renderLogs(s.logs || []);
  });
  es.addEventListener("state", (e) => applyState(JSON.parse(e.data)));
  es.addEventListener("log", (e) => appendLog(JSON.parse(e.data).line));
  es.onopen = () => {
    connectedOnce = true;
    hideExited();
  };
  es.onerror = () => {
    if (!connectedOnce) return;
    checkAlive();
  };
}

connectEvents();
