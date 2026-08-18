// Rendering a performance's results. Shared by the competition page and the
// routine history, which show the same rows from opposite directions.
//
// See docs/activities-plan.md, phase 6.

import * as preact from "preact";
import * as server from "../../server";
import "./results-styles";

export const ResultKindAdjudication = "adjudication";
export const ResultKindPlacement = "placement";
export const ResultKindAward = "award";
export const ResultKindScore = "score";

export const resultKindOptions: { value: string; label: string }[] = [
  { value: ResultKindAdjudication, label: "Adjudication" },
  { value: ResultKindPlacement, label: "Placement" },
  { value: ResultKindAward, label: "Award" },
  { value: ResultKindScore, label: "Score" },
];

function ordinal(n: number): string {
  const mod100 = n % 100;
  if (mod100 >= 11 && mod100 <= 13) return `${n}th`;
  switch (n % 10) {
    case 1:
      return `${n}st`;
    case 2:
      return `${n}nd`;
    case 3:
      return `${n}rd`;
    default:
      return `${n}th`;
  }
}

// resultText is the one-line form of a result. The four kinds use disjoint
// fields, so each reads differently: a placement leads with its rank, a score
// with its number, and the two label kinds are just the label a judge wrote.
//
// The optional fields are `omitempty` pointers on the wire, so a missing rank
// or field size arrives as undefined rather than null. Hence the loose checks:
// a placement with no field size reads "1st", not "1st of undefined".
export function resultText(result: server.Result): string {
  switch (result.kind) {
    case ResultKindPlacement: {
      const place = result.rank == null ? result.label : ordinal(result.rank);
      const field = result.outOf == null ? "" : ` of ${result.outOf}`;
      return `${place}${field}`;
    }
    case ResultKindScore: {
      const score = result.score == null ? result.label : String(result.score);
      return result.label && result.score != null ? `${score} ${result.label}` : score;
    }
    default:
      return result.label;
  }
}

// The category and the person a result narrows to are the context that makes
// "2nd of 14" mean something, but they belong under the result rather than
// inside it.
function resultDetail(result: server.Result, personName: string): string {
  return [result.category, personName].filter(part => part).join(" · ");
}

export const ResultList = ({
  results,
  people,
}: {
  results: server.Result[] | null;
  people: server.Person[];
}): preact.ComponentChild => {
  const rows = results ?? [];
  if (rows.length === 0) return null;

  return (
    <ul className="result-list">
      {rows.map(result => {
        const personName =
          result.personId == null
            ? ""
            : (people.find(person => person.id === result.personId)?.name ?? "");
        const detail = resultDetail(result, personName);
        return (
          <li key={result.id} className={`result-row result-${result.kind}`}>
            <span className="result-text">{resultText(result)}</span>
            {detail && <span className="result-detail">{detail}</span>}
            {result.notes && <span className="result-detail">{result.notes}</span>}
          </li>
        );
      })}
    </ul>
  );
};
