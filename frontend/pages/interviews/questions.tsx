import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView, ensureAuthInFetch } from "../../lib/authHelpers";
import { canContribute } from "../../lib/authCache";
import { FamilySelect } from "../../components/FamilySelect";
import { ViewOnlyPage } from "../../components/ErrorPage";
import { copy } from "../../lib/copy";
import {
  QuestionDraft,
  QuestionRow,
  activeIn,
  addRow,
  addSection,
  draftFrom,
  moveBefore,
  moveStep,
  moveToSection,
  questionsForSave,
  renameSection,
  resetToDefaults,
  retired,
} from "../../lib/interviewQuestions";
import "./questions-styles";

const t = copy.interview.questions;

export async function fetch(route: string, prefix: string) {
  if (!(await ensureAuthInFetch())) {
    return rpc.ok<server.InterviewQuestionsResponse>({
      familyId: 0,
      revision: 0,
      questions: [],
      suggested: [],
      defaults: [],
      usage: {},
    });
  }
  return server.GetInterviewQuestions({ familyId: 0 });
}

type EditorState = {
  loaded: server.InterviewQuestionsResponse | null;
  draft: QuestionDraft;
  deleteIds: number[];
  dirty: boolean;
  saving: boolean;
  error: string;
  newText: Record<string, string>;
  newSection: string;
  dragKey: number;
  showSuggested: boolean;
};

const useEditor = vlens.declareHook(
  (): EditorState => ({
    loaded: null,
    draft: draftFrom([]),
    deleteIds: [],
    dirty: false,
    saving: false,
    error: "",
    newText: {},
    newSection: "",
    dragKey: 0,
    showSuggested: false,
  })
);

function load(state: EditorState, resp: server.InterviewQuestionsResponse) {
  state.loaded = resp;
  state.draft = draftFrom(resp.questions ?? []);
  state.deleteIds = [];
  state.dirty = false;
  state.error = "";
}

function changed(state: EditorState) {
  state.dirty = true;
  vlens.scheduleRedraw();
}

export function view(
  route: string,
  prefix: string,
  data: server.InterviewQuestionsResponse
): preact.ComponentChild {
  if (!requireAuthInView()) return;
  const state = useEditor();
  if (!state.loaded) load(state, data);
  const loaded = state.loaded!;

  if (!canContribute(loaded.familyId)) {
    return <ViewOnlyPage backLink="/dashboard" />;
  }

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="iq-page">
        <h1>{t.title}</h1>
        <p className="iq-intro">{t.intro}</p>
        <p className="iq-intro">{t.placeholders}</p>

        <FamilySelect
          id="questionFamily"
          value={loaded.familyId}
          onChange={vlens.cachePartial(switchFamily, state)}
          disabled={state.saving}
        />

        {state.error && (
          <div className="error-message" role="alert">
            {state.error}
          </div>
        )}

        <Suggested state={state} />

        {state.draft.sections.map(section => (
          <Section key={section} state={state} section={section} />
        ))}

        <div className="iq-add-section">
          <input
            type="text"
            aria-label={t.addSection}
            placeholder={t.sectionPlaceholder}
            value={state.newSection}
            onInput={e => {
              state.newSection = e.currentTarget.value;
              vlens.scheduleRedraw();
            }}
          />
          <button
            type="button"
            className="btn btn-secondary"
            disabled={!state.newSection.trim()}
            onClick={vlens.cachePartial(onAddSection, state)}
          >
            {t.addSection}
          </button>
        </div>

        <Retired state={state} />

        <div className="iq-footer">
          <button
            type="button"
            className="btn btn-secondary"
            onClick={vlens.cachePartial(onReset, state)}
          >
            {t.reset}
          </button>
          <span className="iq-status">{state.dirty ? t.unsaved : t.saved}</span>
          <button
            type="button"
            className="btn btn-primary"
            disabled={!state.dirty || state.saving}
            onClick={vlens.cachePartial(onSave, state)}
          >
            {state.saving ? t.saving : t.save}
          </button>
        </div>
      </main>
      <Footer />
    </div>
  );
}

const Suggested = ({ state }: { state: EditorState }) => {
  const have = new Set(state.draft.rows.map(r => r.defaultKey));
  const suggested = (state.loaded?.suggested ?? []).filter(s => !have.has(s.defaultKey));
  if (suggested.length === 0) return null;
  return (
    <div className="iq-suggested">
      <button
        type="button"
        className="iq-suggested-toggle"
        aria-expanded={state.showSuggested ? "true" : "false"}
        onClick={() => {
          state.showSuggested = !state.showSuggested;
          vlens.scheduleRedraw();
        }}
      >
        {t.suggested(suggested.length)}
      </button>
      {state.showSuggested && (
        <ul>
          {suggested.map(s => (
            <li key={s.defaultKey}>
              <span>{s.text}</span>
              <button
                type="button"
                className="btn btn-secondary btn-small"
                onClick={vlens.cachePartial(adopt, state, [s])}
              >
                {t.add}
              </button>
            </li>
          ))}
        </ul>
      )}
      <button
        type="button"
        className="btn btn-secondary btn-small"
        onClick={vlens.cachePartial(adopt, state, suggested)}
      >
        {t.addAll}
      </button>
    </div>
  );
};

const Section = ({ state, section }: { state: EditorState; section: string }) => {
  const rows = activeIn(state.draft, section);
  return (
    <section
      className="iq-section"
      onDragOver={e => state.dragKey && e.preventDefault()}
      onDrop={vlens.cachePartial(dropOnSection, state, section)}
    >
      <input
        type="text"
        className="iq-section-name"
        aria-label={t.renameSection(section)}
        defaultValue={section}
        onChange={e => {
          renameSection(state.draft, section, e.currentTarget.value);
          changed(state);
        }}
      />
      {rows.length === 0 && <p className="iq-empty">{t.emptySection}</p>}
      <ol className="iq-list">
        {rows.map(row => (
          <Question key={row.key} state={state} row={row} />
        ))}
      </ol>
      <div className="iq-add">
        <input
          type="text"
          aria-label={`${t.addQuestion}: ${section}`}
          placeholder={t.addQuestionPlaceholder}
          value={state.newText[section] ?? ""}
          onInput={e => {
            state.newText[section] = e.currentTarget.value;
            vlens.scheduleRedraw();
          }}
          onKeyDown={e => {
            if (e.key === "Enter") {
              e.preventDefault();
              onAddQuestion(state, section);
            }
          }}
        />
        <button
          type="button"
          className="btn btn-secondary"
          disabled={!(state.newText[section] ?? "").trim()}
          onClick={vlens.cachePartial(onAddQuestion, state, section)}
        >
          {t.addQuestion}
        </button>
      </div>
    </section>
  );
};

const Question = ({ state, row }: { state: EditorState; row: QuestionRow }) => {
  const usage = state.loaded?.usage?.[row.id];
  const used = !!usage && (usage.interviews > 0 || usage.answers > 0);
  const mergeTargets = state.draft.rows.filter(r => r.id > 0 && r.id !== row.id && !r.archived);
  return (
    <li
      className={state.dragKey === row.key ? "iq-question dragging" : "iq-question"}
      draggable
      onDragStart={e => {
        state.dragKey = row.key;
        e.dataTransfer?.setData("text/plain", String(row.key));
        vlens.scheduleRedraw();
      }}
      onDragEnd={() => {
        state.dragKey = 0;
        vlens.scheduleRedraw();
      }}
      onDragOver={e => state.dragKey && e.preventDefault()}
      onDrop={vlens.cachePartial(dropOnQuestion, state, row.key)}
    >
      <span className="iq-handle" aria-hidden="true" title={t.dragHandle}>
        ⠿
      </span>
      <div className="iq-question-body">
        <input
          type="text"
          className="iq-text"
          aria-label={t.questionText}
          value={row.text}
          onInput={e => {
            row.text = e.currentTarget.value;
            changed(state);
          }}
        />
        <div className="iq-meta">
          <span>{t.ageRange(row.minAgeYears, row.maxAgeYears)}</span>
          <span>{t.asked(usage?.interviews ?? 0)}</span>
        </div>
        <details className="iq-more">
          <summary>{t.edit}</summary>
          <div className="iq-more-body">
            <label className="iq-age">
              {t.ages}
              <input
                type="number"
                min={0}
                max={120}
                aria-label={t.minAge}
                placeholder="–"
                value={row.minAgeYears || ""}
                onInput={e => {
                  row.minAgeYears = Number(e.currentTarget.value) || 0;
                  changed(state);
                }}
              />
              –
              <input
                type="number"
                min={0}
                max={120}
                aria-label={t.maxAge}
                placeholder="–"
                value={row.maxAgeYears || ""}
                onInput={e => {
                  row.maxAgeYears = Number(e.currentTarget.value) || 0;
                  changed(state);
                }}
              />
            </label>
            {state.draft.sections.length > 1 && (
              <select
                aria-label={t.moveTo}
                value=""
                onChange={e => {
                  moveToSection(state.draft, row.key, e.currentTarget.value);
                  changed(state);
                }}
              >
                <option value="">{t.moveTo}</option>
                {state.draft.sections
                  .filter(s => s !== row.section)
                  .map(s => (
                    <option key={s} value={s}>
                      {s}
                    </option>
                  ))}
              </select>
            )}
            {row.id > 0 && mergeTargets.length > 0 && (
              <select
                aria-label={t.mergeInto}
                title={state.dirty ? t.mergeHint : undefined}
                disabled={state.dirty || state.saving}
                value=""
                onChange={e => onMerge(state, row, Number(e.currentTarget.value))}
              >
                <option value="">{state.dirty ? t.mergeHint : t.mergeInto}</option>
                {mergeTargets.map(r => (
                  <option key={r.key} value={String(r.id)}>
                    {r.text}
                  </option>
                ))}
              </select>
            )}
            <button
              type="button"
              className="btn btn-secondary btn-small"
              onClick={() => {
                row.archived = true;
                changed(state);
              }}
            >
              {t.retire}
            </button>
            {!used && (
              <button
                type="button"
                className="btn btn-secondary btn-small"
                onClick={vlens.cachePartial(onDelete, state, row)}
              >
                {t.delete}
              </button>
            )}
          </div>
        </details>
      </div>
      <div className="iq-actions">
        <button
          type="button"
          className="iq-icon"
          aria-label={`${t.moveUp}: ${row.text}`}
          onClick={() => {
            moveStep(state.draft, row.key, -1);
            changed(state);
          }}
        >
          ↑
        </button>
        <button
          type="button"
          className="iq-icon"
          aria-label={`${t.moveDown}: ${row.text}`}
          onClick={() => {
            moveStep(state.draft, row.key, 1);
            changed(state);
          }}
        >
          ↓
        </button>
      </div>
    </li>
  );
};

const Retired = ({ state }: { state: EditorState }) => {
  const rows = retired(state.draft);
  if (rows.length === 0) return null;
  return (
    <details className="iq-retired">
      <summary>{t.retired(rows.length)}</summary>
      <p className="iq-empty">{t.retiredHint}</p>
      <ul>
        {rows.map(row => (
          <li key={row.key}>
            <span>{row.text}</span>
            <span className="iq-usage">
              {t.asked(state.loaded?.usage?.[row.id]?.interviews ?? 0)}
            </span>
            <button
              type="button"
              className="btn btn-secondary btn-small"
              onClick={() => {
                row.archived = false;
                changed(state);
              }}
            >
              {t.restore}
            </button>
          </li>
        ))}
      </ul>
    </details>
  );
};

function dropOnQuestion(state: EditorState, targetKey: number, e: DragEvent) {
  e.preventDefault();
  e.stopPropagation();
  if (!state.dragKey) return;
  moveBefore(state.draft, state.dragKey, targetKey);
  state.dragKey = 0;
  changed(state);
}

function dropOnSection(state: EditorState, section: string, e: DragEvent) {
  e.preventDefault();
  if (!state.dragKey) return;
  moveToSection(state.draft, state.dragKey, section);
  state.dragKey = 0;
  changed(state);
}

function adopt(state: EditorState, questions: server.InterviewQuestion[]) {
  for (const q of questions) {
    if (!state.draft.sections.includes(q.section)) state.draft.sections.push(q.section);
    const rows = state.draft.rows;
    const last = rows.map(r => r.section).lastIndexOf(q.section);
    addRow(state.draft, q, last < 0 ? rows.length : last + 1);
  }
  changed(state);
}

function onAddQuestion(state: EditorState, section: string) {
  const text = (state.newText[section] ?? "").trim();
  if (!text) return;
  const rows = state.draft.rows;
  const last = rows.map(r => r.section).lastIndexOf(section);
  addRow(state.draft, { text, section }, last < 0 ? rows.length : last + 1);
  state.newText[section] = "";
  changed(state);
}

function onAddSection(state: EditorState) {
  if (addSection(state.draft, state.newSection)) {
    state.newSection = "";
    changed(state);
  }
}

function onDelete(state: EditorState, row: QuestionRow) {
  state.draft.rows = state.draft.rows.filter(r => r.key !== row.key);
  if (row.id > 0) state.deleteIds.push(row.id);
  changed(state);
}

function onReset(state: EditorState) {
  if (!confirm(t.confirmReset)) return;
  resetToDefaults(state.draft, state.loaded?.defaults ?? []);
  changed(state);
}

async function switchFamily(state: EditorState, familyId: number) {
  const [resp, err] = await server.GetInterviewQuestions({ familyId });
  if (!resp) {
    state.error = err || "That list could not be loaded.";
  } else {
    load(state, resp);
  }
  vlens.scheduleRedraw();
}

async function onSave(state: EditorState) {
  const loaded = state.loaded!;
  state.saving = true;
  state.error = "";
  vlens.scheduleRedraw();
  const [resp, err] = await server.SaveInterviewQuestions({
    familyId: loaded.familyId,
    revision: loaded.revision,
    questions: questionsForSave(state.draft),
    deleteIds: state.deleteIds,
  });
  state.saving = false;
  if (!resp) {
    state.error = err || "The questions could not be saved. Please try again.";
  } else {
    load(state, resp);
  }
  vlens.scheduleRedraw();
}

async function onMerge(state: EditorState, from: QuestionRow, intoId: number) {
  const into = state.draft.rows.find(r => r.id === intoId);
  if (!into || !confirm(t.confirmMerge(from.text, into.text))) {
    vlens.scheduleRedraw();
    return;
  }
  state.saving = true;
  vlens.scheduleRedraw();
  const [resp, err] = await server.MergeInterviewQuestions({
    fromId: from.id,
    intoId,
    revision: state.loaded!.revision,
  });
  state.saving = false;
  if (!resp) {
    state.error = err || "Those questions could not be merged.";
  } else {
    load(state, resp);
  }
  vlens.scheduleRedraw();
}
