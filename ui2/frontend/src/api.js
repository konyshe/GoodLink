export async function generateKey() {
  const resp = await fetch("/api/key/generate", { method: "POST" });
  if (!resp.ok) return null;
  const data = await resp.json();
  return data.tunKey || null;
}

export async function start(workType, tunKey) {
  await fetch("/api/start", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ workType, tunKey }),
  });
}

export async function stop() {
  await fetch("/api/stop", { method: "POST" });
}

export async function fetchState() {
  const resp = await fetch("/api/state", { cache: "no-store" });
  return resp;
}
