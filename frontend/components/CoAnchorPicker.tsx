import * as preact from "preact";
import * as vlens from "vlens";
import * as server from "../server";
import { CoAnchorState, CoAnchorSuggestion } from "../lib/relations";
import "./family-links-styles";

interface CoAnchorPickerProps {
  suggestions: CoAnchorSuggestion[];
  state: CoAnchorState;
  people: server.Person[];
  relationLabel: string;
  disabled?: boolean;
}

function toggle(state: CoAnchorState, anchorId: number, event: Event) {
  const checked = (event.currentTarget as HTMLInputElement).checked;
  state.ids = checked ? [...state.ids, anchorId] : state.ids.filter(id => id !== anchorId);
  vlens.scheduleRedraw();
}

export const CoAnchorPicker = ({
  suggestions,
  state,
  people,
  relationLabel,
  disabled,
}: CoAnchorPickerProps): preact.ComponentChild => {
  if (suggestions.length === 0) {
    return null;
  }

  return (
    <div className="co-anchor-picker">
      <span className="co-anchor-title">Also applies to</span>
      {suggestions.map(suggestion => {
        const person = people.find(p => p.id === suggestion.anchorId);
        if (!person) return null;
        return (
          <label key={person.id} className="co-anchor-option">
            <input
              type="checkbox"
              checked={state.ids.includes(person.id)}
              disabled={disabled}
              onInput={vlens.cachePartial(toggle, state, person.id)}
            />
            <span>
              {relationLabel} of {person.name}
            </span>
          </label>
        );
      })}
    </div>
  );
};
