import * as preact from "preact";
import * as rpc from "vlens/rpc";
import * as auth from "../../lib/authCache";
import * as core from "vlens/core";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { ensureAuthInFetch, requireAuthInView } from "../../lib/authHelpers";
import { ThumbnailImage } from "../../components/ResponsiveImage";
import { usePhotoStatus, Status } from "../../hooks/usePhotoStatus";
import { usePhotoFilter } from "../../hooks/usePhotoFilter";
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
  const allPhotos = data.photos || [];
  const photoFilter = usePhotoFilter();
  const photoStatus = usePhotoStatus();

  const filteredPhotos = photoFilter.filterPhotos(allPhotos);
  const hasPhotos = allPhotos.length > 0;
  const hasFilteredPhotos = filteredPhotos.length > 0;

  // Initialize monitoring for processing photos
  if (hasPhotos) {
    allPhotos.forEach(photoWithPeople => {
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
                {photoFilter.hasActiveFilters()
                  ? `${filteredPhotos.length} of ${allPhotos.length} photos`
                  : `${allPhotos.length} photo${allPhotos.length !== 1 ? "s" : ""}`}
              </div>
            )}
          </div>
          <div className="header-actions">
            {hasPhotos && (
              <button
                className="btn btn-secondary filter-toggle"
                onClick={photoFilter.toggleFilterPanel}
              >
                🔍 Filter {photoFilter.hasActiveFilters() && `(${photoFilter.getFilterSummary()})`}
              </button>
            )}
            <a href="/add-photo" className="btn btn-primary">
              📸 Add Photo
            </a>
          </div>
        </div>
      </div>

      {/* Filter Panel */}
      {hasPhotos && photoFilter.isFilterPanelOpen && (
        <div className="filter-panel">
          <div className="filter-section">
            <h3>Filter by People</h3>
            <div className="people-filter">
              {photoFilter.peopleLoading ? (
                <div className="loading-state">Loading people...</div>
              ) : (
                photoFilter.people.map(person => (
                  <label key={person.id} className="person-checkbox">
                    <input
                      type="checkbox"
                      checked={photoFilter.selectedPeopleIds.includes(person.id)}
                      onChange={() => photoFilter.togglePerson(person.id)}
                    />
                    <span className="person-label">{person.name}</span>
                  </label>
                ))
              )}
            </div>
          </div>

          <div className="filter-section">
            <h3>Filter by Date</h3>
            <div className="date-filter">
              <div className="date-input-group">
                <label htmlFor="date-from">From:</label>
                <input
                  id="date-from"
                  type="date"
                  value={photoFilter.dateFrom}
                  onChange={e => photoFilter.setDateFrom(e.currentTarget.value)}
                />
              </div>
              <div className="date-input-group">
                <label htmlFor="date-to">To:</label>
                <input
                  id="date-to"
                  type="date"
                  value={photoFilter.dateTo}
                  onChange={e => photoFilter.setDateTo(e.currentTarget.value)}
                />
              </div>
            </div>
          </div>

          <div className="filter-actions">
            {photoFilter.hasActiveFilters() && (
              <button className="btn btn-secondary" onClick={photoFilter.clearAllFilters}>
                Clear All Filters
              </button>
            )}
            <button className="btn btn-secondary" onClick={photoFilter.toggleFilterPanel}>
              Close Filters
            </button>
          </div>
        </div>
      )}

      {/* Photos Grid */}
      <div className="photos-content">
        {hasPhotos ? (
          hasFilteredPhotos ? (
            <div className="photos-gallery has-photos">
              {filteredPhotos.map((photoWithPeople, index) => (
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
                <div className="empty-icon">🔍</div>
                <h2>No Photos Match Your Filters</h2>
                <p>Try adjusting your filter criteria to see more photos.</p>
                <button className="btn btn-primary" onClick={photoFilter.clearAllFilters}>
                  Clear All Filters
                </button>
              </div>
            </div>
          )
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
