import * as preact from "preact";
import * as vlens from "vlens";
import * as core from "vlens/core";
import * as auth from "../../lib/authCache";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { calculateAge, formatDate } from "../../lib/dateUtils";
import { ThumbnailImage } from "../../components/ResponsiveImage";
import { usePhotoStatus } from "../../hooks/usePhotoStatus";
import "./compare-styles";

type CompareState = {
  selectedPersonIds: Set<number>;
  selectedAgeFilter: string; // "all" or year number as string
  visibleTypes: {
    milestones: boolean;
    measurements: boolean;
    photos: boolean;
  };
};

const useCompareState = vlens.declareHook(
  (): CompareState => ({
    selectedPersonIds: new Set(),
    selectedAgeFilter: "all",
    visibleTypes: {
      milestones: true,
      measurements: true,
      photos: true,
    },
  })
);

export async function fetch(route: string, prefix: string) {
  // Fetch list of all people for selection
  return server.ListPeople({});
}

type CompareData = server.ListPeopleResponse;

// Helper function to extract numeric age in years from an age string
const getAgeInYears = (ageString: string): number => {
  if (!ageString || ageString === "Newborn") return 0;

  const yearMatch = ageString.match(/(\d+)\s+years?/);
  if (yearMatch) {
    return parseInt(yearMatch[1]);
  }

  if (ageString.includes("month")) {
    return 0;
  }

  return 0;
};

const togglePersonSelection = (
  state: CompareState,
  loadState: ComparisonLoadState,
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
  // Trigger comparison load
  loadComparison(state, loadState);
};

const setAgeFilter = (state: CompareState, filter: string) => {
  state.selectedAgeFilter = filter;
  vlens.scheduleRedraw();
};

const toggleType = (state: CompareState, type: "milestones" | "measurements" | "photos") => {
  state.visibleTypes[type] = !state.visibleTypes[type];
  vlens.scheduleRedraw();
};

const getCategoryIcon = (category: string) => {
  switch (category) {
    case "development":
      return "🌱";
    case "behavior":
      return "😊";
    case "health":
      return "🏥";
    case "achievement":
      return "🏆";
    case "first":
      return "⭐";
    case "other":
      return "📝";
    default:
      return "📝";
  }
};

const getCategoryLabel = (category: string) => {
  switch (category) {
    case "development":
      return "Development";
    case "behavior":
      return "Behavior";
    case "health":
      return "Health";
    case "achievement":
      return "Achievement";
    case "first":
      return "First Time";
    case "other":
      return "Other";
    default:
      return "Other";
  }
};

const getMeasurementTypeLabel = (type: server.MeasurementType) => {
  return type === server.Height ? "Height" : "Weight";
};

export function view(route: string, prefix: string, data: CompareData): preact.ComponentChild {
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
      <main id="app" className="compare-page">
        <ComparePage people={people} />
      </main>
      <Footer />
    </div>
  );
}

interface ComparePageProps {
  people: server.Person[];
}

type ComparisonLoadState = {
  data: server.ComparePeopleResponse | null;
  loading: boolean;
  error: string | null;
};

const useComparisonLoad = vlens.declareHook(
  (): ComparisonLoadState => ({
    data: null,
    loading: false,
    error: null,
  })
);

const loadComparison = async (state: CompareState, loadState: ComparisonLoadState) => {
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
    loadState.error = err || "Failed to load comparison data";
    loadState.data = null;
  } else {
    loadState.data = resp;
  }

  loadState.loading = false;
  vlens.scheduleRedraw();
};

const ComparePage = ({ people }: ComparePageProps) => {
  const state = useCompareState();
  const loadState = useComparisonLoad();
  const photoStatus = usePhotoStatus();

  // Calculate available age filters from all selected people's data
  const ageYears = new Set<number>();
  if (loadState.data) {
    loadState.data.people.forEach((personData: server.PersonComparisonData) => {
      const allItems = [
        ...(state.visibleTypes.milestones && personData.milestones
          ? personData.milestones.map((m: server.Milestone) => ({
              date: m.milestoneDate,
              birthday: personData.person.birthday,
            }))
          : []),
        ...(state.visibleTypes.measurements && personData.growthData
          ? personData.growthData.map((g: server.GrowthData) => ({
              date: g.measurementDate,
              birthday: personData.person.birthday,
            }))
          : []),
        ...(state.visibleTypes.photos && personData.photos
          ? personData.photos.map((p: server.Image) => ({
              date: p.photoDate,
              birthday: personData.person.birthday,
            }))
          : []),
      ];

      allItems.forEach(item => {
        const age = calculateAge(item.birthday, item.date);
        const ageInYears = getAgeInYears(age);
        ageYears.add(ageInYears);
      });
    });
  }
  const sortedAgeYears = Array.from(ageYears).sort((a, b) => a - b);

  return (
    <div>
      {/* Header */}
      <div className="compare-header">
        <h1>Compare People</h1>
        <p>Select 2-5 people to compare their timelines, milestones, and photos at similar ages</p>
      </div>

      {/* Person Selector */}
      <div className="person-selector">
        <h2>Select People to Compare</h2>
        <div className="person-checkboxes">
          {people.map(person => (
            <div
              key={person.id}
              className={`person-checkbox-item ${state.selectedPersonIds.has(person.id) ? "selected" : ""}`}
              onClick={() => togglePersonSelection(state, loadState, person.id)}
            >
              <input
                type="checkbox"
                checked={state.selectedPersonIds.has(person.id)}
                onChange={() => togglePersonSelection(state, loadState, person.id)}
              />
              <label>{person.name}</label>
            </div>
          ))}
        </div>
      </div>

      {/* Show filters only if people are selected */}
      {state.selectedPersonIds.size > 0 && (
        <div className="compare-filters">
          <div className="filter-row">
            {/* Content Type Filter */}
            <div className="filter-group">
              <label>Show:</label>
              <div className="filter-buttons">
                <button
                  className={`filter-btn ${state.visibleTypes.milestones ? "active" : ""}`}
                  onClick={() => toggleType(state, "milestones")}
                >
                  Milestones
                </button>
                <button
                  className={`filter-btn ${state.visibleTypes.measurements ? "active" : ""}`}
                  onClick={() => toggleType(state, "measurements")}
                >
                  Measurements
                </button>
                <button
                  className={`filter-btn ${state.visibleTypes.photos ? "active" : ""}`}
                  onClick={() => toggleType(state, "photos")}
                >
                  Photos
                </button>
              </div>
            </div>

            {/* Age Filter */}
            {sortedAgeYears.length > 0 && (
              <div className="filter-group">
                <label>Age:</label>
                <div className="filter-buttons">
                  <button
                    className={`filter-btn ${state.selectedAgeFilter === "all" ? "active" : ""}`}
                    onClick={() => setAgeFilter(state, "all")}
                  >
                    All Ages
                  </button>
                  {sortedAgeYears.map(year => (
                    <button
                      key={year}
                      className={`filter-btn ${state.selectedAgeFilter === year.toString() ? "active" : ""}`}
                      onClick={() => setAgeFilter(state, year.toString())}
                    >
                      {year === 0 ? "0-1 year" : `Age ${year}`}
                    </button>
                  ))}
                </div>
              </div>
            )}
          </div>
        </div>
      )}

      {/* Loading State */}
      {loadState.loading && (
        <div className="compare-empty-state">
          <h3>Loading comparison data...</h3>
        </div>
      )}

      {/* Error State */}
      {loadState.error && (
        <div className="compare-empty-state">
          <h3>Error</h3>
          <p>{loadState.error}</p>
        </div>
      )}

      {/* Empty State */}
      {!loadState.loading && !loadState.error && state.selectedPersonIds.size === 0 && (
        <div className="compare-empty-state">
          <h3>No people selected</h3>
          <p>Select 2 or more people above to start comparing</p>
        </div>
      )}

      {/* Comparison Grid */}
      {!loadState.loading &&
        !loadState.error &&
        loadState.data &&
        state.selectedPersonIds.size > 0 && (
          <div
            className={`comparison-grid ${
              state.selectedPersonIds.size === 2
                ? "two-column"
                : state.selectedPersonIds.size === 3
                  ? "three-column"
                  : state.selectedPersonIds.size === 4
                    ? "four-column"
                    : "five-column"
            }`}
          >
            {loadState.data.people.map((personData: server.PersonComparisonData) => (
              <PersonColumn
                key={personData.person.id}
                personData={personData}
                visibleTypes={state.visibleTypes}
                selectedAgeFilter={state.selectedAgeFilter}
                photoStatus={photoStatus}
              />
            ))}
          </div>
        )}
    </div>
  );
};

interface PersonColumnProps {
  personData: server.PersonComparisonData;
  visibleTypes: {
    milestones: boolean;
    measurements: boolean;
    photos: boolean;
  };
  selectedAgeFilter: string;
  photoStatus: ReturnType<typeof usePhotoStatus>;
}

const PersonColumn = ({
  personData,
  visibleTypes,
  selectedAgeFilter,
  photoStatus,
}: PersonColumnProps) => {
  const { person, milestones, growthData, photos } = personData;

  // Build unified timeline items
  type TimelineItem = {
    id: number;
    type: "milestone" | "measurement" | "photo";
    date: string;
    age: string;
    data: server.Milestone | server.GrowthData | server.Image;
  };

  const timelineItems: TimelineItem[] = [];

  if (visibleTypes.milestones && milestones) {
    milestones.forEach((milestone: server.Milestone) => {
      timelineItems.push({
        id: milestone.id,
        type: "milestone",
        date: milestone.milestoneDate,
        age: calculateAge(person.birthday, milestone.milestoneDate),
        data: milestone,
      });
    });
  }

  if (visibleTypes.measurements && growthData) {
    growthData.forEach((measurement: server.GrowthData) => {
      timelineItems.push({
        id: measurement.id,
        type: "measurement",
        date: measurement.measurementDate,
        age: calculateAge(person.birthday, measurement.measurementDate),
        data: measurement,
      });
    });
  }

  if (visibleTypes.photos && photos) {
    photos.forEach((photo: server.Image) => {
      timelineItems.push({
        id: photo.id,
        type: "photo",
        date: photo.photoDate,
        age: calculateAge(person.birthday, photo.photoDate),
        data: photo,
      });
    });
  }

  // Filter by age if selected
  const filteredItems =
    selectedAgeFilter === "all"
      ? timelineItems
      : timelineItems.filter(item => {
          const ageInYears = getAgeInYears(item.age);
          return ageInYears.toString() === selectedAgeFilter;
        });

  // Sort by date (newest first)
  const sortedItems = [...filteredItems].sort(
    (a, b) => new Date(b.date).getTime() - new Date(a.date).getTime()
  );

  const getGenderIcon = (gender: number) => {
    switch (gender) {
      case 0:
        return "👨";
      case 1:
        return "👩";
      default:
        return "👤";
    }
  };

  return (
    <div className="person-column">
      <div className="person-column-header">
        <div className="person-column-avatar">{getGenderIcon(person.gender)}</div>
        <div className="person-column-info">
          <h3>{person.name}</h3>
          <p>
            Age {person.age} • {sortedItems.length} item{sortedItems.length !== 1 ? "s" : ""}
          </p>
        </div>
      </div>

      <div className="person-column-timeline">
        {sortedItems.length === 0 ? (
          <div className="no-data-message">
            <p>No data for selected filters</p>
          </div>
        ) : (
          <div className="timeline-items">
            {sortedItems.map(item => {
              switch (item.type) {
                case "milestone": {
                  const milestone = item.data as server.Milestone;
                  return (
                    <div key={`milestone-${item.id}`} className="timeline-item milestone-item">
                      <div className="timeline-item-icon">
                        {getCategoryIcon(milestone.category)}
                      </div>
                      <div className="timeline-item-content">
                        <div className="timeline-item-header">
                          <span className="timeline-item-type milestone-type">
                            {getCategoryLabel(milestone.category)}
                          </span>
                          {item.age && <span className="timeline-item-age">{item.age}</span>}
                          <span className="timeline-item-date">{formatDate(item.date)}</span>
                        </div>
                        <div className="timeline-item-description">{milestone.description}</div>
                      </div>
                    </div>
                  );
                }

                case "measurement": {
                  const measurement = item.data as server.GrowthData;
                  return (
                    <div key={`measurement-${item.id}`} className="timeline-item measurement-item">
                      <div className="timeline-item-icon">📏</div>
                      <div className="timeline-item-content">
                        <div className="timeline-item-header">
                          <span className="timeline-item-type measurement-type">
                            {getMeasurementTypeLabel(measurement.measurementType)}
                          </span>
                          {item.age && <span className="timeline-item-age">{item.age}</span>}
                          <span className="timeline-item-date">{formatDate(item.date)}</span>
                        </div>
                        <div className="timeline-item-description measurement-value">
                          {measurement.value} {measurement.unit}
                        </div>
                      </div>
                    </div>
                  );
                }

                case "photo": {
                  const photo = item.data as server.Image;
                  return (
                    <div key={`photo-${item.id}`} className="timeline-item photo-item">
                      <div className="timeline-item-icon">📸</div>
                      <div className="timeline-item-content">
                        <div className="timeline-item-header">
                          <span className="timeline-item-type photo-type">Photo</span>
                          {item.age && <span className="timeline-item-age">{item.age}</span>}
                          <span className="timeline-item-date">{formatDate(item.date)}</span>
                        </div>
                        <div className="photo-item-details">
                          <div
                            className="photo-thumbnail"
                            onClick={() => core.setRoute(`/view-photo/${photo.id}`)}
                          >
                            <ThumbnailImage
                              photoId={photo.id}
                              alt={photo.title}
                              className="timeline-photo-image"
                              loading="lazy"
                              fetchpriority="auto"
                              status={photoStatus.getStatus(photo.id)}
                            />
                          </div>
                          <div className="photo-info">
                            <div className="photo-title">{photo.title}</div>
                            {photo.description && (
                              <div className="photo-description">{photo.description}</div>
                            )}
                          </div>
                        </div>
                      </div>
                    </div>
                  );
                }

                default:
                  return null;
              }
            })}
          </div>
        )}
      </div>
    </div>
  );
};
