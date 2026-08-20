import * as preact from "preact";
import * as server from "../../../server";
import { calculateAge, formatDate, isRealDate } from "../../../lib/dateUtils";
import { labelsForKind } from "../../activities/labels";
import {
  getCategoryIcon,
  getCategoryLabel,
  getMeasurementTypeLabel,
} from "../../../lib/milestoneHelpers";
import {
  getAgeInYears,
  handleDeleteMilestone,
  handleDeleteGrowthData,
} from "../../../lib/timelineHelpers";
import { ThumbnailImage } from "../../../components/ResponsiveImage";
import { usePhotoStatus, Status } from "../../../hooks/usePhotoStatus";
import { useTagCache } from "../../../hooks/useTagCache";
import {
  ageInMonths,
  computePercentileLabel,
  isValidBirthday,
} from "../../../lib/growthPercentiles";
import "./timeline-styles";

interface UnifiedTimelineProps {
  person: server.Person;
  milestones: server.Milestone[];
  growthData: server.GrowthData[];
  photos: server.Image[];
  performances: server.AppearanceDetail[];
  activitySeasons: server.SeasonSummary[];
  visibleTypes: {
    milestones: boolean;
    measurements: boolean;
    photos: boolean;
    performances: boolean;
  };
  selectedAgeFilter: string;
  sortOrder: "newest" | "oldest";
  onAgeFilterChange: (filter: string) => void;
  selectedTagIds: number[];
  onToggleTag: (tagId: number) => void;
}

// Unified timeline item type
type TimelineItemType = "milestone" | "measurement" | "photo" | "performance";

interface TimelineItem {
  id: number;
  type: TimelineItemType;
  date: string;
  age: string;
  data: server.Milestone | server.GrowthData | server.Image | server.AppearanceDetail;
}

export const UnifiedTimeline = ({
  person,
  milestones,
  growthData,
  photos,
  performances,
  activitySeasons,
  visibleTypes,
  selectedAgeFilter,
  sortOrder,
  onAgeFilterChange,
  selectedTagIds,
  onToggleTag,
}: UnifiedTimelineProps) => {
  const photoStatus = usePhotoStatus();
  const tagCache = useTagCache();
  tagCache.loadTags();

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

  // A performance's time is optional; the competition start date is the same
  // chronological fallback used by the activities views.
  if (visibleTypes.performances && performances) {
    performances.forEach(performance => {
      const date = isRealDate(performance.appearance.occurredAt)
        ? performance.appearance.occurredAt
        : performance.event.startDate;
      timelineItems.push({
        id: performance.appearance.id,
        type: "performance",
        date,
        age: calculateAge(person.birthday, date),
        data: performance,
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
  const ageFilteredItems =
    selectedAgeFilter === "all"
      ? sortedItems
      : sortedItems.filter(item => {
          const ageInYears = getAgeInYears(item.age);
          return ageInYears.toString() === selectedAgeFilter;
        });

  // Filter by tag if selected
  const filteredItems =
    selectedTagIds.length === 0
      ? ageFilteredItems
      : ageFilteredItems.filter(item => {
          if (item.type === "milestone") {
            const m = item.data as server.Milestone;
            return selectedTagIds.some(id => m.tagIds?.includes(id));
          }
          if (item.type === "photo") {
            const p = item.data as server.Image;
            return selectedTagIds.some(id => p.tagIds?.includes(id));
          }
          return false;
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
          <p>
            Start building {person.name}'s story by adding milestones, measurements, photos, or
            performances.
          </p>
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

      {/* Tag Filter */}
      {tagCache.tags.length > 0 && (
        <div className="age-filter">
          {tagCache.tags.map(tag => (
            <button
              key={tag.id}
              className={`filter-btn${selectedTagIds.includes(tag.id) ? " active" : ""}`}
              style={
                selectedTagIds.includes(tag.id) ? { borderColor: tag.color, color: tag.color } : {}
              }
              onClick={() => onToggleTag(tag.id)}
            >
              <span className="tag-color-dot" style={{ background: tag.color }} />
              {tag.name}
            </button>
          ))}
        </div>
      )}

      {/* Item count */}
      {(selectedAgeFilter !== "all" || selectedTagIds.length > 0) && hasFilteredData && (
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
                      {milestone.photoIds && milestone.photoIds.length > 0 && (
                        <div className="milestone-photos">
                          {milestone.photoIds.map(photoId => (
                            <a
                              key={photoId}
                              className="milestone-photo-link"
                              href={`/view-photo/${photoId}`}
                              aria-label="View photo"
                            >
                              <ThumbnailImage
                                photoId={photoId}
                                alt=""
                                className="milestone-photo-thumb"
                              />
                            </a>
                          ))}
                        </div>
                      )}
                      {milestone.tagIds && milestone.tagIds.length > 0 && (
                        <div className="milestone-tags">
                          {milestone.tagIds.map(tagId => {
                            const tag = tagCache.getTag(tagId);
                            if (!tag) return null;
                            return (
                              <span
                                key={tagId}
                                className="milestone-tag-badge"
                                style={{ borderColor: tag.color, color: tag.color }}
                              >
                                {tag.name}
                              </span>
                            );
                          })}
                        </div>
                      )}
                    </div>
                    <div className="timeline-item-actions">
                      <a
                        href={`/edit-milestone/${milestone.id}`}
                        className="btn-action btn-edit"
                        title="Edit"
                        aria-label="Edit milestone"
                      >
                        ✏️
                      </a>
                      <button
                        className="btn-action btn-delete"
                        title="Delete"
                        aria-label="Delete milestone"
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
                const pctLabel = isValidBirthday(person.birthday)
                  ? computePercentileLabel(
                      measurement.value,
                      measurement.unit,
                      ageInMonths(person.birthday, measurement.measurementDate),
                      person.gender,
                      measurement.measurementType === server.Height ? "height" : "weight"
                    )
                  : null;
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
                        {pctLabel && (
                          <span className="percentile-badge" style={{ marginLeft: "10px" }}>
                            {pctLabel}
                          </span>
                        )}
                      </div>
                    </div>
                    <div className="timeline-item-actions">
                      <a
                        href={`/view-growth/${measurement.id}`}
                        className="btn-action btn-view"
                        title="View"
                        aria-label="View measurement"
                      >
                        👁️
                      </a>
                      <a
                        href={`/edit-growth/${measurement.id}`}
                        className="btn-action btn-edit"
                        title="Edit"
                        aria-label="Edit measurement"
                      >
                        ✏️
                      </a>
                      <button
                        className="btn-action btn-delete"
                        title="Delete"
                        aria-label="Delete measurement"
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
                        <a
                          className="photo-thumbnail"
                          href={`/view-photo/${photo.id}`}
                          aria-label={`View photo: ${photo.title}`}
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
                        </a>
                        <div className="photo-info">
                          <div className="photo-title">{photo.title}</div>
                          {photo.description && (
                            <div className="photo-description">{photo.description}</div>
                          )}
                          {photo.tagIds && photo.tagIds.length > 0 && (
                            <div className="milestone-tags">
                              {photo.tagIds.map(tagId => {
                                const tag = tagCache.getTag(tagId);
                                if (!tag) return null;
                                return (
                                  <span
                                    key={tagId}
                                    className="milestone-tag-badge"
                                    style={{ borderColor: tag.color, color: tag.color }}
                                  >
                                    {tag.name}
                                  </span>
                                );
                              })}
                            </div>
                          )}
                        </div>
                      </div>
                    </div>
                    <div className="timeline-item-actions">
                      <a
                        href={`/view-photo/${photo.id}`}
                        className="btn-action btn-view"
                        title="View"
                        aria-label="View photo"
                      >
                        👁️
                      </a>
                    </div>
                  </div>
                );
              }

              case "performance": {
                const performance = item.data as server.AppearanceDetail;
                const season = activitySeasons.find(
                  candidate => candidate.id === performance.entry.seasonId
                );
                const labels = labelsForKind(season?.kind ?? "generic");
                const resultLabels = (performance.results ?? [])
                  .map(result => result.label)
                  .filter(label => label.trim());
                return (
                  <div key={`performance-${item.id}`} className="timeline-item performance-item">
                    <div className="timeline-item-icon">🏆</div>
                    <div className="timeline-item-content">
                      <div className="timeline-item-header">
                        <span className="timeline-item-type performance-type">
                          {labels.appearance}
                        </span>
                        {item.age && <span className="timeline-item-age">{item.age}</span>}
                        <span className="timeline-item-date">{formatDate(item.date)}</span>
                      </div>
                      <div className="timeline-item-description performance-title">
                        {performance.entry.name} at {performance.event.name}
                      </div>
                      {[performance.event.host, performance.event.location].filter(Boolean).length >
                        0 && (
                        <div className="performance-meta">
                          {[performance.event.host, performance.event.location]
                            .filter(Boolean)
                            .join(" · ")}
                        </div>
                      )}
                      {resultLabels.length > 0 && (
                        <div className="performance-results">{resultLabels.join(" · ")}</div>
                      )}
                      {performance.appearance.notes && (
                        <div className="performance-notes">{performance.appearance.notes}</div>
                      )}
                      {performance.photoIds?.length > 0 && (
                        <div className="milestone-photos">
                          {performance.photoIds.map(photoId => (
                            <a
                              key={photoId}
                              className="milestone-photo-link"
                              href={`/view-photo/${photoId}`}
                              aria-label="View photo"
                            >
                              <ThumbnailImage
                                photoId={photoId}
                                alt=""
                                className="milestone-photo-thumb"
                              />
                            </a>
                          ))}
                        </div>
                      )}
                    </div>
                    <div className="timeline-item-actions">
                      <a
                        href={`/routine/${performance.entry.id}`}
                        className="btn-action btn-view"
                        title={`View ${labels.entry.toLowerCase()}`}
                        aria-label={`View ${labels.entry.toLowerCase()}`}
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
          <p>No entries found for the selected filters.</p>
          <button className="btn btn-secondary" onClick={() => onAgeFilterChange("all")}>
            Show All Ages
          </button>
        </div>
      )}
    </div>
  );
};
