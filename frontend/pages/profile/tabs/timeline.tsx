import * as preact from "preact";
import * as vlens from "vlens";
import * as core from "vlens/core";
import * as server from "../../../server";
import { calculateAge, formatDate } from "../../../lib/dateUtils";
import { getCategoryIcon, getCategoryLabel } from "../../../lib/milestoneHelpers";
import { getAgeInYears, handleDeleteMilestone } from "../../../lib/timelineHelpers";
import { ThumbnailImage } from "../../../components/ResponsiveImage";
import "./timeline-styles";

interface TimelineTabProps {
  person: server.Person;
  milestones: server.Milestone[];
}

type TimelineState = {
  selectedAgeFilter: string; // "all" or year number as string like "0", "1", "2"
};

const useTimelineState = vlens.declareHook(
  (): TimelineState => ({
    selectedAgeFilter: "all",
  })
);

const setAgeFilter = (state: TimelineState, filter: string) => {
  state.selectedAgeFilter = filter;
  vlens.scheduleRedraw();
};

export const TimelineTab = ({ person, milestones }: TimelineTabProps) => {
  const state = useTimelineState();

  // Sort milestones by date (newest first)
  const milestonesArray = milestones || [];
  const sortedMilestones = [...milestonesArray].sort(
    (a, b) => new Date(b.milestoneDate).getTime() - new Date(a.milestoneDate).getTime()
  );

  if (!milestonesArray || milestonesArray.length === 0) {
    return (
      <div className="timeline-tab">
        <h2>Timeline for {person.name}</h2>
        <div className="timeline-content">
          <div className="empty-state">
            <p>No timeline entries yet.</p>
            <a href={`/add-milestone/${person.id}`} className="btn btn-primary">
              Add First Milestone
            </a>
          </div>
        </div>
      </div>
    );
  }

  // Extract unique age years from milestones for filter options
  const ageYears = new Set<number>();
  sortedMilestones.forEach(milestone => {
    const age = calculateAge(person.birthday, milestone.milestoneDate);
    const ageInYears = getAgeInYears(age);
    ageYears.add(ageInYears);
  });
  const sortedAgeYears = Array.from(ageYears).sort((a, b) => a - b);

  // Filter milestones based on selected age
  const filteredMilestones =
    state.selectedAgeFilter === "all"
      ? sortedMilestones
      : sortedMilestones.filter(milestone => {
          const age = calculateAge(person.birthday, milestone.milestoneDate);
          const ageInYears = getAgeInYears(age);
          return ageInYears.toString() === state.selectedAgeFilter;
        });

  return (
    <div className="timeline-tab">
      <h2>Timeline for {person.name}</h2>
      <div className="timeline-content">
        {/* Age Filter */}
        <div className="age-filter">
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

        {/* Milestone count */}
        {state.selectedAgeFilter !== "all" && (
          <div className="filter-info">
            Showing {filteredMilestones.length} of {sortedMilestones.length} milestones
          </div>
        )}

        <div className="milestone-list">
          {filteredMilestones.map(milestone => {
            const age = calculateAge(person.birthday, milestone.milestoneDate);
            return (
              <div key={milestone.id} className="milestone-item">
                <div className="milestone-icon">{getCategoryIcon(milestone.category)}</div>
                <div className="milestone-content">
                  <div className="milestone-header">
                    <span className="milestone-category">
                      {getCategoryLabel(milestone.category)}
                    </span>
                    {age && <span className="milestone-age">{age}</span>}
                    <span className="milestone-date">{formatDate(milestone.milestoneDate)}</span>
                  </div>
                  <div className="milestone-description">{milestone.description}</div>
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
                <div className="milestone-actions">
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
          })}
        </div>
        <div className="timeline-actions">
          <a href={`/add-milestone/${person.id}`} className="btn btn-primary">
            Add Another Milestone
          </a>
        </div>
      </div>
    </div>
  );
};
