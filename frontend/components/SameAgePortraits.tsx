import * as preact from "preact";
import * as vlens from "vlens";
import * as server from "../server";
import { FaceCrop } from "./FaceCrop";
import { FullImage } from "./ResponsiveImage";
import { chipLabels } from "../lib/familyGroups";
import { ageTitle, photoAge, portraitOrder } from "../lib/sameAge";
import {
  attrsModalDialog,
  closeModalDialog,
  newModalDialog,
  type ModalDialogState,
} from "../lib/modalDialog";
import { copy } from "../lib/copy";
import "./same-age-styles";

interface PortraitsState {
  picks: Record<number, number>;
  openPersonId: number;
  dialog: ModalDialogState;
}

const usePortraits = vlens.declareHook((ageMonths: number): PortraitsState => {
  const state: PortraitsState = { picks: {}, openPersonId: 0, dialog: newModalDialog() };
  state.dialog.onDismiss = () => closePortrait(state);
  return state;
});

function openPortrait(state: PortraitsState, personId: number) {
  state.openPersonId = personId;
  vlens.scheduleRedraw();
}

function closePortrait(state: PortraitsState) {
  if (!state.openPersonId) return;
  state.openPersonId = 0;
  closeModalDialog(state.dialog);
  vlens.scheduleRedraw();
}

function backdropClicked(state: PortraitsState, event: MouseEvent) {
  if (event.target === event.currentTarget) closePortrait(state);
}

function nextPick(state: PortraitsState, row: server.SameAgeRow) {
  state.picks[row.person.id] = ((state.picks[row.person.id] ?? 0) + 1) % row.portraits.length;
  vlens.scheduleRedraw();
}

interface SameAgePortraitsProps {
  rows: server.SameAgeRow[];
  ageMonths: number;
}

export const SameAgePortraits = ({ rows, ageMonths }: SameAgePortraitsProps) => {
  const state = usePortraits(ageMonths);
  const ordered = portraitOrder(rows);
  const names = chipLabels(ordered.map(r => r.person));
  const pictured = ordered.filter(r => (r.portraits ?? []).length > 0);
  const missing = ordered.filter(r => (r.portraits ?? []).length === 0);
  const age = ageTitle(ageMonths);
  const portraitOf = (row: server.SameAgeRow) => row.portraits[state.picks[row.person.id] ?? 0];
  const open = pictured.find(r => r.person.id === state.openPersonId);

  return (
    <section className="same-age-portraits" aria-label={copy.sameAge.portraits}>
      {pictured.length === 0 ? (
        <div className="same-age-empty" role="status">
          <p>{copy.sameAge.noPortraits(age)}</p>
          <p>{copy.sameAge.tryAnotherAge}</p>
        </div>
      ) : (
        <ul className="same-age-grid">
          {pictured.map(row => {
            const portrait = portraitOf(row);
            const name = names.get(row.person.id) ?? row.person.name;
            const actual = photoAge(row.person.birthday, portrait.date);
            return (
              <li key={row.person.id} className="same-age-portrait">
                <button
                  type="button"
                  aria-haspopup="dialog"
                  aria-label={copy.sameAge.openPortrait(name, actual)}
                  onClick={vlens.cachePartial(openPortrait, state, row.person.id)}
                >
                  <FaceCrop photoId={portrait.photoId} box={portrait.box} fluid alt="" />
                  <span className="same-age-portrait-name">{name}</span>
                  <span className="same-age-portrait-age">{actual}</span>
                </button>
              </li>
            );
          })}
        </ul>
      )}
      {pictured.length === 1 && (
        <p className="same-age-none">
          {copy.sameAge.onlyPictured(names.get(pictured[0].person.id) ?? "")}
        </p>
      )}
      {pictured.length > 0 && missing.length > 0 && (
        <details className="same-age-missing">
          <summary>{copy.sameAge.missingPhotos(age, missing.length)}</summary>
          <p>{missing.map(r => names.get(r.person.id)).join(", ")}</p>
        </details>
      )}
      {open && (
        <PortraitDialog
          state={state}
          row={open}
          name={names.get(open.person.id) ?? open.person.name}
          portrait={portraitOf(open)}
        />
      )}
    </section>
  );
};

interface PortraitDialogProps {
  state: PortraitsState;
  row: server.SameAgeRow;
  name: string;
  portrait: server.PortraitPhoto;
}

const PortraitDialog = ({ state, row, name, portrait }: PortraitDialogProps) => {
  const count = row.portraits.length;
  return (
    <div className="same-age-viewer-backdrop" onClick={vlens.cachePartial(backdropClicked, state)}>
      <div
        className="same-age-viewer"
        role="dialog"
        aria-modal="true"
        aria-labelledby="sameAgeViewerTitle"
        {...attrsModalDialog(state.dialog)}
      >
        <div className="same-age-viewer-head">
          <h2 id="sameAgeViewerTitle">
            {name} <span>{photoAge(row.person.birthday, portrait.date)}</span>
          </h2>
          <button
            type="button"
            className="same-age-viewer-close"
            aria-label={copy.sameAge.close}
            onClick={vlens.cachePartial(closePortrait, state)}
          >
            ×
          </button>
        </div>
        <FullImage
          photoId={portrait.photoId}
          alt={copy.sameAge.photoOf(name)}
          className="same-age-viewer-image"
        />
        <div className="same-age-viewer-actions">
          {count > 1 && (
            <button
              type="button"
              className="same-age-another"
              onClick={vlens.cachePartial(nextPick, state, row)}
            >
              {copy.sameAge.anotherPhoto}
              <span aria-live="polite">
                {" "}
                · {copy.sameAge.photoCount((state.picks[row.person.id] ?? 0) + 1, count)}
              </span>
            </button>
          )}
          <a href={`/view-photo/${portrait.photoId}`}>{copy.sameAge.openPhotoPage}</a>
        </div>
      </div>
    </div>
  );
};
