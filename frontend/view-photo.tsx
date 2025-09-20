import * as preact from "preact";
import * as vlens from "vlens";
import * as core from "vlens/core";
import * as auth from "./authCache";
import * as server from "./server";
import { Header, Footer } from "./layout";
import "./view-photo-styles";

export async function fetch(route: string, prefix: string) {
  const photoId = parseInt(route.split('/')[2]);
  return server.GetPhoto({ id: photoId });
}

type ViewPhotoData = server.GetPhotoResponse | { image: null };

const formatPhotoDate = (dateString: string) => {
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

export function view(
  route: string,
  prefix: string,
  data: ViewPhotoData,
): preact.ComponentChild {
  const currentAuth = auth.getAuth();
  if (!currentAuth || currentAuth.id <= 0) {
    auth.clearAuth();
    core.setRoute('/login');
    return;
  }

  if (!data.image) {
    return (
      <div>
        <Header isHome={false} />
        <main id="app" className="view-photo-container">
          <div className="error-page">
            <h1>Error</h1>
            <p>Photo not found or access denied</p>
            <a href="/dashboard" className="btn btn-primary">Back to Dashboard</a>
          </div>
        </main>
        <Footer />
      </div>
    );
  }

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="view-photo-container">
        <ViewPhotoPage photo={data.image} />
      </main>
      <Footer />
    </div>
  );
}

interface ViewPhotoPageProps {
  photo: server.Image;
}

async function handleDeletePhoto(photo: server.Image) {
  const confirmed = confirm(`Are you sure you want to delete "${photo.title}"? This action cannot be undone.`);
  if (!confirmed) return;

  try {
    const [resp, err] = await server.DeletePhoto({ id: photo.id });
    if (err) {
      alert(err || "Failed to delete photo");
      return;
    }

    if (resp && resp.success) {
      alert("Photo deleted successfully");
      // Navigate back to the person's profile
      core.setRoute(`/profile/${photo.personId}`);
    } else {
      alert("Failed to delete photo");
    }
  } catch (error) {
    alert("Failed to delete photo");
  }
}

const ViewPhotoPage = ({ photo }: ViewPhotoPageProps) => {
  return (
    <div className="view-photo-page">
      {/* Header with navigation */}
      <div className="photo-header">
        <a href={`/profile/${photo.personId}`} className="back-link">
          ← Back to Profile
        </a>
      </div>

      {/* Main photo display */}
      <div className="photo-display">
        <img
          src={`/api/photo/${photo.id}`}
          alt={photo.title}
          className="photo-main-image"
        />
      </div>

      {/* Photo information */}
      <div className="photo-info-panel">
        <div className="photo-metadata">
          <h1 className="photo-title">{photo.title}</h1>
          <div className="photo-date">
            📅 {formatPhotoDate(photo.photoDate)}
          </div>
          {photo.description && (
            <div className="photo-description">
              {photo.description}
            </div>
          )}
          <div className="photo-details">
            <small>
              Uploaded: {formatPhotoDate(photo.createdAt)} • {photo.originalFilename}
            </small>
          </div>
        </div>

        {/* Action buttons */}
        <div className="photo-actions">
          <a href={`/edit-photo/${photo.id}`} className="btn btn-secondary">
            ✏️ Edit
          </a>
          <button
            className="btn btn-danger"
            onClick={() => handleDeletePhoto(photo)}
          >
            🗑️ Delete
          </button>
        </div>
      </div>
    </div>
  );
};