import { useEffect, useRef, useState } from "react";
import { applyForwards, generateKey, start, stop } from "./api";
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

const ADMIN_HINT = "TUN模式需要管理员权限重新启动Goodlink";

function needsAdminHint(nextWorkType, nextLocalMode, isAdmin) {
  return nextWorkType === "Local" && nextLocalMode === "tun" && isAdmin === false;
}

function newRow() {
  return {
    id: nextRowId++,
    proto: "tcp",
    listenHost: "127.0.0.1",
    listenPort: "",
    remoteHost: "127.0.0.1",
    remotePort: "",
  };
}

function splitHostPort(addr) {
  const value = addr || "";
  const i = value.lastIndexOf(":");
  if (i <= 0) return { host: "", port: "" };
  return { host: value.slice(0, i), port: value.slice(i + 1) };
}

function rowsFromRules(rules) {
  if (!rules || !rules.length) return [];
  return rules.map((r) => {
    const listen = splitHostPort(r.listen);
    const remote = splitHostPort(r.remote);
    return {
      id: nextRowId++,
      proto: r.proto || "tcp",
      listenHost: listen.host || "127.0.0.1",
      listenPort: listen.port || "",
      remoteHost: remote.host || "127.0.0.1",
      remotePort: remote.port || "",
    };
  });
}

function rulesFromRows(rows) {
  return rows
    .filter((r) => String(r.listenPort || "").trim() !== "" || String(r.remotePort || "").trim() !== "")
    .map((r) => ({
      proto: r.proto || "tcp",
      listen: `${(r.listenHost || "").trim()}:${(r.listenPort || "").trim()}`,
      remote: `${(r.remoteHost || "").trim()}:${(r.remotePort || "").trim()}`,
    }));
}

export default function App() {
  const { state, logs, exited } = useUIEvents();
  const [workType, setWorkType] = useState("Local");
  const [tunKey, setTunKey] = useState("");
  const [localMode, setLocalMode] = useState("tun");
  const [rows, setRows] = useState([]);
  const [formError, setFormError] = useState("");
  const [adminHint, setAdminHint] = useState(false);
  const initialized = useRef(false);
  const keyInputRef = useRef(null);
  const logRef = useRef(null);

  useEffect(() => {
    if (!state) return;
    if (!initialized.current) {
      setWorkType(state.workType || "Local");
      setTunKey(state.tunKey || "");
      setLocalMode(state.localMode || "tun");
      setRows(rowsFromRules(state.forwardRules));
      initialized.current = true;
      if (needsAdminHint(state.workType || "Local", state.localMode || "tun", state.isAdmin)) {
        setAdminHint(true);
      }
      return;
    }
    if (!state.button.othersEnabled) {
      setWorkType(state.workType || "Local");
      if (document.activeElement !== keyInputRef.current) {
        setTunKey(state.tunKey || "");
      }
      setLocalMode(state.localMode || "tun");
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
  const mappingEnabled = !exited && workType === "Local" && localMode === "forward";
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

  function updateRow(id, field, value) {
    setRows((prev) => prev.map((row) => (row.id === id ? { ...row, [field]: value } : row)));
  }

  function removeRow(id) {
    setRows((prev) => prev.filter((row) => row.id !== id));
  }

  async function onConfirm() {
    if (!mappingEnabled) return;
    setFormError("");
    const result = await applyForwards(localMode, rulesFromRows(rows));
    if (result.error) {
      setFormError(result.error);
    }
  }

  async function onStart() {
    if (!startEnabled) return;
    if (button.kind === "idle") {
      setFormError("");
      const result = await start(workType, tunKey, localMode, rulesFromRows(rows));
      if (result.error) {
        setFormError(result.error);
        if (String(result.error).includes("管理员权限")) {
          setAdminHint(true);
        }
      }
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
              onClick={() => {
                setWorkType("Local");
                if (needsAdminHint("Local", localMode, state?.isAdmin)) {
                  setAdminHint(true);
                }
              }}
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

        {workType === "Local" && (
          <section className="row">
            <span className="label">工作模式:</span>
            <div className="work-type">
              <button
                type="button"
                className={"work-btn" + (localMode === "tun" ? " active" : "")}
                disabled={!othersEnabled}
                onClick={() => {
                  setLocalMode("tun");
                  if (needsAdminHint(workType, "tun", state?.isAdmin)) {
                    setAdminHint(true);
                  }
                }}
              >
                TUN模式
              </button>
              <span className="vsep"></span>
              <button
                type="button"
                className={"work-btn" + (localMode === "forward" ? " active" : "")}
                disabled={!othersEnabled}
                onClick={() => setLocalMode("forward")}
              >
                转发模式
              </button>
            </div>
          </section>
        )}

        {workType === "Local" && localMode === "forward" && (
          <section className="forwards">
            <div className="forwards-head">
              <span className="label">端口映射:</span>
              <div className="forwards-actions">
                <button type="button" disabled={!mappingEnabled} onClick={() => setRows((prev) => [...prev, newRow()])}>
                  添加
                </button>
                <button type="button" disabled={!mappingEnabled} onClick={onConfirm}>
                  确认
                </button>
              </div>
            </div>
            <div className="forwards-table">
              <div className="forwards-row forwards-header">
                <span>协议</span>
                <span>本地地址</span>
                <span>本地端口</span>
                <span>Remote地址</span>
                <span>Remote端口</span>
                <span></span>
              </div>
              {rows.length === 0 && (
                <div className="forwards-empty">点击添加以配置端口映射，确认后立即生效</div>
              )}
              {rows.map((row) => (
                <div className="forwards-row" key={row.id}>
                  <select
                    value={row.proto}
                    disabled={!mappingEnabled}
                    onChange={(e) => updateRow(row.id, "proto", e.target.value)}
                  >
                    <option value="tcp">TCP</option>
                    <option value="udp">UDP</option>
                  </select>
                  <input
                    type="text"
                    value={row.listenHost}
                    disabled={!mappingEnabled}
                    spellCheck="false"
                    autoComplete="off"
                    onChange={(e) => updateRow(row.id, "listenHost", e.target.value)}
                  />
                  <input
                    type="text"
                    inputMode="numeric"
                    value={row.listenPort}
                    disabled={!mappingEnabled}
                    spellCheck="false"
                    autoComplete="off"
                    onChange={(e) => updateRow(row.id, "listenPort", e.target.value)}
                  />
                  <input
                    type="text"
                    value={row.remoteHost}
                    disabled={!mappingEnabled}
                    spellCheck="false"
                    autoComplete="off"
                    onChange={(e) => updateRow(row.id, "remoteHost", e.target.value)}
                  />
                  <input
                    type="text"
                    inputMode="numeric"
                    value={row.remotePort}
                    disabled={!mappingEnabled}
                    spellCheck="false"
                    autoComplete="off"
                    onChange={(e) => updateRow(row.id, "remotePort", e.target.value)}
                  />
                  <button type="button" disabled={!mappingEnabled} onClick={() => removeRow(row.id)}>
                    删除
                  </button>
                </div>
              ))}
            </div>
          </section>
        )}

        {formError && <div className="form-error">{formError}</div>}

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
      <div
        className={"exit-overlay" + (!exited && adminHint ? "" : " hidden")}
        onClick={() => setAdminHint(false)}
      >
        <div className="exit-card admin-card" onClick={(e) => e.stopPropagation()}>
          <div>{ADMIN_HINT}</div>
          <button type="button" onClick={() => setAdminHint(false)}>确定</button>
        </div>
      </div>
    </>
  );
}
