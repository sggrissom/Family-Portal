import * as preact from "preact";
import * as vlens from "vlens";
import * as core from "vlens/core";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView } from "../../lib/authHelpers";
import { SameAgeRows } from "../../components/SameAgeRows";
import { SameAgeMontage } from "../../components/SameAgeMontage";
import { localDateString } from "../../lib/when";
import { copy } from "../../lib/copy";
import {
  ageStep,
  ageTitle,
  hasSameAgeRecords,
  nearbyRecordedAge,
  parseAgeParam,
  sameAgePath,
} from "../../lib/sameAge";
import "./same-age-styles";

export async function fetch(route: string, prefix: string) {
  const params = new URLSearchParams(route.split("?")[1] ?? "");
  return server.GetSameAge({
    includeAvailableAges: true,
    ageMonths: parseAgeParam(params.get("age")),
    fromPersonId: parseInt(params.get("from") ?? "") || 0,
    today: localDateString(new Date()),
  });
}

type SliderState = { value: number | null };
const useSlider = vlens.declareHook(
  (data: server.GetSameAgeResponse): SliderState => ({ value: null })
);

export function view(
  route: string,
  prefix: string,
  data: server.GetSameAgeResponse
): preact.ComponentChild {
  if (!requireAuthInView()) return;

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="same-age-container">
        <SameAgePage data={data} />
      </main>
      <Footer />
    </div>
  );
}

function goTo(data: server.GetSameAgeResponse, ageMonths: number) {
  core.replaceRoute(sameAgePath(ageMonths, data.fromPersonId));
}

function snap(ageMonths: number): number {
  const step = ageStep(ageMonths);
  return Math.round(ageMonths / step) * step;
}

function sliderInput(slider: SliderState, event: Event) {
  const input = event.target as HTMLInputElement;
  slider.value = Math.min(Number(input.max), snap(Number(input.value)));
  vlens.scheduleRedraw();
}

function sliderChange(slider: SliderState, data: server.GetSameAgeResponse) {
  if (slider.value !== null && slider.value !== data.ageMonths) goTo(data, slider.value);
}

const SameAgePage = ({ data }: { data: server.GetSameAgeResponse }) => {
  const slider = useSlider(data);
  const shown = slider.value ?? data.ageMonths;
  const today = localDateString(new Date());
  const max = Math.max(data.maxAgeMonths, data.ageMonths);
  const rows = data.rows ?? [];
  const recorded = rows.filter(hasSameAgeRecords);
  const ages = data.availableAges ?? [];
  const younger = nearbyRecordedAge(
    ages.map(a => a.ageMonths),
    data.ageMonths,
    -1
  );
  const older = nearbyRecordedAge(
    ages.map(a => a.ageMonths),
    data.ageMonths,
    1
  );
  const missing = rows.length - recorded.length;

  return (
    <div className="same-age-page">
      <h1>{copy.sameAge.title}</h1>
      <p className="same-age-none">{copy.sameAge.intro}</p>
      {ages.length > 0 && (
        <label className="same-age-browse">
          {copy.sameAge.recordedAges}
          <select
            value={data.ageMonths}
            onChange={event => goTo(data, Number((event.target as HTMLSelectElement).value))}
          >
            {!ages.some(a => a.ageMonths === data.ageMonths) && (
              <option value={data.ageMonths}>
                {ageTitle(data.ageMonths)} —{" "}
                {recorded.length
                  ? copy.sameAge.peopleWithRecords(recorded.length)
                  : copy.sameAge.noRecords}
              </option>
            )}
            {ages.map(age => (
              <option key={age.ageMonths} value={age.ageMonths}>
                {ageTitle(age.ageMonths)} — {copy.sameAge.peopleWithRecords(age.peopleCount)}
              </option>
            ))}
          </select>
        </label>
      )}
      <div className="same-age-control">
        <span className="same-age-at">{copy.sameAge.at}</span>
        <button
          type="button"
          className="same-age-step"
          aria-label={copy.sameAge.youngerWithRecords}
          disabled={younger === null}
          onClick={() => younger !== null && goTo(data, younger)}
        >
          ◀
        </button>
        <strong className="same-age-age" aria-live="polite">
          {ageTitle(shown)}
        </strong>
        <button
          type="button"
          className="same-age-step"
          aria-label={copy.sameAge.olderWithRecords}
          disabled={older === null}
          onClick={() => older !== null && goTo(data, older)}
        >
          ▶
        </button>
      </div>
      <input
        type="range"
        className="same-age-slider"
        aria-label={copy.sameAge.title}
        min={0}
        max={max}
        value={shown}
        onInput={vlens.cachePartial(sliderInput, slider)}
        onChange={() => sliderChange(slider, data)}
      />

      {recorded.length === 0 ? (
        <p className="same-age-empty" role="status">
          {data.peopleCount === 0
            ? copy.sameAge.empty
            : ages.length
              ? copy.sameAge.nothingAtAge
              : copy.sameAge.noSavedRecords}
        </p>
      ) : (
        <>
          <p className="same-age-none" role="status">
            {copy.sameAge.peopleWithRecords(recorded.length)}
            {recorded.length === 1 && ` · ${copy.sameAge.onlyOne}`}
          </p>
          <SameAgeMontage rows={rows} ageMonths={data.ageMonths} showMissing={false} />
          <SameAgeRows rows={rows} ageMonths={data.ageMonths} today={today} hideEmpty />
          {missing > 0 && <p className="same-age-none">{copy.sameAge.missingRecords(missing)}</p>}
        </>
      )}
    </div>
  );
};
