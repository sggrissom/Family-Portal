import * as preact from "preact";
import * as server from "../server";
import { LoadMore } from "./LoadMore";
import {
  hasMorePhotos,
  loadMorePhotos,
  syncPhotoPages,
  usePhotoPages,
} from "../hooks/usePhotoPages";
import "./photo-picker-styles";

export const PhotoPicker = ({
  photos,
  selectedIds,
  onToggle,
  disabled,
  emptyText,
  loading = false,
  onLoadMore,
}: {
  photos: { image: { id: number } }[];
  selectedIds: number[];
  onToggle: (photoId: number) => void;
  disabled?: boolean;
  emptyText: string;
  loading?: boolean;
  onLoadMore?: () => void;
}): preact.ComponentChild => {
  // A selected photo may sit on a page not loaded yet; it still needs to show
  // so it can be unselected.
  const listed = new Set(photos.map(photo => photo.image.id));
  const ids = [...selectedIds.filter(id => !listed.has(id)), ...photos.map(p => p.image.id)];

  if (ids.length === 0) {
    return (
      <div className="photo-picker">
        <p className="photo-picker-empty">{loading ? "Loading photos..." : emptyText}</p>
      </div>
    );
  }

  return (
    <div className="photo-picker">
      {ids.map(id => {
        const isSelected = selectedIds.includes(id);
        return (
          <button
            key={id}
            type="button"
            className={`photo-picker-item${isSelected ? " selected" : ""}`}
            aria-pressed={isSelected}
            disabled={disabled}
            onClick={() => onToggle(id)}
          >
            <img
              src={`/api/photo/${id}/thumb`}
              className="photo-picker-img"
              alt=""
              loading="lazy"
            />
            {isSelected && <div className="photo-picker-check">✓</div>}
          </button>
        );
      })}
      {onLoadMore && (
        <div className="photo-picker-more">
          <LoadMore loading={loading} onLoad={onLoadMore} />
        </div>
      )}
    </div>
  );
};

// A PhotoPicker that loads the family's photos a page at a time.
export const PagedPhotoPicker = ({
  pageKey,
  filters,
  ...picker
}: {
  pageKey: string;
  filters: Partial<server.ListFamilyPhotosRequest>;
  selectedIds: number[];
  onToggle: (photoId: number) => void;
  disabled?: boolean;
  emptyText: string;
}): preact.ComponentChild => {
  const pages = usePhotoPages(pageKey);
  syncPhotoPages(pages, filters);
  return (
    <PhotoPicker
      {...picker}
      photos={pages.photos}
      loading={!pages.started || pages.loading}
      onLoadMore={pages.started && hasMorePhotos(pages) ? () => loadMorePhotos(pages) : undefined}
    />
  );
};

export const PhotoStrip = ({ photoIds }: { photoIds: number[] | null }): preact.ComponentChild => {
  const ids = photoIds ?? [];
  if (ids.length === 0) return null;

  return (
    <div className="photo-strip">
      {ids.map(id => (
        <a key={id} className="photo-strip-item" href={`/view-photo/${id}`} aria-label="View photo">
          <img src={`/api/photo/${id}/thumb`} className="photo-strip-img" alt="" loading="lazy" />
        </a>
      ))}
    </div>
  );
};
