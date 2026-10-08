import * as preact from "preact";
import * as vlens from "vlens";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView } from "../../lib/authHelpers";
import { SameAgeRows } from "../../components/SameAgeRows";
import { SameAgePortraits } from "../../components/SameAgePortraits";
import { localDateString } from "../../lib/when";
import { copy } from "../../lib/copy";
import {
  AGE_SHORTCUTS,
  ageHeading,
  ageTitle,
  hasSameAgeRecords,
  nearbyRecordedAge,
  parseAgeParam,
  parseViewParam,
  sameAgePath,
  type SameAgeView,
} from "../../lib/sameAge";
import {
  sameAgeNavigation,
  selectSameAge,
  type SameAgeNavigation,
} from "../../lib/sameAgeNavigation";
import "./same-age-styles";

function routeParams(route: string) {
  return new URLSearchParams(route.split("?")[1] ?? "");
}

export async function fetch(route: string, prefix: string) {
  const params = routeParams(route);
  return server.GetSameAge({
    includeAvailableAges: true,
    details: parseViewParam(params.get("view")) === "details",
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
        <SameAgePage data={data} view={parseViewParam(routeParams(route).get("view"))} />
      </main>
      <Footer />
    </div>
  );
}

function updateURL(state: SameAgeNavigation) {
  history.replaceState(
    history.state,
    "",
    sameAgePath(state.selectedAge, state.data.fromPersonId, state.view)
  );
}

function goTo(state: SameAgeNavigation, age: number) {
  void selectSameAge(
    state,
    age,
    ageMonths =>
      server.GetSameAge({
        includeAvailableAges: false,
        details: false,
        ageMonths,
        fromPersonId: state.data.fromPersonId,
        today: localDateString(new Date()),
      }),
    () => updateURL(state),
    vlens.scheduleRedraw
  );
}

function switchView(state: SameAgeNavigation, view: SameAgeView) {
  state.view = view;
  updateURL(state);
  vlens.scheduleRedraw();
}

function pickAge(state: SameAgeNavigation, event: Event) {
  goTo(state, Number((event.target as HTMLSelectElement).value));
}

interface SameAgePageProps {
  data: server.GetSameAgeResponse;
  view: SameAgeView;
}

export const SameAgePage = ({ data: initial, view }: SameAgePageProps) => {
  const navigation = useNavigation(initial, view);
  const data = navigation.data;
  const selected = navigation.selectedAge;
  const portraits = navigation.view === "portraits";
  const ages = (portraits ? data.portraitAges : data.availableAges) ?? [];
  const ageMonths = ages.map(a => a.ageMonths);
  const younger = nearbyRecordedAge(ageMonths, selected, -1);
  const older = nearbyRecordedAge(ageMonths, selected, 1);
  const headingAge = navigation.error ? selected : data.ageMonths;
  const count = portraits ? copy.sameAge.pictured : copy.sameAge.withRecords;

  return (
    <div className="same-age-page">
      <h1>{ageHeading(headingAge)}</h1>

      <div className="same-age-views" role="group" aria-label={copy.sameAge.views}>
        {(["portraits", "details"] as SameAgeView[]).map(v => (
          <button
            key={v}
            type="button"
            aria-pressed={navigation.view === v}
            onClick={vlens.cachePartial(switchView, navigation, v)}
          >
            {copy.sameAge[v]}
          </button>
        ))}
      </div>

      <div className="same-age-shortcuts" role="group" aria-label={copy.sameAge.shortcuts}>
        {AGE_SHORTCUTS.map(age => (
          <button
            key={age}
            type="button"
            aria-pressed={selected === age}
            onClick={vlens.cachePartial(goTo, navigation, age)}
          >
            {ageTitle(age)}
          </button>
        ))}
      </div>

      <div className="same-age-control">
        <button
          type="button"
          className="same-age-step"
          aria-label={copy.sameAge.younger}
          disabled={younger === null}
          onClick={() => younger !== null && goTo(navigation, younger)}
        >
          ◀
        </button>
        <select
          className="same-age-picker"
          aria-label={copy.sameAge.chooseAge}
          value={selected}
          onChange={vlens.cachePartial(pickAge, navigation)}
        >
          {!ageMonths.includes(selected) && <option value={selected}>{ageTitle(selected)}</option>}
          {ages.map(age => (
            <option key={age.ageMonths} value={age.ageMonths}>
              {ageTitle(age.ageMonths)} · {count(age.peopleCount)}
            </option>
          ))}
        </select>
        <button
          type="button"
          className="same-age-step"
          aria-label={copy.sameAge.older}
          disabled={older === null}
          onClick={() => older !== null && goTo(navigation, older)}
        >
          ▶
        </button>
      </div>
      {selected === 0 && <p className="same-age-none same-age-help">{copy.sameAge.newbornHelp}</p>}

      <div className="same-age-load-status" role="status" aria-live="polite">
        {navigation.loading && copy.sameAge.loading(ageTitle(selected))}
        {navigation.error && (
          <>
            <span>{navigation.error}</span>{" "}
            <button type="button" onClick={() => goTo(navigation, selected)}>
              {copy.sameAge.retry}
            </button>
          </>
        )}
      </div>

      <div className="same-age-results" aria-busy={navigation.loading}>
        {navigation.error ? null : data.peopleCount === 0 ? (
          <p className="same-age-empty" role="status">
            {copy.sameAge.empty}
          </p>
        ) : portraits ? (
          <SameAgePortraits rows={data.rows ?? []} ageMonths={data.ageMonths} />
        ) : (
          <SameAgeDetails data={data} hasAnyRecords={data.availableAges.length > 0} />
        )}
      </div>
    </div>
  );
};

const SameAgeDetails = ({
  data,
  hasAnyRecords,
}: {
  data: server.GetSameAgeResponse;
  hasAnyRecords: boolean;
}) => {
  const rows = data.rows ?? [];
  const recorded = rows.filter(hasSameAgeRecords);
  const missing = rows.length - recorded.length;
  if (recorded.length === 0) {
    return (
      <p className="same-age-empty" role="status">
        {hasAnyRecords ? copy.sameAge.nothingAtAge : copy.sameAge.noSavedRecords}
      </p>
    );
  }
  return (
    <>
      <p className="same-age-none">
        {copy.sameAge.peopleWithRecords(recorded.length)}
        {recorded.length === 1 && ` · ${copy.sameAge.onlyOne}`}
      </p>
      <SameAgeRows
        rows={rows}
        ageMonths={data.ageMonths}
        today={localDateString(new Date())}
        hideEmpty
      />
      {missing > 0 && <p className="same-age-none">{copy.sameAge.missingRecords(missing)}</p>}
    </>
  );
};
