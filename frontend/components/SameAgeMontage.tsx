import * as preact from "preact";
import * as vlens from "vlens";
import * as server from "../server";
import { FaceCrop } from "./FaceCrop";
import { chipLabels } from "../lib/familyGroups";
import { formatLongDate } from "../lib/dateUtils";
import { photoAge } from "../lib/sameAge";
import { copy } from "../lib/copy";
import "./same-age-styles";

type Picks = Record<number, number>;
const usePicks = vlens.declareHook((ageMonths: number): Picks => ({}));

function nextPick(picks: Picks, row: server.SameAgeRow) {
  picks[row.person.id] = ((picks[row.person.id] ?? 0) + 1) % row.portraits.length;
  vlens.scheduleRedraw();
}

interface SameAgeMontageProps {
  rows: server.SameAgeRow[];
  ageMonths: number;
}

export const SameAgeMontage = ({ rows, ageMonths }: SameAgeMontageProps) => {
  const picks = usePicks(ageMonths);
  const pictured = rows.filter(r => (r.portraits ?? []).length > 0);
  if (pictured.length < 2) return null;
  const names = chipLabels(rows.map(r => r.person));
  const missing = rows.filter(r => (r.portraits ?? []).length === 0);

  return (
    <section className="same-age-montage" aria-label={copy.sameAge.sideBySide}>
      <div className="same-age-tiles">
        {pictured.map(row => {
          const portrait = row.portraits[picks[row.person.id] ?? 0];
          return (
            <figure key={row.person.id} className="same-age-tile">
              <a href={`/view-photo/${portrait.photoId}`} aria-label={copy.sameAge.viewPhoto}>
                <FaceCrop photoId={portrait.photoId} box={portrait.box} size={112} alt="" />
              </a>
              <figcaption>
                <strong>{names.get(row.person.id)}</strong>
                <span>{photoAge(row.person.birthday, portrait.date)}</span>
                <span className="same-age-tile-date">{formatLongDate(portrait.date)}</span>
              </figcaption>
              {row.portraits.length > 1 && (
                <button
                  type="button"
                  className="same-age-another"
                  onClick={vlens.cachePartial(nextPick, picks, row)}
                >
                  {copy.sameAge.anotherPhoto}
                </button>
              )}
            </figure>
          );
        })}
      </div>
      {missing.length > 0 && (
        <p className="same-age-gap">
          {copy.sameAge.noPhoto(missing.map(r => names.get(r.person.id)).join(", "))}
        </p>
      )}
    </section>
  );
};
