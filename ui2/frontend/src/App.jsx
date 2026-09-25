import {
  BookOutlined,
  CloudDownloadOutlined,
  CloudServerOutlined,
  CommentOutlined,
  DesktopOutlined,
  ExportOutlined,
  GlobalOutlined,
  LoadingOutlined,
  PauseCircleOutlined,
  PlayCircleOutlined,
  SafetyCertificateOutlined,
  ThunderboltOutlined,
} from "@ant-design/icons";
import {
  Alert,
  App as AntdApp,
  Badge,
  Button,
  Card,
  Flex,
  Form,
  Modal,
  Progress,
  Result,
  Space,
  Spin,
  Switch,
  Tag,
  Typography,
} from "antd";
import { useEffect, useRef, useState } from "react";
import { applyForwards, applyUpgrade, createService, deleteService, fetchUpgradeInfo, generateKey, importConfig, probeState, renameService, saveAutoStart, start, startUpgradeDownload, stop } from "./api";
import ChoiceGroup from "./ChoiceGroup";
import LocalPanel, { rowsFromRules, rulesFromRows } from "./LocalPanel";
import RemotePanel from "./RemotePanel";
import ServicePanel from "./ServicePanel";
import ServiceTabs from "./ServiceTabs";
import { useUIEvents } from "./useUIEvents";

const STATUS_DETECTING = 0;
const STATUS_IDLE = 1;
const STATUS_CONNECTING_NAT4 = 4;
const STATUS_CONNECT_FAILED = 8;
const STATUS_CONNECTED = 5;
const STATUS_STARTED = 6;

function viewFromStatus(status, workType) {
  const remote = workType === "Remote";
  switch (status) {
    case 0:
      return { text: "检测网络中...", importance: "high", enabled: false, activity: false, othersEnabled: true };
    case 1:
      return { text: remote ? "点击启动" : "点击连接", importance: "high", enabled: true, activity: false, othersEnabled: true };
    case 2:
      return { text: "启动中, 点击停止", importance: "warning", enabled: true, activity: true, othersEnabled: false };
    case 3:
      return { text: "连接中, 点击停止", importance: "warning", enabled: true, activity: true, othersEnabled: false };
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

function natTagColor(nat) {
  if (!nat?.ready) return "default";
  return nat.isNAT4 ? "warning" : "success";
}

function startBtnTone(importance) {
  switch (importance) {
    case "warning":
      return { color: "gold", variant: "solid" };
    case "danger":
      return { color: "danger", variant: "solid" };
    case "success":
      return { color: "green", variant: "solid" };
    default:
      return { color: "primary", variant: "solid" };
  }
}

function shortStatus(status) {
  switch (status) {
    case STATUS_IDLE:
      return "待启动";
    case 2:
      return "启动中";
    case 3:
      return "连接中";
    case STATUS_CONNECTING_NAT4:
      return "无法直连";
    case STATUS_CONNECTED:
      return "已连接";
    case STATUS_STARTED:
      return "运行中";
    case 7:
      return "停止中";
    case STATUS_CONNECT_FAILED:
      return "连接失败";
    case STATUS_DETECTING:
    default:
      return "检测中";
  }
}

function BrandMark() {
  return (
    <svg className="gl-logo" viewBox="0 0 32 32" aria-hidden="true">
      <defs>
        <linearGradient id="gl-logo-g" x1="0" y1="0" x2="1" y2="1">
          <stop offset="0%" stopColor="#2dd4bf" />
          <stop offset="100%" stopColor="#38bdf8" />
        </linearGradient>
      </defs>
      <rect x="1.5" y="1.5" width="29" height="29" rx="9" fill="url(#gl-logo-g)" opacity="0.16" />
      <rect x="1.5" y="1.5" width="29" height="29" rx="9" fill="none" stroke="url(#gl-logo-g)" strokeWidth="1.4" />
      <path d="M12.2 16a3.8 3.8 0 0 1 3.8-3.8h4.2" fill="none" stroke="url(#gl-logo-g)" strokeWidth="2.3" strokeLinecap="round" />
      <path d="M19.8 16a3.8 3.8 0 0 1-3.8 3.8h-4.2" fill="none" stroke="url(#gl-logo-g)" strokeWidth="2.3" strokeLinecap="round" />
      <circle cx="10.4" cy="16" r="2.1" fill="#2dd4bf" />
      <circle cx="21.6" cy="16" r="2.1" fill="#38bdf8" />
    </svg>
  );
}

function isKeyInputFocused(ref) {
  const cur = ref?.current;
  if (!cur) return false;
  const el = cur.input || cur.nativeElement || cur;
  return document.activeElement === el;
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
  if (proxy?.socks) {
    return {
      socks: proxy.socks,
      http: proxy.http || String(proxy.socks).replace("socks5://", "http://"),
      fallback: !!proxy.fallback,
    };
  }
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

function formFromService(svc) {
  return {
    name: svc.name || "",
    workType: svc.workType || "Local",
    tunKey: svc.tunKey || "",
    localMode: svc.localMode || "tun",
    transport: svc.transport || "kcp",
    autoStart: !!svc.autoStart,
    rows: rowsFromRules(svc.forwardRules),
  };
}

export default function App() {
  const { message } = AntdApp.useApp();
  const { state, logs, exited, upgradeProgress } = useUIEvents();
  const [activeId, setActiveId] = useState("");
  const [forms, setForms] = useState({});
  const [keyHint, setKeyHint] = useState("");
  const [formError, setFormError] = useState("");
  const [failHint, setFailHint] = useState("");
  const [conflictHint, setConflictHint] = useState("");
  const [upgradeOpen, setUpgradeOpen] = useState(false);
  const [upgradeBody, setUpgradeBody] = useState("");
  const [upgradeModalError, setUpgradeModalError] = useState("");
  const [applying, setApplying] = useState(false);
  const [reconnectTimedOut, setReconnectTimedOut] = useState(false);
  const seen = useRef({});
  const keyInputRef = useRef(null);
  const logRef = useRef(null);
  const applyFromVersionRef = useRef("");

  const restarting = applying;

  const services = state?.services || [];
  const active = services.find((s) => s.id === activeId) || services[0];
  const form = (active && forms[active.id]) || formFromService(active || {});
  const workType = form.workType;
  const tunKey = form.tunKey;
  const localMode = form.localMode;
  const transport = form.transport;
  const autoStart = form.autoStart;
  const rows = form.rows || [];
  function patchForm(patch) {
    if (!active) return;
    setForms((prev) => ({ ...prev, [active.id]: { ...(prev[active.id] || form), ...patch } }));
  }
  const setWorkType = (v) => patchForm({ workType: v });
  const setTunKey = (v) => patchForm({ tunKey: v });
  const setLocalMode = (v) => patchForm({ localMode: v });
  const setTransport = (v) => patchForm({ transport: v });
  const setAutoStart = (v) => patchForm({ autoStart: v });
  const setRows = (updater) => {
    setForms((prev) => {
      if (!active) return prev;
      const cur = prev[active.id] || form;
      const nextRows = typeof updater === "function" ? updater(cur.rows || []) : updater;
      return { ...prev, [active.id]: { ...cur, rows: nextRows } };
    });
  };

  useEffect(() => {
    if (!state?.services) return;
    setForms((prev) => {
      const next = { ...prev };
      for (const svc of state.services) {
        const cur = next[svc.id];
        const running = svc.status !== STATUS_DETECTING && svc.status !== STATUS_IDLE;
        if (!cur || !seen.current[svc.id]) {
          next[svc.id] = formFromService(svc);
          seen.current[svc.id] = true;
        } else if (running) {
          next[svc.id] = {
            ...formFromService(svc),
            tunKey: isKeyInputFocused(keyInputRef) ? cur.tunKey : svc.tunKey,
          };
        } else {
          next[svc.id] = { ...cur, autoStart: !!svc.autoStart, name: svc.name };
        }
      }
      return next;
    });
    setActiveId((id) => (id && state.services.some((s) => s.id === id) ? id : state.services[0]?.id || ""));
  }, [state]);

  useEffect(() => {
    if (logRef.current) {
      logRef.current.scrollTop = logRef.current.scrollHeight;
    }
  }, [logs, activeId]);

  useEffect(() => {
    if (state?.version) {
      document.title = `Goodlink v${state.version} · ${state.isAdmin ? "管理员" : "普通权限"}`;
    }
  }, [state?.version, state?.isAdmin]);

  useEffect(() => {
    if (!applying) return;
    const fromVersion = applyFromVersionRef.current;
    if (isRestartedVersion(fromVersion, state?.version)) {
      window.location.reload();
    }
  }, [applying, state?.version]);

  useEffect(() => {
    const st = active?.status ?? STATUS_DETECTING;
    if (st === STATUS_CONNECTING_NAT4) {
      setFailHint(NAT4_HINT);
      if (active) stop(active.id);
      return;
    }
    if (st === STATUS_CONNECT_FAILED) {
      setFailHint(RETRY_FAIL_HINT);
      if (active) stop(active.id);
    }
  }, [active?.status, active?.id]);

  useEffect(() => {
    if (!restarting) return;
    let stopped = false;
    const fromVersion = applyFromVersionRef.current;
    const reconnectMs = APPLY_RECONNECT_MS;
    const overallMs = APPLY_OVERALL_MS;
    (async () => {
      let sawDown = false;
      let downAt = 0;
      const startedAt = Date.now();
      for (; ;) {
        if (stopped) return;
        const { up, version, timedOut } = await probeState(APPLY_FETCH_TIMEOUT_MS);
        if (stopped) return;
        const restarted = isRestartedVersion(fromVersion, version) || sawDown;
        if (up && restarted) {
          window.location.reload();
          return;
        }
        const now = Date.now();
        if (!up && !timedOut && !sawDown) {
          sawDown = true;
          downAt = now;
        }
        if (sawDown && now - downAt >= reconnectMs) {
          setReconnectTimedOut(true);
          return;
        }
        if (now - startedAt >= overallMs) {
          setReconnectTimedOut(true);
          return;
        }
        const wait = sawDown
          ? Math.min(500, Math.max(0, reconnectMs - (now - downAt)))
          : 200;
        if (wait > 0) {
          await sleep(wait);
        }
      }
    })();
    return () => {
      stopped = true;
    };
  }, [restarting]);

  const status = active?.status ?? STATUS_DETECTING;
  const viewLogs = [...(logs.shared || []), ...((active && logs.byId?.[active.id]) || [])];
  const view = viewFromStatus(status, workType);
  const nat = state?.nat || {};
  const othersEnabled = !exited && !!view.othersEnabled;
  const startEnabled = !exited && !!view.enabled;
  const mappingEnabled = !exited && workType === "Local" && localMode === "forward";
  const autoLabel = workType === "Remote" ? "程序运行后，自动启动Remote端" : "程序运行后，自动连接Remote端";
  const running = status === STATUS_CONNECTED || status === STATUS_STARTED;

  async function onGenerate() {
    try {
      const key = await generateKey();
      if (key) {
        setTunKey(key);
        message.success("已生成密钥");
        return;
      }
    } catch (_) { }
    message.error("生成密钥失败");
  }

  async function onCopy() {
    try {
      await navigator.clipboard.writeText(tunKey || "");
      message.success("已复制密钥");
    } catch (_) {
      message.error("复制失败");
    }
  }

  async function onPaste() {
    try {
      const text = await navigator.clipboard.readText();
      if (text) {
        setTunKey(text);
        message.success("已粘贴密钥");
      }
    } catch (_) {
      message.error("粘贴失败");
    }
  }

  function onExportConfig() {
    const cfg = {
      services: services.map((svc) => {
        const f = forms[svc.id] || formFromService(svc);
        return {
          id: svc.id,
          name: f.name || svc.name,
          work_type: f.workType,
          tun_key: f.tunKey || "",
          local_mode: f.localMode || "tun",
          transport: f.transport || "kcp",
          tun_ip: svc.tunIP,
          auto_start: f.autoStart,
          forward_rules: rulesFromRows(f.rows || []),
        };
      }),
    };
    const blob = new Blob([JSON.stringify(cfg, null, 2) + "\n"], { type: "application/json;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "goodlink.json";
    a.click();
    URL.revokeObjectURL(url);
    message.success("已导出配置");
  }

  async function onImportFile(file) {
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
    if (!cfg.services && (cfg.work_type || cfg.tun_key)) {
      cfg = {
        services: [{
          name: "服务1",
          work_type: cfg.work_type,
          tun_key: cfg.tun_key,
          local_mode: cfg.local_mode,
          transport: cfg.transport,
          auto_start: cfg.auto_start,
          forward_rules: cfg.forward_rules,
        }],
      };
    }
    const result = await importConfig(cfg);
    if (result.error) {
      setFormError(result.error);
      return;
    }
    seen.current = {};
    message.success("已导入配置");
  }

  async function onConfirm() {
    if (!mappingEnabled) return;
    setFormError("");
    setConflictHint("");
    const result = await applyForwards(active.id, localMode, rulesFromRows(rows));
    if (result.error) {
      if (String(result.error).includes("端口已被占用")) {
        setConflictHint(result.error);
        return;
      }
      setFormError(result.error);
      return;
    }
    message.success("端口转发已生效");
  }

  async function onStart() {
    if (!startEnabled) return;
    if (status === STATUS_IDLE) {
      setFormError("");
      setConflictHint("");
      const result = await start(active.id, workType, tunKey, localMode, rulesFromRows(rows), transport);
      if (result.error) {
        if (String(result.error).includes("端口已被占用")) {
          setConflictHint(result.error);
          return;
        }
        if (String(result.error).includes("连接密钥")) {
          setKeyHint(result.error);
          return;
        }
        setFormError(result.error);
        return;
      }
      return;
    }
    await stop(active.id);
  }

  async function onAutoStartToggle(enabled) {
    if (exited) return;
    setFormError("");
    setConflictHint("");
    const othersHadAutoStart =
      enabled && Object.entries(forms).some(([id, f]) => id !== active.id && f?.autoStart);
    setAutoStart(enabled);
    const result = await saveAutoStart(active.id, enabled, workType, tunKey, localMode, rulesFromRows(rows), transport);
    if (result.error) {
      if (String(result.error).includes("端口已被占用")) {
        setConflictHint(result.error);
        return;
      }
      setFormError(result.error);
      setAutoStart(!enabled);
      return;
    }
    if (othersHadAutoStart) {
      message.info("已关闭其他服务的自动连接");
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
    if (!viewLogs.length) return;
    const pad = (n) => String(n).padStart(2, "0");
    const d = new Date();
    const safe = String(active?.name || "goodlink").replace(/[\\/:*?"<>|\s]+/g, "_");
    const name = `${safe}-${d.getFullYear()}${pad(d.getMonth() + 1)}${pad(d.getDate())}-${pad(d.getHours())}${pad(d.getMinutes())}${pad(d.getSeconds())}.log`;
    const blob = new Blob([viewLogs.join("\n") + "\n"], { type: "text/plain;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = name;
    a.click();
    URL.revokeObjectURL(url);
    message.success("已导出日志");
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

  const upgradeErrorText = upgradeModalError || upgradeProgress.error;
  const progressStatus = upgradeProgress.error ? "exception" : upgradeProgress.done ? "success" : upgradeProgress.downloading ? "active" : "normal";

  return (
    <div className="gl-shell">
      <div className="gl-workspace">
        <aside className="gl-sider">
          <div className="gl-sider-title">服务列表(双击可修改名称)</div>
          <ServiceTabs
            services={services}
            activeId={active?.id}
            onChange={setActiveId}
            onCreate={async () => {
              const result = await createService();
              if (result.error) setFormError(result.error);
              else if (result.state?.id) setActiveId(result.state.id);
            }}
            onDelete={async (id) => {
              const svc = services.find((s) => s.id === id);
              if (!window.confirm(`确定删除服务[${svc?.name || ""}]？`)) return;
              const result = await deleteService(id);
              if (result.error) setFormError(result.error);
            }}
            onRename={async (id, name) => {
              const result = await renameService(id, name);
              if (result.error) setFormError(result.error);
            }}
            onExportConfig={onExportConfig}
            onImportFile={onImportFile}
            othersEnabled={othersEnabled}
          />
        </aside>
        <div className="gl-main">
          <ServicePanel>
            <Form component="div" layout="vertical" requiredMark={false}>
              <Card className="gl-panel" size="small">
                <Form.Item label="工作端侧">
                  <ChoiceGroup
                    value={workType}
                    disabled={!othersEnabled}
                    onChange={(next) => setWorkType(next)}
                    options={[
                      {
                        value: "Local",
                        icon: <DesktopOutlined />,
                        label: "Local 端",
                        desc: "本地客户端，主动发起连接",
                      },
                      {
                        value: "Remote",
                        icon: <CloudServerOutlined />,
                        label: "Remote 端",
                        desc: "被连接服务端，等待对端接入",
                      },
                    ]}
                  />
                </Form.Item>

                {workType === "Remote" && (
                  <Form.Item label="传输协议">
                    <ChoiceGroup
                      value={transport}
                      disabled={!othersEnabled}
                      onChange={(next) => setTransport(next)}
                      options={[
                        {
                          value: "kcp",
                          icon: <ThunderboltOutlined />,
                          label: "KCP",
                          desc: "CPU 降低约 40%，适合个人使用",
                        },
                        {
                          value: "quic",
                          icon: <SafetyCertificateOutlined />,
                          label: "QUIC",
                          desc: "二次加密，传输更稳，适合企业",
                        },
                      ]}
                    />
                  </Form.Item>
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
                    proxy={resolveProxy(localMode, active?.proxy)}
                    isAdmin={state?.isAdmin}
                  />
                ) : (
                  <RemotePanel {...keyProps} />
                )}
              </Card>
            </Form>
          </ServicePanel>

          {formError ? <Alert type="error" showIcon message={formError} /> : null}

          <Card className="gl-panel" title="连接控制" size="small">
            <div className="gl-start-wrap">
              <div className="gl-auto">
                <div className="gl-auto-copy">
                  <span className="gl-auto-title">{autoLabel}</span>
                </div>
                <Switch checked={autoStart} disabled={exited} onChange={onAutoStartToggle} />
              </div>
              <Button
                size="large"
                block
                className={`gl-start-btn gl-start-${view.importance}${view.activity ? " is-busy" : ""}`}
                disabled={!startEnabled}
                icon={view.activity ? <LoadingOutlined /> : running ? <PauseCircleOutlined /> : startEnabled ? <PlayCircleOutlined /> : undefined}
                onClick={onStart}
                {...startBtnTone(view.importance)}
              >
                {view.text}
              </Button>
            </div>
          </Card>

          <Card
            className="gl-panel"
            title="运行日志"
            size="small"
            extra={
              <Button size="small" icon={<ExportOutlined />} disabled={!viewLogs.length} onClick={onExportLogs}>
                导出日志
              </Button>
            }
          >
            <pre ref={logRef} className={"gl-log" + (viewLogs.length ? "" : " is-empty")}>{viewLogs.join("\n")}</pre>
          </Card>

          <footer className="gl-footer">
            <Tag className="gl-nat" color={natTagColor(nat)}>{natHint(nat)}</Tag>
            <Space wrap size={4}>
              {state?.upgrade?.checkError ? (
                <Button
                  type="link"
                  href="https://gitee.com/konyshe/goodlink/releases"
                  target="_blank"
                  rel="noopener"
                >
                  {state.upgrade.checkError}
                </Button>
              ) : state?.upgrade?.need ? (
                <Badge dot>
                  <Button
                    color="green"
                    variant="solid"
                    size="small"
                    icon={<CloudDownloadOutlined />}
                    onClick={openUpgradeModal}
                  >
                    有新版本 v{state.upgrade.latest}
                  </Button>
                </Badge>
              ) : null}
              <Button type="link" icon={<GlobalOutlined />} href="https://goodlink.kony.vip" target="_blank" rel="noopener">
                官方网站
              </Button>
              <Button
                type="link"
                icon={<BookOutlined />}
                href="https://gitee.com/konyshe/goodlink/wikis/pages"
                target="_blank"
                rel="noopener"
              >
                使用教程
              </Button>
              <Button
                type="link"
                icon={<CommentOutlined />}
                href="https://gitee.com/konyshe/goodlink/issues"
                target="_blank"
                rel="noopener"
              >
                反馈问题
              </Button>
            </Space>
          </footer>
        </div>
      </div>

      <Modal
        open={exited && !restarting}
        footer={null}
        closable={false}
        keyboard={false}
        mask={{ closable: false }}
        centered
        destroyOnHidden
      >
        <Result status="warning" title="程序已退出" subTitle="请重新启动 Goodlink" />
      </Modal>

      <Modal
        title="连接失败"
        open={!exited && !!failHint}
        onCancel={() => setFailHint("")}
        mask={{ closable: true }}
        centered
        destroyOnHidden
        footer={[
          <Button key="help" href={CONNECT_FAIL_HELP_URL} target="_blank" rel="noopener">
            查看解决方法
          </Button>,
          <Button key="ok" type="primary" onClick={() => setFailHint("")}>
            确定
          </Button>,
        ]}
      >
        {failHint}
      </Modal>

      <Modal
        title="端口冲突"
        open={!exited && !!conflictHint}
        onCancel={() => setConflictHint("")}
        mask={{ closable: true }}
        centered
        destroyOnHidden
        footer={[
          <Button key="ok" type="primary" onClick={() => setConflictHint("")}>
            确定
          </Button>,
        ]}
      >
        {conflictHint}
      </Modal>

      <Modal
        title="连接密钥重复"
        open={!exited && !!keyHint}
        onCancel={() => setKeyHint("")}
        mask={{ closable: true }}
        centered
        destroyOnHidden
        footer={[
          <Button key="ok" type="primary" onClick={() => setKeyHint("")}>
            确定
          </Button>,
        ]}
      >
        {keyHint}
      </Modal>

      <Modal
        title={`新版本 v${state?.upgrade?.latest || ""}`}
        open={upgradeOpen && !restarting && !exited}
        onCancel={() => setUpgradeOpen(false)}
        mask={{ closable: false }}
        centered
        destroyOnHidden
        width={560}
        footer={[
          <Button key="back" onClick={() => setUpgradeOpen(false)}>
            返回
          </Button>,
          upgradeProgress.done ? (
            <Button key="apply" type="primary" onClick={onUpgradeApply}>
              更新
            </Button>
          ) : (
            <Button key="download" type="primary" loading={upgradeProgress.downloading} onClick={onUpgradeDownload}>
              {upgradeProgress.downloading ? "下载中..." : "下载"}
            </Button>
          ),
        ]}
      >
        <Flex vertical gap={16}>
          <div
            style={{
              maxHeight: 280,
              overflow: "auto",
              whiteSpace: "pre-wrap",
              wordBreak: "break-word",
              padding: 12,
              background: "rgba(8, 13, 24, 0.7)",
              borderRadius: 10,
            }}
          >
            {upgradeBody || "暂无更新说明"}
          </div>
          <Progress percent={upgradeProgress.percent || 0} status={progressStatus} />
          {upgradeErrorText ? <Alert type="error" showIcon message={upgradeErrorText} /> : null}
        </Flex>
      </Modal>

      <Modal
        open={restarting}
        footer={null}
        closable={false}
        keyboard={false}
        mask={{ closable: false }}
        centered
        destroyOnHidden
        width={480}
      >
        {reconnectTimedOut ? (
          <Result
            status="warning"
            title="更新完成"
            subTitle="超过 5 秒未能连接主程序，请手动重启 Goodlink"
          />
        ) : (
          <Flex vertical align="center" gap={16} style={{ padding: "16px 0" }}>
            <Spin size="large" />
            <Typography.Title level={5} style={{ margin: 0 }}>
              {`正在更新到 v${state?.upgrade?.latest || ""}`}
            </Typography.Title>
            <Typography.Text>程序将自动重启，请稍候</Typography.Text>
            <Typography.Text type="secondary">重启完成后页面会自动恢复，无需手动关闭</Typography.Text>
          </Flex>
        )}
      </Modal>
    </div>
  );
}
