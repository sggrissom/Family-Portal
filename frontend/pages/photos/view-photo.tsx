import * as preact from "preact";
import * as vlens from "vlens";
import * as core from "vlens/core";
import * as auth from "../../lib/authCache";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { FullImage } from "../../components/ResponsiveImage";
import { usePhotoStatus } from "../../hooks/usePhotoStatus";
import "./view-photo-styles";

export async function fetch(route: string, prefix: string) {
  const photoId = parseInt(route.split('/')[2]);
  return server.GetPhoto({ id: photoId });
}

type ViewPhotoData = { image: server.Image | null; person: server.Person | null };

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

  if (!data.image || !data.person) {
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
        <ViewPhotoPage photo={data.image} person={data.person} />
      </main>
      <Footer />
    </div>
  );
}

interface ViewPhotoPageProps {
  photo: server.Image;
  person: server.Person;
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

async function handleSetProfilePhoto(photo: server.Image) {
  try {
    const [resp, err] = await server.SetProfilePhoto({
      personId: photo.personId,
      photoId: photo.id
    });

    if (err) {
      alert(err || "Failed to set profile photo");
      return;
    }

    if (resp && resp.person) {
      alert("Profile photo set successfully");
      // Refresh the page to show updated data
      core.setRoute(`/view-photo/${photo.id}`);
    } else {
      alert("Failed to set profile photo");
    }
  } catch (error) {
    alert("Failed to set profile photo");
  }
}

const ViewPhotoPage = ({ photo, person }: ViewPhotoPageProps) => {
  const photoStatus = usePhotoStatus();
  const isProfilePhoto = person.profilePhotoId === photo.id;
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
        <FullImage
          photoId={photo.id}
          alt={photo.title}
          className="photo-main-image"
          status={photoStatus.getStatus(photo.id)}
        />
      </div>

      {/* Photo information */}
      <div className="photo-info-panel">
        <div className="photo-metadata">
          <h1 className="view-photo-title">{photo.title}</h1>
          <div className="view-photo-date">
            📅 {formatPhotoDate(photo.photoDate)}
          </div>
          {photo.description && (
            <div className="view-photo-description">
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
          {isProfilePhoto ? (
            <button className="btn btn-success" disabled>
              ✓ Profile Photo
            </button>
          ) : (
            <button
              className="btn btn-primary"
              onClick={() => handleSetProfilePhoto(photo)}
            >
              👤 Set as Profile Photo
            </button>
          )}
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
