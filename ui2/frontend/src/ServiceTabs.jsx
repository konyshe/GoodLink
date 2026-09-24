import { Badge, Tabs, Typography } from "antd";

function badgeStatus(status) {
  if (status === 5 || status === 6) return "success";
  if (status === 4 || status === 8) return "error";
  if (status === 2 || status === 3 || status === 7) return "warning";
  return "default";
}

function isTextEditable(el) {
  if (!el) return false;
  const tag = el.tagName;
  return tag === "INPUT" || tag === "TEXTAREA" || !!el.isContentEditable;
}

export default function ServiceTabs({ services, activeId, onChange, onCreate, onDelete, onRename }) {
  // rc-tabs 在标签页上按 Backspace/Delete 会触发 onEdit('remove')，这里在捕获阶段拦截，
  // 删除服务只保留标签上的 × 按钮入口；改名编辑态仍可正常删除文字。
  function blockRemoveKey(e) {
    if (e.key !== "Backspace" && e.key !== "Delete") return;
    if (isTextEditable(e.target)) {
      e.stopPropagation();
      return;
    }
    e.preventDefault();
    e.stopPropagation();
  }

  return (
    <div className="gl-service-tabs-wrap" onKeyDownCapture={blockRemoveKey}>
      <Tabs
        className="gl-service-tabs"
        tabPosition="left"
        type="editable-card"
        activeKey={activeId}
        onChange={onChange}
        onEdit={(key, action) => {
          if (action === "add") onCreate();
          if (action === "remove") onDelete(key);
        }}
        items={(services || []).map((svc) => ({
          key: svc.id,
          label: (
            <span className="gl-service-tab">
              <Badge status={badgeStatus(svc.status)} />
              <Typography.Text
                className="gl-service-tab-name"
                ellipsis
                editable={{
                  text: svc.name,
                  onChange: (next) => onRename(svc.id, next),
                  triggerType: ["text"],
                }}
              >
                {svc.name}
              </Typography.Text>
            </span>
          ),
        }))}
      />
    </div>
  );
}
