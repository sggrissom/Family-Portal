import * as preact from "preact";
import * as server from "../../server";
import { matchupParts } from "../../lib/matchups";
import {
  ResultKindAdjudication,
  ResultKindAward,
  ResultKindPlacement,
  ResultKindScore,
  resultText,
} from "../../lib/activityResults";
import "./results-styles";

export {
  ResultKindAdjudication,
  ResultKindAward,
  ResultKindPlacement,
  ResultKindScore,
  resultText,
};

export const resultKindOptions: { value: string; label: string }[] = [
  { value: ResultKindAdjudication, label: "Adjudication" },
  { value: ResultKindPlacement, label: "Placement" },
  { value: ResultKindAward, label: "Award" },
  { value: ResultKindScore, label: "Score" },
];

function personName(people: server.Person[], personId: number | null): string {
  if (personId == null) return "";
  return people.find(person => person.id === personId)?.name ?? "";
}

function resultDetail(result: server.Result, name: string): string {
  return [result.category, name].filter(part => part).join(" · ");
}

export const MatchupLine = ({ appearance }: { appearance: server.Appearance }) => {
  const parts = matchupParts(appearance);
  if (parts.length === 0) return null;
  return (
    <span className={`matchup-line${appearance.outcome ? ` matchup-${appearance.outcome}` : ""}`}>
      {parts.join(" · ")}
    </span>
  );
};

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
        const detail = resultDetail(result, personName(people, result.personId));
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

function podiumClass(result: server.Result): string {
  if (result.kind !== ResultKindPlacement || result.rank == null || result.rank > 3) return "";
  return ` result-badge-podium result-badge-p${result.rank}`;
}

const badgeOrder = [ResultKindPlacement, ResultKindAdjudication, ResultKindAward, ResultKindScore];

export const ResultBadges = ({
  results,
  people,
}: {
  results: server.Result[] | null;
  people: server.Person[];
}): preact.ComponentChild => {
  const rows = [...(results ?? [])].sort(
    (a, b) => badgeOrder.indexOf(a.kind) - badgeOrder.indexOf(b.kind)
  );
  if (rows.length === 0) return null;

  return (
    <ul className="result-badges">
      {rows.map(result => {
        const detail = resultDetail(result, personName(people, result.personId));
        return (
          <li
            key={result.id}
            className={`result-badge result-badge-${result.kind}${podiumClass(result)}`}
            title={result.notes || undefined}
          >
            <strong>{resultText(result)}</strong>
            {detail && <small>{detail}</small>}
          </li>
        );
      })}
    </ul>
  );
};

export const MediaGrid = ({
  photoIds,
  size = "large",
}: {
  photoIds: number[] | null;
  size?: "large" | "small";
}): preact.ComponentChild => {
  const ids = photoIds ?? [];
  if (ids.length === 0) return null;

  return (
    <div className={`media-grid media-grid-${size}`}>
      {ids.map(id => (
        <a key={id} className="media-grid-item" href={`/view-photo/${id}`} aria-label="View photo">
          <img src={`/api/photo/${id}/thumb`} alt="" loading="lazy" />
        </a>
      ))}
    </div>
  );
};
