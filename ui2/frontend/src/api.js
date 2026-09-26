export async function generateKey() {
  const resp = await fetch("/api/key/generate", { method: "POST" });
  if (!resp.ok) return null;
  const data = await resp.json();
  return data.tunKey || null;
}

async function postJSON(url, body) {
  const resp = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body || {}),
  });
  const data = await resp.json().catch(() => ({}));
  if (!resp.ok) return { error: data.error || "请求失败", state: null };
  return { error: null, state: data };
}

export function createService() {
  return postJSON("/api/services", {});
}

export function deleteService(id) {
  return fetch(`/api/services/${encodeURIComponent(id)}`, { method: "DELETE" }).then(async (resp) => {
    const data = await resp.json().catch(() => ({}));
    if (!resp.ok) return { error: data.error || "删除失败" };
    return { error: null, state: data };
  });
}

export function renameService(id, name) {
  return postJSON(`/api/services/${encodeURIComponent(id)}/rename`, { name });
}

export function start(id, workType, tunKey, localMode, forwardRules, transport, allowLocalProxy) {
  return postJSON(`/api/services/${encodeURIComponent(id)}/start`, {
    workType,
    tunKey,
    localMode,
    transport,
    forwardRules,
    allowLocalProxy: allowLocalProxy !== false,
  });
}

export function stop(id) {
  return postJSON(`/api/services/${encodeURIComponent(id)}/stop`, {});
}

export function applyForwards(id, localMode, forwardRules) {
  return postJSON(`/api/services/${encodeURIComponent(id)}/forwards`, { localMode, forwardRules });
}

export function saveAutoStart(id, autoStart, workType, tunKey, localMode, forwardRules, transport, allowLocalProxy) {
  return postJSON(`/api/services/${encodeURIComponent(id)}/autostart`, {
    autoStart,
    workType,
    tunKey,
    localMode,
    transport,
    forwardRules,
    allowLocalProxy: allowLocalProxy !== false,
  });
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
