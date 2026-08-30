export async function generateKey() {
  const resp = await fetch("/api/key/generate", { method: "POST" });
  if (!resp.ok) return null;
  const data = await resp.json();
  return data.tunKey || null;
}

export async function start(workType, tunKey, localMode, forwardRules, transport) {
  const resp = await fetch("/api/start", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ workType, tunKey, localMode, transport, forwardRules }),
  });
  const data = await resp.json().catch(() => ({}));
  if (!resp.ok) {
    return { error: data.error || "启动失败" };
  }
  return { error: null };
}

export async function stop() {
  await fetch("/api/stop", { method: "POST" });
}

export async function applyForwards(localMode, forwardRules) {
  const resp = await fetch("/api/forwards", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ localMode, forwardRules }),
  });
  const data = await resp.json().catch(() => ({}));
  if (!resp.ok) {
    return { error: data.error || "保存失败" };
  }
  return { error: null };
}

export async function saveAutoStart(autoStart, workType, tunKey, localMode, forwardRules, transport) {
  const resp = await fetch("/api/autostart", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ autoStart, workType, tunKey, localMode, transport, forwardRules }),
  });
  const data = await resp.json().catch(() => ({}));
  if (!resp.ok) {
    return { error: data.error || "保存失败" };
  }
  return { error: null };
}

export async function fetchState() {
  const resp = await fetch("/api/state", { cache: "no-store" });
  return resp;
}

export async function probeState(timeoutMs = 2500) {
  const ctrl = new AbortController();
  const timer = setTimeout(() => ctrl.abort(), timeoutMs);
  try {
    const resp = await fetch("/api/state", { cache: "no-store", signal: ctrl.signal });
    if (!resp.ok) {
      return { up: false, version: "", timedOut: false };
    }
    const data = await resp.json().catch(() => ({}));
    return { up: true, version: data.version || "", timedOut: false };
  } catch (err) {
    const timedOut = !!err && (err.name === "AbortError" || err.name === "TimeoutError");
    return { up: false, version: "", timedOut };
  } finally {
    clearTimeout(timer);
  }
}

export async function importConfig(cfg) {
  const resp = await fetch("/api/config", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(cfg),
  });
  const data = await resp.json().catch(() => ({}));
  if (!resp.ok) {
    return { error: data.error || "导入失败" };
  }
  return { error: null, state: data };
}

export async function fetchUpgradeInfo() {
  const resp = await fetch("/api/upgrade/info", { cache: "no-store" });
  const data = await resp.json().catch(() => ({}));
  if (!resp.ok) {
    return { error: data.error || "获取更新信息失败" };
  }
  return { error: null, version: data.version || "", body: data.body || "", downloadUrl: data.downloadUrl || "" };
}

export async function startUpgradeDownload() {
  const resp = await fetch("/api/upgrade/download", { method: "POST" });
  const data = await resp.json().catch(() => ({}));
  if (!resp.ok) {
    return { error: data.error || "开始下载失败" };
  }
  return { error: null };
}

export async function applyUpgrade() {
  const resp = await fetch("/api/upgrade/apply", { method: "POST" });
  const data = await resp.json().catch(() => ({}));
  if (!resp.ok) {
    return { error: data.error || "更新失败" };
  }
  return { error: null };
}
