import { Badge, Tabs, Typography } from "antd";

function badgeStatus(status) {
  if (status === 5 || status === 6) return "success";
  if (status === 4 || status === 8) return "error";
  if (status === 2 || status === 3 || status === 7) return "warning";
  return "default";
}

export default function ServiceTabs({ services, activeId, onChange, onCreate, onDelete, onRename }) {
  return (
    <Tabs
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
          <span>
            <Badge status={badgeStatus(svc.status)} />{" "}
            <Typography.Text
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
  );
}
