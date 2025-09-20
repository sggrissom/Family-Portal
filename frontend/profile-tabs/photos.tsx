import * as preact from "preact";
import * as server from "../server";

interface PhotosTabProps {
  person: server.Person;
}

export const PhotosTab = ({ person }: PhotosTabProps) => {
  return (
    <div className="photos-tab">
      <h2>Photos of {person.name}</h2>
      <div className="photos-content">
        <div className="photos-gallery">
          <div className="empty-state">
            <p>No photos yet.</p>
            <a href={`/add-photo/${person.id}`} className="btn btn-primary">Add First Photo</a>
          </div>
        </div>
      </div>
    </div>
  );
};