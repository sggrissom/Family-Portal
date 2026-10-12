import {
  acknowledgeInterviewRequest,
  callInterview,
  interviewRequestId,
} from "../../lib/interviewSave";
import * as preact from "preact";
import * as vlens from "vlens";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView } from "../../lib/authHelpers";
import { canContribute } from "../../lib/authCache";
import { localDateString } from "../../lib/when";
import { copy } from "../../lib/copy";
import {
  ImportDraft,
  ageYearsAt,
  duplicateRefs,
  firstName,
  importDrafts,
  importRequest,
  interviewTitle,
} from "../../lib/interviews";
import "./interview-styles";

const t = copy.interview.importer;
const IGNORE = "ignore";
const NEW = "new";

export async function fetch(route: string, prefix: string) {
  return server.GetFamilyInterviews({ today: localDateString(new Date()) });
}

type ImportState = {
  personId: number;
  text: string;
  csv: boolean;
  fileName: string;
  fallbackDate: string;
  step: "paste" | "review" | "done";
  busy: boolean;
  error: string;
  questions: server.InterviewQuestion[];
  drafts: ImportDraft[];
  saved: server.Interview[];
};

const useImport = vlens.declareHook(
  (): ImportState => ({
    personId: 0,
    text: "",
    csv: false,
    fileName: "",
    fallbackDate: "",
    step: "paste",
    busy: false,
    error: "",
    questions: [],
    drafts: [],
    saved: [],
  })
);

export function view(
  route: string,
  prefix: string,
  data: server.GetFamilyInterviewsResponse
): preact.ComponentChild {
  if (!requireAuthInView()) return;
  const state = useImport();
  const people = (data.people ?? []).map(r => r.person).filter(p => canContribute(p.familyId));
  if (!state.personId) {
    const asked = Number(new URLSearchParams(window.location.search).get("person"));
    state.personId = people.find(p => p.id === asked)?.id ?? people[0]?.id ?? 0;
  }
  const person = people.find(p => p.id === state.personId);

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="iv-page iv-import">
        <h1>{t.title}</h1>
        {state.error && (
          <div className="error-message" role="alert">
            {state.error}
          </div>
        )}
        {state.step === "paste" && <PasteStep state={state} people={people} />}
        {state.step === "review" && person && <ReviewStep state={state} person={person} />}
        {state.step === "done" && person && <DoneStep state={state} person={person} />}
      </main>
      <Footer />
    </div>
  );
}

const PasteStep = ({ state, people }: { state: ImportState; people: server.Person[] }) => (
  <>
    <p className="iv-hint">{t.intro}</p>
    <ul className="iv-hint iv-formats">
      <li>{t.formatLines}</li>
      <li>{t.formatAlternating}</li>
      <li>{t.formatTable}</li>
    </ul>

    <div className="iv-field">
      <label htmlFor="importPerson">{t.who}</label>
      <select
        id="importPerson"
        className="iv-select"
        value={String(state.personId)}
        onChange={e => {
          state.personId = Number(e.currentTarget.value);
          vlens.scheduleRedraw();
        }}
      >
        {people.map(p => (
          <option key={p.id} value={String(p.id)}>
            {p.name}
          </option>
        ))}
      </select>
    </div>

    <div
      className="iv-drop"
      onDragOver={e => e.preventDefault()}
      onDrop={e => {
        e.preventDefault();
        const file = e.dataTransfer?.files?.[0];
        if (file) void readFile(state, file);
      }}
    >
      <textarea
        className="iv-paste"
        aria-label={t.pasteLabel}
        placeholder={t.pastePlaceholder}
        rows={12}
        value={state.text}
        onInput={e => {
          state.text = e.currentTarget.value;
          state.csv = false;
          state.fileName = "";
          vlens.scheduleRedraw();
        }}
      />
      <label className="iv-link">
        {state.fileName ? t.fileLoaded(state.fileName) : t.chooseFile}
        <input
          type="file"
          accept=".csv,.tsv,.txt,text/csv,text/plain"
          hidden
          onChange={e => {
            const file = e.currentTarget.files?.[0];
            if (file) void readFile(state, file);
          }}
        />
      </label>
    </div>

    <div className="iv-field">
      <label htmlFor="fallbackDate">{t.fallbackDate}</label>
      <input
        id="fallbackDate"
        type="date"
        value={state.fallbackDate}
        max={localDateString(new Date())}
        onInput={e => (state.fallbackDate = e.currentTarget.value)}
      />
      <span className="iv-hint">{t.fallbackHint}</span>
    </div>

    <div className="iv-actions">
      <button
        type="button"
        className="btn btn-primary"
        disabled={state.busy || !state.text.trim() || !state.personId}
        onClick={vlens.cachePartial(preview, state)}
      >
        {state.busy ? t.reading : t.preview}
      </button>
    </div>
  </>
);

const ReviewStep = ({ state, person }: { state: ImportState; person: server.Person }) => {
  const sections: string[] = [];
  for (const q of state.questions) if (!sections.includes(q.section)) sections.push(q.section);
  const active = state.drafts.filter(d => !d.skip);
  return (
    <>
      <p className="iv-hint">{t.reviewIntro(state.drafts.length, firstName(person.name))}</p>
      {state.drafts.map((draft, i) => {
        const dups = duplicateRefs(draft);
        const age = draft.date ? ageYearsAt(person.birthday, draft.date) : null;
        return (
          <section key={i} className={draft.skip ? "iv-review skipped" : "iv-review"}>
            <header className="iv-review-head">
              <h2>{draft.header || t.interviewN(i + 1)}</h2>
              <label>
                {t.date}
                {!draft.date && <span className="iv-hint">{t.knownDateHint}</span>}
                <input
                  type="date"
                  value={draft.date}
                  max={localDateString(new Date())}
                  onInput={e => {
                    draft.date = e.currentTarget.value;
                    vlens.scheduleRedraw();
                  }}
                />
              </label>
              {age !== null && <span className="iv-hint">{t.age(age)}</span>}
              <label className="iv-toggle">
                <input
                  type="checkbox"
                  checked={!draft.skip}
                  onChange={e => {
                    draft.skip = !e.currentTarget.checked;
                    vlens.scheduleRedraw();
                  }}
                />
                {t.include}
              </label>
            </header>
            {!draft.skip && (
              <div className="iv-review-rows">
                {draft.rows.map((row, j) => (
                  <div
                    key={j}
                    className={row.choice === IGNORE ? "iv-review-row ignored" : "iv-review-row"}
                  >
                    <span className="iv-review-raw">{row.raw}</span>
                    <input
                      type="text"
                      aria-label={t.answerFor(row.raw)}
                      value={row.answer}
                      onInput={e => {
                        row.answer = e.currentTarget.value;
                      }}
                    />
                    <span className="iv-review-match">
                      <select
                        aria-label={t.matchFor(row.raw)}
                        value={row.choice}
                        onChange={e => {
                          row.choice = e.currentTarget.value;
                          row.confidence = "exact";
                          vlens.scheduleRedraw();
                        }}
                      >
                        <option value="" disabled>
                          {t.chooseQuestion}
                        </option>
                        <option value={NEW}>{t.newQuestion}</option>
                        <option value={IGNORE}>{t.ignore}</option>
                        {sections.map(section => (
                          <optgroup key={section} label={section}>
                            {state.questions
                              .filter(q => q.section === section)
                              .map(q => {
                                const ref = q.id ? `id:${q.id}` : `key:${q.defaultKey}`;
                                return (
                                  <option key={ref} value={ref}>
                                    {q.text}
                                  </option>
                                );
                              })}
                          </optgroup>
                        ))}
                      </select>
                      {(row.confidence === "close" || row.confidence === "guess") && (
                        <span className="iv-badge" title={t.uncertainHint}>
                          {t.uncertain}
                        </span>
                      )}
                      {dups.has(row.choice) && <span className="iv-badge">{t.duplicate}</span>}
                      {row.choice === NEW && (
                        <label className="iv-toggle">
                          <input
                            type="checkbox"
                            checked={row.keep}
                            onChange={e => {
                              row.keep = e.currentTarget.checked;
                            }}
                          />
                          {t.addToList}
                        </label>
                      )}
                    </span>
                  </div>
                ))}
              </div>
            )}
          </section>
        );
      })}
      <div className="iv-actions">
        <button
          type="button"
          className="btn btn-secondary"
          onClick={() => {
            state.step = "paste";
            vlens.scheduleRedraw();
          }}
        >
          {t.back}
        </button>
        <button
          type="button"
          className="btn btn-primary"
          disabled={state.busy || active.length === 0}
          onClick={vlens.cachePartial(runImport, state)}
        >
          {state.busy ? t.importing : t.importN(active.length)}
        </button>
      </div>
    </>
  );
};

const DoneStep = ({ state, person }: { state: ImportState; person: server.Person }) => (
  <>
    <p>{t.done(state.saved.length, firstName(person.name))}</p>
    <ul className="iv-done-list">
      {state.saved.map(i => (
        <li key={i.id}>
          <a href={`/interview/${i.id}`}>{interviewTitle(i, person)}</a>
        </li>
      ))}
    </ul>
    <div className="iv-actions">
      <button
        type="button"
        className="btn btn-secondary"
        onClick={() => {
          state.step = "paste";
          state.text = "";
          state.fileName = "";
          vlens.scheduleRedraw();
        }}
      >
        {t.another}
      </button>
      <a className="btn btn-primary" href={`/profile/${person.id}?tab=interviews`}>
        {t.seeAll}
      </a>
    </div>
  </>
);

async function readFile(state: ImportState, file: File) {
  state.text = await file.text();
  state.csv = /\.csv$/i.test(file.name) || file.type === "text/csv";
  state.fileName = file.name;
  vlens.scheduleRedraw();
}

async function preview(state: ImportState) {
  state.busy = true;
  state.error = "";
  vlens.scheduleRedraw();
  const [resp, err] = await callInterview(
    server.PreviewInterviewImport({
      personId: state.personId,
      text: state.text,
      csv: state.csv,
    })
  );
  state.busy = false;
  if (!resp) {
    state.error = err || t.readFailed;
    vlens.scheduleRedraw();
    return;
  }
  state.questions = resp.questions ?? [];
  state.drafts = importDrafts(resp.proposals ?? [], state.fallbackDate);
  state.step = "review";
  vlens.scheduleRedraw();
}

async function runImport(state: ImportState) {
  const missing = state.drafts.findIndex(d => !d.skip && !d.date);
  if (missing >= 0) {
    state.error = t.needsDate(missing + 1);
    vlens.scheduleRedraw();
    return;
  }
  if (state.drafts.some(d => !d.skip && d.rows.some(r => r.answer.trim() && !r.choice))) {
    state.error = t.needsMatch;
    vlens.scheduleRedraw();
    return;
  }
  const dup = state.drafts.findIndex(d => !d.skip && duplicateRefs(d).size > 0);
  if (dup >= 0) {
    state.error = t.hasDuplicates(dup + 1);
    vlens.scheduleRedraw();
    return;
  }
  state.busy = true;
  state.error = "";
  vlens.scheduleRedraw();
  const [resp, err] = await callInterview(
    server.ImportInterviews({
      ...importRequest(state.personId, state.drafts),
      requestId: interviewRequestId(
        `import:${state.personId}`,
        importRequest(state.personId, state.drafts)
      ),
    })
  );
  state.busy = false;
  if (!resp) {
    state.error = err || t.importFailed;
    vlens.scheduleRedraw();
    return;
  }
  acknowledgeInterviewRequest(`import:${state.personId}`);
  state.saved = resp.interviews ?? [];
  state.step = "done";
  vlens.scheduleRedraw();
}
