import * as server from "../server";

export function ageYearsAt(birthday: string, date: string): number {
  if (!birthday || !date) return 0;
  const b = new Date(birthday);
  const d = new Date(date);
  if (d < b) return 0;
  let years = d.getUTCFullYear() - b.getUTCFullYear();
  const beforeBirthday =
    d.getUTCMonth() < b.getUTCMonth() ||
    (d.getUTCMonth() === b.getUTCMonth() && d.getUTCDate() < b.getUTCDate());
  if (beforeBirthday) years--;
  return Math.max(0, years);
}

export function firstName(name: string): string {
  return name.trim().split(/\s+/)[0] ?? name;
}

export function interviewTitle(interview: server.Interview, person: server.Person): string {
  if (interview.title) return interview.title;
  const age = ageYearsAt(person.birthday, interview.date);
  return `${firstName(person.name)}'s interview at ${age}`;
}

export type QuestionStatus = "answered" | "skipped" | "pending";

export function answersByQuestion(answers: server.Milestone[]): Map<number, server.Milestone> {
  return new Map(answers.map(a => [a.questionId, a]));
}

export function questionStatus(
  interview: server.Interview,
  answers: Map<number, unknown>,
  questionId: number
): QuestionStatus {
  if (answers.has(questionId)) return "answered";
  if ((interview.skippedIds ?? []).includes(questionId)) return "skipped";
  return "pending";
}

export function pendingQuestions(
  interview: server.Interview,
  questions: server.InterviewQuestion[],
  answers: Map<number, unknown>
): server.InterviewQuestion[] {
  return questions.filter(q => questionStatus(interview, answers, q.id) === "pending");
}

export function previousLine(previous: server.PreviousAnswer[] | undefined, limit = 3): string {
  return (previous ?? [])
    .slice(0, limit)
    .map(p => `Age ${p.ageYears}: ${p.text}`)
    .join(" · ");
}

const DRAFT_PREFIX = "interview-draft:";
const SHOW_PREVIOUS_KEY = "interview-show-previous";

function storage(): Storage | null {
  try {
    return typeof localStorage === "undefined" ? null : localStorage;
  } catch {
    return null;
  }
}

export function draftKey(interviewId: number, questionId: number): string {
  return `${DRAFT_PREFIX}${interviewId}:${questionId}`;
}

export function saveDraft(interviewId: number, questionId: number, text: string) {
  try {
    storage()?.setItem(draftKey(interviewId, questionId), text);
  } catch {}
}

export function clearDraft(interviewId: number, questionId: number) {
  try {
    storage()?.removeItem(draftKey(interviewId, questionId));
  } catch {}
}

export function loadDrafts(interviewId: number): Map<number, string> {
  const drafts = new Map<number, string>();
  const store = storage();
  if (!store) return drafts;
  const prefix = `${DRAFT_PREFIX}${interviewId}:`;
  try {
    for (let i = 0; i < store.length; i++) {
      const key = store.key(i);
      if (key?.startsWith(prefix)) {
        drafts.set(Number(key.slice(prefix.length)), store.getItem(key) ?? "");
      }
    }
  } catch {}
  return drafts;
}

export function showPreviousPreference(): boolean {
  try {
    return storage()?.getItem(SHOW_PREVIOUS_KEY) !== "off";
  } catch {
    return true;
  }
}

export function setShowPreviousPreference(on: boolean) {
  try {
    storage()?.setItem(SHOW_PREVIOUS_KEY, on ? "on" : "off");
  } catch {}
}

export function questionFitsAge(q: server.InterviewQuestion, age: number): boolean {
  return (!q.minAgeYears || age >= q.minAgeYears) && (!q.maxAgeYears || age <= q.maxAgeYears);
}
