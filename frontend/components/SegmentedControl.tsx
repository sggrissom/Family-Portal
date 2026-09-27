import * as preact from "preact";
import * as vlens from "vlens";
import "./segmented-styles";

interface SegmentedControlProps<T extends string> {
  label: string;
  options: { value: T; label: string }[];
  value: T;
  onChange: (value: T) => void;
  disabled?: boolean;
}

export const SegmentedControl = <T extends string>({
  label,
  options,
  value,
  onChange,
  disabled,
}: SegmentedControlProps<T>) => (
  <div className="segmented" role="group" aria-label={label}>
    {options.map(option => (
      <button
        key={option.value}
        type="button"
        className={option.value === value ? "segmented-option selected" : "segmented-option"}
        aria-pressed={option.value === value ? "true" : "false"}
        disabled={disabled}
        onClick={vlens.cachePartial(onChange, option.value)}
      >
        {option.label}
      </button>
    ))}
  </div>
);
