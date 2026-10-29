import * as preact from "preact";
import * as vlens from "vlens";
import { When, ageDate, localDateString } from "../lib/when";
import { formatLongDate } from "../lib/dateUtils";
import { copy } from "../lib/copy";
import "./when-control-styles";

interface WhenControlProps {
  when: When;
  disabled?: boolean;
  birthday?: string;
}

export const WhenControl = ({ when, disabled, birthday }: WhenControlProps) => (
  <div className="when-control">
    <select
      className="when-select"
      aria-label={copy.when.label}
      disabled={disabled}
      {...vlens.attrsBindInput(vlens.ref(when, "mode"))}
    >
      <option value="today">{copy.when.today}</option>
      <option value="yesterday">{copy.when.yesterday}</option>
      <option value="date">{copy.when.pickDate}</option>
      <option value="age">{copy.when.byAge}</option>
    </select>

    {when.mode === "date" && (
      <input
        type="date"
        className="when-date"
        aria-label={copy.when.date}
        max={localDateString(new Date())}
        disabled={disabled}
        {...vlens.attrsBindInput(vlens.ref(when, "date"))}
      />
    )}

    {when.mode === "age" && (
      <span className="when-age">
        <input
          type="number"
          min="0"
          max="100"
          inputmode="numeric"
          aria-label={copy.when.years}
          placeholder="yrs"
          disabled={disabled}
          {...vlens.attrsBindInput(vlens.ref(when, "ageYears"))}
        />
        <input
          type="number"
          min="0"
          max="11"
          inputmode="numeric"
          aria-label={copy.when.months}
          placeholder="mo"
          disabled={disabled}
          {...vlens.attrsBindInput(vlens.ref(when, "ageMonths"))}
        />
      </span>
    )}

    {when.mode === "age" && <AgePreview when={when} birthday={birthday} />}
  </div>
);

const AgePreview = ({ when, birthday }: { when: When; birthday?: string }) => {
  const date = birthday ? ageDate(birthday, when) : null;
  if (!date) return null;
  const future = date > localDateString(new Date());
  return (
    <span className={future ? "when-age-date when-age-future" : "when-age-date"}>
      {future ? "That age hasn't happened yet" : formatLongDate(date)}
    </span>
  );
};
