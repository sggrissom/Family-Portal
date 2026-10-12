import * as preact from "preact";
import * as vlens from "vlens";
import * as core from "vlens/core";
import * as server from "../../server";
import { copy } from "../../lib/copy";
import { canContribute } from "../../lib/authCache";
import { formatLongDate } from "../../lib/dateUtils";
import { localDateString } from "../../lib/when";
import { uploadPhoto } from "../../lib/photoUpload";
import { photoFileProblem } from "../../lib/photoUploadQueue";
import { ResponsiveImage } from "../../components/ResponsiveImage";
import {
  ageYearsAt,
  answersByQuestion,
  changesSincePrevious,
  interviewTitle,
  questionStatus,
} from "../../lib/interviews";

const t = copy.interview.view;

type ViewState = {
  source: server.GetInterviewResponse | null;
  interview: server.Interview;
  answers: Map<number, server.Milestone>;
  editingAnswers: boolean;
  drafts: Record<number, string>;
  savingAnswer: number;
  editingDetails: boolean;
  details: { title: string; date: string; notes: string };
  busy: boolean;
  error: string;
};

const useViewState = vlens.declareHook(
  (_id: number): ViewState => ({
    source: null,
    interview: {} as server.Interview,
    answers: new Map(),
    editingAnswers: false,
    drafts: {},
    savingAnswer: 0,
    editingDetails: false,
    details: { title: "", date: "", notes: "" },
    busy: false,
    error: "",
  })
);

export function InterviewPage({
  data,
}: {
  data: server.GetInterviewResponse;
}): preact.ComponentChild {
  const state = useViewState(data.interview.id);
  if (state.source !== data) {
    state.source = data;
    state.interview = data.interview;
    state.answers = answersByQuestion(data.answers ?? []);
    state.editingAnswers = false;
    state.editingDetails = false;
    state.error = "";
  }
  const { person, questions } = data;
  const interview = state.interview;
  const answers = state.answers;
  const editable = canContribute(interview.familyId);
  const age = ageYearsAt(person.birthday, interview.date);
  const changes = changesSincePrevious(questions, answers, data.previous ?? {});
  const sections: string[] = [];
  for (const q of questions) if (!sections.includes(q.section)) sections.push(q.section);

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
          <p className="iv-no-print">
            <a className="iv-link" href={`/profile/${person.id}?tab=interviews`}>
              {t.backTo(person.name)}
            </a>
          </p>
        </div>
      </header>

      {interview.notes && !state.editingDetails && <p className="iv-notes">{interview.notes}</p>}

      {state.error && (
        <div className="error-message" role="alert">
          {state.error}
        </div>
      )}

      {editable && (
        <div className="iv-toolbar iv-no-print">
          {interview.status === "in-progress" && (
            <a className="btn btn-primary btn-small" href={`/interview/${interview.id}/ask`}>
              {t.continue}
            </a>
          )}
          <button
            type="button"
            className="btn btn-secondary btn-small"
            aria-pressed={state.editingAnswers ? "true" : "false"}
            onClick={() => {
              state.editingAnswers = !state.editingAnswers;
              state.drafts = {};
              vlens.scheduleRedraw();
            }}
          >
            {state.editingAnswers ? t.doneEditing : t.editAnswers}
          </button>
          <button
            type="button"
            className="btn btn-secondary btn-small"
            onClick={() => {
              state.editingDetails = !state.editingDetails;
              state.details = {
                title: interview.title,
                date: interview.date.slice(0, 10),
                notes: interview.notes,
              };
              vlens.scheduleRedraw();
            }}
          >
            {t.editDetails}
          </button>
          <button
            type="button"
            className="btn btn-secondary btn-small"
            onClick={() => window.print()}
          >
            {t.print}
          </button>
        </div>
      )}

      {state.editingDetails && <DetailsForm state={state} person={person} />}

      {changes.length > 0 && (
        <section className="iv-changes">
          <h2>{t.whatChanged}</h2>
          <ul>
            {changes.map(c => (
              <li key={c.question.id}>
                <span className="iv-changes-q">
                  {answers.get(c.question.id)?.context || c.question.text}
                </span>
                <span className="iv-changes-a">
                  <s>{c.before}</s> → <strong>{c.after}</strong>
                </span>
              </li>
            ))}
          </ul>
        </section>
      )}

      {sections.map(section => {
        const inSection = questions.filter(
          q =>
            q.section === section &&
            (state.editingAnswers || questionStatus(interview, answers, q.id) !== "pending")
        );
        if (inSection.length === 0) return null;
        return (
          <section key={section} className="iv-view-section">
            <h2>{section}</h2>
            <dl>
              {inSection.map(q => (
                <QA key={q.id} state={state} q={q} />
              ))}
            </dl>
          </section>
        );
      })}

      {editable && (
        <div className="iv-actions iv-no-print">
          <button
            type="button"
            className="btn btn-secondary"
            disabled={state.busy}
            onClick={vlens.cachePartial(onDelete, state)}
          >
            {t.delete}
          </button>
        </div>
      )}
    </article>
  );
}

const QA = ({ state, q }: { state: ViewState; q: server.InterviewQuestion }) => {
  const a = state.answers.get(q.id);
  const label = a?.context || q.text;
  if (state.editingAnswers) {
    const value = state.drafts[q.id] ?? a?.description ?? "";
    return (
      <div className="iv-qa">
        <dt>
          <label htmlFor={`answer-${q.id}`}>{label}</label>
        </dt>
        <dd>
          <input
            id={`answer-${q.id}`}
            type="text"
            value={value}
            disabled={state.savingAnswer === q.id}
            onInput={e => {
              state.drafts[q.id] = e.currentTarget.value;
            }}
            onBlur={vlens.cachePartial(saveAnswer, state, q.id)}
            onKeyDown={e => {
              if (e.key === "Enter") e.currentTarget.blur();
            }}
          />
        </dd>
      </div>
    );
  }
  return (
    <div className="iv-qa">
      <dt>{label}</dt>
      <dd>
        {a ? (
          <a href={`/milestone/${a.id}`}>{a.description}</a>
        ) : (
          <span className="iv-hint">{t.skipped}</span>
        )}
      </dd>
    </div>
  );
};

const DetailsForm = ({ state, person }: { state: ViewState; person: server.Person }) => {
  const d = state.details;
  const interview = state.interview;
  return (
    <form
      className="iv-details iv-no-print"
      onSubmit={e => {
        e.preventDefault();
        void saveDetails(state, {});
      }}
    >
      <label>
        {t.title}
        <input
          type="text"
          placeholder={t.titlePlaceholder}
          value={d.title}
          onInput={e => (d.title = e.currentTarget.value)}
        />
      </label>
      <label>
        {t.date}
        <input
          type="date"
          value={d.date}
          max={localDateString(new Date())}
          onInput={e => (d.date = e.currentTarget.value)}
        />
      </label>
      <label>
        {t.notes}
        <textarea
          rows={2}
          placeholder={t.notesPlaceholder}
          value={d.notes}
          onInput={e => (d.notes = e.currentTarget.value)}
        />
      </label>
      <div className="iv-inline">
        <label className="btn btn-secondary btn-small">
          {interview.photoId ? t.changePortrait : t.addPortrait}
          <input
            type="file"
            accept="image/*"
            hidden
            disabled={state.busy}
            onChange={vlens.cachePartial(onPortrait, state, person)}
          />
        </label>
        {interview.photoId > 0 && (
          <button
            type="button"
            className="btn btn-secondary btn-small"
            disabled={state.busy}
            onClick={() => saveDetails(state, { photoId: 0 })}
          >
            {t.removePortrait}
          </button>
        )}
        {interview.status === "complete" && (
          <button
            type="button"
            className="btn btn-secondary btn-small"
            disabled={state.busy}
            onClick={() => saveDetails(state, { status: "in-progress" })}
          >
            {t.reopen}
          </button>
        )}
      </div>
      <div className="iv-inline">
        <button
          type="button"
          className="btn btn-secondary"
          onClick={() => {
            state.editingDetails = false;
            vlens.scheduleRedraw();
          }}
        >
          {t.cancel}
        </button>
        <button type="submit" className="btn btn-primary" disabled={state.busy}>
          {state.busy ? t.saving : t.save}
        </button>
      </div>
    </form>
  );
};

async function saveDetails(state: ViewState, change: Partial<server.UpdateInterviewRequest>) {
  const i = state.interview;
  const d = state.details;
  state.busy = true;
  state.error = "";
  vlens.scheduleRedraw();
  const [resp, err] = await server.UpdateInterview({
    id: i.id,
    version: i.version,
    date: d.date || i.date.slice(0, 10),
    title: d.title,
    notes: d.notes,
    photoId: i.photoId,
    status: i.status,
    ...change,
  });
  state.busy = false;
  if (!resp) {
    state.error = err || t.saveFailed;
    vlens.scheduleRedraw();
    return;
  }
  if (change.status === "in-progress") {
    core.setRoute(`/interview/${i.id}/ask`);
    return;
  }
  const dateMoved = resp.interview.date !== i.date;
  state.interview = resp.interview;
  if (!("photoId" in change)) state.editingDetails = false;
  if (dateMoved) {
    for (const a of state.answers.values()) a.milestoneDate = resp.interview.date;
  }
  vlens.scheduleRedraw();
}

async function onPortrait(state: ViewState, person: server.Person, e: Event) {
  const input = e.currentTarget as HTMLInputElement;
  const file = input.files?.[0];
  input.value = "";
  if (!file) return;
  const problem = photoFileProblem(file);
  if (problem) {
    state.error = problem;
    vlens.scheduleRedraw();
    return;
  }
  state.busy = true;
  vlens.scheduleRedraw();
  try {
    const image = await uploadPhoto(file, [person.id], state.interview.familyId);
    await saveDetails(state, { photoId: image.id });
  } catch (err) {
    state.busy = false;
    state.error = err instanceof Error ? err.message : t.saveFailed;
    vlens.scheduleRedraw();
  }
}

async function saveAnswer(state: ViewState, questionId: number) {
  const draft = state.drafts[questionId];
  const current = state.answers.get(questionId)?.description ?? "";
  if (draft === undefined || draft.trim() === current) return;
  state.savingAnswer = questionId;
  vlens.scheduleRedraw();
  const [resp, err] = await server.SaveInterviewAnswer({
    interviewId: state.interview.id,
    questionId,
    answer: draft,
    skipped: false,
  });
  state.savingAnswer = 0;
  if (!resp) {
    state.error = err || t.saveFailed;
  } else {
    state.interview = resp.interview;
    if (resp.answer.id) state.answers.set(questionId, resp.answer);
    else state.answers.delete(questionId);
    delete state.drafts[questionId];
  }
  vlens.scheduleRedraw();
}

async function onDelete(state: ViewState) {
  if (!confirm(t.confirmDelete)) return;
  state.busy = true;
  vlens.scheduleRedraw();
  const [resp, err] = await server.DeleteInterview({ id: state.interview.id });
  state.busy = false;
  if (!resp) {
    state.error = err || t.deleteFailed;
    vlens.scheduleRedraw();
    return;
  }
  core.setRoute(`/profile/${resp.personId}?tab=interviews`);
}
