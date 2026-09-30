import * as preact from "preact";
import * as vlens from "vlens";
import { Ref } from "vlens/refs";
import { copy } from "../lib/copy";
import { CheckupEntry, HEIGHT_UNITS, HeightUnit, WEIGHT_UNITS, WeightUnit } from "../lib/checkup";
import { SegmentedControl } from "./SegmentedControl";
import "./entry-form-styles";

function setHeightUnit(entry: CheckupEntry, unit: HeightUnit) {
  entry.heightUnit = unit;
  entry.height = entry.feet = entry.inches = "";
  vlens.scheduleRedraw();
}

function setWeightUnit(entry: CheckupEntry, unit: WeightUnit) {
  entry.weightUnit = unit;
  entry.weight = entry.pounds = entry.ounces = "";
  vlens.scheduleRedraw();
}

interface FieldProps {
  entry: CheckupEntry;
  disabled: boolean;
  note?: string;
}

export const HeightField = ({ entry, disabled, note }: FieldProps) => (
  <div className="checkup-field">
    <div className="checkup-field-head">
      <label htmlFor={entry.heightUnit === "ft-in" ? "feet" : "height"}>
        {copy.measurement.height}
      </label>
      <SegmentedControl
        label={copy.measurement.heightUnit}
        options={HEIGHT_UNITS}
        value={entry.heightUnit}
        onChange={vlens.cachePartial(setHeightUnit, entry)}
        disabled={disabled}
      />
    </div>
    <div className="checkup-inputs">
      {entry.heightUnit === "ft-in" ? (
        <>
          <UnitInput
            id="feet"
            suffix="ft"
            label={copy.measurement.feet}
            bind={vlens.ref(entry, "feet")}
            disabled={disabled}
          />
          <UnitInput
            id="inches"
            suffix="in"
            label={copy.measurement.inches}
            bind={vlens.ref(entry, "inches")}
            disabled={disabled}
          />
        </>
      ) : (
        <UnitInput
          id="height"
          suffix={entry.heightUnit}
          bind={vlens.ref(entry, "height")}
          disabled={disabled}
        />
      )}
    </div>
    {note && <small className="checkup-last">{note}</small>}
  </div>
);

export const WeightField = ({
  entry,
  disabled,
  note,
  units = WEIGHT_UNITS,
}: FieldProps & { units?: { value: WeightUnit; label: string }[] }) => (
  <div className="checkup-field">
    <div className="checkup-field-head">
      <label htmlFor={entry.weightUnit === "lb-oz" ? "pounds" : "weight"}>
        {copy.measurement.weight}
      </label>
      <SegmentedControl
        label={copy.measurement.weightUnit}
        options={units}
        value={entry.weightUnit}
        onChange={vlens.cachePartial(setWeightUnit, entry)}
        disabled={disabled}
      />
    </div>
    <div className="checkup-inputs">
      {entry.weightUnit === "lb-oz" ? (
        <>
          <UnitInput
            id="pounds"
            suffix="lb"
            label={copy.measurement.pounds}
            bind={vlens.ref(entry, "pounds")}
            disabled={disabled}
          />
          <UnitInput
            id="ounces"
            suffix="oz"
            label={copy.measurement.ounces}
            bind={vlens.ref(entry, "ounces")}
            disabled={disabled}
          />
        </>
      ) : (
        <UnitInput
          id="weight"
          suffix={entry.weightUnit}
          bind={vlens.ref(entry, "weight")}
          disabled={disabled}
        />
      )}
    </div>
    {note && <small className="checkup-last">{note}</small>}
  </div>
);

interface UnitInputProps {
  id: string;
  suffix: string;
  label?: string;
  bind: Ref;
  disabled: boolean;
}

const UnitInput = ({ id, suffix, label, bind, disabled }: UnitInputProps) => (
  <span className="unit-input">
    <input
      id={id}
      type="text"
      inputmode="decimal"
      autocomplete="off"
      aria-label={label}
      disabled={disabled}
      {...vlens.attrsBindInput(bind)}
    />
    <span className="unit-input-suffix" aria-hidden="true">
      {suffix}
    </span>
  </span>
);
