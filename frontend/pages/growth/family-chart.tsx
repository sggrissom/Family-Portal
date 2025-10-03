import * as preact from "preact";
import * as vlens from "vlens";
import * as core from "vlens/core";
import * as auth from "../../lib/authCache";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { MultiPersonChart, PersonGrowthData } from "../../components/chart/multi-person-chart";
import "./family-chart-styles";

type FamilyChartState = {
  selectedPersonIds: Set<number>;
  showHeight: boolean;
  showWeight: boolean;
};

const useFamilyChartState = vlens.declareHook(
  (): FamilyChartState => ({
    selectedPersonIds: new Set(),
    showHeight: true,
    showWeight: true,
  })
);

type ChartDataLoadState = {
  data: server.ComparePeopleResponse | null;
  loading: boolean;
  error: string | null;
};

const useChartDataLoad = vlens.declareHook(
  (): ChartDataLoadState => ({
    data: null,
    loading: false,
    error: null,
  })
);

// Color palette for different people (works in both light and dark themes)
const PERSON_COLORS = [
  "#3b82f6", // blue
  "#10b981", // green
  "#8b5cf6", // purple
  "#f59e0b", // orange
  "#ec4899", // pink
];

export async function fetch(route: string, prefix: string) {
  return server.ListPeople({});
}

type FamilyChartData = server.ListPeopleResponse;

const loadChartData = async (state: FamilyChartState, loadState: ChartDataLoadState) => {
  if (state.selectedPersonIds.size === 0) {
    loadState.data = null;
    loadState.loading = false;
    loadState.error = null;
    vlens.scheduleRedraw();
    return;
  }

  loadState.loading = true;
  loadState.error = null;
  vlens.scheduleRedraw();

  const personIds = Array.from(state.selectedPersonIds);
  const [resp, err] = await server.ComparePeople({ personIds });

  if (err || !resp) {
    loadState.error = err || "Failed to load growth data";
    loadState.data = null;
  } else {
    loadState.data = resp;
  }

  loadState.loading = false;
  vlens.scheduleRedraw();
};

const togglePersonSelection = (
  state: FamilyChartState,
  loadState: ChartDataLoadState,
  personId: number
) => {
  if (state.selectedPersonIds.has(personId)) {
    state.selectedPersonIds.delete(personId);
  } else {
    if (state.selectedPersonIds.size >= 5) {
      alert("You can compare up to 5 people at once");
      return;
    }
    state.selectedPersonIds.add(personId);
  }
  vlens.scheduleRedraw();
  loadChartData(state, loadState);
};

const clearSelection = (state: FamilyChartState, loadState: ChartDataLoadState) => {
  state.selectedPersonIds.clear();
  loadState.data = null;
  loadState.error = null;
  vlens.scheduleRedraw();
};

const toggleMeasurementType = (state: FamilyChartState, type: "height" | "weight") => {
  if (type === "height") {
    state.showHeight = !state.showHeight;
  } else {
    state.showWeight = !state.showWeight;
  }
  vlens.scheduleRedraw();
};

export function view(route: string, prefix: string, data: FamilyChartData): preact.ComponentChild {
  const currentAuth = auth.getAuth();
  if (!currentAuth || currentAuth.id <= 0) {
    auth.clearAuth();
    core.setRoute("/login");
    return;
  }

  const people = data.people || [];

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="family-chart-page">
        <FamilyChartPage people={people} />
      </main>
      <Footer />
    </div>
  );
}

interface FamilyChartPageProps {
  people: server.Person[];
}

const FamilyChartPage = ({ people }: FamilyChartPageProps) => {
  const state = useFamilyChartState();
  const loadState = useChartDataLoad();

  // Prepare data for chart
  const chartPeopleData: PersonGrowthData[] = [];
  if (loadState.data && loadState.data.people) {
    loadState.data.people.forEach((personData, idx) => {
      chartPeopleData.push({
        person: personData.person,
        growthData: personData.growthData || [],
        color: PERSON_COLORS[idx % PERSON_COLORS.length],
      });
    });
  }

  return (
    <div className="family-chart-container">
      <div className="family-chart-header">
        <h1>Family Growth Chart</h1>
        <p className="subtitle">Compare growth measurements across family members</p>
      </div>

      <div className="chart-controls">
        <div className="person-selection">
          <h3>Select People (up to 5)</h3>
          <div className="person-checkboxes">
            {people.map(person => (
              <label key={person.id} className="person-checkbox">
                <input
                  type="checkbox"
                  checked={state.selectedPersonIds.has(person.id)}
                  onChange={() => togglePersonSelection(state, loadState, person.id)}
                />
                <span className="checkbox-label">{person.name}</span>
              </label>
            ))}
          </div>
          {state.selectedPersonIds.size > 0 && (
            <button className="btn-clear" onClick={() => clearSelection(state, loadState)}>
              Clear Selection
            </button>
          )}
        </div>

        <div className="measurement-filters">
          <h3>Show Measurements</h3>
          <div className="filter-toggles">
            <label className="filter-toggle">
              <input
                type="checkbox"
                checked={state.showHeight}
                onChange={() => toggleMeasurementType(state, "height")}
              />
              <span className="toggle-label">Height (solid line)</span>
            </label>
            <label className="filter-toggle">
              <input
                type="checkbox"
                checked={state.showWeight}
                onChange={() => toggleMeasurementType(state, "weight")}
              />
              <span className="toggle-label">Weight (dashed line)</span>
            </label>
          </div>
        </div>
      </div>

      <div className="chart-display">
        {loadState.loading && (
          <div className="loading-state">
            <p>Loading growth data...</p>
          </div>
        )}

        {loadState.error && (
          <div className="error-state">
            <p>Error: {loadState.error}</p>
          </div>
        )}

        {!loadState.loading && !loadState.error && state.selectedPersonIds.size === 0 && (
          <div className="empty-state">
            <p>📈 Select one or more people above to view their growth chart</p>
          </div>
        )}

        {!loadState.loading &&
          !loadState.error &&
          state.selectedPersonIds.size > 0 &&
          chartPeopleData.length > 0 && (
            <div className="chart-wrapper">
              <MultiPersonChart
                peopleData={chartPeopleData}
                width={800}
                height={500}
                showHeight={state.showHeight}
                showWeight={state.showWeight}
              />
            </div>
          )}
      </div>

      <div className="chart-info">
        <h3>How to use</h3>
        <ul>
          <li>Select up to 5 family members to compare their growth measurements</li>
          <li>Toggle height and weight measurements on or off</li>
          <li>Solid lines represent height measurements</li>
          <li>Dashed lines represent weight measurements</li>
          <li>Each person is assigned a different color</li>
          <li>Click on any data point to see detailed information</li>
          <li>Use Ctrl/Cmd + scroll to zoom, or pinch on mobile devices</li>
        </ul>
      </div>
    </div>
  );
};
