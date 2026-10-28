import { Trash2 } from "lucide-preact";
import { Icon } from "../../components/Icon";
import * as preact from "preact";
import * as vlens from "vlens";
import * as server from "../../server";
import {
  ResultKindAdjudication,
  ResultKindAward,
  ResultKindPlacement,
  ResultKindScore,
  resultKindOptions,
} from "./results";
import "./results-editor-styles";

export type ResultRow = {
  kind: string;
  label: string;
  rank: string;
  outOf: string;
  category: string;
  score: string;
  personId: number;
  notes: string;
};

export const blankResultRow = (): ResultRow => ({
  kind: ResultKindAdjudication,
  label: "",
  rank: "",
  outOf: "",
  category: "",
  score: "",
  personId: 0,
  notes: "",
});

export function resultToRow(result: server.Result): ResultRow {
  return {
    kind: result.kind,
    label: result.label,
    rank: result.rank == null ? "" : String(result.rank),
    outOf: result.outOf == null ? "" : String(result.outOf),
    category: result.category,
    score: result.score == null ? "" : String(result.score),
    personId: result.personId ?? 0,
    notes: result.notes,
  };
}

function intOrNull(value: string): number | null {
  const trimmed = value.trim();
  if (trimmed === "") return null;
  const parsed = Number(trimmed);
  return Number.isFinite(parsed) ? Math.trunc(parsed) : null;
}

function numberOrNull(value: string): number | null {
  const trimmed = value.trim();
  if (trimmed === "") return null;
  const parsed = Number(trimmed);
  return Number.isFinite(parsed) ? parsed : null;
}

export function rowToInput(row: ResultRow): server.ResultInput {
  const usesRank = row.kind === ResultKindPlacement;
  const usesScore = row.kind === ResultKindScore;
  return {
    kind: row.kind,
    label: row.label.trim(),
    rank: usesRank ? intOrNull(row.rank) : null,
    outOf: usesRank ? intOrNull(row.outOf) : null,
    category: usesRank ? row.category.trim() : "",
    score: usesScore ? numberOrNull(row.score) : null,
    personId: row.personId === 0 ? null : row.personId,
    notes: row.notes.trim(),
  };
}

export function rowError(row: ResultRow): string {
  switch (row.kind) {
    case ResultKindAdjudication:
      if (!row.label.trim()) return "An adjudication needs a label.";
      break;
    case ResultKindAward:
      if (!row.label.trim()) return "An award needs a label.";
      break;
    case ResultKindPlacement: {
      const rank = intOrNull(row.rank);
      if (rank === null) return "A placement needs a rank.";
      if (rank < 1) return "A rank must be 1 or greater.";
      const outOf = intOrNull(row.outOf);
      if (outOf !== null && outOf < 1) return "A field size must be 1 or greater.";
      if (outOf !== null && rank > outOf) return "A rank cannot be larger than the field.";
      break;
    }
    case ResultKindScore:
      if (numberOrNull(row.score) === null) return "A score result needs a score.";
      break;
  }
  return "";
}

export interface ResultsEditorHost {
  resultRows: ResultRow[];
  saving: boolean;
}

export function onAddResultRow(host: ResultsEditorHost, kind: string, label: string) {
  host.resultRows.push({ ...blankResultRow(), kind, label });
  vlens.scheduleRedraw();
}

export function onRemoveResultRow(host: ResultsEditorHost, index: number) {
  host.resultRows.splice(index, 1);
  vlens.scheduleRedraw();
}

export function resultRowsBlocked(host: ResultsEditorHost): boolean {
  return host.resultRows.some(row => rowError(row) !== "");
}

export const ResultsEditor = ({
  host,
  vocabulary,
  roster,
  quick,
}: {
  host: ResultsEditorHost;
  vocabulary: server.ListActivityVocabularyResponse;
  roster: server.Person[];
  quick: string[];
}): preact.ComponentChild => (
  <div className="results-editor">
    {host.resultRows.map((row, index) => (
      <ResultRowFields
        key={index}
        host={host}
        row={row}
        index={index}
        vocabulary={vocabulary}
        roster={roster}
      />
    ))}

    <div className="result-quick-add">
      <span className="result-quick-label">
        {host.resultRows.length === 0 ? "Add a result" : "Add another"}
      </span>
      {quick.map(label => (
        <button
          key={`adj-${label}`}
          type="button"
          className="result-chip result-chip-adjudication"
          onClick={vlens.cachePartial(onAddResultRow, host, ResultKindAdjudication, label)}
          disabled={host.saving}
        >
          {label}
        </button>
      ))}
      {resultKindOptions.map(option => (
        <button
          key={option.value}
          type="button"
          className="result-chip"
          onClick={vlens.cachePartial(onAddResultRow, host, option.value, "")}
          disabled={host.saving}
        >
          + {option.label}
        </button>
      ))}
    </div>
  </div>
);

const Suggestions = ({ id, values }: { id: string; values: string[] | null }) => (
  <datalist id={id}>
    {(values ?? []).map(value => (
      <option key={value} value={value} />
    ))}
  </datalist>
);

function textField(
  host: ResultsEditorHost,
  row: ResultRow,
  field: "label" | "rank" | "outOf" | "category" | "score" | "notes",
  props: preact.JSX.InputHTMLAttributes<HTMLInputElement>
) {
  return (
    <input
      type="text"
      {...props}
      value={row[field]}
      onInput={e => {
        row[field] = e.currentTarget.value;
        vlens.scheduleRedraw();
      }}
      disabled={host.saving}
    />
  );
}

const ResultRowFields = ({
  host,
  row,
  index,
  vocabulary,
  roster,
}: {
  host: ResultsEditorHost;
  row: ResultRow;
  index: number;
  vocabulary: server.ListActivityVocabularyResponse;
  roster: server.Person[];
}) => {
  const error = rowError(row);

  return (
    <div className={`result-edit-row result-edit-${row.kind}`}>
      <div className="result-edit-fields">
        <select
          className="result-edit-kind"
          aria-label="Kind of result"
          value={row.kind}
          onInput={e => {
            row.kind = e.currentTarget.value;
            vlens.scheduleRedraw();
          }}
          disabled={host.saving}
        >
          {resultKindOptions.map(option => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </select>

        {(row.kind === ResultKindAdjudication || row.kind === ResultKindAward) && (
          <>
            {textField(host, row, "label", {
              className: "result-edit-wide",
              "aria-label": row.kind === ResultKindAdjudication ? "Adjudication" : "Award",
              list: `resultLabelOptions${index}`,
              placeholder: row.kind === ResultKindAdjudication ? "High Gold" : "Judges' Choice",
            })}
            <Suggestions
              id={`resultLabelOptions${index}`}
              values={
                row.kind === ResultKindAdjudication ? vocabulary.adjudications : vocabulary.awards
              }
            />
          </>
        )}

        {row.kind === ResultKindPlacement && (
          <>
            {textField(host, row, "rank", {
              className: "result-edit-num",
              inputMode: "numeric",
              "aria-label": "Place",
              placeholder: "Place",
            })}
            <span className="result-edit-of">of</span>
            {textField(host, row, "outOf", {
              className: "result-edit-num",
              inputMode: "numeric",
              "aria-label": "Out of",
              placeholder: "—",
            })}
            {textField(host, row, "category", {
              className: "result-edit-wide",
              "aria-label": "Category",
              list: `resultCategoryOptions${index}`,
              placeholder: "Category, e.g. Teen Small Group Jazz",
            })}
            <Suggestions id={`resultCategoryOptions${index}`} values={vocabulary.categories} />
          </>
        )}

        {row.kind === ResultKindScore && (
          <>
            {textField(host, row, "score", {
              className: "result-edit-num",
              inputMode: "decimal",
              "aria-label": "Score",
              placeholder: "Score",
            })}
            {textField(host, row, "label", {
              className: "result-edit-wide",
              "aria-label": "What the score is for",
              placeholder: "What for, e.g. Technique",
            })}
          </>
        )}

        {roster.length > 1 && (
          <select
            className="result-edit-person"
            aria-label="Who this result is for"
            value={String(row.personId)}
            onInput={e => {
              row.personId = Number(e.currentTarget.value);
              vlens.scheduleRedraw();
            }}
            disabled={host.saving}
          >
            <option value="0">Everyone</option>
            {roster.map(person => (
              <option key={person.id} value={String(person.id)}>
                {person.name}
              </option>
            ))}
          </select>
        )}

        <button
          type="button"
          className="icon-btn result-remove"
          title="Remove this result"
          aria-label="Remove this result"
          onClick={vlens.cachePartial(onRemoveResultRow, host, index)}
          disabled={host.saving}
        >
          <Icon symbol={Trash2} />
        </button>
      </div>

      {row.notes !== "" &&
        textField(host, row, "notes", {
          className: "result-edit-notes",
          "aria-label": "Notes on this result",
          placeholder: "Notes",
        })}

      {error && <p className="result-row-error">{error}</p>}
    </div>
  );
};
