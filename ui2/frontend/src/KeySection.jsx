import {
  CopyOutlined,
  KeyOutlined,
  SnippetsOutlined,
  UploadOutlined,
} from "@ant-design/icons";
import { Button, Form, Input, Upload } from "antd";

export default function KeySection({
  tunKey,
  setTunKey,
  keyInputRef,
  othersEnabled,
  onGenerate,
  onCopy,
  onPaste,
  onImportFile,
}) {
  return (
    <Form.Item label="连接密钥">
      <div className="gl-key-block">
        <Input
          ref={keyInputRef}
          size="large"
          variant="filled"
          prefix={<KeyOutlined />}
          placeholder="16-64 字节长度"
          autoComplete="off"
          spellCheck={false}
          value={tunKey}
          disabled={!othersEnabled}
          allowClear={othersEnabled}
          onChange={(e) => setTunKey(e.target.value)}
        />
        <div className="gl-key-actions">
          <Button type="primary" icon={<KeyOutlined />} disabled={!othersEnabled} onClick={onGenerate}>
            生成密钥
          </Button>
          <Button icon={<CopyOutlined />} onClick={onCopy}>
            复制密钥
          </Button>
          <Button icon={<SnippetsOutlined />} disabled={!othersEnabled} onClick={onPaste}>
            粘贴密钥
          </Button>
          <Upload
            accept=".json,application/json"
            showUploadList={false}
            disabled={!othersEnabled}
            beforeUpload={(file) => {
              onImportFile(file);
              return false;
            }}
          >
            <Button icon={<UploadOutlined />} disabled={!othersEnabled}>
              导入配置
            </Button>
          </Upload>
        </div>
      </div>
    </Form.Item>
  );
}
