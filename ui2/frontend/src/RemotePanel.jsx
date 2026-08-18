import KeySection from "./KeySection";

export default function RemotePanel({ tunKey, setTunKey, keyInputRef, othersEnabled, onGenerate, onCopy, onPaste }) {
  return (
    <KeySection
      tunKey={tunKey}
      setTunKey={setTunKey}
      keyInputRef={keyInputRef}
      othersEnabled={othersEnabled}
      onGenerate={onGenerate}
      onCopy={onCopy}
      onPaste={onPaste}
    />
  );
}
