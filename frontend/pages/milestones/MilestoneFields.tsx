import * as preact from "preact";
import * as vlens from "vlens";
import * as server from "../../server";
import { copy } from "../../lib/copy";
import { WhenRequest } from "../../lib/when";
import { MILESTONE_CATEGORIES } from "../../lib/milestoneHelpers";
import "./add-milestone-styles";
import "../../components/entry-form-styles";

const CATEGORY_ORDER = ["first", "development", "achievement", "behavior", "health", "other"];
const CATEGORIES = CATEGORY_ORDER.map(value => MILESTONE_CATEGORIES.find(c => c.value === value)!);

export function toggleId(ids: number[], id: number) {
  const idx = ids.indexOf(id);
  if (idx >= 0) ids.splice(idx, 1);
  else ids.push(id);
  vlens.scheduleRedraw();
}

interface CategoryChipsProps {
  value: string;
  suggested?: boolean;
  onChange: (category: string) => void;
  disabled: boolean;
}

export const CategoryChips = ({ value, suggested, onChange, disabled }: CategoryChipsProps) => (
  <div className="entry-field">
    <span className="entry-label" id="categoryLabel">
      {copy.milestone.category}
      {suggested && <span className="entry-suggested"> · suggested</span>}
    </span>
    <div className="category-chips" role="group" aria-labelledby="categoryLabel">
      {CATEGORIES.map(category => (
        <button
          key={category.value}
          type="button"
          className={value === category.value ? "category-chip selected" : "category-chip"}
          aria-pressed={value === category.value ? "true" : "false"}
          disabled={disabled}
          onClick={vlens.cachePartial(onChange, category.value)}
        >
          <span aria-hidden="true">{category.icon}</span>
          {category.label}
        </button>
      ))}
    </div>
  </div>
);

export async function suggestPhotos(
  personId: number,
  description: string,
  when: WhenRequest,
  excludeIds: number[]
): Promise<number[]> {
  const [resp] = await server.SuggestMilestonePhotos({
    personId,
    description,
    inputType: when.inputType,
    milestoneDate: when.date,
    ageYears: when.ageYears,
    ageMonths: when.ageMonths,
    excludeIds,
  });
  return resp?.photoIds ?? [];
}

interface SuggestedPhotosProps {
  photoIds: number[];
  selected: number[];
  disabled: boolean;
}

export const SuggestedPhotos = ({ photoIds, selected, disabled }: SuggestedPhotosProps) =>
  photoIds.length > 0 && (
    <div className="entry-field">
      <span className="entry-label">Photos from around then. Attach any?</span>
      <div className="suggested-photos">
        {photoIds.map(id => {
          const isSelected = selected.includes(id);
          return (
            <button
              key={id}
              type="button"
              className={`suggested-photo${isSelected ? " selected" : ""}`}
              aria-pressed={isSelected}
              aria-label={isSelected ? "Don't attach this photo" : "Attach this photo"}
              disabled={disabled}
              onClick={vlens.cachePartial(toggleId, selected, id)}
            >
              <img src={`/api/photo/${id}/thumb`} alt="" loading="lazy" />
            </button>
          );
        })}
      </div>
    </div>
  );

interface TagPickerProps {
  tags: server.Tag[];
  selected: number[];
  disabled: boolean;
}

export const TagPicker = ({ tags, selected, disabled }: TagPickerProps) =>
  tags.length > 0 && (
    <div className="entry-field">
      <span className="entry-label" id="tagPickerLabel">
        {copy.milestone.tags}
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
