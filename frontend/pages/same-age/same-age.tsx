import * as preact from "preact";
import * as vlens from "vlens";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView } from "../../lib/authHelpers";
import { SameAgeRows } from "../../components/SameAgeRows";
import { SameAgeMontage } from "../../components/SameAgeMontage";
import { localDateString } from "../../lib/when";
import { copy } from "../../lib/copy";
import {
  ageTitle,
  hasSameAgeRecords,
  nearbyRecordedAge,
  parseAgeParam,
  sameAgePath,
} from "../../lib/sameAge";
import {
  sameAgeNavigation,
  selectSameAge,
  recordedAgeIndex,
  type SameAgeNavigation,
} from "../../lib/sameAgeNavigation";
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

const useNavigation = vlens.declareHook(sameAgeNavigation);

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

function goTo(state: SameAgeNavigation, age: number) {
  void selectSameAge(
    state,
    age,
    ageMonths =>
      server.GetSameAge({
        ageMonths,
        fromPersonId: state.data.fromPersonId,
        today: localDateString(new Date()),
      }),
    ageMonths =>
      history.replaceState(history.state, "", sameAgePath(ageMonths, state.data.fromPersonId)),
    vlens.scheduleRedraw
  );
}

export const SameAgePage = ({ data: initial }: { data: server.GetSameAgeResponse }) => {
  const navigation = useNavigation(initial);
  const data = navigation.data;
  const shown = navigation.previewAge ?? navigation.selectedAge;
  const today = localDateString(new Date());
  const rows = data.rows ?? [];
  const recorded = rows.filter(hasSameAgeRecords);
  const ages = data.availableAges ?? [];
  const ageMonths = ages.map(a => a.ageMonths);
  const younger = nearbyRecordedAge(ageMonths, navigation.selectedAge, -1);
  const older = nearbyRecordedAge(ageMonths, navigation.selectedAge, 1);
  const missing = rows.length - recorded.length;

  return (
    <div className="same-age-page">
      <h1>{copy.sameAge.title}</h1>
      <p className="same-age-none">{copy.sameAge.intro}</p>
      {ages.length > 0 && (
        <label className="same-age-browse">
          {copy.sameAge.recordedAges}
          <select
            value={navigation.selectedAge}
            onChange={event => goTo(navigation, Number((event.target as HTMLSelectElement).value))}
          >
            {!ages.some(a => a.ageMonths === navigation.selectedAge) && (
              <option value={navigation.selectedAge}>
                {ageTitle(navigation.selectedAge)} —{" "}
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
        <button
          type="button"
          className="same-age-step"
          aria-label={copy.sameAge.youngerWithRecords}
          disabled={younger === null}
          onClick={() => younger !== null && goTo(navigation, younger)}
        >
          ◀
        </button>
        <strong className="same-age-age" aria-live="polite">
          {copy.sameAge.at} {ageTitle(shown)}
        </strong>
        <button
          type="button"
          className="same-age-step"
          aria-label={copy.sameAge.olderWithRecords}
          disabled={older === null}
          onClick={() => older !== null && goTo(navigation, older)}
        >
          ▶
        </button>
      </div>
      {ages.length > 1 && (
        <>
          <input
            type="range"
            className="same-age-slider"
            aria-label={copy.sameAge.recordedAges}
            aria-valuetext={ageTitle(shown)}
            min={0}
            max={ages.length - 1}
            step={1}
            value={recordedAgeIndex(ageMonths, shown)}
            onInput={event => {
              navigation.previewAge = ageMonths[Number((event.target as HTMLInputElement).value)];
              vlens.scheduleRedraw();
            }}
            onChange={event =>
              goTo(navigation, ageMonths[Number((event.target as HTMLInputElement).value)])
            }
          />
          <p className="same-age-none same-age-slider-help">{copy.sameAge.sliderHelp}</p>
        </>
      )}
      <div className="same-age-load-status" role="status" aria-live="polite">
        {navigation.loading && copy.sameAge.loading}
        {navigation.error && (
          <>
            <span>{navigation.error}</span>{" "}
            <button type="button" onClick={() => goTo(navigation, navigation.selectedAge)}>
              {copy.sameAge.retry}
            </button>
          </>
        )}
      </div>
      <div className="same-age-results" aria-busy={navigation.loading}>
        {navigation.error ? null : recorded.length === 0 ? (
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
    </div>
  );
};
