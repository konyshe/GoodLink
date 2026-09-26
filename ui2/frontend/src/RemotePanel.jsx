import { Form, Switch } from "antd";
import KeySection from "./KeySection";

export default function RemotePanel({ allowLocalProxy, setAllowLocalProxy, othersEnabled, ...keyProps }) {
  return (
    <>
      <KeySection {...keyProps} />
      <Form.Item label="允许 Local 开启代理" extra="关闭后，Local 下次连接时不会开启 SOCKS/HTTP 代理">
        <Switch checked={allowLocalProxy !== false} disabled={!othersEnabled} onChange={setAllowLocalProxy} />
      </Form.Item>
    </>
  );
}
