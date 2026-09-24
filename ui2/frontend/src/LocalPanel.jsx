import {
  CheckOutlined,
  DeleteOutlined,
  PlusOutlined,
  SwapOutlined,
  WifiOutlined,
} from "@ant-design/icons";
import { Alert, Button, Flex, Form, Input, Select, Table, Tooltip, Typography } from "antd";
import ChoiceGroup from "./ChoiceGroup";
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
  onExportConfig,
  onImportFile,
  localMode,
  setLocalMode,
  rows,
  setRows,
  mappingEnabled,
  onConfirm,
  proxy,
  isAdmin,
}) {
  function updateRow(id, field, value) {
    setRows((prev) => prev.map((row) => (row.id === id ? { ...row, [field]: value } : row)));
  }

  function removeRow(id) {
    setRows((prev) => prev.filter((row) => row.id !== id));
  }

  const socks = proxy?.socks || "";
  const http = proxy?.http || "";
  const fallback = !!proxy?.fallback;

  const columns = [
    {
      title: "协议",
      dataIndex: "proto",
      width: 96,
      render: (value, row) => (
        <Select
          size="small"
          value={value}
          disabled={!mappingEnabled}
          options={[
            { value: "tcp", label: "TCP" },
            { value: "udp", label: "UDP" },
          ]}
          onChange={(val) => updateRow(row.id, "proto", val)}
          style={{ width: "100%" }}
        />
      ),
    },
    {
      title: "本地地址",
      dataIndex: "listenHost",
      render: (value, row) => (
        <Input
          size="small"
          value={value}
          disabled={!mappingEnabled}
          spellCheck={false}
          autoComplete="off"
          onChange={(e) => updateRow(row.id, "listenHost", e.target.value)}
        />
      ),
    },
    {
      title: "本地端口",
      dataIndex: "listenPort",
      width: 110,
      render: (value, row) => (
        <Input
          size="small"
          value={value}
          disabled={!mappingEnabled}
          inputMode="numeric"
          spellCheck={false}
          autoComplete="off"
          onChange={(e) => updateRow(row.id, "listenPort", e.target.value)}
        />
      ),
    },
    {
      title: "Remote地址",
      dataIndex: "remoteHost",
      render: (value, row) => (
        <Input
          size="small"
          value={value}
          disabled={!mappingEnabled}
          spellCheck={false}
          autoComplete="off"
          onChange={(e) => updateRow(row.id, "remoteHost", e.target.value)}
        />
      ),
    },
    {
      title: "Remote端口",
      dataIndex: "remotePort",
      width: 120,
      render: (value, row) => (
        <Input
          size="small"
          value={value}
          disabled={!mappingEnabled}
          inputMode="numeric"
          spellCheck={false}
          autoComplete="off"
          onChange={(e) => updateRow(row.id, "remotePort", e.target.value)}
        />
      ),
    },
    {
      title: "",
      key: "action",
      width: 72,
      render: (_, row) => (
        <Tooltip title="删除">
          <Button
            type="link"
            danger
            size="small"
            icon={<DeleteOutlined />}
            disabled={!mappingEnabled}
            onClick={() => removeRow(row.id)}
          />
        </Tooltip>
      ),
    },
  ];

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
        onExportConfig={onExportConfig}
        onImportFile={onImportFile}
      />

      <Form.Item label="工作模式">
        <ChoiceGroup
          value={localMode}
          disabled={!othersEnabled}
          onChange={(next) => setLocalMode(next)}
          options={[
            {
              value: "tun",
              icon: <WifiOutlined />,
              label: "TUN 模式",
              desc: isAdmin === false ? "需要管理员权限，请重启 Goodlink 并在 UAC 弹窗选择同意" : "简单易用，系统级虚拟网卡",
              disabled: isAdmin === false,
            },
            {
              value: "forward",
              icon: <SwapOutlined />,
              label: "转发模式",
              desc: "灵活强大，无需管理员权限",
            },
          ]}
        />
      </Form.Item>

      <Form.Item label="代理地址">
        <div className="gl-proxy-list">
          <div className="gl-proxy-row">
            <span className="gl-proxy-label">SOCKS</span>
            <Typography.Text copyable={socks ? true : false} style={{ flex: 1 }}>
              {socks || "暂无"}
            </Typography.Text>
          </div>
          <div className="gl-proxy-row">
            <span className="gl-proxy-label">HTTP</span>
            <Typography.Text copyable={http ? true : false} style={{ flex: 1 }}>
              {http || "暂无"}
            </Typography.Text>
          </div>
          {fallback ? <Alert type="warning" showIcon message="1080 已被占用，已改用随机端口" /> : null}
        </div>
      </Form.Item>

      {localMode === "forward" && (
        <Form.Item label="端口转发">
          <Flex vertical gap={10}>
            <div className="gl-fwd-toolbar">
              <Button
                type="dashed"
                icon={<PlusOutlined />}
                disabled={!mappingEnabled}
                onClick={() => setRows((prev) => [...prev, newRow()])}
              >
                添加规则
              </Button>
              <Button type="primary" icon={<CheckOutlined />} disabled={!mappingEnabled} onClick={onConfirm}>
                确认生效
              </Button>
            </div>
            <Table
              size="small"
              pagination={false}
              rowKey="id"
              dataSource={rows}
              columns={columns}
              locale={{
                emptyText: "点击确认立即生效，无需重新连接"
              }}
            />
          </Flex>
        </Form.Item>
      )}
    </>
  );
}
