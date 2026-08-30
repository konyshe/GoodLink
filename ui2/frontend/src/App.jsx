import { useEffect, useRef, useState } from "react";
import { applyForwards, applyUpgrade, fetchUpgradeInfo, generateKey, importConfig, probeState, saveAutoStart, start, startUpgradeDownload, stop } from "./api";
import LocalPanel, { rowsFromRules, rulesFromRows } from "./LocalPanel";
import RemotePanel from "./RemotePanel";
import { useUIEvents } from "./useUIEvents";

const STATUS_DETECTING = 0;
const STATUS_IDLE = 1;
const STATUS_CONNECTING_NAT4 = 4;
const STATUS_CONNECT_FAILED = 8;

function viewFromStatus(status, workType) {
  const remote = workType === "Remote";
  switch (status) {
    case 0:
      return { text: "检测网络中...", importance: "high", enabled: false, activity: false, othersEnabled: true };
    case 1:
      return { text: remote ? "点击启动" : "点击连接", importance: "high", enabled: true, activity: false, othersEnabled: true };
    case 2:
      return { text: "启动中...", importance: "warning", enabled: true, activity: true, othersEnabled: false };
    case 3:
      return { text: "连接中...", importance: "warning", enabled: true, activity: true, othersEnabled: false };
    case 4:
      return { text: "两端网络都是NAT4, 无法直连", importance: "danger", enabled: false, activity: false, othersEnabled: false };
    case 5:
      return { text: "连接成功, 点击断开", importance: "success", enabled: true, activity: false, othersEnabled: false };
    case 6:
      return { text: "启动成功, 点击停止", importance: "success", enabled: true, activity: false, othersEnabled: false };
    case 7:
      return { text: "停止中...", importance: "warning", enabled: false, activity: false, othersEnabled: false };
    case 8:
      return { text: "多次尝试仍无法连接", importance: "danger", enabled: false, activity: false, othersEnabled: false };
    default:
      return { text: "检测网络中...", importance: "high", enabled: false, activity: false, othersEnabled: true };
  }
}

function natHint(nat) {
  if (!nat?.ready) return "正在检测本地网络环境...";
  if (nat.isNAT4) return "本地网络为NAT4，如果对端也是NAT4，则无法直连";
  return "本地网络为NAT1-3";
}

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
const NAT4_HINT = "两端网络都是NAT4，无法直连，已停止连接。";
const RETRY_FAIL_HINT = "已尝试15次仍无法连接，已停止。";
const CONNECT_FAIL_HELP_URL = "https://gitee.com/konyshe/goodlink/wikis/%E4%B8%BA%E4%BD%95%E6%97%A0%E6%B3%95%E8%BF%9E%E6%8E%A5";
const APPLY_FETCH_TIMEOUT_MS = 2500;
const APPLY_RECONNECT_MS = 5000;
const APPLY_OVERALL_MS = 45000;

function sleep(ms) {
  return new Promise((r) => setTimeout(r, ms));
}

function isRestartedVersion(fromVersion, version) {
  return !!(fromVersion && version && version !== fromVersion);
}

export default function App() {
  const { state, logs, exited, upgradeProgress } = useUIEvents();
  const [workType, setWorkType] = useState("Local");
  const [tunKey, setTunKey] = useState("");
  const [localMode, setLocalMode] = useState("tun");
  const [transport, setTransport] = useState("kcp");
  const [autoStart, setAutoStart] = useState(false);
  const [rows, setRows] = useState([]);
  const [formError, setFormError] = useState("");
  const [adminHint, setAdminHint] = useState(false);
  const [failHint, setFailHint] = useState("");
  const [conflictHint, setConflictHint] = useState("");
  const [upgradeOpen, setUpgradeOpen] = useState(false);
  const [upgradeBody, setUpgradeBody] = useState("");
  const [upgradeModalError, setUpgradeModalError] = useState("");
  const [applying, setApplying] = useState(false);
  const [reconnectTimedOut, setReconnectTimedOut] = useState(false);
  const initialized = useRef(false);
  const keyInputRef = useRef(null);
  const fileInputRef = useRef(null);
  const logRef = useRef(null);
  const applyFromVersionRef = useRef("");

  useEffect(() => {
    if (!state) return;
    if (!initialized.current) {
      setWorkType(state.workType || "Local");
      setTunKey(state.tunKey || "");
      setLocalMode(state.localMode || "tun");
      setTransport(state.transport || "kcp");
      setAutoStart(!!state.autoStart);
      setRows(rowsFromRules(state.forwardRules));
      initialized.current = true;
      if (needsAdminHint(state.workType || "Local", state.localMode || "tun", state.isAdmin)) {
        setAdminHint(true);
      }
      return;
    }
    setAutoStart(!!state.autoStart);
    const st = state.status ?? STATUS_DETECTING;
    if (st !== STATUS_DETECTING && st !== STATUS_IDLE) {
      setWorkType(state.workType || "Local");
      if (document.activeElement !== keyInputRef.current) {
        setTunKey(state.tunKey || "");
      }
      setLocalMode(state.localMode || "tun");
      setTransport(state.transport || "kcp");
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

  useEffect(() => {
    if (!applying) return;
    const fromVersion = applyFromVersionRef.current;
    if (isRestartedVersion(fromVersion, state?.version)) {
      window.location.reload();
    }
  }, [applying, state?.version]);

  useEffect(() => {
    const st = state?.status ?? STATUS_DETECTING;
    if (st === STATUS_CONNECTING_NAT4) {
      setFailHint(NAT4_HINT);
      stop();
      return;
    }
    if (st === STATUS_CONNECT_FAILED) {
      setFailHint(RETRY_FAIL_HINT);
      stop();
    }
  }, [state?.status]);

  useEffect(() => {
    if (!applying) return;
    let stopped = false;
    const fromVersion = applyFromVersionRef.current;
    (async () => {
      let sawDown = false;
      let downAt = 0;
      const startedAt = Date.now();
      for (; ;) {
        if (stopped) return;
        const { up, version, timedOut } = await probeState(APPLY_FETCH_TIMEOUT_MS);
        if (stopped) return;
        if (up && (isRestartedVersion(fromVersion, version) || sawDown)) {
          window.location.reload();
          return;
        }
        const now = Date.now();
        if (!up && !timedOut && !sawDown) {
          sawDown = true;
          downAt = now;
        }
        if (sawDown && now - downAt >= APPLY_RECONNECT_MS) {
          setReconnectTimedOut(true);
          return;
        }
        if (now - startedAt >= APPLY_OVERALL_MS) {
          setReconnectTimedOut(true);
          return;
        }
        const wait = sawDown
          ? Math.min(500, Math.max(0, APPLY_RECONNECT_MS - (now - downAt)))
          : 200;
        if (wait > 0) {
          await sleep(wait);
        }
      }
    })();
    return () => {
      stopped = true;
    };
  }, [applying]);

  const status = state?.status ?? STATUS_DETECTING;
  const view = viewFromStatus(status, workType);
  const nat = state?.nat || {};
  const othersEnabled = !exited && !!view.othersEnabled;
  const startEnabled = !exited && !!view.enabled;
  const mappingEnabled = !exited && workType === "Local" && localMode === "forward";
  const natClass = nat.ready ? (nat.isNAT4 ? "warn" : "ok") : "detecting";
  const autoLabel = workType === "Remote" ? "程序启动后，自动启动服务端" : "程序启动后，自动连接Remote端";

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

  function onExportConfig() {
    const cfg = {
      work_type: workType,
      tun_key: tunKey || "",
      local_mode: localMode || "tun",
      transport: transport || "kcp",
      auto_start: autoStart,
      forward_rules: rulesFromRows(rows),
    };
    const blob = new Blob([JSON.stringify(cfg, null, 2) + "\n"], { type: "application/json;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "goodlink.json";
    a.click();
    URL.revokeObjectURL(url);
  }

  function onImportClick() {
    if (!othersEnabled) return;
    fileInputRef.current?.click();
  }

  async function onImportFile(e) {
    const file = e.target.files?.[0];
    e.target.value = "";
    if (!file) return;
    setFormError("");
    setConflictHint("");
    let cfg;
    try {
      cfg = JSON.parse(await file.text());
    } catch (_) {
      setFormError("配置文件格式错误");
      return;
    }
    const result = await importConfig(cfg);
    if (result.error) {
      setFormError(result.error);
      return;
    }
    const next = result.state || {};
    const nextWorkType = next.workType || "Local";
    const nextLocalMode = next.localMode || "tun";
    setWorkType(nextWorkType);
    setTunKey(next.tunKey || "");
    setLocalMode(nextLocalMode);
    setTransport(next.transport || "kcp");
    setAutoStart(!!next.autoStart);
    setRows(rowsFromRules(next.forwardRules));
    if (needsAdminHint(nextWorkType, nextLocalMode, state?.isAdmin)) {
      setAdminHint(true);
    }
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
    if (status === STATUS_IDLE) {
      setFormError("");
      setConflictHint("");
      const result = await start(workType, tunKey, localMode, rulesFromRows(rows), transport);
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

  async function onAutoStartToggle(enabled) {
    if (exited) return;
    setFormError("");
    setConflictHint("");
    setAutoStart(enabled);
    const result = await saveAutoStart(enabled, workType, tunKey, localMode, rulesFromRows(rows), transport);
    if (result.error) {
      if (String(result.error).includes("端口已被占用")) {
        setConflictHint(result.error);
        return;
      }
      setFormError(result.error);
      if (String(result.error).includes("管理员权限")) {
        setAdminHint(true);
        return;
      }
      setAutoStart(!enabled);
    }
  }

  async function openUpgradeModal() {
    setUpgradeOpen(true);
    setUpgradeModalError("");
    setUpgradeBody(state?.upgrade?.body || "");
    const info = await fetchUpgradeInfo();
    if (info.error) {
      setUpgradeModalError(info.error);
      return;
    }
    setUpgradeBody(info.body || "");
  }

  async function onUpgradeDownload() {
    if (upgradeProgress.downloading) return;
    setUpgradeModalError("");
    const result = await startUpgradeDownload();
    if (result.error) {
      setUpgradeModalError(result.error);
    }
  }

  async function onUpgradeApply() {
    if (!upgradeProgress.done || applying) return;
    setUpgradeModalError("");
    setReconnectTimedOut(false);
    applyFromVersionRef.current = state?.version || "";
    setApplying(true);
    const result = await applyUpgrade();
    if (result.error) {
      setApplying(false);
      setUpgradeModalError(result.error);
    }
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
    fileInputRef,
    othersEnabled,
    onGenerate,
    onCopy,
    onPaste,
    onExportConfig,
    onImportClick,
    onImportFile,
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

        {workType === "Remote" && (
          <section className="row">
            <span className="label">传输协议:</span>
            <div className="work-type work-type-sm">
              <button
                type="button"
                className={"work-btn" + (transport === "kcp" ? " active" : "")}
                disabled={!othersEnabled}
                onClick={() => setTransport("kcp")}
              >
                KCP(CPU降低40%, 响应提升30%, 适合个人)
              </button>
              <span className="vsep"></span>
              <button
                type="button"
                className={"work-btn" + (transport === "quic" ? " active" : "")}
                disabled={!othersEnabled}
                onClick={() => setTransport("quic")}
              >
                QUIC(二次加密, 传输稳定, 适合企业)
              </button>
            </div>
          </section>
        )}

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

        <section className="row">
          <span className="label">{autoLabel}:</span>
          <button
            type="button"
            className={"switch" + (autoStart ? " on" : "")}
            disabled={exited}
            role="switch"
            aria-label={autoLabel}
            aria-checked={autoStart}
            onClick={() => onAutoStartToggle(!autoStart)}
          >
            <span className="switch-knob"></span>
          </button>
        </section>

        <button
          type="button"
          className={"start-btn " + (view.importance || "high")}
          disabled={!startEnabled}
          onClick={onStart}
        >
          <span className={"spinner" + (view.activity ? "" : " hidden")}></span>
          <span>{view.text}</span>
        </button>

        <section className="logs">
          <div className="logs-head">
            <div className="label">运行日志:</div>
            <button type="button" disabled={!logs.length} onClick={onExportLogs}>导出日志</button>
          </div>
          <pre id="log-list" ref={logRef}>{logs.join("\n")}</pre>
        </section>

        <footer className="footer">
          <div className={"nat " + natClass}>{natHint(nat)}</div>
          <div className="footer-links">
            {state?.upgrade?.checkError ? (
              <a
                className="footer-chip upgrade-fail"
                href="https://gitee.com/konyshe/goodlink/releases"
                target="_blank"
                rel="noopener"
              >
                {state.upgrade.checkError}
              </a>
            ) : (
              <button
                type="button"
                className={"footer-chip upgrade" + (state?.upgrade?.need ? "" : " hidden")}
                onClick={openUpgradeModal}
              >
                <span className="upgrade-aura" aria-hidden="true"></span>
                <span className="upgrade-shine" aria-hidden="true"></span>
                <span className="upgrade-spark" aria-hidden="true"></span>
                <span className="upgrade-dot" aria-hidden="true"></span>
                <span className="upgrade-label">有新版本</span>
                {state?.upgrade?.need ? (
                  <span className="upgrade-ver">v{state.upgrade.latest}</span>
                ) : null}
              </button>
            )}
            <a
              className="footer-chip feedback"
              href="https://goodlink.kony.vip"
              target="_blank"
              rel="noopener"
            >
              官方网站
            </a>
            <a
              className="footer-chip feedback"
              href="https://gitee.com/konyshe/goodlink/wikis/pages"
              target="_blank"
              rel="noopener"
            >
              使用教程
            </a>
            <a
              className="footer-chip feedback"
              href="https://gitee.com/konyshe/goodlink/issues"
              target="_blank"
              rel="noopener"
            >
              反馈问题
            </a>
          </div>
        </footer>
      </div>
      <div className={"exit-overlay" + (exited && !applying ? "" : " hidden")}>
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
        className={"exit-overlay" + (!exited && failHint ? "" : " hidden")}
        onClick={() => setFailHint("")}
      >
        <div className="exit-card admin-card" onClick={(e) => e.stopPropagation()}>
          <div>{failHint}</div>
          <a className="admin-help" href={CONNECT_FAIL_HELP_URL} target="_blank" rel="noopener">
            查看解决方法
          </a>
          <button type="button" onClick={() => setFailHint("")}>确定</button>
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
      <div
        className={"exit-overlay" + ((applying || (!exited && upgradeOpen)) ? "" : " hidden")}
      >
        {applying ? (
          reconnectTimedOut ? (
            <div className="exit-card admin-card upgrade-card upgrade-applying" onClick={(e) => e.stopPropagation()}>
              <div className="upgrade-applying-title">更新完成</div>
              <div className="upgrade-applying-tip">超过 5 秒未能连接主程序，请手动重启 Goodlink</div>
            </div>
          ) : (
            <div className="exit-card admin-card upgrade-card upgrade-applying" onClick={(e) => e.stopPropagation()}>
              <div className="upgrade-loader">
                <span className="upgrade-ring"></span>
                <span className="upgrade-ring upgrade-ring-2"></span>
              </div>
              <div className="upgrade-applying-title">正在更新到 v{state?.upgrade?.latest || ""}</div>
              <div className="upgrade-applying-tip">程序将自动重启，请稍候<span className="dots"><i>.</i><i>.</i><i>.</i></span></div>
              <div className="upgrade-applying-sub">重启完成后页面会自动恢复，无需手动关闭</div>
            </div>
          )
        ) : (
          <div className="exit-card admin-card upgrade-card" onClick={(e) => e.stopPropagation()}>
            <div className="upgrade-title">新版本 v{state?.upgrade?.latest || ""}</div>
            <pre className="upgrade-body">{upgradeBody || "暂无更新说明"}</pre>
            <div className="upgrade-progress">
              <div className="progress-bar">
                <div className="progress-fill" style={{ width: Math.max(0, Math.min(100, upgradeProgress.percent || 0)) + "%" }}></div>
              </div>
              <span className="progress-pct">{upgradeProgress.percent || 0}%</span>
            </div>
            {upgradeModalError ? <div className="upgrade-error">{upgradeModalError}</div> : null}
            {upgradeProgress.error ? <div className="upgrade-error">{upgradeProgress.error}</div> : null}
            <div className="upgrade-actions">
              <button type="button" className="upgrade-back" onClick={() => setUpgradeOpen(false)}>返回</button>
              {upgradeProgress.done ? (
                <button type="button" onClick={onUpgradeApply}>更新</button>
              ) : (
                <button type="button" disabled={upgradeProgress.downloading} onClick={onUpgradeDownload}>
                  {upgradeProgress.downloading ? "下载中..." : "下载"}
                </button>
              )}
            </div>
          </div>
        )}
      </div>
    </>
  );
}
