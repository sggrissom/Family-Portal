import * as preact from "preact";
import * as vlens from "vlens";
import { navigate } from "../../lib/navigate";
import * as server from "../../server";
import { copy } from "../../lib/copy";
import { uploadPhoto } from "../../lib/photoUpload";
import { photoFileProblem } from "../../lib/photoUploadQueue";
import {
  QuestionStatus,
  answersByQuestion,
  clearDraft,
  firstName,
  interviewTitle,
  loadDrafts,
  pendingQuestions,
  previousLine,
  questionStatus,
  saveDraft,
  setShowPreviousPreference,
  showPreviousPreference,
} from "../../lib/interviews";

const t = copy.interview.ask;
const RETRY_MS = 4000;

type AskState = {
  source: server.GetInterviewResponse | null;
  interview: server.Interview;
  person: server.Person;
  questions: server.InterviewQuestion[];
  previous: Record<number, server.PreviousAnswer[]>;
  answers: Map<number, server.Milestone>;
  text: Record<number, string>;
  saved: Record<number, string>;
  inflight: Set<number>;
  again: Map<number, boolean>;
  failed: Set<number>;
  index: number;
  showPrevious: boolean;
  expandPrevious: boolean;
  showJump: boolean;
  followUp: { open: boolean; text: string; keep: boolean; adding: boolean };
  uploading: boolean;
  finishing: boolean;
  error: string;
};

const useAsk = vlens.declareHook(
  (_id: number): AskState => ({
    source: null,
    interview: {} as server.Interview,
    person: {} as server.Person,
    questions: [],
    previous: {},
    answers: new Map(),
    text: {},
    saved: {},
    inflight: new Set(),
    again: new Map(),
    failed: new Set(),
    index: 0,
    showPrevious: true,
    expandPrevious: false,
    showJump: false,
    followUp: { open: false, text: "", keep: false, adding: false },
    uploading: false,
    finishing: false,
    error: "",
  })
);

function init(state: AskState, data: server.GetInterviewResponse) {
  state.source = data;
  state.interview = data.interview;
  state.person = data.person;
  state.questions = [...data.questions];
  state.previous = data.previous ?? {};
  state.answers = answersByQuestion(data.answers ?? []);
  state.text = {};
  state.saved = {};
  for (const q of state.questions) {
    const saved = state.answers.get(q.id)?.description ?? "";
    state.text[q.id] = saved;
    state.saved[q.id] = saved;
  }
  for (const [qid, draft] of loadDrafts(data.interview.id)) {
    if (qid in state.text && draft !== state.saved[qid]) state.text[qid] = draft;
  }
  const firstPending = state.questions.findIndex(
    q => questionStatus(state.interview, state.answers, q.id) === "pending"
  );
  state.index = firstPending < 0 ? state.questions.length : firstPending;
  state.showPrevious = showPreviousPreference();
  state.error = "";
}

export function AskView({ data }: { data: server.GetInterviewResponse }): preact.ComponentChild {
  const state = useAsk(data.interview.id);
  if (state.source !== data) init(state, data);

  const total = state.questions.length;
  const atEnd = state.index >= total;
  const q = state.questions[state.index];

  return (
    <div className="iv-ask">
      <div className="iv-ask-top">
        <a className="iv-link" href={`/interview/${state.interview.id}`}>
          {interviewTitle(state.interview, state.person)}
        </a>
        <span className="iv-progress">{atEnd ? t.done : t.progress(state.index + 1, total)}</span>
        <button
          type="button"
          className="iv-link"
          aria-expanded={state.showJump ? "true" : "false"}
          onClick={() => {
            state.showJump = !state.showJump;
            vlens.scheduleRedraw();
          }}
        >
          {t.allQuestions}
        </button>
      </div>
      <div className="iv-bar" aria-hidden="true">
        <div
          className="iv-bar-fill"
          style={{ width: `${(Math.min(state.index, total) / total) * 100}%` }}
        />
      </div>

      <SaveStatus state={state} />

      {state.showJump && <JumpList state={state} />}

      {atEnd ? <FinishPanel state={state} /> : <QuestionPanel state={state} q={q} />}
    </div>
  );
}

const SaveStatus = ({ state }: { state: AskState }) =>
  state.failed.size > 0 ? (
    <p className="iv-save-status" role="status">
      {t.retrying}
    </p>
  ) : state.error ? (
    <div className="error-message" role="alert">
      {state.error}
    </div>
  ) : null;

const QuestionPanel = ({ state, q }: { state: AskState; q: server.InterviewQuestion }) => {
  const prev = state.previous[q.id] ?? [];
  return (
    <div className="iv-question-panel">
      <p className="iv-section-label">{q.section}</p>
      <h1 className="iv-question" id="ivQuestion">
        {q.text}
      </h1>
      <textarea
        key={q.id}
        className="iv-answer"
        aria-labelledby="ivQuestion"
        rows={3}
        placeholder={t.answerPlaceholder}
        value={state.text[q.id] ?? ""}
        ref={el => {
          if (el && document.activeElement !== el && !state.followUp.open) el.focus();
        }}
        onInput={e => {
          state.text[q.id] = e.currentTarget.value;
          saveDraft(state.interview.id, q.id, e.currentTarget.value);
        }}
        onKeyDown={e => {
          if (e.key === "Enter" && !e.shiftKey && !e.isComposing) {
            e.preventDefault();
            go(state, state.index + 1);
          }
        }}
      />
      {questionStatus(state.interview, state.answers, q.id) === "skipped" && !state.text[q.id] && (
        <p className="iv-hint">{t.wasSkipped}</p>
      )}

      {state.showPrevious && prev.length > 0 && (
        <button
          type="button"
          className="iv-previous"
          aria-expanded={state.expandPrevious ? "true" : "false"}
          onClick={() => {
            state.expandPrevious = !state.expandPrevious;
            vlens.scheduleRedraw();
          }}
        >
          {state.expandPrevious ? (
            <ul>
              {prev.map(p => (
                <li key={p.milestoneId}>
                  <span className="iv-previous-age">{t.atAge(p.ageYears)}</span> {p.text}
                </li>
              ))}
            </ul>
          ) : (
            previousLine(prev)
          )}
        </button>
      )}

      <div className="iv-ask-buttons">
        <button
          type="button"
          className="btn btn-secondary"
          disabled={state.index === 0}
          onClick={() => go(state, state.index - 1)}
        >
          {t.back}
        </button>
        <button
          type="button"
          className="btn btn-secondary"
          onClick={vlens.cachePartial(skip, state, q)}
        >
          {t.skip}
        </button>
        <button
          type="button"
          className="btn btn-primary"
          onClick={() => go(state, state.index + 1)}
        >
          {state.index === state.questions.length - 1 ? t.finishUp : t.next}
        </button>
      </div>

      <div className="iv-ask-extras">
        {state.followUp.open ? (
          <FollowUp state={state} q={q} />
        ) : (
          <button
            type="button"
            className="iv-link"
            onClick={() => {
              state.followUp.open = true;
              vlens.scheduleRedraw();
            }}
          >
            {t.addFollowUp}
          </button>
        )}
        <Portrait state={state} />
        <label className="iv-toggle">
          <input
            type="checkbox"
            checked={state.showPrevious}
            onChange={e => {
              state.showPrevious = e.currentTarget.checked;
              setShowPreviousPreference(state.showPrevious);
              vlens.scheduleRedraw();
            }}
          />
          {t.showPrevious}
        </label>
      </div>
    </div>
  );
};

const FollowUp = ({ state, q }: { state: AskState; q: server.InterviewQuestion }) => (
  <div className="iv-followup">
    <input
      type="text"
      aria-label={t.followUpText}
      placeholder={t.followUpText}
      value={state.followUp.text}
      ref={el => {
        if (el && document.activeElement?.tagName !== "INPUT") el.focus();
      }}
      onInput={e => {
        state.followUp.text = e.currentTarget.value;
        vlens.scheduleRedraw();
      }}
      onKeyDown={e => {
        if (e.key === "Enter") {
          e.preventDefault();
          addFollowUp(state, q);
        }
      }}
    />
    <div className="iv-followup-options">
      <label className="iv-toggle">
        <input
          type="radio"
          name="followUpKeep"
          checked={!state.followUp.keep}
          onChange={() => {
            state.followUp.keep = false;
            vlens.scheduleRedraw();
          }}
        />
        {t.justOnce}
      </label>
      <label className="iv-toggle">
        <input
          type="radio"
          name="followUpKeep"
          checked={state.followUp.keep}
          onChange={() => {
            state.followUp.keep = true;
            vlens.scheduleRedraw();
          }}
        />
        {t.keepIt}
      </label>
    </div>
    <div className="iv-inline">
      <button
        type="button"
        className="btn btn-secondary btn-small"
        onClick={() => {
          state.followUp.open = false;
          vlens.scheduleRedraw();
        }}
      >
        {t.cancel}
      </button>
      <button
        type="button"
        className="btn btn-primary btn-small"
        disabled={!state.followUp.text.trim() || state.followUp.adding}
        onClick={vlens.cachePartial(addFollowUp, state, q)}
      >
        {t.addQuestion}
      </button>
    </div>
  </div>
);

const Portrait = ({ state }: { state: AskState }) => (
  <label className="iv-link iv-portrait-button">
    {state.uploading ? t.uploading : state.interview.photoId ? t.retakePortrait : t.takePortrait}
    <input
      type="file"
      accept="image/*"
      capture="user"
      hidden
      disabled={state.uploading}
      onChange={vlens.cachePartial(onPortrait, state)}
    />
  </label>
);

const STATUS_LABEL: Record<QuestionStatus, string> = {
  answered: t.statusAnswered,
  skipped: t.statusSkipped,
  pending: t.statusPending,
};

const JumpList = ({ state }: { state: AskState }) => (
  <ol className="iv-jump">
    {state.questions.map((q, i) => {
      const status = questionStatus(state.interview, state.answers, q.id);
      return (
        <li key={q.id}>
          <button
            type="button"
            className={i === state.index ? "iv-jump-item current" : "iv-jump-item"}
            onClick={() => {
              state.showJump = false;
              go(state, i);
            }}
          >
            <span className={`iv-dot ${status}`} aria-label={STATUS_LABEL[status]} />
            {q.text}
          </button>
        </li>
      );
    })}
  </ol>
);

const FinishPanel = ({ state }: { state: AskState }) => {
  const pending = pendingQuestions(state.interview, state.questions, state.answers);
  const name = firstName(state.person.name);
  return (
    <div className="iv-question-panel">
      <h1 className="iv-question">{t.allDone(name)}</h1>
      <p>{t.answeredCount(state.answers.size, state.questions.length)}</p>
      {pending.length > 0 && (
        <div className="iv-pending">
          <p>{t.pendingIntro(pending.length)}</p>
          <ul>
            {pending.map(q => (
              <li key={q.id}>
                <span>{q.text}</span>
                <button
                  type="button"
                  className="btn btn-secondary btn-small"
                  onClick={() => go(state, state.questions.indexOf(q))}
                >
                  {t.askNow}
                </button>
              </li>
            ))}
          </ul>
          <p className="iv-hint">{t.leaveBlank}</p>
        </div>
      )}
      <div className="iv-ask-buttons">
        <button
          type="button"
          className="btn btn-secondary"
          onClick={() => go(state, state.questions.length - 1)}
        >
          {t.back}
        </button>
        <button
          type="button"
          className="btn btn-primary"
          disabled={state.finishing}
          onClick={vlens.cachePartial(finish, state)}
        >
          {state.finishing ? t.finishing : t.finish}
        </button>
      </div>
    </div>
  );
};

function go(state: AskState, index: number) {
  const q = state.questions[state.index];
  if (q) commit(state, q.id);
  state.index = Math.max(0, Math.min(index, state.questions.length));
  state.expandPrevious = false;
  state.followUp = { open: false, text: "", keep: false, adding: false };
  vlens.scheduleRedraw();
}

function commit(state: AskState, questionId: number) {
  const text = (state.text[questionId] ?? "").trim();
  if (text === (state.saved[questionId] ?? "").trim()) {
    clearDraft(state.interview.id, questionId);
    return;
  }
  void save(state, questionId, false);
}

async function save(state: AskState, questionId: number, skipped: boolean) {
  if (state.inflight.has(questionId)) {
    state.again.set(questionId, skipped);
    return;
  }
  state.inflight.add(questionId);
  const text = skipped ? "" : (state.text[questionId] ?? "").trim();
  const [resp, err] = await server.SaveInterviewAnswer({
    interviewId: state.interview.id,
    questionId,
    answer: text,
    skipped,
  });
  state.inflight.delete(questionId);
  if (!resp) {
    if (err === server.ErrInterviewQuestionNotFound || err?.startsWith("Interview not found")) {
      state.error = err;
      state.failed.delete(questionId);
    } else {
      state.failed.add(questionId);
      window.setTimeout(() => save(state, questionId, skipped), RETRY_MS);
    }
    vlens.scheduleRedraw();
    return;
  }
  state.failed.delete(questionId);
  state.interview = resp.interview;
  if (resp.answer.id) state.answers.set(questionId, resp.answer);
  else state.answers.delete(questionId);
  state.saved[questionId] = text;
  const next = state.again.get(questionId);
  state.again.delete(questionId);
  if (next !== undefined) {
    void save(state, questionId, next);
  } else if ((state.text[questionId] ?? "").trim() === text) {
    clearDraft(state.interview.id, questionId);
  }
  vlens.scheduleRedraw();
}

function skip(state: AskState, q: server.InterviewQuestion) {
  state.text[q.id] = "";
  clearDraft(state.interview.id, q.id);
  state.saved[q.id] = "";
  void save(state, q.id, true);
  state.index = Math.min(state.index + 1, state.questions.length);
  state.expandPrevious = false;
  vlens.scheduleRedraw();
}

async function addFollowUp(state: AskState, after: server.InterviewQuestion) {
  const text = state.followUp.text.trim();
  if (!text || state.followUp.adding) return;
  state.followUp.adding = true;
  vlens.scheduleRedraw();
  const [resp, err] = await server.AddInterviewQuestion({
    interviewId: state.interview.id,
    text,
    afterQuestionId: after.id,
    keep: state.followUp.keep,
  });
  state.followUp.adding = false;
  if (!resp) {
    state.error = err || "That question could not be added.";
    vlens.scheduleRedraw();
    return;
  }
  state.interview = resp.interview;
  const at = state.questions.indexOf(after) + 1;
  state.questions.splice(at, 0, resp.question);
  state.text[resp.question.id] = "";
  state.saved[resp.question.id] = "";
  go(state, at);
}

async function onPortrait(state: AskState, e: Event) {
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
  state.uploading = true;
  state.error = "";
  vlens.scheduleRedraw();
  try {
    const image = await uploadPhoto(file, [state.person.id], state.interview.familyId);
    await updateInterview(state, { photoId: image.id });
  } catch (err) {
    state.error = err instanceof Error ? err.message : "The photo could not be uploaded.";
  }
  state.uploading = false;
  vlens.scheduleRedraw();
}

async function updateInterview(
  state: AskState,
  change: Partial<server.UpdateInterviewRequest>
): Promise<boolean> {
  const i = state.interview;
  const [resp, err] = await server.UpdateInterview({
    id: i.id,
    version: i.version,
    date: i.date.slice(0, 10),
    title: i.title,
    notes: i.notes,
    photoId: i.photoId,
    status: i.status,
    ...change,
  });
  if (!resp) {
    state.error = err || "The interview could not be saved.";
    return false;
  }
  state.interview = resp.interview;
  return true;
}

async function finish(state: AskState) {
  state.finishing = true;
  vlens.scheduleRedraw();
  for (const q of state.questions) commit(state, q.id);
  for (let tries = 0; state.inflight.size > 0 && tries < 50; tries++) {
    await new Promise(r => setTimeout(r, 100));
  }
  if (state.failed.size > 0) {
    state.finishing = false;
    state.error = t.unsavedOnFinish;
    vlens.scheduleRedraw();
    return;
  }
  const ok = await updateInterview(state, { status: "complete" });
  state.finishing = false;
  if (!ok) {
    vlens.scheduleRedraw();
    return;
  }
  state.source = null;
  navigate(`/interview/${state.interview.id}`);
}
