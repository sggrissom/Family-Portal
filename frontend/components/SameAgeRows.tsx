import * as preact from "preact";
import * as server from "../server";
import { chipLabels } from "../lib/familyGroups";
import { getCategoryIcon } from "../lib/milestoneHelpers";
import { MilestoneText } from "./MilestoneText";
import { formatMeasurement } from "../lib/weightFormat";
import { ageTitle, hasSameAgeRecords, monthsOld, sameAgePath } from "../lib/sameAge";
import { copy } from "../lib/copy";
import "./same-age-styles";

const MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

function whenLabel(row: server.SameAgeRow, ageMonths: number, today: string): string {
  if (monthsOld(row.person.birthday, today + "T00:00:00Z") === ageMonths) return copy.sameAge.now;
  const date = new Date(row.date);
  return `${MONTHS[date.getUTCMonth()]} ${date.getUTCFullYear()}`;
}

function measurements(row: server.SameAgeRow): string {
  return [row.height, row.weight]
    .filter((g): g is server.GrowthData => !!g)
    .map(g => formatMeasurement(g.value, g.unit, monthsOld(row.person.birthday, g.measurementDate)))
    .join(" · ");
}

interface SameAgeRowsProps {
  rows: server.SameAgeRow[];
  ageMonths: number;
  today: string;
  photoLimit?: number;
  hideEmpty?: boolean;
}

export const SameAgeRows = ({
  rows,
  ageMonths,
  today,
  photoLimit = 6,
  hideEmpty = false,
}: SameAgeRowsProps) => {
  // Resolve names against the full comparison before hiding empty rows.
  // Otherwise two people named Alex can become indistinguishable when
  // only one of them has records at the selected age.
  const names = chipLabels(rows.map(r => r.person));
  const visible = hideEmpty ? rows.filter(hasSameAgeRecords) : rows;

  return (
    <div className="same-age-rows">
      {visible.map(row =>
        hasSameAgeRecords(row) ? (
          <div key={row.person.id} className="same-age-row">
            <div className="same-age-row-head">
              <a href={`/profile/${row.person.id}`} className="same-age-name">
                {names.get(row.person.id)}
              </a>
              <span className="same-age-when">{whenLabel(row, ageMonths, today)}</span>
              {measurements(row) && <span className="same-age-measure">{measurements(row)}</span>}
            </div>
            {(row.photoIds ?? []).length > 0 && (
              <div className="same-age-photos">
                {row.photoIds.slice(0, photoLimit).map(id => (
                  <a key={id} href={`/view-photo/${id}`} aria-label="View photo">
                    <img src={`/api/photo/${id}/thumb`} alt="" loading="lazy" />
                  </a>
                ))}
              </div>
            )}
            {(row.milestones ?? []).map(m => (
              <a key={m.id} href={`/milestone/${m.id}`} className="same-age-milestone">
                <span aria-hidden="true">{getCategoryIcon(m.category)}</span>{" "}
                <MilestoneText milestone={m} />
              </a>
            ))}
          </div>
        ) : (
          <div key={row.person.id} className="same-age-row empty">
            <a href={`/profile/${row.person.id}`} className="same-age-name">
              {names.get(row.person.id)}
            </a>
            <span className="same-age-when">{copy.sameAge.noRecords}</span>
          </div>
        )
      )}
    </div>
  );
};

interface SameAgeStripProps {
  data: server.GetSameAgeResponse | null;
  today: string;
  exceptPersonId: number;
  hideWhenEmpty?: boolean;
}

export const SameAgeStrip = ({ data, today, exceptPersonId, hideWhenEmpty }: SameAgeStripProps) => {
  if (!data) return null;
  const others = (data.rows ?? []).filter(
    r => r.person.id !== exceptPersonId && hasSameAgeRecords(r)
  );
  if (hideWhenEmpty && others.length === 0) return null;
  return (
    <section className="same-age-strip">
      <div className="same-age-strip-head">
        <h2>{copy.sameAge.atThisAge(ageTitle(data.ageMonths))}</h2>
        <a href={sameAgePath(data.ageMonths, data.fromPersonId, "details")}>
          {copy.sameAge.seeAll}
        </a>
      </div>
      {others.length > 0 ? (
        <SameAgeRows rows={others} ageMonths={data.ageMonths} today={today} photoLimit={4} />
      ) : (
        <p className="same-age-none">{copy.sameAge.nobodyElse}</p>
      )}
    </section>
  );
};
