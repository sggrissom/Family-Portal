import * as preact from "preact";
import * as vlens from "vlens";
import { getFamilies } from "../lib/authCache";

interface FamilySelectProps {
  id: string;
  // Current selection. Zero means "the primary family", which is what the
  // backend falls back to when a request names no family.
  value: number;
  onChange: (familyId: number) => void;
  disabled?: boolean;
  label?: string;
}

// FamilySelect asks which family a new record belongs to. It renders nothing
// for the single-family case, where there is no choice to make and the backend
// default is always right.
export const FamilySelect = ({
  id,
  value,
  onChange,
  disabled,
  label = "Family",
}: FamilySelectProps): preact.ComponentChild => {
  const families = getFamilies();
  if (families.length < 2) {
    return null;
  }

  const primary = families.find(family => family.isPrimary) ?? families[0];
  const selected = value || primary.id;

  return (
    <div className="form-group">
      <label htmlFor={id}>{label}</label>
      <select
        id={id}
        value={String(selected)}
        disabled={disabled}
        onInput={event => {
          onChange(Number((event.currentTarget as HTMLSelectElement).value));
          vlens.scheduleRedraw();
        }}
      >
        {families.map(family => (
          <option key={family.id} value={String(family.id)}>
            {family.name || `Family ${family.id}`}
          </option>
        ))}
      </select>
    </div>
  );
};
