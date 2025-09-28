import * as preact from "preact";
import * as rpc from "vlens/rpc";
import * as auth from "../../lib/authCache";
import * as core from "vlens/core";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { ensureAuthInFetch, requireAuthInView } from "../../lib/authHelpers";
import { ThumbnailImage } from "../../components/ResponsiveImage";
import { usePhotoStatus, Status } from "../../hooks/usePhotoStatus";
import "./family-photos-styles";

export async function fetch(route: string, prefix: string) {
  if (!(await ensureAuthInFetch())) {
    return rpc.ok<server.ListFamilyPhotosResponse>({ photos: [] });
  }

  return server.ListFamilyPhotos({});
}

export function view(
  route: string,
  prefix: string,
  data: server.ListFamilyPhotosResponse
): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth) {
    return;
  }

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="family-photos-container">
        <FamilyPhotosPage user={currentAuth} data={data} />
      </main>
      <Footer />
    </div>
  );
}

interface FamilyPhotosPageProps {
  user: auth.AuthCache;
  data: server.ListFamilyPhotosResponse;
}

const formatPhotoDate = (dateString: string) => {
  if (!dateString) return "";
  try {
    const date = new Date(dateString);
    return date.toLocaleDateString();
  } catch {
    return "";
  }
};

const FamilyPhotosPage = ({ user, data }: FamilyPhotosPageProps) => {
  const photos = data.photos || [];
  const hasPhotos = photos.length > 0;
  const photoStatus = usePhotoStatus();

  // Initialize monitoring for processing photos
  if (hasPhotos) {
    photos.forEach(photoWithPeople => {
      const photo = photoWithPeople.image;
      const currentStatus = photoStatus.getStatus(photo.id);

      // Only start monitoring if we haven't seen this photo before (Unknown status)
      // AND the server says it's processing
      if (
        currentStatus === Status.Unknown &&
        photo.status === 1 &&
        !photoStatus.isMonitoring(photo.id)
      ) {
        photoStatus.startMonitoring(photo.id, photo.status);
      }
    });
  }

  return (
    <div className="family-photos-page">
      {/* Page Header */}
      <div className="page-header">
        <div className="header-content">
          <div>
            <h1>Family Photos</h1>
            {hasPhotos && (
              <div className="photos-count">
                {photos.length} photo{photos.length !== 1 ? "s" : ""}
              </div>
            )}
          </div>
          <div className="header-actions">
            <a href="/add-photo" className="btn btn-primary">
              📸 Add Photo
            </a>
          </div>
        </div>
      </div>

      {/* Photos Grid */}
      <div className="photos-content">
        {hasPhotos ? (
          <div className="photos-gallery has-photos">
            {photos.map((photoWithPeople, index) => (
              <div key={photoWithPeople.image.id} className="photo-card">
                <div className="photo-image-container">
                  <ThumbnailImage
                    photoId={photoWithPeople.image.id}
                    alt={photoWithPeople.image.title}
                    className="photo-image"
                    loading={index < 6 ? "eager" : "lazy"}
                    fetchpriority={index < 3 ? "high" : "auto"}
                    onClick={() => core.setRoute(`/view-photo/${photoWithPeople.image.id}`)}
                    status={photoStatus.getStatus(photoWithPeople.image.id)}
                  />
                  {/* Show profile photo badge if any person has this as their profile photo */}
                  {photoWithPeople.people.some(
                    person => person.profilePhotoId === photoWithPeople.image.id
                  ) && <div className="profile-photo-badge">👤 Profile</div>}
                  {/* Show person badges for all tagged people */}
                  {photoWithPeople.people.length > 0 ? (
                    <div className="people-badges">
                      {photoWithPeople.people.map(person => (
                        <div key={person.id} className="person-badge">
                          {person.name}
                        </div>
                      ))}
                    </div>
                  ) : (
                    <div className="people-badges">
                      <div className="person-badge family-badge">Family Photo</div>
                    </div>
                  )}
                </div>
                <div className="photo-info">
                  <h3 className="photo-title">{photoWithPeople.image.title}</h3>
                  <div className="photo-date">
                    {formatPhotoDate(photoWithPeople.image.photoDate)}
                  </div>
                  {photoWithPeople.image.description && (
                    <div className="photo-description">{photoWithPeople.image.description}</div>
                  )}
                </div>
              </div>
            ))}
          </div>
        ) : (
          <div className="photos-gallery">
            <div className="empty-state">
              <div className="empty-icon">📸</div>
              <h2>No Family Photos Yet</h2>
              <p>Start capturing your family memories by adding your first photo.</p>
              <a href="/add-photo" className="btn btn-primary">
                Add First Photo
              </a>
            </div>
          </div>
        )}
      </div>
    </div>
  );
};
