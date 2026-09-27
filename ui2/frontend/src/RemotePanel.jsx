import { Form, Switch } from "antd";
import KeySection from "./KeySection";

export default function RemotePanel({ allowLocalProxy, setAllowLocalProxy, othersEnabled, ...keyProps }) {
  return (
    <>
      <KeySection {...keyProps} othersEnabled={othersEnabled} />
      <Form.Item className="gl-proxy-allow-item">
        <div className="gl-proxy-allow">
          <span className="gl-proxy-allow-label">允许 Local 开启代理</span>
          <span className="gl-proxy-allow-hint">【关闭后，Local 下次连接时不会开启 SOCKS/HTTP 代理】</span>
          <Switch checked={allowLocalProxy !== false} disabled={!othersEnabled} onChange={setAllowLocalProxy} />
        </div>
      </Form.Item>
    </>
  );
}
