import * as preact from "preact";
import * as vlens from "vlens";
import * as server from "../server";
import { chipLabels } from "../lib/familyGroups";
import { ProfileImage } from "./ResponsiveImage";
import "./person-chips-styles";

interface PersonChipsProps {
  people: server.Person[];
  selected: number[];
  onToggle: (personId: number) => void;
  label: string;
  disabled?: boolean;
}

export const PersonChips = ({ people, selected, onToggle, label, disabled }: PersonChipsProps) => {
  const labels = chipLabels(people);
  return (
    <div className="person-chips" role="group" aria-label={label}>
      {people.map(person => {
        const isSelected = selected.includes(person.id);
        return (
          <button
            key={person.id}
            type="button"
            className={isSelected ? "person-chip selected" : "person-chip"}
            aria-pressed={isSelected ? "true" : "false"}
            disabled={disabled}
            onClick={vlens.cachePartial(onToggle, person.id)}
          >
            <span className="person-chip-avatar" aria-hidden="true">
              {person.profilePhotoId ? (
                <ProfileImage
                  photoId={person.profilePhotoId}
                  alt=""
                  cropX={person.profileCropX}
                  cropY={person.profileCropY}
                  cropScale={person.profileCropScale}
                />
              ) : (
                initial(person.name)
              )}
            </span>
            {labels.get(person.id)}
          </button>
        );
      })}
    </div>
  );
};

export function initial(name: string): string {
  const first = name.trim()[0];
  return first ? first.toUpperCase() : "?";
}

export function scrollSelectedChipIntoView() {
  requestAnimationFrame(() =>
    document.querySelectorAll(".person-chips").forEach(row =>
      row.querySelector(".person-chip.selected")?.scrollIntoView({
        block: "nearest",
        inline: "center",
      })
    )
  );
}
