import { useEffect, useRef, useState } from "react";
import { generateKey, start, stop } from "./api";
import { useUIEvents } from "./useUIEvents";

const defaultButton = {
  kind: "initializing",
  text: "检测网络中...",
  importance: "high",
  enabled: false,
  activity: false,
  othersEnabled: true,
};

const defaultNat = {
  ready: false,
  isNAT4: false,
  text: "正在检测当前网络环境...",
};

export default function App() {
  const { state, logs, exited } = useUIEvents();
  const [workType, setWorkType] = useState("Local");
  const [tunKey, setTunKey] = useState("");
  const initialized = useRef(false);
  const keyInputRef = useRef(null);
  const logRef = useRef(null);

  useEffect(() => {
    if (!state) return;
    if (!initialized.current || !state.button.othersEnabled) {
      setWorkType(state.workType || "Local");
      if (document.activeElement !== keyInputRef.current) {
        setTunKey(state.tunKey || "");
      }
      initialized.current = true;
    }
  }, [state]);

  useEffect(() => {
    if (logRef.current) {
      logRef.current.scrollTop = logRef.current.scrollHeight;
    }
  }, [logs]);

  const button = state?.button || defaultButton;
  const nat = state?.nat || defaultNat;
  const othersEnabled = !exited && !!button.othersEnabled;
  const startEnabled = !exited && !!button.enabled;
  const natClass = nat.ready ? (nat.isNAT4 ? "warn" : "ok") : "detecting";

  async function onGenerate() {
    const key = await generateKey();
    if (key) setTunKey(key);
  }

  async function onCopy() {
    try {
      await navigator.clipboard.writeText(tunKey || "");
    } catch (_) { }
  }

  async function onPaste() {
    try {
      const text = await navigator.clipboard.readText();
      if (text) setTunKey(text);
    } catch (_) { }
  }

  async function onStart() {
    if (!startEnabled) return;
    if (button.kind === "idle") {
      await start(workType, tunKey);
      return;
    }
    await stop();
  }

  return (
    <>
      <div className="app">
        <header className="header">
          <h1>Goodlink{state ? "  v" + (state.version || "") : ""}</h1>
        </header>

        <section className="row">
          <span className="label">工作端侧:</span>
          <div className="work-type">
            <button
              type="button"
              className={"work-btn" + (workType === "Local" ? " active" : "")}
              disabled={!othersEnabled}
              onClick={() => setWorkType("Local")}
            >
              Local端
            </button>
            <span className="vsep"></span>
            <button
              type="button"
              className={"work-btn" + (workType === "Remote" ? " active" : "")}
              disabled={!othersEnabled}
              onClick={() => setWorkType("Remote")}
            >
              Remote端
            </button>
          </div>
        </section>

        <section className="row">
          <span className="label">连接密钥:</span>
          <input
            ref={keyInputRef}
            className="key-input"
            type="text"
            placeholder="16-64字节长度"
            autoComplete="off"
            spellCheck="false"
            value={tunKey}
            disabled={!othersEnabled}
            onChange={(e) => setTunKey(e.target.value)}
          />
        </section>

        <section className="key-actions">
          <button type="button" disabled={!othersEnabled} onClick={onGenerate}>生成密钥</button>
          <button type="button" onClick={onCopy}>复制密钥</button>
          <button type="button" disabled={!othersEnabled} onClick={onPaste}>粘贴密钥</button>
        </section>

        <button
          type="button"
          className={"start-btn " + (button.importance || "high")}
          disabled={!startEnabled}
          onClick={onStart}
        >
          <span className={"spinner" + (button.activity ? "" : " hidden")}></span>
          <span>{button.text || ""}</span>
        </button>

        <section className="logs">
          <div className="label">运行日志:</div>
          <pre id="log-list" ref={logRef}>{logs.join("\n")}</pre>
        </section>

        <footer className="footer">
          <div className={"nat " + natClass}>{nat.text || ""}</div>
          <div className="footer-links">
            <a
              className={"upgrade" + (state?.upgrade?.need ? "" : " hidden")}
              href="https://gitee.com/konyshe/goodlink/releases"
              target="_blank"
              rel="noopener"
            >
              有新版本! <span>{state?.upgrade?.need ? "v" + state.upgrade.latest : ""}</span>
            </a>
            <a href="https://gitee.com/konyshe/goodlink/issues" target="_blank" rel="noopener">反馈问题</a>
          </div>
        </footer>
      </div>
      <div className={"exit-overlay" + (exited ? "" : " hidden")}>
        <div className="exit-card">程序已退出，请重新启动 Goodlink</div>
      </div>
    </>
  );
}
