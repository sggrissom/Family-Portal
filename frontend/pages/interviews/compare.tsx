import * as preact from "preact";
import * as vlens from "vlens";
import { navigate } from "../../lib/navigate";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView } from "../../lib/authHelpers";
import { copy } from "../../lib/copy";
import { compareByAge, firstName, normalizeAnswer } from "../../lib/interviews";
import "./interview-styles";

const t = copy.interview.compare;

function params(route: string) {
  const query = new URLSearchParams(route.split("?")[1] ?? "");
  return {
    questionId: Number(query.get("question")) || 0,
    age: query.has("age") ? Number(query.get("age")) : null,
  };
}

export async function fetch(route: string, prefix: string) {
  return server.CompareInterviewAnswers({ questionId: params(route).questionId, personIds: [] });
}

type CompareState = { source: server.CompareInterviewAnswersResponse | null; hidden: Set<number> };

const useCompare = vlens.declareHook((): CompareState => ({ source: null, hidden: new Set() }));

export function view(
  route: string,
  prefix: string,
  data: server.CompareInterviewAnswersResponse
): preact.ComponentChild {
  if (!requireAuthInView()) return;
  const state = useCompare();
  if (state.source !== data) {
    state.source = data;
    state.hidden = new Set();
  }
  const { age } = params(route);
  const rows = (data.rows ?? []).filter(r => !state.hidden.has(r.person.id));
  const byAge = compareByAge(rows);
  const sections: string[] = [];
  for (const q of data.questions ?? []) {
    if (!q.oneOff && !sections.includes(q.section)) sections.push(q.section);
  }

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="iv-page">
        <h1>{t.title}</h1>
        <p className="iv-hint">{t.intro}</p>

        {(data.questions ?? []).length === 0 ? (
          <p className="iv-hint">{t.empty}</p>
        ) : (
          <>
            <div className="iv-field">
              <label htmlFor="compareQuestion">{t.question}</label>
              <select
                id="compareQuestion"
                className="iv-select"
                value={String(data.question.id)}
                onChange={e => {
                  const next = new URLSearchParams(route.split("?")[1] ?? "");
                  next.set("question", e.currentTarget.value);
                  navigate(`/interview/compare?${next.toString()}`);
                }}
              >
                {sections.map(section => (
                  <optgroup key={section} label={section}>
                    {data.questions
                      .filter(q => !q.oneOff && q.section === section)
                      .map(q => (
                        <option key={q.id} value={String(q.id)}>
                          {q.text}
                        </option>
                      ))}
                  </optgroup>
                ))}
              </select>
            </div>

            <div className="iv-compare-people" role="group" aria-label={t.who}>
              {(data.rows ?? []).map(r => (
                <label key={r.person.id} className="iv-toggle">
                  <input
                    type="checkbox"
                    checked={!state.hidden.has(r.person.id)}
                    onChange={e => {
                      if (e.currentTarget.checked) state.hidden.delete(r.person.id);
                      else state.hidden.add(r.person.id);
                      vlens.scheduleRedraw();
                    }}
                  />
                  {firstName(r.person.name)}
                </label>
              ))}
            </div>

            {byAge.length === 0 ? (
              <p className="iv-hint">{t.noAnswers}</p>
            ) : (
              <ol className="iv-compare">
                {byAge.map(group => {
                  const distinct = new Set(group.answers.map(a => normalizeAnswer(a.text))).size;
                  return (
                    <li
                      key={group.age}
                      className={group.age === age ? "iv-compare-age current" : "iv-compare-age"}
                    >
                      <h2>{t.atAge(group.age)}</h2>
                      <ul>
                        {group.answers.map(a => (
                          <li key={a.milestoneId}>
                            <strong>{firstName(a.person.name)}</strong>{" "}
                            <a href={`/milestone/${a.milestoneId}`}>{a.text}</a>
                            <small className="iv-hint">
                              {" "}
                              · {new Date(a.date).getUTCFullYear()}
                            </small>
                          </li>
                        ))}
                      </ul>
                      {group.answers.length > 1 && distinct === 1 && (
                        <p className="iv-hint">{t.allSame}</p>
                      )}
                    </li>
                  );
                })}
              </ol>
            )}
          </>
        )}
      </main>
      <Footer />
    </div>
  );
}
