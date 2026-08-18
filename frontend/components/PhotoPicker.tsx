// Choosing which of a family's photos hang off a record, and showing the ones
// that already do.
//
// This started as the block duplicated in add-milestone.tsx and
// edit-milestone.tsx. The activities pages needed the same thing for
// performances and competitions, and three copies is one too many — so it lives
// here now, and the milestone pages call it rather than owning it.

import * as preact from "preact";
import "./photo-picker-styles";

export const PhotoPicker = ({
  photos,
  selectedIds,
  onToggle,
  disabled,
  emptyText,
}: {
  photos: { image: { id: number } }[];
  selectedIds: number[];
  onToggle: (photoId: number) => void;
  disabled?: boolean;
  // What to say when there is nothing to choose from. The milestone pages
  // narrow to one person's photos and the activities pages do not, so "no
  // photos" means something different on each and the caller says which.
  emptyText: string;
}): preact.ComponentChild => {
  if (photos.length === 0) {
    return (
      <div className="photo-picker">
        <p className="photo-picker-empty">{emptyText}</p>
      </div>
    );
  }

  return (
    <div className="photo-picker">
      {photos.map(photo => {
        const id = photo.image.id;
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
    </div>
  );
};

// PhotoStrip is the read side: the thumbnails already attached to a record,
// each a link to the photo itself. A record with no photos renders nothing
// rather than an empty frame — most of them have none, and a row of empty
// boxes down a season would be all the page.
export const PhotoStrip = ({ photoIds }: { photoIds: number[] | null }): preact.ComponentChild => {
  const ids = photoIds ?? [];
  if (ids.length === 0) return null;

  return (
    <div className="photo-strip">
      {ids.map(id => (
        <a key={id} className="photo-strip-item" href={`/view-photo/${id}`}>
          <img src={`/api/photo/${id}/thumb`} className="photo-strip-img" alt="" loading="lazy" />
        </a>
      ))}
    </div>
  );
};
