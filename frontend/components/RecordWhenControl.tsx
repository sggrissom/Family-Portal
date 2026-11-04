import * as preact from "preact";
import * as vlens from "vlens";
import * as server from "../server";
import {
  RecordWhen,
  RecordWhenMode,
  recordWhenControls,
  recordWhenPreview,
  setRecordWhenMode,
} from "../lib/recordWhen";
import { localDateString } from "../lib/when";
import { copy } from "../lib/copy";
import "./when-control-styles";

interface RecordWhenControlProps {
  when: RecordWhen;
  isMoment: boolean;
  disabled?: boolean;
  birthday?: string;
  anchors?: server.Person[];
}

export const RecordWhenControl = (props: RecordWhenControlProps) => {
  const { when, isMoment, disabled, birthday, anchors } = props;
  const t = copy.recordWhen;
  const now = new Date();
  const preview = recordWhenPreview(when, isMoment, birthday, now);
  const exactModes = when.mode === "today" || when.mode === "yesterday";
  const controls = recordWhenControls(when);
  return (
    <div className="record-when">
      <div className="when-control">
        <select
          className="when-select"
          aria-label={copy.when.label}
          disabled={disabled}
          value={when.mode}
          onChange={e => {
            setRecordWhenMode(when, (e.target as HTMLSelectElement).value as RecordWhenMode);
            vlens.scheduleRedraw();
          }}
        >
          <option value="today">{copy.when.today}</option>
          <option value="yesterday">{copy.when.yesterday}</option>
          <option value="date">{copy.when.pickDate}</option>
          <option value="month">{t.month}</option>
          <option value="year">{t.year}</option>
          <option value="range">{t.range}</option>
          <option value="age">{copy.when.byAge}</option>
          <option value="unknown">{t.unknown}</option>
        </select>

        {when.mode === "date" && (
          <input
            type="date"
            className="when-date"
            aria-label={copy.when.date}
            max={localDateString(now)}
            disabled={disabled}
            {...vlens.attrsBindInput(vlens.ref(when, "date"))}
          />
        )}
        {when.mode === "month" && (
          <input
            type="month"
            className="when-date"
            aria-label={t.month}
            placeholder="YYYY-MM"
            disabled={disabled}
            {...vlens.attrsBindInput(vlens.ref(when, "month"))}
          />
        )}
        {when.mode === "year" && (
          <input
            type="text"
            inputMode="numeric"
            className="when-date when-year"
            aria-label={t.year}
            placeholder="1970"
            disabled={disabled}
            {...vlens.attrsBindInput(vlens.ref(when, "year"))}
          />
        )}
        {when.mode === "range" && (
          <span className="when-age">
            <input
              type="text"
              className="when-date when-range"
              aria-label={t.rangeFrom}
              placeholder="1968"
              disabled={disabled}
              {...vlens.attrsBindInput(vlens.ref(when, "from"))}
            />
            <span aria-hidden="true">–</span>
            <input
              type="text"
              className="when-date when-range"
              aria-label={t.rangeTo}
              placeholder="1972"
              disabled={disabled}
              {...vlens.attrsBindInput(vlens.ref(when, "to"))}
            />
          </span>
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
            <span aria-hidden="true">{t.to}</span>
            <input
              type="number"
              min="0"
              max="100"
              inputmode="numeric"
              aria-label={t.ageTo}
              placeholder="yrs"
              disabled={disabled}
              {...vlens.attrsBindInput(vlens.ref(when, "ageToYears"))}
            />
          </span>
        )}
      </div>

      {when.mode === "age" && anchors && anchors.length > 1 && (
        <label className="record-when-row">
          {t.whoseAge}
          <select
            className="when-select"
            disabled={disabled}
            value={String(when.anchorPersonId)}
            onChange={e => {
              when.anchorPersonId = parseInt((e.target as HTMLSelectElement).value);
              vlens.scheduleRedraw();
            }}
          >
            <option value="0">{t.choosePerson}</option>
            {anchors.map(p => (
              <option key={p.id} value={String(p.id)}>
                {p.name}
              </option>
            ))}
          </select>
        </label>
      )}

      {isMoment && controls.meaning && (
        <div className="record-when-row" role="radiogroup" aria-label={t.meaning}>
          <label>
            <input
              type="radio"
              name="record-when-meaning"
              checked={when.meaning === "occurrence"}
              disabled={disabled}
              onChange={() => setMeaning(when, "occurrence")}
            />{" "}
            {t.happened}
          </label>
          <label>
            <input
              type="radio"
              name="record-when-meaning"
              checked={when.meaning === "period"}
              disabled={disabled}
              onChange={() => setMeaning(when, "period")}
            />{" "}
            {t.describes}
          </label>
        </div>
      )}

      {controls.estimate && (
        <div className="record-when-row">
          <label>
            <input
              type="checkbox"
              disabled={disabled}
              {...vlens.attrsBindInput(vlens.ref(when, "estimated"))}
            />{" "}
            {t.estimate}
          </label>
          {when.estimated && controls.bestGuess && (
            <input
              type="text"
              className="when-date when-range"
              aria-label={t.bestGuess}
              placeholder={when.mode === "age" ? "8" : "1970"}
              disabled={disabled}
              {...vlens.attrsBindInput(vlens.ref(when, "around"))}
            />
          )}
        </div>
      )}

      {!exactModes && preview.label && (
        <p className="record-when-preview" aria-live="polite">
          <strong>{preview.label}</strong>
          {preview.bounds && <span> · {preview.bounds}</span>}
        </p>
      )}
    </div>
  );
};

function setMeaning(when: RecordWhen, meaning: "occurrence" | "period") {
  when.meaning = meaning;
  vlens.scheduleRedraw();
}
