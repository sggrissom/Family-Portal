import * as preact from "preact";
import * as vlens from "vlens";
import * as server from "../server";
import "./tag-picker-styles";

export function toggleId(ids: number[], id: number) {
  const idx = ids.indexOf(id);
  if (idx >= 0) ids.splice(idx, 1);
  else ids.push(id);
  vlens.scheduleRedraw();
}

interface TagPickerProps {
  label: string;
  tags: server.Tag[];
  selected: number[];
  disabled: boolean;
}

export const TagPicker = ({ label, tags, selected, disabled }: TagPickerProps) =>
  tags.length > 0 && (
    <div className="entry-field">
      <span className="entry-label" id="tagPickerLabel">
        {label}
      </span>
      <div className="tag-picker" role="group" aria-labelledby="tagPickerLabel">
        {tags.map(tag => {
          const isSelected = selected.includes(tag.id);
          return (
            <button
              key={tag.id}
              type="button"
              className={`tag-pill${isSelected ? " selected" : ""}`}
              style={{ borderColor: tag.color }}
              aria-pressed={isSelected}
              disabled={disabled}
              onClick={vlens.cachePartial(toggleId, selected, tag.id)}
            >
              <span className="tag-color-dot" style={{ background: tag.color }} />
              {tag.name}
            </button>
          );
        })}
      </div>
    </div>
  );
