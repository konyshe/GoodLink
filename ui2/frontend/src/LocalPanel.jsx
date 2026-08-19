import KeySection from "./KeySection";

let nextRowId = 1;

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

export function rowsFromRules(rules) {
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

export function rulesFromRows(rows) {
  return rows
    .filter((r) => String(r.listenPort || "").trim() !== "" || String(r.remotePort || "").trim() !== "")
    .map((r) => ({
      proto: r.proto || "tcp",
      listen: `${(r.listenHost || "").trim()}:${(r.listenPort || "").trim()}`,
      remote: `${(r.remoteHost || "").trim()}:${(r.remotePort || "").trim()}`,
    }));
}

export default function LocalPanel({
  tunKey,
  setTunKey,
  keyInputRef,
  othersEnabled,
  onGenerate,
  onCopy,
  onPaste,
  localMode,
  setLocalMode,
  rows,
  setRows,
  mappingEnabled,
  onConfirm,
  onNeedAdminHint,
  proxy,
}) {
  function updateRow(id, field, value) {
    setRows((prev) => prev.map((row) => (row.id === id ? { ...row, [field]: value } : row)));
  }

  function removeRow(id) {
    setRows((prev) => prev.filter((row) => row.id !== id));
  }

  async function copyText(text) {
    try {
      await navigator.clipboard.writeText(text || "");
    } catch (_) { }
  }

  const socks = proxy?.socks || "";
  const http = proxy?.http || "";
  const fallback = !!proxy?.fallback;

  return (
    <>
      <KeySection
        tunKey={tunKey}
        setTunKey={setTunKey}
        keyInputRef={keyInputRef}
        othersEnabled={othersEnabled}
        onGenerate={onGenerate}
        onCopy={onCopy}
        onPaste={onPaste}
      />

      <section className="row">
        <span className="label">工作模式:</span>
        <div className="work-type work-type-sm">
          <button
            type="button"
            className={"work-btn" + (localMode === "tun" ? " active" : "")}
            disabled={!othersEnabled}
            onClick={() => {
              setLocalMode("tun");
              onNeedAdminHint?.();
            }}
          >
            TUN模式(简单易用, 需管理权限)
          </button>
          <span className="vsep"></span>
          <button
            type="button"
            className={"work-btn" + (localMode === "forward" ? " active" : "")}
            disabled={!othersEnabled}
            onClick={() => setLocalMode("forward")}
          >
            转发模式(灵活强大, 无需管理权限)
          </button>
        </div>
      </section>

      <section className="row proxy-row">
        <span className="label">代理地址:</span>
        <div className="proxy-addrs">
          <div className="proxy-item">
            <code>{socks}</code>
            <button type="button" disabled={!socks} onClick={() => copyText(socks)}>
              复制
            </button>
          </div>
          <div className="proxy-item">
            <code>{http}</code>
            <button type="button" disabled={!http} onClick={() => copyText(http)}>
              复制
            </button>
          </div>
          {fallback && <div className="proxy-hint">1080 已被占用，已改用随机端口</div>}
        </div>
      </section>

      {localMode === "forward" && (
        <section className="forwards">
          <div className="forwards-head">
            <span className="label">端口转发:</span>
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
              <div className="forwards-empty">点击添加以配置端口转发，点击确认立即生效, 无需重新连接</div>
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
    </>
  );
}
