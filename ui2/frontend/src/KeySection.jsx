export default function KeySection({ tunKey, setTunKey, keyInputRef, othersEnabled, onGenerate, onCopy, onPaste }) {
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
    </>
  );
}
