import { useEffect, useRef, useState } from "react";
import { applyForwards, generateKey, start, stop } from "./api";
import LocalPanel, { rowsFromRules, rulesFromRows } from "./LocalPanel";
import RemotePanel from "./RemotePanel";
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

const TUN_PROXY = {
  socks: "socks5://192.17.19.1:1080",
  http: "http://192.17.19.1:1080",
  fallback: false,
};

const FWD_PROXY = {
  socks: "socks5://127.0.0.1:1080",
  http: "http://127.0.0.1:1080",
  fallback: false,
};

function resolveProxy(localMode, proxy) {
  if (localMode === "tun") {
    return TUN_PROXY;
  }
  if (proxy?.socks && String(proxy.socks).includes("127.0.0.1")) {
    return {
      socks: proxy.socks,
      http: proxy.http || String(proxy.socks).replace("socks5://", "http://"),
      fallback: !!proxy.fallback,
    };
  }
  return FWD_PROXY;
}

function needsAdminHint(nextWorkType, nextLocalMode, isAdmin) {
  return nextWorkType === "Local" && nextLocalMode === "tun" && isAdmin === false;
}

const ADMIN_HINT = "TUN模式, 需要管理员权限重新启动Goodlink";

export default function App() {
  const { state, logs, exited } = useUIEvents();
  const [workType, setWorkType] = useState("Local");
  const [tunKey, setTunKey] = useState("");
  const [localMode, setLocalMode] = useState("tun");
  const [rows, setRows] = useState([]);
  const [formError, setFormError] = useState("");
  const [adminHint, setAdminHint] = useState(false);
  const [conflictHint, setConflictHint] = useState("");
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

  useEffect(() => {
    if (state?.version) {
      document.title = "Goodlink v" + state.version;
    }
  }, [state?.version]);

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

  async function onConfirm() {
    if (!mappingEnabled) return;
    setFormError("");
    setConflictHint("");
    const result = await applyForwards(localMode, rulesFromRows(rows));
    if (result.error) {
      if (String(result.error).includes("端口已被占用")) {
        setConflictHint(result.error);
        return;
      }
      setFormError(result.error);
    }
  }

  async function onStart() {
    if (!startEnabled) return;
    if (button.kind === "idle") {
      setFormError("");
      setConflictHint("");
      const result = await start(workType, tunKey, localMode, rulesFromRows(rows));
      if (result.error) {
        if (String(result.error).includes("端口已被占用")) {
          setConflictHint(result.error);
          return;
        }
        setFormError(result.error);
        if (String(result.error).includes("管理员权限")) {
          setAdminHint(true);
        }
      }
      return;
    }
    await stop();
  }

  function onExportLogs() {
    if (!logs.length) return;
    const pad = (n) => String(n).padStart(2, "0");
    const d = new Date();
    const name = `goodlink-${d.getFullYear()}${pad(d.getMonth() + 1)}${pad(d.getDate())}-${pad(d.getHours())}${pad(d.getMinutes())}${pad(d.getSeconds())}.log`;
    const blob = new Blob([logs.join("\n") + "\n"], { type: "text/plain;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = name;
    a.click();
    URL.revokeObjectURL(url);
  }

  const keyProps = {
    tunKey,
    setTunKey,
    keyInputRef,
    othersEnabled,
    onGenerate,
    onCopy,
    onPaste,
  };

  return (
    <>
      <div className="app">
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
              Local端(本地客户端)
            </button>
            <span className="vsep"></span>
            <button
              type="button"
              className={"work-btn" + (workType === "Remote" ? " active" : "")}
              disabled={!othersEnabled}
              onClick={() => setWorkType("Remote")}
            >
              Remote端(被连接服务端)
            </button>
          </div>
        </section>

        {workType === "Local" ? (
          <LocalPanel
            {...keyProps}
            localMode={localMode}
            setLocalMode={setLocalMode}
            rows={rows}
            setRows={setRows}
            mappingEnabled={mappingEnabled}
            onConfirm={onConfirm}
            onNeedAdminHint={() => {
              if (needsAdminHint(workType, "tun", state?.isAdmin)) {
                setAdminHint(true);
              }
            }}
            proxy={resolveProxy(localMode, state?.proxy)}
          />
        ) : (
          <RemotePanel {...keyProps} />
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
          <div className="logs-head">
            <div className="label">运行日志:</div>
            <button type="button" disabled={!logs.length} onClick={onExportLogs}>导出日志</button>
          </div>
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
      <div
        className={"exit-overlay" + (!exited && conflictHint ? "" : " hidden")}
        onClick={() => setConflictHint("")}
      >
        <div className="exit-card admin-card" onClick={(e) => e.stopPropagation()}>
          <div>{conflictHint}</div>
          <button type="button" onClick={() => setConflictHint("")}>确定</button>
        </div>
      </div>
    </>
  );
}
