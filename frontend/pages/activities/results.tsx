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
