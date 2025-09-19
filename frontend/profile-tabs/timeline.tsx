import * as preact from "preact";
import * as server from "../server";

interface TimelineTabProps {
  person: server.Person;
}

export const TimelineTab = ({ person }: TimelineTabProps) => {
  return (
    <div className="timeline-tab">
      <h2>Timeline for {person.name}</h2>
      <div className="timeline-content">
        <div className="empty-state">
          <p>No timeline entries yet.</p>
          <button className="btn btn-primary">Add First Milestone</button>
        </div>
      </div>
    </div>
  );
};