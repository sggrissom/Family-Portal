import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as auth from "../../lib/authCache";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { ensureAuthInFetch, requireAuthInView } from "../../lib/authHelpers";
import {
  AgeChart,
  AgeSeries,
  ChartZoom,
  SERIES_COLORS,
  newChartZoom,
} from "../../components/AgeChart";
import { PersonChips } from "../../components/PersonChips";
import { SegmentedControl } from "../../components/SegmentedControl";
import { chipOrder } from "../../lib/familyGroups";
import { timelineRequest } from "../../lib/photoPages";
import { Metric, chartPoints, percentileBand } from "../../lib/ageChart";
import { isValidBirthday } from "../../lib/growthPercentiles";
import { monthsOld } from "../../lib/sameAge";
import { copy } from "../../lib/copy";
import "./family-chart-styles";

type GrowthPageData = {
  people: server.Person[];
  growth: Map<number, server.GrowthData[]>;
};

export async function fetch(route: string, prefix: string): Promise<rpc.Response<GrowthPageData>> {
  if (!(await ensureAuthInFetch())) {
    return rpc.ok<GrowthPageData>({ people: [], growth: new Map() });
  }
  const [resp, err] = await server.GetFamilyTimeline(
    timelineRequest({ skipMilestones: true, skipPhotos: true })
  );
  if (!resp) return [null, err];

  const measured = resp.people.filter(
    item => (item.growthData ?? []).length > 0 && isValidBirthday(item.person.birthday)
  );
  return rpc.ok<GrowthPageData>({
    people: chipOrder(
      measured.map(item => item.person),
      resp.relations ?? [],
      auth.getAuth()?.familyId ?? 0
    ),
    growth: new Map(measured.map(item => [item.person.id, item.growthData ?? []])),
  });
}

type Bands = "off" | "girls" | "boys";

type GrowthPageState = {
  selected: number[] | null;
  metric: Metric;
  bands: Bands;
  zoom: ChartZoom;
};

const useGrowthPageState = vlens.declareHook(
  (): GrowthPageState => ({
    selected: null,
    metric: "height",
    bands: "off",
    zoom: newChartZoom(),
  })
);

const CHILD_MONTHS = 18 * 12;

function defaultSelection(data: GrowthPageData): number[] {
  const now = new Date().toISOString();
  const familyId = auth.getAuth()?.familyId ?? 0;
  const kids = data.people.filter(
    p => p.familyId === familyId && monthsOld(p.birthday, now) < CHILD_MONTHS
  );
  return (kids.length > 0 ? kids : data.people).map(p => p.id);
}

function togglePerson(state: GrowthPageState, personId: number) {
  const selected = state.selected ?? [];
  state.selected = selected.includes(personId)
    ? selected.filter(id => id !== personId)
    : [...selected, personId];
  vlens.scheduleRedraw();
}

function chooseMetric(state: GrowthPageState, metric: Metric) {
  state.metric = metric;
  vlens.scheduleRedraw();
}

function chooseBands(state: GrowthPageState, bands: Bands) {
  state.bands = bands;
  vlens.scheduleRedraw();
}

export function view(route: string, prefix: string, data: GrowthPageData): preact.ComponentChild {
  if (!requireAuthInView()) return;

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="growth-page-container">
        <GrowthPage data={data} />
      </main>
      <Footer />
    </div>
  );
}

const GrowthPage = ({ data }: { data: GrowthPageData }) => {
  const state = useGrowthPageState();
  state.selected ??= defaultSelection(data);
  const selected = state.selected;

  const series: AgeSeries[] = data.people
    .filter(p => selected.includes(p.id))
    .map(person => ({
      key: person.id,
      label: person.name.split(" ")[0],
      color: SERIES_COLORS[data.people.indexOf(person) % SERIES_COLORS.length],
      points: chartPoints(data.growth.get(person.id) ?? [], person.birthday, state.metric),
    }))
    .filter(s => s.points.length > 0);

  const maxAge = Math.max(0, ...series.flatMap(s => s.points.map(p => p.ageMonths)));
  const band =
    state.bands === "off"
      ? []
      : percentileBand(state.bands === "boys" ? 0 : 1, state.metric, 0, maxAge);

  return (
    <div className="growth-page">
      <div className="growth-page-head">
        <h1>{copy.growthPage.title}</h1>
        {auth.canContributeAnywhere() && (
          <a href="/add-growth" className="btn btn-primary">
            {copy.growthPage.measure}
          </a>
        )}
      </div>

      {data.people.length === 0 ? (
        <p className="growth-page-empty">{copy.growthPage.noData}</p>
      ) : (
        <>
          <PersonChips
            people={data.people}
            selected={selected}
            onToggle={vlens.cachePartial(togglePerson, state)}
            label={copy.growthPage.people}
          />
          <div className="growth-page-controls">
            <SegmentedControl
              label={copy.growthPage.metric}
              options={[
                { value: "height", label: copy.measurement.height },
                { value: "weight", label: copy.measurement.weight },
              ]}
              value={state.metric}
              onChange={vlens.cachePartial(chooseMetric, state)}
            />
            <span className="growth-page-bands">
              <span>{copy.growthPage.bands}</span>
              <SegmentedControl
                label={copy.growthPage.bands}
                options={[
                  { value: "off", label: copy.growthPage.bandsOff },
                  { value: "girls", label: copy.growthPage.girls },
                  { value: "boys", label: copy.growthPage.boys },
                ]}
                value={state.bands}
                onChange={vlens.cachePartial(chooseBands, state)}
              />
            </span>
          </div>
          {series.length === 0 ? (
            <p className="growth-page-empty">{copy.growthPage.empty}</p>
          ) : (
            <AgeChart
              series={series}
              metric={state.metric}
              band={band}
              label={`${copy.growthPage.title}: ${state.metric}`}
              zoom={state.zoom}
            />
          )}
        </>
      )}
    </div>
  );
};
