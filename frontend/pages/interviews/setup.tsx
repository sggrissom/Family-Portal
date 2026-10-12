import * as preact from "preact";
import * as vlens from "vlens";
import * as core from "vlens/core";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView } from "../../lib/authHelpers";
import { getIdFromRoute } from "../../lib/routeHelpers";
import { localDateString } from "../../lib/when";
import { copy } from "../../lib/copy";
import {
  ageYearsAt,
  firstName,
  questionFitsAge,
  setShowPreviousPreference,
  showPreviousPreference,
} from "../../lib/interviews";
import "./interview-styles";

const t = copy.interview.setup;

export async function fetch(route: string, prefix: string) {
  const personId = getIdFromRoute(route, 3);
  if (!personId) return [null, "Person ID is required"] as [null, string];
  return server.GetInterviewSetup({ personId });
}

type SetupState = {
  date: string;
  checked: Set<string>;
  oneOffs: string[];
  newOneOff: string;
  showPrevious: boolean;
  starting: boolean;
  error: string;
  initialized: boolean;
};

const useSetup = vlens.declareHook(
  (_personId: number): SetupState => ({
    date: "",
    checked: new Set(),
    oneOffs: [],
    newOneOff: "",
    showPrevious: true,
    starting: false,
    error: "",
    initialized: false,
  })
);

const questionKey = (q: server.InterviewQuestion) => (q.id ? `id:${q.id}` : `key:${q.defaultKey}`);

function checkForAge(state: SetupState, data: server.GetInterviewSetupResponse) {
  const age = ageYearsAt(data.person.birthday, state.date);
  state.checked = new Set(data.questions.filter(q => questionFitsAge(q, age)).map(questionKey));
}

export function view(
  route: string,
  prefix: string,
  data: server.GetInterviewSetupResponse
): preact.ComponentChild {
  if (!requireAuthInView()) return;
  const state = useSetup(data.person.id);
  if (!state.initialized) {
    state.initialized = true;
    state.date = localDateString(new Date());
    state.showPrevious = showPreviousPreference();
    checkForAge(state, data);
  }

  const person = data.person;
  const name = firstName(person.name);
  const age = ageYearsAt(person.birthday, state.date);
  const sections: string[] = [];
  for (const q of data.questions) if (!sections.includes(q.section)) sections.push(q.section);
  const count = state.checked.size + state.oneOffs.length;

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="iv-page">
        <h1>{t.title(name)}</h1>

        {data.inProgress.id > 0 && (
          <div className="iv-callout">
            <p>{t.inProgress(name, data.answered, data.inProgress.questionIds.length)}</p>
            <a className="btn btn-primary" href={`/interview/${data.inProgress.id}/ask`}>
              {t.continue}
            </a>
          </div>
        )}

        {state.error && (
          <div className="error-message" role="alert">
            {state.error}
          </div>
        )}

        <div className="iv-field">
          <label htmlFor="interviewDate">{t.date}</label>
          <input
            id="interviewDate"
            type="date"
            value={state.date}
            max={localDateString(new Date())}
            onInput={e => {
              state.date = e.currentTarget.value;
              checkForAge(state, data);
              vlens.scheduleRedraw();
            }}
          />
          {person.birthday && <span className="iv-hint">{t.age(name, age)}</span>}
        </div>

        <label className="iv-toggle">
          <input
            type="checkbox"
            checked={state.showPrevious}
            onChange={e => {
              state.showPrevious = e.currentTarget.checked;
              setShowPreviousPreference(state.showPrevious);
            }}
          />
          {t.showPrevious}
        </label>

        <div className="iv-setup-head">
          <h2>{t.questions(count)}</h2>
          <a className="iv-link" href="/interview-questions">
            {t.editList}
          </a>
        </div>

        {sections.map(section => {
          const qs = data.questions.filter(q => q.section === section);
          const allOn = qs.every(q => state.checked.has(questionKey(q)));
          return (
            <fieldset key={section} className="iv-setup-section">
              <legend>
                {section}
                <button
                  type="button"
                  className="iv-link"
                  onClick={() => {
                    for (const q of qs) {
                      if (allOn) state.checked.delete(questionKey(q));
                      else state.checked.add(questionKey(q));
                    }
                    vlens.scheduleRedraw();
                  }}
                >
                  {allOn ? t.none : t.all}
                </button>
              </legend>
              {qs.map(q => (
                <label key={questionKey(q)} className="iv-check">
                  <input
                    type="checkbox"
                    checked={state.checked.has(questionKey(q))}
                    onChange={e => {
                      if (e.currentTarget.checked) state.checked.add(questionKey(q));
                      else state.checked.delete(questionKey(q));
                      vlens.scheduleRedraw();
                    }}
                  />
                  <span>{q.text}</span>
                  {!questionFitsAge(q, age) && <span className="iv-hint">{t.outsideAges}</span>}
                </label>
              ))}
            </fieldset>
          );
        })}

        <fieldset className="iv-setup-section">
          <legend>{t.justThisTime}</legend>
          {state.oneOffs.map((text, i) => (
            <div key={i} className="iv-check">
              <span>{text}</span>
              <button
                type="button"
                className="iv-link"
                onClick={() => {
                  state.oneOffs.splice(i, 1);
                  vlens.scheduleRedraw();
                }}
              >
                {t.remove}
              </button>
            </div>
          ))}
          <div className="iv-inline">
            <input
              type="text"
              aria-label={t.addOneOff}
              placeholder={t.addOneOff}
              value={state.newOneOff}
              onInput={e => {
                state.newOneOff = e.currentTarget.value;
                vlens.scheduleRedraw();
              }}
              onKeyDown={e => {
                if (e.key === "Enter") {
                  e.preventDefault();
                  addOneOff(state);
                }
              }}
            />
            <button
              type="button"
              className="btn btn-secondary"
              disabled={!state.newOneOff.trim()}
              onClick={vlens.cachePartial(addOneOff, state)}
            >
              {t.add}
            </button>
          </div>
        </fieldset>

        <div className="iv-actions">
          <a className="btn btn-secondary" href={`/profile/${person.id}`}>
            {t.cancel}
          </a>
          <button
            type="button"
            className="btn btn-primary"
            disabled={state.starting || count === 0 || data.inProgress.id > 0}
            onClick={vlens.cachePartial(start, state, data)}
          >
            {state.starting ? t.starting : t.start(count)}
          </button>
        </div>
      </main>
      <Footer />
    </div>
  );
}

function addOneOff(state: SetupState) {
  const text = state.newOneOff.trim();
  if (!text) return;
  state.oneOffs.push(text);
  state.newOneOff = "";
  vlens.scheduleRedraw();
}

async function start(state: SetupState, data: server.GetInterviewSetupResponse) {
  const chosen = data.questions.filter(q => state.checked.has(questionKey(q)));
  state.starting = true;
  state.error = "";
  vlens.scheduleRedraw();
  const [resp, err] = await server.StartInterview({
    personId: data.person.id,
    date: state.date,
    questionIds: chosen.filter(q => q.id).map(q => q.id),
    defaultKeys: chosen.filter(q => !q.id).map(q => q.defaultKey),
    oneOffQuestions: state.oneOffs,
  });
  state.starting = false;
  if (!resp) {
    state.error = err || "The interview could not be started.";
    vlens.scheduleRedraw();
    return;
  }
  core.setRoute(`/interview/${resp.interview.id}/ask`);
}
