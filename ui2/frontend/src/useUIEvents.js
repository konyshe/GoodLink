import { useEffect, useState } from "react";
import { fetchState } from "./api";

export function useUIEvents() {
  const [state, setState] = useState(null);
  const [logs, setLogs] = useState([]);
  const [exited, setExited] = useState(false);

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

    es.addEventListener("snapshot", (e) => {
      connectedOnce = true;
      hideExited();
      const s = JSON.parse(e.data);
      setState(s);
      setLogs(s.logs || []);
    });
    es.addEventListener("state", (e) => setState(JSON.parse(e.data)));
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

  return { state, logs, exited };
}
