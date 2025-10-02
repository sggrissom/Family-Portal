import * as preact from "preact";
import * as core from "vlens/core";
import * as server from "../../../server";
import { calculateAge, formatDate } from "../../../lib/dateUtils";
import { ThumbnailImage } from "../../../components/ResponsiveImage";
import { usePhotoStatus, Status } from "../../../hooks/usePhotoStatus";

interface UnifiedTimelineProps {
  person: server.Person;
  milestones: server.Milestone[];
  growthData: server.GrowthData[];
  photos: server.Image[];
  visibleTypes: {
    milestones: boolean;
    measurements: boolean;
    photos: boolean;
  };
  selectedAgeFilter: string;
  sortOrder: "newest" | "oldest";
  onAgeFilterChange: (filter: string) => void;
}

// Unified timeline item type
type TimelineItemType = "milestone" | "measurement" | "photo";

interface TimelineItem {
  id: number;
  type: TimelineItemType;
  date: string;
  age: string;
  data: server.Milestone | server.GrowthData | server.Image;
}

// Helper function to extract numeric age in years from an age string
const getAgeInYears = (ageString: string): number => {
  if (!ageString || ageString === "Newborn") return 0;

  // Extract year number from strings like "2 years 3 months" or "1 year"
  const yearMatch = ageString.match(/(\d+)\s+years?/);
  if (yearMatch) {
    return parseInt(yearMatch[1]);
  }

  // If it's only months (e.g., "5 months"), return 0
  if (ageString.includes("month")) {
    return 0;
  }

  return 0;
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

const handleDeleteMilestone = async (id: number, description: string) => {
  const confirmed = confirm(`Are you sure you want to delete this milestone: "${description}"?`);

  if (confirmed) {
    try {
      let [resp, err] = await server.DeleteMilestone({ id });

      if (resp && resp.success) {
        window.location.reload();
      } else {
        alert(err || "Failed to delete milestone");
      }
    } catch (error) {
      alert("Network error. Please try again.");
    }
  }
};

const handleDeleteGrowthData = async (
  id: number,
  type: server.MeasurementType,
  value: number,
  unit: string
) => {
  const typeLabel = type === server.Height ? "Height" : "Weight";
  const confirmed = confirm(
    `Are you sure you want to delete this ${typeLabel.toLowerCase()} measurement of ${value} ${unit}?`
  );

  if (confirmed) {
    try {
      let [resp, err] = await server.DeleteGrowthData({ id });

      if (resp && resp.success) {
        window.location.reload();
      } else {
        alert(err || "Failed to delete growth measurement");
      }
    } catch (error) {
      alert("Network error. Please try again.");
    }
  }
};

export const UnifiedTimeline = ({
  person,
  milestones,
  growthData,
  photos,
  visibleTypes,
  selectedAgeFilter,
  sortOrder,
  onAgeFilterChange,
}: UnifiedTimelineProps) => {
  const photoStatus = usePhotoStatus();

  // Initialize monitoring for processing photos
  if (photos && photos.length > 0) {
    photos.forEach(photo => {
      const currentStatus = photoStatus.getStatus(photo.id);
      if (
        currentStatus === Status.Unknown &&
        photo.status === 1 &&
        !photoStatus.isMonitoring(photo.id)
      ) {
        photoStatus.startMonitoring(photo.id, photo.status);
      }
    });
  }

  // Combine all data types into unified timeline items
  const timelineItems: TimelineItem[] = [];

  // Add milestones
  if (visibleTypes.milestones && milestones) {
    milestones.forEach(milestone => {
      timelineItems.push({
        id: milestone.id,
        type: "milestone",
        date: milestone.milestoneDate,
        age: calculateAge(person.birthday, milestone.milestoneDate),
        data: milestone,
      });
    });
  }

  // Add growth measurements
  if (visibleTypes.measurements && growthData) {
    growthData.forEach(measurement => {
      timelineItems.push({
        id: measurement.id,
        type: "measurement",
        date: measurement.measurementDate,
        age: calculateAge(person.birthday, measurement.measurementDate),
        data: measurement,
      });
    });
  }

  // Add photos
  if (visibleTypes.photos && photos) {
    photos.forEach(photo => {
      timelineItems.push({
        id: photo.id,
        type: "photo",
        date: photo.photoDate,
        age: calculateAge(person.birthday, photo.photoDate),
        data: photo,
      });
    });
  }

  // Sort timeline items by date
  const sortedItems = [...timelineItems].sort((a, b) => {
    const dateA = new Date(a.date).getTime();
    const dateB = new Date(b.date).getTime();
    return sortOrder === "newest" ? dateB - dateA : dateA - dateB;
  });

  // Filter by age if selected
  const filteredItems =
    selectedAgeFilter === "all"
      ? sortedItems
      : sortedItems.filter(item => {
          const ageInYears = getAgeInYears(item.age);
          return ageInYears.toString() === selectedAgeFilter;
        });

  // Extract unique age years for filter options
  const ageYears = new Set<number>();
  sortedItems.forEach(item => {
    const ageInYears = getAgeInYears(item.age);
    ageYears.add(ageInYears);
  });
  const sortedAgeYears = Array.from(ageYears).sort((a, b) => a - b);

  // Check if there's any data at all
  const hasAnyData = sortedItems.length > 0;
  const hasFilteredData = filteredItems.length > 0;

  if (!hasAnyData) {
    return (
      <div className="unified-timeline">
        <div className="empty-state">
          <h3>No entries yet</h3>
          <p>Start building {person.name}'s story by adding milestones, measurements, or photos.</p>
          <div className="empty-state-actions">
            <a href={`/add-milestone/${person.id}`} className="btn btn-primary">
              📝 Add Milestone
            </a>
            <a href={`/add-growth/${person.id}`} className="btn btn-primary">
              📏 Add Measurement
            </a>
            <a href={`/add-photo/${person.id}`} className="btn btn-primary">
              📸 Add Photo
            </a>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="unified-timeline">
      {/* Age Filter */}
      {sortedAgeYears.length > 1 && (
        <div className="age-filter">
          <button
            className={`filter-btn ${selectedAgeFilter === "all" ? "active" : ""}`}
            onClick={() => onAgeFilterChange("all")}
          >
            All Ages
          </button>
          {sortedAgeYears.map(year => (
            <button
              key={year}
              className={`filter-btn ${selectedAgeFilter === year.toString() ? "active" : ""}`}
              onClick={() => onAgeFilterChange(year.toString())}
            >
              {year === 0 ? "0-1 year" : `Age ${year}`}
            </button>
          ))}
        </div>
      )}

      {/* Item count */}
      {selectedAgeFilter !== "all" && hasFilteredData && (
        <div className="filter-info">
          Showing {filteredItems.length} of {sortedItems.length} entries
        </div>
      )}

      {/* Timeline Items */}
      {hasFilteredData ? (
        <div className="timeline-items">
          {filteredItems.map(item => {
            switch (item.type) {
              case "milestone": {
                const milestone = item.data as server.Milestone;
                return (
                  <div key={`milestone-${item.id}`} className="timeline-item milestone-item">
                    <div className="timeline-item-icon">{getCategoryIcon(milestone.category)}</div>
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
                    <div className="timeline-item-actions">
                      <a
                        href={`/edit-milestone/${milestone.id}`}
                        className="btn-action btn-edit"
                        title="Edit"
                      >
                        ✏️
                      </a>
                      <button
                        className="btn-action btn-delete"
                        title="Delete"
                        onClick={() => handleDeleteMilestone(milestone.id, milestone.description)}
                      >
                        🗑️
                      </button>
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
                    <div className="timeline-item-actions">
                      <a
                        href={`/edit-growth/${measurement.id}`}
                        className="btn-action btn-edit"
                        title="Edit"
                      >
                        ✏️
                      </a>
                      <button
                        className="btn-action btn-delete"
                        title="Delete"
                        onClick={() =>
                          handleDeleteGrowthData(
                            measurement.id,
                            measurement.measurementType,
                            measurement.value,
                            measurement.unit
                          )
                        }
                      >
                        🗑️
                      </button>
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
                          {person.profilePhotoId === photo.id && (
                            <div className="profile-photo-badge">👤 Profile</div>
                          )}
                        </div>
                        <div className="photo-info">
                          <div className="photo-title">{photo.title}</div>
                          {photo.description && (
                            <div className="photo-description">{photo.description}</div>
                          )}
                        </div>
                      </div>
                    </div>
                    <div className="timeline-item-actions">
                      <a
                        href={`/view-photo/${photo.id}`}
                        className="btn-action btn-view"
                        title="View"
                      >
                        👁️
                      </a>
                    </div>
                  </div>
                );
              }

              default:
                return null;
            }
          })}
        </div>
      ) : (
        <div className="empty-state">
          <p>No entries found for the selected age range.</p>
          <button className="btn btn-secondary" onClick={() => onAgeFilterChange("all")}>
            Show All Ages
          </button>
        </div>
      )}
    </div>
  );
};
