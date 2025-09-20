import * as preact from "preact";
import * as core from "vlens/core";
import * as server from "../server";
import "./photos-styles";

interface PhotosTabProps {
  person: server.Person;
  photos: server.Image[];
}

const formatPhotoDate = (dateString: string) => {
  if (!dateString) return '';
  try {
    const date = new Date(dateString);
    return date.toLocaleDateString();
  } catch {
    return '';
  }
};

export const PhotosTab = ({ person, photos }: PhotosTabProps) => {
  const hasPhotos = photos && photos.length > 0;

  return (
    <div className="photos-tab">
      <div className="photos-header">
        <div>
          <h2>Photos of {person.name}</h2>
          {hasPhotos && (
            <div className="photos-count">
              {photos.length} photo{photos.length !== 1 ? 's' : ''}
            </div>
          )}
        </div>
        <div className="photos-actions">
          <a href={`/add-photo/${person.id}`} className="btn btn-primary">
            📸 Add Photo
          </a>
        </div>
      </div>

      <div className="photos-content">
        {hasPhotos ? (
          <div className={`photos-gallery has-photos`}>
            {photos.map((photo) => (
              <div key={photo.id} className="photo-card">
                <img
                  src={`/api/photo/${photo.id}/thumb`}
                  alt={photo.title}
                  className="photo-image"
                  loading="lazy"
                  onClick={() => core.setRoute(`/view-photo/${photo.id}`)}
                />
                <div className="photo-info">
                  <h3 className="photo-title">{photo.title}</h3>
                  <div className="photo-date">
                    {formatPhotoDate(photo.photoDate)}
                  </div>
                  {photo.description && (
                    <div className="photo-description">
                      {photo.description}
                    </div>
                  )}
                </div>
              </div>
            ))}
          </div>
        ) : (
          <div className="photos-gallery">
            <div className="empty-state">
              <p>No photos yet.</p>
              <a href={`/add-photo/${person.id}`} className="btn btn-primary">Add First Photo</a>
            </div>
          </div>
        )}
      </div>
    </div>
  );
};