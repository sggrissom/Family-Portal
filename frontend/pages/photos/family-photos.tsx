import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as auth from "../../lib/authCache";
import * as core from "vlens/core";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { ensureAuthInFetch, requireAuthInView } from "../../lib/authHelpers";
import { ThumbnailImage } from "../../components/ResponsiveImage";
import { usePhotoStatus, Status } from "../../hooks/usePhotoStatus";
import { usePhotoFilter } from "../../hooks/usePhotoFilter";
import {
  hasMorePhotos,
  loadMorePhotos,
  seedPhotoPages,
  syncPhotoPages,
  usePhotoPages,
} from "../../hooks/usePhotoPages";
import { LoadMore } from "../../components/LoadMore";
import { parseFilterQuery, serverFilters } from "../../lib/photoFilterQuery";
import { PHOTO_PAGE_SIZE, photosRequest } from "../../lib/photoPages";
import { saveSequence, viewPhotoRoute } from "../../lib/photoSequence";
import "./family-photos-styles";

type FamilyPhotosData = {
  first: server.ListFamilyPhotosResponse;
  filters: ReturnType<typeof serverFilters>;
};

export async function fetch(
  route: string,
  prefix: string
): Promise<rpc.Response<FamilyPhotosData>> {
  const filters = serverFilters(parseFilterQuery(route.split("?")[1] ?? ""));
  if (!(await ensureAuthInFetch())) {
    return rpc.ok<FamilyPhotosData>({
      first: { photos: [], nextCursor: "", matchedPersonIds: [], searchMode: "" },
      filters,
    });
  }

  const [first, err] = await server.ListFamilyPhotos(
    photosRequest({ ...filters, limit: PHOTO_PAGE_SIZE })
  );
  if (!first) return [null, err];
  return rpc.ok<FamilyPhotosData>({ first, filters });
}

export function view(route: string, prefix: string, data: FamilyPhotosData): preact.ComponentChild {
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
  data: FamilyPhotosData;
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

function openPhoto(photoId: number, photos: server.PhotoWithPeople[]) {
  saveSequence({ ids: photos.map(p => p.image.id), backRoute: core.getRoute() });
  core.setRoute(viewPhotoRoute(photoId, true));
}

const useSearchDraft = vlens.declareHook(() => ({ text: "", syncedTo: "" }));

function matchedNames(photos: server.PhotoWithPeople[], ids: number[]): string {
  const names = ids.map(
    id =>
      photos
        .flatMap(p => p.people)
        .find(person => person.id === id)
        ?.name.split(" ")[0] ?? ""
  );
  return names.filter(Boolean).join(" and ");
}

function openStack(cover: server.PhotoWithPeople) {
  saveSequence({ ids: [cover.image.id, ...(cover.similar ?? [])], backRoute: core.getRoute() });
  core.setRoute(viewPhotoRoute(cover.image.id, true));
}

const FamilyPhotosPage = ({ user, data }: FamilyPhotosPageProps) => {
  const photoFilter = usePhotoFilter();
  const photoStatus = usePhotoStatus();
  const pages = usePhotoPages("family-photos");

  seedPhotoPages(pages, data.filters, data.first);
  syncPhotoPages(pages, photoFilter.serverFilters());

  const filteredPhotos = pages.photos;
  const hasMore = hasMorePhotos(pages);
  const hasFilteredPhotos = filteredPhotos.length > 0;
  const hasPhotos =
    hasFilteredPhotos ||
    photoFilter.hasActiveFilters() ||
    photoFilter.query !== "" ||
    !pages.started;
  const countLabel = `${filteredPhotos.length}${hasMore ? "+" : ""}`;
  const searching = photoFilter.query !== "";
  const draft = useSearchDraft();
  if (draft.syncedTo !== photoFilter.query) {
    draft.text = photoFilter.query;
    draft.syncedTo = photoFilter.query;
  }
  const withNames = matchedNames(filteredPhotos, pages.matchedPersonIds);

  if (hasFilteredPhotos) {
    filteredPhotos.forEach(photoWithPeople => {
      const photo = photoWithPeople.image;
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

  return (
    <div className="family-photos-page">
      <div className="page-header">
        <div className="header-content">
          <div>
            <h1>Family Photos</h1>
            {hasPhotos && (
              <div className="photos-count">
                {searching
                  ? `${countLabel} result${countLabel !== "1" ? "s" : ""}`
                  : photoFilter.hasActiveFilters()
                    ? `${countLabel} matching photo${countLabel !== "1" ? "s" : ""}`
                    : `${countLabel} photo${countLabel !== "1" ? "s" : ""}`}
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
            {hasPhotos && !searching && (
              <button
                className="btn btn-secondary"
                onClick={photoFilter.toggleShowSimilar}
                title={
                  photoFilter.showSimilar
                    ? "Show each burst of similar photos as one stack"
                    : "Show every photo, including near-duplicates"
                }
              >
                {photoFilter.showSimilar ? "🗂️ Stack similar" : "🗂️ Show all"}
              </button>
            )}
            {hasPhotos && (
              <a href="/faces" className="btn btn-secondary">
                🙂 Faces
              </a>
            )}
            <a href="/add-photo" className="btn btn-primary">
              📸 Add Photo
            </a>
          </div>
        </div>
      </div>

      {hasPhotos && (
        <form
          className="photo-search"
          role="search"
          onSubmit={e => {
            e.preventDefault();
            photoFilter.setQuery(draft.text);
          }}
        >
          <input
            type="search"
            aria-label="Search photos"
            placeholder="Search photos: “birthday cake”, “at the beach”, a name…"
            maxLength={200}
            value={draft.text}
            onInput={e => (draft.text = e.currentTarget.value)}
          />
          <button type="submit" className="btn btn-primary">
            Search
          </button>
        </form>
      )}

      {searching && (
        <div className="photo-search-summary">
          <span>
            Best matches for “{photoFilter.query}”{withNames && ` with ${withNames}`}
          </span>
          {pages.searchMode === "text" && (
            <span className="photo-search-note">
              Image search is unavailable right now, so only titles and descriptions were searched.
            </span>
          )}
          <button className="photo-search-clear" onClick={() => photoFilter.setQuery("")}>
            Clear search
          </button>
        </div>
      )}

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
            <h3>Filter by Tags</h3>
            <div className="tags-filter">
              {photoFilter.tagsLoading ? (
                <div className="loading-state">Loading tags...</div>
              ) : photoFilter.tags.length === 0 ? (
                <div className="empty-state-inline">No tags created yet</div>
              ) : (
                photoFilter.tags.map(tag => (
                  <label key={tag.id} className="tag-filter-label">
                    <input
                      type="checkbox"
                      checked={photoFilter.selectedTagIds.includes(tag.id)}
                      onChange={() => photoFilter.toggleTag(tag.id)}
                    />
                    <span className="tag-color-dot" style={{ background: tag.color }} />
                    <span>{tag.name}</span>
                  </label>
                ))
              )}
            </div>
          </div>

          {photoFilter.places.length > 0 && (
            <div className="filter-section">
              <h3>Filter by Place</h3>
              <div className="places-filter">
                <label className="tag-filter-label">
                  <input
                    type="radio"
                    name="place-filter"
                    checked={photoFilter.placeKey === ""}
                    onChange={() => photoFilter.setPlace("")}
                  />
                  <span>Anywhere</span>
                </label>
                {photoFilter.places.map(place => (
                  <label key={place.key} className="tag-filter-label">
                    <input
                      type="radio"
                      name="place-filter"
                      checked={photoFilter.placeKey === place.key}
                      onChange={() => photoFilter.setPlace(place.key)}
                    />
                    <span>
                      {place.key.startsWith("f") ? "📍 " : ""}
                      {place.name} <span className="place-count">({place.count})</span>
                    </span>
                  </label>
                ))}
              </div>
            </div>
          )}

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

      <div className="photos-content">
        {hasPhotos ? (
          hasFilteredPhotos ? (
            <>
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
                        onClick={() => openPhoto(photoWithPeople.image.id, filteredPhotos)}
                        status={photoStatus.getStatus(photoWithPeople.image.id)}
                      />
                      {photoWithPeople.people.some(
                        person => person.profilePhotoId === photoWithPeople.image.id
                      ) && <div className="profile-photo-badge">👤 Profile</div>}
                      {(photoWithPeople.similar?.length ?? 0) > 0 && (
                        <button
                          className="similar-stack-badge"
                          onClick={() => openStack(photoWithPeople)}
                          title="Similar photos taken around the same time"
                        >
                          +{photoWithPeople.similar!.length} similar
                        </button>
                      )}
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
                    {photoWithPeople.image.tagIds && photoWithPeople.image.tagIds.length > 0 && (
                      <div className="tag-badges">
                        {photoWithPeople.image.tagIds.map(tagId => {
                          const tag = photoFilter.tags.find(t => t.id === tagId);
                          return tag ? (
                            <span
                              key={tagId}
                              className="tag-badge"
                              style={{ background: tag.color }}
                              title={tag.name}
                            />
                          ) : null;
                        })}
                      </div>
                    )}
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
              {hasMore && <LoadMore loading={pages.loading} onLoad={() => loadMorePhotos(pages)} />}
              {pages.error && <div className="error-message">{pages.error}</div>}
            </>
          ) : !pages.started || pages.loading ? (
            <div className="photos-gallery">
              <div className="loading-state">Loading photos...</div>
            </div>
          ) : (
            <div className="photos-gallery">
              <div className="empty-state">
                <div className="empty-icon">🔍</div>
                <h2>
                  {searching ? "Nothing Matched That Search" : "No Photos Match Your Filters"}
                </h2>
                <p>
                  {searching
                    ? "Try describing what's in the photo differently, or clear the search."
                    : "Try adjusting your filter criteria to see more photos."}
                </p>
                <button
                  className="btn btn-primary"
                  onClick={() =>
                    searching ? photoFilter.setQuery("") : photoFilter.clearAllFilters()
                  }
                >
                  {searching ? "Clear Search" : "Clear All Filters"}
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
