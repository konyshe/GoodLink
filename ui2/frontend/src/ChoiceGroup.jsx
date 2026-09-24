export default function ChoiceGroup({ value, disabled, onChange, options }) {
  return (
    <div className="gl-choices" role="radiogroup">
      {options.map((opt) => {
        const active = value === opt.value;
        return (
          <button
            key={opt.value}
            type="button"
            role="radio"
            aria-checked={active}
            className={"gl-choice" + (active ? " is-active" : "")}
            disabled={disabled || opt.disabled}
            title={opt.disabled ? opt.desc : undefined}
            onClick={() => {
              if (disabled || opt.disabled || value === opt.value) return;
              onChange(opt.value);
            }}
          >
            {opt.icon ? <span className="gl-choice-icon">{opt.icon}</span> : null}
            <span className="gl-choice-copy">
              <span className="gl-choice-title">{opt.label}</span>
              {opt.desc ? <span className="gl-choice-desc">{opt.desc}</span> : null}
            </span>
          </button>
        );
      })}
    </div>
  );
}
