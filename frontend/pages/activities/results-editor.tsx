// Editing a performance's results.
//
// Results are replace-all on the wire (SetAppearanceResults), because that is
// how they arrive: one results sheet, read off in one sitting. So this is a
// row editor over a local list, and saving sends the whole set.
//
// See docs/activities-plan.md, phase 6.

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

// ResultRow holds every field as a string because an empty numeric input has
// to stay distinguishable from a zero: the backend takes nil pointers for
// "no rank" and "no score", and <input type="number"> valueAsNumber gives NaN
// for both empty and garbage.
export type ResultRow = {
  kind: string;
  label: string;
  rank: string;
  outOf: string;
  category: string;
  score: string;
  // 0 means the result belongs to the whole entry, which is the common case.
  // A non-zero id narrows an award to one person on the roster.
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

// rowToInput drops the fields the chosen kind does not use rather than sending
// them. The four kinds have disjoint fields, and a rank left over from a row
// that used to be a placement would otherwise ride along into an award.
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

// rowError mirrors validateResultInput on the server. Duplicating it is worth
// one thing only: the message lands under the field that is wrong, instead of
// arriving at the top of the page after a round trip that rejected the whole
// sheet.
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

export function onAddResultRow(host: ResultsEditorHost) {
  host.resultRows.push(blankResultRow());
  vlens.scheduleRedraw();
}

export function onRemoveResultRow(host: ResultsEditorHost, index: number) {
  host.resultRows.splice(index, 1);
  vlens.scheduleRedraw();
}

export const ResultsEditor = ({
  host,
  vocabulary,
  roster,
  onSave,
  onCancel,
}: {
  host: ResultsEditorHost;
  vocabulary: server.ListActivityVocabularyResponse;
  // The people on this entry's roster. A result can only narrow to one of
  // them — the backend rejects anyone else, since a stray id would land in
  // ResultByPersonIndex and surface under another kid's awards.
  roster: server.Person[];
  onSave: () => void;
  onCancel: () => void;
}): preact.ComponentChild => {
  const blocked = host.resultRows.some(row => rowError(row) !== "");

  return (
    <div className="activities-form results-editor">
      {host.resultRows.length === 0 && (
        <p className="form-hint">No results yet. Add a row for each line on the results sheet.</p>
      )}

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

      <div className="form-actions">
        <button
          className="btn btn-secondary"
          onClick={vlens.cachePartial(onAddResultRow, host)}
          disabled={host.saving}
        >
          Add result
        </button>
        <button className="btn btn-primary" onClick={onSave} disabled={host.saving || blocked}>
          Save results
        </button>
        <button className="btn btn-secondary" onClick={onCancel} disabled={host.saving}>
          Cancel
        </button>
      </div>
    </div>
  );
};

const Suggestions = ({ id, values }: { id: string; values: string[] | null }) => (
  <datalist id={id}>
    {(values ?? []).map(value => (
      <option key={value} value={value} />
    ))}
  </datalist>
);

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
    <div className="result-edit-row">
      <div className="form-row">
        <div className="form-group flex-1">
          <label htmlFor={`resultKind${index}`}>Kind</label>
          <select
            id={`resultKind${index}`}
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
        </div>

        {(row.kind === ResultKindAdjudication || row.kind === ResultKindAward) && (
          <div className="form-group flex-2">
            <label htmlFor={`resultLabel${index}`}>Label</label>
            <input
              id={`resultLabel${index}`}
              type="text"
              list={`resultLabelOptions${index}`}
              placeholder={row.kind === ResultKindAdjudication ? "High Gold" : "Judges' Choice"}
              value={row.label}
              onInput={e => {
                row.label = e.currentTarget.value;
                vlens.scheduleRedraw();
              }}
              disabled={host.saving}
            />
            <Suggestions
              id={`resultLabelOptions${index}`}
              values={
                row.kind === ResultKindAdjudication ? vocabulary.adjudications : vocabulary.awards
              }
            />
          </div>
        )}

        {row.kind === ResultKindPlacement && (
          <>
            <div className="form-group flex-1">
              <label htmlFor={`resultRank${index}`}>Place</label>
              <input
                id={`resultRank${index}`}
                type="number"
                min="1"
                value={row.rank}
                onInput={e => {
                  row.rank = e.currentTarget.value;
                  vlens.scheduleRedraw();
                }}
                disabled={host.saving}
              />
            </div>
            <div className="form-group flex-1">
              <label htmlFor={`resultOutOf${index}`}>Out of</label>
              <input
                id={`resultOutOf${index}`}
                type="number"
                min="1"
                value={row.outOf}
                onInput={e => {
                  row.outOf = e.currentTarget.value;
                  vlens.scheduleRedraw();
                }}
                disabled={host.saving}
              />
            </div>
          </>
        )}

        {row.kind === ResultKindScore && (
          <>
            <div className="form-group flex-1">
              <label htmlFor={`resultScore${index}`}>Score</label>
              <input
                id={`resultScore${index}`}
                type="number"
                step="any"
                value={row.score}
                onInput={e => {
                  row.score = e.currentTarget.value;
                  vlens.scheduleRedraw();
                }}
                disabled={host.saving}
              />
            </div>
            <div className="form-group flex-1">
              <label htmlFor={`resultScoreLabel${index}`}>Label</label>
              <input
                id={`resultScoreLabel${index}`}
                type="text"
                placeholder="optional"
                value={row.label}
                onInput={e => {
                  row.label = e.currentTarget.value;
                  vlens.scheduleRedraw();
                }}
                disabled={host.saving}
              />
            </div>
          </>
        )}

        <button
          className="icon-btn result-remove"
          title="Remove this result"
          aria-label="Remove this result"
          onClick={vlens.cachePartial(onRemoveResultRow, host, index)}
          disabled={host.saving}
        >
          🗑️
        </button>
      </div>

      <div className="form-row">
        {row.kind === ResultKindPlacement && (
          <div className="form-group flex-2">
            <label htmlFor={`resultCategory${index}`}>Category</label>
            <input
              id={`resultCategory${index}`}
              type="text"
              list={`resultCategoryOptions${index}`}
              placeholder="Teen Small Group Jazz"
              value={row.category}
              onInput={e => {
                row.category = e.currentTarget.value;
                vlens.scheduleRedraw();
              }}
              disabled={host.saving}
            />
            <Suggestions id={`resultCategoryOptions${index}`} values={vocabulary.categories} />
          </div>
        )}

        {roster.length > 0 && (
          <div className="form-group flex-1">
            <label htmlFor={`resultPerson${index}`}>For</label>
            <select
              id={`resultPerson${index}`}
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
          </div>
        )}

        <div className="form-group flex-2">
          <label htmlFor={`resultNotes${index}`}>Notes</label>
          <input
            id={`resultNotes${index}`}
            type="text"
            value={row.notes}
            onInput={e => {
              row.notes = e.currentTarget.value;
              vlens.scheduleRedraw();
            }}
            disabled={host.saving}
          />
        </div>
      </div>

      {error && <p className="result-row-error">{error}</p>}
    </div>
  );
};
