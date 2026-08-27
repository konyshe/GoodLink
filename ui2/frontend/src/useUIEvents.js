import { useEffect, useState } from "react";
import { fetchState } from "./api";

const defaultUpgrade = {
  downloading: false,
  percent: 0,
  done: false,
  error: "",
};

export function useUIEvents() {
  const [state, setState] = useState(null);
  const [logs, setLogs] = useState([]);
  const [exited, setExited] = useState(false);
  const [upgradeProgress, setUpgradeProgress] = useState(defaultUpgrade);

  useEffect(() => {
    const es = new EventSource("/api/events");
    let connectedOnce = false;

    const hideExited = () => setExited(false);

    const checkAlive = async () => {
      try {
        const resp = await fetchState();
        if (resp.ok) return;
      } catch (_) { }
      setExited(true);
    };

    const applyUpgrade = (u) => {
      if (!u) return;
      setUpgradeProgress({
        downloading: !!u.downloading,
        percent: u.percent || 0,
        done: !!u.done,
        error: u.error || "",
      });
      setState((prev) => (prev ? { ...prev, upgrade: { ...prev.upgrade, ...u } } : prev));
    };

    es.addEventListener("snapshot", (e) => {
      connectedOnce = true;
      hideExited();
      const s = JSON.parse(e.data);
      setState(s);
      setLogs(s.logs || []);
      if (s.upgrade) {
        setUpgradeProgress({
          downloading: !!s.upgrade.downloading,
          percent: s.upgrade.percent || 0,
          done: !!s.upgrade.done,
          error: s.upgrade.error || "",
        });
      }
    });
    es.addEventListener("state", (e) => setState(JSON.parse(e.data)));
    es.addEventListener("upgrade", (e) => applyUpgrade(JSON.parse(e.data)));
    es.addEventListener("log", (e) => {
      const line = JSON.parse(e.data).line;
      setLogs((prev) => [...prev, line]);
    });
    es.onopen = () => {
      connectedOnce = true;
      hideExited();
    };
    es.onerror = () => {
      if (!connectedOnce) return;
      checkAlive();
    };

    return () => es.close();
  }, []);

  return { state, logs, exited, upgradeProgress };
}
