import * as preact from "preact";
import * as vlens from "vlens";
import * as core from "vlens/core";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView } from "../../lib/authHelpers";
import { getIdFromRoute } from "../../lib/routeHelpers";
import { When, whenProblem, whenRequest } from "../../lib/when";
import { WhenControl } from "../../components/WhenControl";
import { copy } from "../../lib/copy";
import { firstName, questionRef } from "../../lib/interviews";
import "./interview-styles";

const t = copy.interview.past;

export async function fetch(route: string, prefix: string) {
  const personId = getIdFromRoute(route, 3);
  if (!personId) return [null, "Person ID is required"] as [null, string];
  return server.GetInterviewSetup({ personId });
}

type Extra = { text: string; answer: string; keep: boolean };

type PastState = {
  source: server.GetInterviewSetupResponse | null;
  when: When;
  answers: Record<string, string>;
  extras: Extra[];
  showRetired: boolean;
  title: string;
  notes: string;
  saving: boolean;
  error: string;
};

const usePast = vlens.declareHook(
  (_personId: number): PastState => ({
    source: null,
    when: { mode: "age", date: "", ageYears: "", ageMonths: "" },
    answers: {},
    extras: [],
    showRetired: false,
    title: "",
    notes: "",
    saving: false,
    error: "",
  })
);

export function view(
  route: string,
  prefix: string,
  data: server.GetInterviewSetupResponse
): preact.ComponentChild {
  if (!requireAuthInView()) return;
  const state = usePast(data.person.id);
  if (state.source !== data) {
    state.source = data;
    state.answers = {};
    state.extras = [];
    state.error = "";
  }
  const name = firstName(data.person.name);
  const questions = state.showRetired ? [...data.questions, ...data.retired] : data.questions;
  const sections: string[] = [];
  for (const q of questions) if (!sections.includes(q.section)) sections.push(q.section);
  const filled =
    Object.values(state.answers).filter(a => a.trim()).length +
    state.extras.filter(e => e.text.trim() && e.answer.trim()).length;

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="iv-page">
        <h1>{t.title(name)}</h1>
        <p className="iv-hint">{t.intro}</p>
        <p>
          <a className="iv-link" href={`/interview/import?person=${data.person.id}`}>
            {t.pasteInstead}
          </a>
        </p>

        {state.error && (
          <div className="error-message" role="alert">
            {state.error}
          </div>
        )}

        <div className="iv-field">
          <span className="entry-label">{t.when}</span>
          <WhenControl when={state.when} disabled={state.saving} />
        </div>

        {sections.map(section => (
          <fieldset key={section} className="iv-setup-section">
            <legend>{section}</legend>
            {questions
              .filter(q => q.section === section)
              .map(q => {
                const ref = questionRef(q);
                return (
                  <label key={ref} className="iv-past-row">
                    <span>
                      {q.text}
                      {q.archived && <span className="iv-hint"> · {t.retired}</span>}
                    </span>
                    <input
                      type="text"
                      value={state.answers[ref] ?? ""}
                      onInput={e => {
                        state.answers[ref] = e.currentTarget.value;
                        vlens.scheduleRedraw();
                      }}
                    />
                  </label>
                );
              })}
          </fieldset>
        ))}

        {data.retired.length > 0 && (
          <label className="iv-toggle">
            <input
              type="checkbox"
              checked={state.showRetired}
              onChange={e => {
                state.showRetired = e.currentTarget.checked;
                vlens.scheduleRedraw();
              }}
            />
            {t.showRetired(data.retired.length)}
          </label>
        )}

        <fieldset className="iv-setup-section">
          <legend>{t.notOnList}</legend>
          {state.extras.map((extra, i) => (
            <div key={i} className="iv-past-extra">
              <input
                type="text"
                aria-label={t.question}
                placeholder={t.question}
                value={extra.text}
                onInput={e => {
                  extra.text = e.currentTarget.value;
                  vlens.scheduleRedraw();
                }}
              />
              <input
                type="text"
                aria-label={t.answer}
                placeholder={t.answer}
                value={extra.answer}
                onInput={e => {
                  extra.answer = e.currentTarget.value;
                  vlens.scheduleRedraw();
                }}
              />
              <label className="iv-toggle">
                <input
                  type="checkbox"
                  checked={extra.keep}
                  onChange={e => {
                    extra.keep = e.currentTarget.checked;
                  }}
                />
                {t.addToList}
              </label>
            </div>
          ))}
          <button
            type="button"
            className="iv-link"
            onClick={() => {
              state.extras.push({ text: "", answer: "", keep: true });
              vlens.scheduleRedraw();
            }}
          >
            {t.addQuestion}
          </button>
        </fieldset>

        <details className="iv-more-details">
          <summary>{t.titleAndNotes}</summary>
          <div className="iv-details">
            <label>
              {copy.interview.view.title}
              <input
                type="text"
                value={state.title}
                onInput={e => (state.title = e.currentTarget.value)}
              />
            </label>
            <label>
              {copy.interview.view.notes}
              <textarea
                rows={2}
                value={state.notes}
                onInput={e => (state.notes = e.currentTarget.value)}
              />
            </label>
          </div>
        </details>

        <div className="iv-actions">
          <a className="btn btn-secondary" href={`/profile/${data.person.id}?tab=interviews`}>
            {copy.interview.setup.cancel}
          </a>
          <button
            type="button"
            className="btn btn-primary"
            disabled={state.saving || filled === 0}
            onClick={vlens.cachePartial(save, state, data)}
          >
            {state.saving ? t.saving : t.save(filled)}
          </button>
        </div>
      </main>
      <Footer />
    </div>
  );
}

async function save(state: PastState, data: server.GetInterviewSetupResponse) {
  const problem = whenProblem(state.when);
  if (problem) {
    state.error = problem;
    vlens.scheduleRedraw();
    return;
  }
  const when = whenRequest(state.when, new Date());
  const answers: server.ImportAnswer[] = Object.entries(state.answers)
    .filter(([, a]) => a.trim())
    .map(([ref, answer]) => ({ ref, answer, newText: "", keep: false }));
  for (const e of state.extras) {
    if (e.text.trim() && e.answer.trim()) {
      answers.push({ ref: "", answer: e.answer, newText: e.text, keep: e.keep });
    }
  }
  state.saving = true;
  state.error = "";
  vlens.scheduleRedraw();
  const [resp, err] = await server.SaveHistoricalInterview({
    personId: data.person.id,
    interview: {
      inputType: when.inputType,
      date: when.date ?? "",
      ageYears: when.ageYears ?? 0,
      ageMonths: when.ageMonths ?? 0,
      title: state.title,
      notes: state.notes,
      answers,
    },
  });
  state.saving = false;
  if (!resp) {
    state.error = err || t.saveFailed;
    vlens.scheduleRedraw();
    return;
  }
  state.source = null;
  core.setRoute(`/interview/${resp.interview.id}`);
}
