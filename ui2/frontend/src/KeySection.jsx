export default function KeySection({
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
}) {
  return (
    <>
      <section className="row">
        <span className="label">连接密钥:</span>
        <input
          ref={keyInputRef}
          className="key-input"
          type="text"
          placeholder="16-64字节长度"
          autoComplete="off"
          spellCheck="false"
          value={tunKey}
          disabled={!othersEnabled}
          onChange={(e) => setTunKey(e.target.value)}
        />
      </section>

      <section className="key-actions">
        <button type="button" disabled={!othersEnabled} onClick={onGenerate}>生成密钥</button>
        <button type="button" onClick={onCopy}>复制密钥</button>
        <button type="button" disabled={!othersEnabled} onClick={onPaste}>粘贴密钥</button>
      </section>

      <section className="key-actions config-actions">
        <button type="button" onClick={onExportConfig}>导出配置</button>
        <button type="button" disabled={!othersEnabled} onClick={onImportClick}>导入配置</button>
        <input
          ref={fileInputRef}
          type="file"
          accept=".json,application/json"
          hidden
          onChange={onImportFile}
        />
      </section>
    </>
  );
}
