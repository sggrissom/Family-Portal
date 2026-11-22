import * as preact from "preact";
import * as vlens from "vlens";
import { copy } from "../lib/copy";
import { BirthForm } from "../lib/birthDetails";
import { HeightField, WeightField } from "./MeasurementFields";
import "./entry-form-styles";
import "./birth-details-styles";

export const BirthDetailsFields = ({ form, disabled }: { form: BirthForm; disabled: boolean }) => (
  <details
    className="birth-details"
    open={form.open}
    onToggle={event => {
      form.open = (event.currentTarget as HTMLDetailsElement).open;
    }}
  >
    <summary>{copy.birth.section}</summary>
    <small>{copy.birth.optional}</small>
    <div className="birth-details-row">
      <div className="form-group">
        <label htmlFor="birth-time">{copy.birth.time}</label>
        <input
          type="time"
          id="birth-time"
          {...vlens.attrsBindInput(vlens.ref(form, "time"))}
          disabled={disabled}
        />
      </div>
      <div className="form-group">
        <label htmlFor="birth-place">{copy.birth.place}</label>
        <input
          type="text"
          id="birth-place"
          maxLength={200}
          placeholder={copy.birth.placeHint}
          {...vlens.attrsBindInput(vlens.ref(form, "place"))}
          disabled={disabled}
        />
      </div>
    </div>
    <HeightField entry={form.entry} disabled={disabled} label={copy.birth.length} />
    <WeightField entry={form.entry} disabled={disabled} label={copy.birth.weight} />
  </details>
);
