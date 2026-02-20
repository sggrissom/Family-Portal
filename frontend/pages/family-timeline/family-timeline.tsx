import * as preact from "preact";
import * as rpc from "vlens/rpc";
import * as auth from "../../lib/authCache";
import * as core from "vlens/core";
import * as vlens from "vlens";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { ensureAuthInFetch, requireAuthInView } from "../../lib/authHelpers";
import { calculateAge, formatDate } from "../../lib/dateUtils";
import {
  getCategoryIcon,
  getCategoryLabel,
  getMeasurementTypeLabel,
} from "../../lib/milestoneHelpers";
import { ThumbnailImage } from "../../components/ResponsiveImage";
import { usePhotoStatus, Status } from "../../hooks/usePhotoStatus";
import "./family-timeline-styles";

export async function fetch(route: string, prefix: string) {
  if (!(await ensureAuthInFetch())) {
    return rpc.ok<server.GetFamilyTimelineResponse>({ people: [] });
  }

  return server.GetFamilyTimeline({});
}

export function view(
  route: string,
  prefix: string,
  data: server.GetFamilyTimelineResponse
): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth) {
    return;
  }

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="family-timeline-container">
        <FamilyTimelinePage data={data} />
      </main>
      <Footer />
    </div>
  );
}

interface FamilyTimelinePageProps {
  data: server.GetFamilyTimelineResponse;
}

type TimelineItemType = "milestone" | "measurement" | "photo";

interface TimelineItem {
  id: number;
  type: TimelineItemType;
  date: string;
  personId: number;
  personName: string;
  age: string;
  data: server.Milestone | server.GrowthData | server.Image;
}

type FamilyTimelineState = {
  selectedPerson: string;
  selectedType: "all" | "milestones" | "measurements" | "photos";
  sortOrder: "newest" | "oldest";
  searchQuery: string;
  searchResults: server.Milestone[] | null;
  isSearching: boolean;
};

const useFamilyTimelineState = vlens.declareHook(
  (): FamilyTimelineState => ({
    selectedPerson: "all",
    selectedType: "all",
    sortOrder: "newest",
    searchQuery: "",
    searchResults: null,
    isSearching: false,
  })
);

const FamilyTimelinePage = ({ data }: FamilyTimelinePageProps) => {
  const photoStatus = usePhotoStatus();
  const state = useFamilyTimelineState();

  const people = data.people || [];

  // Search handler
  const handleSearch = async () => {
    const query = state.searchQuery.trim();
    if (!query) {
      return;
    }

    state.isSearching = true;
    vlens.scheduleRedraw();

    try {
      const [result, error] = await server.SearchMilestones({ query, limit: 50 });
      if (result && !error) {
        state.searchResults = result.milestones;
      } else {
        console.error("Search failed:", error);
        state.searchResults = [];
      }
    } catch (error) {
      console.error("Search error:", error);
      state.searchResults = [];
    } finally {
      state.isSearching = false;
      vlens.scheduleRedraw();
    }
  };

  const clearSearch = () => {
    state.searchQuery = "";
    state.searchResults = null;
    vlens.scheduleRedraw();
  };

  // Build combined timeline from all people
  const allTimelineItems: TimelineItem[] = [];

  for (const personData of people) {
    const person = personData.person;

    // Add milestones
    if (personData.milestones) {
      personData.milestones.forEach((milestone: server.Milestone) => {
        allTimelineItems.push({
          id: milestone.id,
          type: "milestone",
          date: milestone.milestoneDate,
          personId: person.id,
          personName: person.name,
          age: calculateAge(person.birthday, milestone.milestoneDate),
          data: milestone,
        });
      });
    }

    // Add growth measurements
    if (personData.growthData) {
      personData.growthData.forEach((measurement: server.GrowthData) => {
        allTimelineItems.push({
          id: measurement.id,
          type: "measurement",
          date: measurement.measurementDate,
          personId: person.id,
          personName: person.name,
          age: calculateAge(person.birthday, measurement.measurementDate),
          data: measurement,
        });
      });
    }

    // Add photos
    if (personData.photos) {
      personData.photos.forEach((photo: server.Image) => {
        allTimelineItems.push({
          id: photo.id,
          type: "photo",
          date: photo.photoDate,
          personId: person.id,
          personName: person.name,
          age: calculateAge(person.birthday, photo.photoDate),
          data: photo,
        });
      });
    }
  }

  // Initialize monitoring for processing photos
  allTimelineItems.forEach(item => {
    if (item.type === "photo") {
      const photo = item.data as server.Image;
      const currentStatus = photoStatus.getStatus(photo.id);
      if (
        currentStatus === Status.Unknown &&
        photo.status === 1 &&
        !photoStatus.isMonitoring(photo.id)
      ) {
        photoStatus.startMonitoring(photo.id, photo.status);
      }
    }
  });

  // Filter by person
  let filteredItems = allTimelineItems;
  if (state.selectedPerson !== "all") {
    const personId = parseInt(state.selectedPerson);
    filteredItems = filteredItems.filter(item => item.personId === personId);
  }

  // Filter by type
  if (state.selectedType !== "all") {
    filteredItems = filteredItems.filter(item => {
      if (state.selectedType === "milestones") return item.type === "milestone";
      if (state.selectedType === "measurements") return item.type === "measurement";
      if (state.selectedType === "photos") return item.type === "photo";
      return true;
    });
  }

  // Sort items by date
  const sortedItems = [...filteredItems].sort((a, b) => {
    const dateA = new Date(a.date).getTime();
    const dateB = new Date(b.date).getTime();
    return state.sortOrder === "newest" ? dateB - dateA : dateA - dateB;
  });

  const hasAnyData = allTimelineItems.length > 0;
  const hasFilteredData = sortedItems.length > 0;

  return (
    <div className="family-timeline-page">
      <div className="timeline-header">
        <h1>Family Timeline</h1>
        <p>All memories, milestones, and moments in one place</p>
      </div>

      {hasAnyData ? (
        <>
          <div className="search-container">
            <input
              type="text"
              className="search-input"
              placeholder="Search milestones..."
              value={state.searchQuery}
              onInput={e => {
                state.searchQuery = e.currentTarget.value;
                vlens.scheduleRedraw();
              }}
              onKeyDown={e => {
                if (e.key === "Enter") {
                  handleSearch();
                }
              }}
            />
            <button
              className="btn btn-primary search-btn"
              onClick={handleSearch}
              disabled={state.isSearching || !state.searchQuery.trim()}
            >
              {state.isSearching ? "Searching..." : "Search"}
            </button>
            {state.searchResults !== null && (
              <button className="btn btn-secondary" onClick={clearSearch}>
                Clear Search
              </button>
            )}
          </div>

          <div className="timeline-filters">
            <div className="filter-group">
              <label>Person:</label>
              <select
                value={state.selectedPerson}
                onChange={e => {
                  state.selectedPerson = e.currentTarget.value;
                  vlens.scheduleRedraw();
                }}
              >
                <option value="all">All Family Members</option>
                {people.map((p: server.FamilyTimelineItem) => (
                  <option key={p.person.id} value={p.person.id.toString()}>
                    {p.person.name}
                  </option>
                ))}
              </select>
            </div>

            <div className="filter-group">
              <label>Type:</label>
              <select
                value={state.selectedType}
                onChange={e => {
                  state.selectedType = e.currentTarget.value as any;
                  vlens.scheduleRedraw();
                }}
              >
                <option value="all">All Types</option>
                <option value="milestones">Milestones</option>
                <option value="measurements">Measurements</option>
                <option value="photos">Photos</option>
              </select>
            </div>

            <div className="filter-group">
              <label>Sort:</label>
              <select
                value={state.sortOrder}
                onChange={e => {
                  state.sortOrder = e.currentTarget.value as any;
                  vlens.scheduleRedraw();
                }}
              >
                <option value="newest">Newest First</option>
                <option value="oldest">Oldest First</option>
              </select>
            </div>
          </div>

          {state.searchResults !== null ? (
            <>
              <div className="timeline-stats">
                Found {state.searchResults.length} milestone(s) for "{state.searchQuery}"
              </div>
              {state.searchResults.length > 0 ? (
                <div className="search-results">
                  {state.searchResults.map(milestone => {
                    const person = people.find(p => p.person.id === milestone.personId);
                    const personName = person?.person.name || "Unknown";
                    const age = person
                      ? calculateAge(person.person.birthday, milestone.milestoneDate)
                      : "";

                    return (
                      <div key={milestone.id} className="timeline-item milestone-item">
                        <div className="timeline-item-header">
                          <div className="timeline-item-icon">
                            {getCategoryIcon(milestone.category)}
                          </div>
                          <div className="timeline-item-meta">
                            <div className="timeline-item-person">{personName}</div>
                            <div className="timeline-item-age">{age}</div>
                          </div>
                          <div className="timeline-item-date">
                            {formatDate(milestone.milestoneDate)}
                          </div>
                        </div>
                        <div className="timeline-item-content">
                          <div className="milestone-category">
                            {getCategoryLabel(milestone.category)}
                          </div>
                          <div className="milestone-description">{milestone.description}</div>
                        </div>
                      </div>
                    );
                  })}
                </div>
              ) : (
                <div className="empty-state">
                  <p>No milestones found for "{state.searchQuery}"</p>
                </div>
              )}
            </>
          ) : (
            <>
              {hasFilteredData ? (
                <div className="timeline-stats">
                  Showing {sortedItems.length} of {allTimelineItems.length} entries
                </div>
              ) : null}

              {hasFilteredData ? (
                <div className="timeline-items">
                  {sortedItems.map(item => (
                    <TimelineItemComponent
                      key={`${item.type}-${item.id}`}
                      item={item}
                      photoStatus={photoStatus}
                    />
                  ))}
                </div>
              ) : (
                <div className="empty-state">
                  <p>No entries match your filters.</p>
                  <button
                    className="btn btn-secondary"
                    onClick={() => {
                      state.selectedPerson = "all";
                      state.selectedType = "all";
                      vlens.scheduleRedraw();
                    }}
                  >
                    Clear Filters
                  </button>
                </div>
              )}
            </>
          )}
        </>
      ) : (
        <div className="empty-state">
          <h3>No family timeline entries yet</h3>
          <p>Start building your family's story by adding milestones, measurements, or photos.</p>
          <div className="empty-state-actions">
            <a href="/add-person" className="btn btn-primary">
              Add Family Member
            </a>
            <a href="/add-milestone" className="btn btn-primary">
              📝 Add Milestone
            </a>
            <a href="/add-photo" className="btn btn-primary">
              📸 Add Photo
            </a>
          </div>
        </div>
      )}
    </div>
  );
};

interface TimelineItemComponentProps {
  item: TimelineItem;
  photoStatus: ReturnType<typeof usePhotoStatus>;
}

const TimelineItemComponent = ({ item, photoStatus }: TimelineItemComponentProps) => {
  switch (item.type) {
    case "milestone": {
      const milestone = item.data as server.Milestone;
      return (
        <div className="timeline-item milestone-item">
          <div className="timeline-item-icon">{getCategoryIcon(milestone.category)}</div>
          <div className="timeline-item-content">
            <div className="timeline-item-header">
              <span className="timeline-item-person">{item.personName}</span>
              <span className="timeline-item-type milestone-type">
                {getCategoryLabel(milestone.category)}
              </span>
              {item.age && <span className="timeline-item-age">{item.age}</span>}
              <span className="timeline-item-date">{formatDate(item.date)}</span>
            </div>
            <div className="timeline-item-description">{milestone.description}</div>
            {milestone.photoIds && milestone.photoIds.length > 0 && (
              <div className="milestone-photos">
                {milestone.photoIds.map(photoId => (
                  <ThumbnailImage
                    key={photoId}
                    photoId={photoId}
                    alt=""
                    className="milestone-photo-thumb"
                    onClick={() => core.setRoute(`/view-photo/${photoId}`)}
                  />
                ))}
              </div>
            )}
          </div>
          <div className="timeline-item-actions">
            <a
              href={`/edit-milestone/${milestone.id}`}
              className="btn-action btn-edit"
              title="Edit"
            >
              ✏️
            </a>
          </div>
        </div>
      );
    }

    case "measurement": {
      const measurement = item.data as server.GrowthData;
      return (
        <div className="timeline-item measurement-item">
          <div className="timeline-item-icon">📏</div>
          <div className="timeline-item-content">
            <div className="timeline-item-header">
              <span className="timeline-item-person">{item.personName}</span>
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
          <div className="timeline-item-actions">
            <a href={`/edit-growth/${measurement.id}`} className="btn-action btn-edit" title="Edit">
              ✏️
            </a>
          </div>
        </div>
      );
    }

    case "photo": {
      const photo = item.data as server.Image;
      return (
        <div className="timeline-item photo-item">
          <div className="timeline-item-icon">📸</div>
          <div className="timeline-item-content">
            <div className="timeline-item-header">
              <span className="timeline-item-person">{item.personName}</span>
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
                {photo.description && <div className="photo-description">{photo.description}</div>}
              </div>
            </div>
          </div>
          <div className="timeline-item-actions">
            <a href={`/view-photo/${photo.id}`} className="btn-action btn-view" title="View">
              👁️
            </a>
          </div>
        </div>
      );
    }

    default:
      return null;
  }
};
