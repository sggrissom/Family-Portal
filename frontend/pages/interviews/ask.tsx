import * as preact from "preact";
import * as vlens from "vlens";
import * as core from "vlens/core";
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
  loadDraftStates,
  pendingQuestions,
  previousLine,
  questionStatus,
  saveDraft,
  setShowPreviousPreference,
  showPreviousPreference,
} from "../../lib/interviews";

const t = copy.interview.ask;
import { callInterview, InterviewSaveQueue } from "../../lib/interviewSave";

type AskState = {
  queue: InterviewSaveQueue;
  timers: Map<number, ReturnType<typeof setTimeout>>;
  paused: boolean;
  intentions: Map<number, boolean>;
  storageFailed: boolean;
  source: server.GetInterviewResponse | null;
  interview: server.Interview;
  person: server.Person;
  questions: server.InterviewQuestion[];
  previous: Record<number, server.PreviousAnswer[]>;
  answers: Map<number, server.Milestone>;
  text: Record<number, string>;
  saved: Record<number, string>;
  inflight: Set<number>;
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
    queue: new InterviewSaveQueue(),
    timers: new Map(),
    paused: false,
    intentions: new Map(),
    storageFailed: false,
    source: null,
    interview: {} as server.Interview,
    person: {} as server.Person,
    questions: [],
    previous: {},
    answers: new Map(),
    text: {},
    saved: {},
    inflight: new Set(),
    failed: new Set(),
    index: 0,
    showPrevious: false,
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
  state.intentions.clear();
  state.failed.clear();
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
  state.paused = false;
  state.error = "";
  for (const [id, draft] of loadDraftStates(data.interview.id)) {
    if (!(id in state.text)) continue;
    const matches =
      draft.text.trim() === (state.saved[id] ?? "").trim() &&
      (data.interview.skippedIds ?? []).includes(id) === draft.skipped;
    if (!matches) {
      state.intentions.set(id, draft.skipped);
      if (draft.version !== data.interview.version) {
        state.paused = true;
        state.failed.add(id);
        state.error = t.savePaused;
      }
    }
  }
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

const SaveStatus = ({ state }: { state: AskState }) => (
  <div className="iv-save-status" role="status">
    {state.error ? (
      <p role="alert">{state.error}</p>
    ) : state.storageFailed ? (
      <p>{t.deviceSaveFailed}</p>
    ) : (
      <p>{state.queue.pending ? t.saving : state.timers.size ? t.deviceSaved : t.saved}</p>
    )}
    {state.paused && (
      <>
        <a href={`/interview/${state.interview.id}`} target="_blank" rel="noopener">
          {t.reviewSaved}
        </a>
        <button
          type="button"
          className="btn btn-secondary btn-small"
          onClick={() => retrySaves(state)}
        >
          {t.retrySaved}
        </button>
      </>
    )}
  </div>
);

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
        disabled={state.finishing || state.uploading || state.followUp.adding}
        onInput={e => {
          state.text[q.id] = e.currentTarget.value;
          state.storageFailed = !saveDraft(
            state.interview.id,
            q.id,
            e.currentTarget.value,
            state.interview.version
          );
          state.intentions.set(q.id, false);
          const timer = state.timers.get(q.id);
          if (timer) clearTimeout(timer);
          state.timers.set(
            q.id,
            setTimeout(() => {
              state.timers.delete(q.id);
              commit(state, q.id);
            }, 600)
          );
          vlens.scheduleRedraw();
        }}
        onKeyDown={e => {
          if (e.key === "Enter" && (e.metaKey || e.ctrlKey) && !e.isComposing) {
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
          disabled={state.finishing || state.uploading || state.followUp.adding}
          onClick={vlens.cachePartial(finish, state)}
        >
          {state.finishing ? t.finishing : t.finish}
        </button>
      </div>
    </div>
  );
};

function go(state: AskState, index: number) {
  if (state.finishing) return;
  const q = state.questions[state.index];
  if (q) commit(state, q.id);
  state.index = Math.max(0, Math.min(index, state.questions.length));
  state.expandPrevious = false;
  state.followUp = { open: false, text: "", keep: false, adding: false };
  vlens.scheduleRedraw();
}

function commit(state: AskState, questionId: number) {
  const timer = state.timers.get(questionId);
  if (timer) clearTimeout(timer);
  state.timers.delete(questionId);
  if (state.paused) return;
  const text = (state.text[questionId] ?? "").trim();
  const skipped =
    state.intentions.get(questionId) ?? (state.interview.skippedIds ?? []).includes(questionId);
  if (
    !state.inflight.has(questionId) &&
    text === (state.saved[questionId] ?? "").trim() &&
    skipped === (state.interview.skippedIds ?? []).includes(questionId)
  ) {
    clearDraft(state.interview.id, questionId);
    return;
  }
  void save(state, questionId, skipped);
}

async function save(state: AskState, questionId: number, skipped: boolean) {
  const text = skipped ? "" : (state.text[questionId] ?? "").trim();
  state.intentions.set(questionId, skipped);
  state.inflight.add(questionId);
  await state.queue.run(async () => {
    if (state.paused) {
      state.failed.add(questionId);
      return;
    }
    const expectedVersion = state.interview.version;
    const [resp, err] = await callInterview(
      server.SaveInterviewAnswer({
        interviewId: state.interview.id,
        questionId,
        answer: text,
        skipped,
        version: expectedVersion,
      })
    );
    if (!resp) {
      // A lost response may follow a committed write. Refetch, never blindly retry.
      const [latest] = await callInterview(server.GetInterview({ id: state.interview.id }));
      const answer = latest?.answers?.find(a => a.questionId === questionId);
      const matches =
        latest &&
        (answer?.description ?? "") === text &&
        (latest.interview.skippedIds ?? []).includes(questionId) === skipped;
      if (
        matches &&
        latest.interview.version === expectedVersion + 1 &&
        err !== server.ErrInterviewChanged
      ) {
        state.interview = latest.interview;
        state.saved[questionId] = text;
        if (answer) state.answers.set(questionId, answer);
        else state.answers.delete(questionId);
        if (
          (state.text[questionId] ?? "").trim() === text &&
          state.intentions.get(questionId) === skipped
        )
          clearDraft(state.interview.id, questionId);
        return;
      }
      state.failed.add(questionId);
      state.paused = true;
      state.error = err || t.savePaused;
      return;
    }
    state.failed.delete(questionId);
    state.interview = resp.interview;
    if (resp.answer.id) state.answers.set(questionId, resp.answer);
    else state.answers.delete(questionId);
    state.saved[questionId] = text;
    if (
      (state.text[questionId] ?? "").trim() === text &&
      state.intentions.get(questionId) === skipped
    )
      clearDraft(state.interview.id, questionId);
  });
  state.inflight.delete(questionId);
  vlens.scheduleRedraw();
}

async function retrySaves(state: AskState) {
  await state.queue.flush();
  const [latest, err] = await callInterview(server.GetInterview({ id: state.interview.id }));
  if (!latest) {
    state.error = err || t.savePaused;
    vlens.scheduleRedraw();
    return;
  }
  const remote = answersByQuestion(latest.answers);
  const dirty = state.questions.filter(
    q =>
      (state.text[q.id] ?? "").trim() !== (state.saved[q.id] ?? "").trim() || state.failed.has(q.id)
  );
  const conflicts = dirty.filter(
    q =>
      state.failed.has(q.id) ||
      (remote.get(q.id)?.description ?? "") !== (state.saved[q.id] ?? "") ||
      (latest.interview.skippedIds ?? []).includes(q.id) !==
        (state.interview.skippedIds ?? []).includes(q.id)
  );
  if (
    conflicts.length &&
    !confirm(
      t.confirmReplace(
        conflicts
          .map(q => `${q.text}\n${remote.get(q.id)?.description || t.statusSkipped}`)
          .join("\n\n")
      )
    )
  )
    return;
  state.interview = latest.interview;
  state.questions = latest.questions;
  state.answers = remote;
  for (const q of latest.questions) {
    const saved = remote.get(q.id)?.description ?? "";
    state.saved[q.id] = saved;
    if (!dirty.some(d => d.id === q.id)) state.text[q.id] = saved;
  }
  state.paused = false;
  state.error = "";
  state.failed.clear();
  for (const q of dirty) void save(state, q.id, state.intentions.get(q.id) ?? false);
  vlens.scheduleRedraw();
}

function skip(state: AskState, q: server.InterviewQuestion) {
  if (state.finishing || state.paused) return;
  const timer = state.timers.get(q.id);
  if (timer) clearTimeout(timer);
  state.timers.delete(q.id);
  state.text[q.id] = "";
  state.storageFailed = !saveDraft(state.interview.id, q.id, "", state.interview.version, true);
  void save(state, q.id, true);
  state.index = Math.min(state.index + 1, state.questions.length);
  state.expandPrevious = false;
  vlens.scheduleRedraw();
}

async function addFollowUp(state: AskState, after: server.InterviewQuestion) {
  const text = state.followUp.text.trim();
  if (!text || state.followUp.adding) return;
  commit(state, after.id);
  state.followUp.adding = true;
  vlens.scheduleRedraw();
  const [resp, err] = await state.queue.run(async () => {
    if (state.paused) return [null, t.savePaused] as [null, string];
    const result = await callInterview(
      server.AddInterviewQuestion({
        version: state.interview.version,
        interviewId: state.interview.id,
        text,
        afterQuestionId: after.id,
        keep: state.followUp.keep,
      })
    );
    if (result[0]) state.interview = result[0].interview;
    return result;
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
  return state.queue.run(async () => {
    if (state.paused) return false;
    const i = state.interview;
    const [resp, err] = await callInterview(
      server.UpdateInterview({
        id: i.id,
        version: i.version,
        date: i.date.slice(0, 10),
        title: i.title,
        notes: i.notes,
        photoId: i.photoId,
        status: i.status,
        ...change,
      })
    );
    if (!resp) {
      state.paused = true;
      state.error = err || "The interview could not be saved.";
      return false;
    }
    state.interview = resp.interview;
    return true;
  });
}

async function finish(state: AskState) {
  state.finishing = true;
  vlens.scheduleRedraw();
  for (const q of state.questions) commit(state, q.id);
  await state.queue.flush();
  if (state.failed.size > 0 || state.paused || state.queue.pending > 0) {
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
  core.setRoute(`/interview/${state.interview.id}`);
}
