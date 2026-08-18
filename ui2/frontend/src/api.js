export async function generateKey() {
  const resp = await fetch("/api/key/generate", { method: "POST" });
  if (!resp.ok) return null;
  const data = await resp.json();
  return data.tunKey || null;
}

export async function start(workType, tunKey, localMode, forwardRules) {
  const resp = await fetch("/api/start", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ workType, tunKey, localMode, forwardRules }),
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

export async function fetchState() {
  const resp = await fetch("/api/state", { cache: "no-store" });
  return resp;
}
