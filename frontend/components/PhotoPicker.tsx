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
