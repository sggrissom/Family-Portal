import * as preact from "preact";
import * as server from "../server";
import "../timeline-styles";

interface TimelineTabProps {
  person: server.Person;
  milestones: server.Milestone[];
}

const formatDate = (dateString: string) => {
  if (!dateString) return '';
  if (dateString.includes('T') && dateString.endsWith('Z')) {
    const dateParts = dateString.split('T')[0].split('-');
    const year = parseInt(dateParts[0]);
    const month = parseInt(dateParts[1]) - 1; // Month is 0-indexed
    const day = parseInt(dateParts[2]);
    return new Date(year, month, day).toLocaleDateString();
  }
  return new Date(dateString).toLocaleDateString();
};

const getCategoryIcon = (category: string) => {
  switch (category) {
    case "development": return "🌱";
    case "behavior": return "😊";
    case "health": return "🏥";
    case "achievement": return "🏆";
    case "first": return "⭐";
    case "other": return "📝";
    default: return "📝";
  }
};

const getCategoryLabel = (category: string) => {
  switch (category) {
    case "development": return "Development";
    case "behavior": return "Behavior";
    case "health": return "Health";
    case "achievement": return "Achievement";
    case "first": return "First Time";
    case "other": return "Other";
    default: return "Other";
  }
};

export const TimelineTab = ({ person, milestones }: TimelineTabProps) => {
  // Sort milestones by date (newest first)
  const milestonesArray = milestones || [];
  const sortedMilestones = [...milestonesArray].sort((a, b) =>
    new Date(b.milestoneDate).getTime() - new Date(a.milestoneDate).getTime()
  );

  if (!milestonesArray || milestonesArray.length === 0) {
    return (
      <div className="timeline-tab">
        <h2>Timeline for {person.name}</h2>
        <div className="timeline-content">
          <div className="empty-state">
            <p>No timeline entries yet.</p>
            <a href={`/add-milestone/${person.id}`} className="btn btn-primary">Add First Milestone</a>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="timeline-tab">
      <h2>Timeline for {person.name}</h2>
      <div className="timeline-content">
        <div className="milestone-list">
          {sortedMilestones.map(milestone => (
            <div key={milestone.id} className="milestone-item">
              <div className="milestone-icon">
                {getCategoryIcon(milestone.category)}
              </div>
              <div className="milestone-content">
                <div className="milestone-header">
                  <span className="milestone-category">{getCategoryLabel(milestone.category)}</span>
                  <span className="milestone-date">{formatDate(milestone.milestoneDate)}</span>
                </div>
                <div className="milestone-description">
                  {milestone.description}
                </div>
              </div>
            </div>
          ))}
        </div>
        <div className="timeline-actions">
          <a href={`/add-milestone/${person.id}`} className="btn btn-primary">Add Another Milestone</a>
        </div>
      </div>
    </div>
  );
};