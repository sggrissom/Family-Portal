import * as preact from "preact";
import * as core from "vlens/core";
import * as server from "../../server";
import { copy } from "../../lib/copy";
import { canContribute } from "../../lib/authCache";
import { formatLongDate } from "../../lib/dateUtils";
import { ResponsiveImage } from "../../components/ResponsiveImage";
import {
  ageYearsAt,
  answersByQuestion,
  interviewTitle,
  questionStatus,
} from "../../lib/interviews";

const t = copy.interview.view;

export function InterviewPage({
  data,
}: {
  data: server.GetInterviewResponse;
}): preact.ComponentChild {
  const { interview, person, questions } = data;
  const answers = answersByQuestion(data.answers ?? []);
  const editable = canContribute(interview.familyId);
  const sections: string[] = [];
  for (const q of questions) if (!sections.includes(q.section)) sections.push(q.section);
  const age = ageYearsAt(person.birthday, interview.date);

  return (
    <article className="iv-view">
      <header className="iv-view-head">
        {interview.photoId > 0 && (
          <ResponsiveImage
            photoId={interview.photoId}
            alt={t.portraitOf(person.name)}
            className="iv-portrait"
            sizes="160px"
          />
        )}
        <div>
          <h1>{interviewTitle(interview, person)}</h1>
          <p className="iv-hint">
            {formatLongDate(interview.date)} · {t.age(age)}
            {interview.status === "in-progress" && ` · ${t.inProgress}`}
          </p>
          <p>
            <a className="iv-link" href={`/profile/${person.id}`}>
              {t.backTo(person.name)}
            </a>
          </p>
        </div>
      </header>

      {interview.notes && <p className="iv-notes">{interview.notes}</p>}

      {editable && interview.status === "in-progress" && (
        <a className="btn btn-primary" href={`/interview/${interview.id}/ask`}>
          {t.continue}
        </a>
      )}

      {sections.map(section => {
        const qs = questions.filter(
          q => answers.has(q.id) || questionStatus(interview, answers, q.id) === "skipped"
        );
        const inSection = qs.filter(q => q.section === section);
        if (inSection.length === 0) return null;
        return (
          <section key={section} className="iv-view-section">
            <h2>{section}</h2>
            <dl>
              {inSection.map(q => {
                const a = answers.get(q.id);
                return (
                  <div key={q.id} className="iv-qa">
                    <dt>{a?.context || q.text}</dt>
                    <dd>
                      {a ? (
                        <a href={`/milestone/${a.id}`}>{a.description}</a>
                      ) : (
                        <span className="iv-hint">{t.skipped}</span>
                      )}
                    </dd>
                  </div>
                );
              })}
            </dl>
          </section>
        );
      })}

      {editable && (
        <div className="iv-actions">
          <button type="button" className="btn btn-secondary" onClick={() => onDelete(interview)}>
            {t.delete}
          </button>
        </div>
      )}
    </article>
  );
}

async function onDelete(interview: server.Interview) {
  if (!confirm(t.confirmDelete)) return;
  const [resp, err] = await server.DeleteInterview({ id: interview.id });
  if (!resp) {
    alert(err || t.deleteFailed);
    return;
  }
  core.setRoute(`/profile/${resp.personId}`);
}
