import { getAuth } from "./authCache";
import * as server from "../server";

export function ageYearsAt(birthday: string, date: string): number {
  if (!birthday || !date || birthday.startsWith("0001")) return -1;
  const b = new Date(birthday);
  const d = new Date(date);
  if (Number.isNaN(b.getTime()) || Number.isNaN(d.getTime()) || d < b) return -1;
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
  return age < 0
    ? `${firstName(person.name)}'s interview`
    : `${firstName(person.name)}'s interview at ${age}`;
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
function accountScope(): string {
  const a = getAuth();
  return `${a?.id ?? 0}:${a?.familyId ?? 0}:`;
}

function storage(): Storage | null {
  try {
    return typeof localStorage === "undefined" ? null : localStorage;
  } catch {
    return null;
  }
}

export function draftKey(interviewId: number, questionId: number): string {
  return `${DRAFT_PREFIX}${accountScope()}${interviewId}:${questionId}`;
}

export function saveDraft(
  interviewId: number,
  questionId: number,
  text: string,
  version = 0,
  skipped = false
): boolean {
  try {
    const store = storage();
    if (!store) return false;
    store.setItem(draftKey(interviewId, questionId), JSON.stringify({ text, version, skipped }));
    return true;
  } catch {
    return false;
  }
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
  const prefix = `${DRAFT_PREFIX}${accountScope()}${interviewId}:`;
  try {
    for (let i = 0; i < store.length; i++) {
      const key = store.key(i);
      if (key?.startsWith(prefix)) {
        drafts.set(Number(key.slice(prefix.length)), readDraft(store.getItem(key)).text);
      }
    }
  } catch {}
  return drafts;
}

type InterviewDraft = { text: string; version: number; skipped: boolean };
function readDraft(raw: string | null): InterviewDraft {
  try {
    const d = JSON.parse(raw ?? "");
    if (typeof d.text === "string") return d;
  } catch {}
  return { text: raw ?? "", version: 0, skipped: false };
}
export function loadDraftStates(interviewId: number): Map<number, InterviewDraft> {
  const store = storage();
  const out = new Map<number, InterviewDraft>();
  if (!store) return out;
  const prefix = `${DRAFT_PREFIX}${accountScope()}${interviewId}:`;
  try {
    for (const key of Object.keys(store))
      if (key.startsWith(prefix))
        out.set(Number(key.slice(prefix.length)), readDraft(store.getItem(key)));
  } catch {}
  return out;
}

export function showPreviousPreference(): boolean {
  try {
    return storage()?.getItem(SHOW_PREVIOUS_KEY + accountScope()) === "on";
  } catch {
    return false;
  }
}

export function setShowPreviousPreference(on: boolean) {
  try {
    storage()?.setItem(SHOW_PREVIOUS_KEY + accountScope(), on ? "on" : "off");
  } catch {}
}

export function questionFitsAge(q: server.InterviewQuestion, age: number): boolean {
  return (!q.minAgeYears || age >= q.minAgeYears) && (!q.maxAgeYears || age <= q.maxAgeYears);
}

export function normalizeAnswer(text: string): string {
  return text.toLowerCase().trim().replace(/\s+/gu, " ");
}

export type AnswerChange = {
  question: server.InterviewQuestion;
  before: string;
  after: string;
};

// Answers that differ from the person's most recent earlier interview.
export function changesSincePrevious(
  questions: server.InterviewQuestion[],
  answers: Map<number, server.Milestone>,
  previous: Record<number, server.PreviousAnswer[]>
): AnswerChange[] {
  const changes: AnswerChange[] = [];
  for (const q of questions) {
    const now = answers.get(q.id);
    const before = [...(previous[q.id] ?? [])].sort(
      (a, b) => b.date.localeCompare(a.date) || b.interviewId - a.interviewId
    )[0];
    if (now && before && normalizeAnswer(now.description) !== normalizeAnswer(before.text)) {
      changes.push({ question: q, before: before.text, after: now.description });
    }
  }
  return changes;
}

export type GridCell = {
  answer: server.Milestone | null;
  skipped: boolean;
  asked: boolean;
  trend: "first" | "same" | "changed" | "";
};

export type GridRow = { question: server.InterviewQuestion; cells: GridCell[] };
export type GridSection = { name: string; rows: GridRow[] };

export const ALSO_ASKED = "Also asked";

export function interviewGrid(bundle: server.GetPersonInterviewsResponse): {
  interviews: server.Interview[];
  sections: GridSection[];
} {
  const interviews = bundle.interviews ?? [];
  const byCell = new Map<string, server.Milestone>();
  for (const a of bundle.answers ?? []) byCell.set(`${a.interviewId}:${a.questionId}`, a);

  const sections: GridSection[] = [];
  const sectionFor = (name: string) => {
    let s = sections.find(x => x.name === name);
    if (!s) {
      s = { name, rows: [] };
      sections.push(s);
    }
    return s;
  };

  const questions = [...(bundle.questions ?? [])].sort(
    (a, b) => Number(a.oneOff) - Number(b.oneOff) || a.position - b.position || a.id - b.id
  );
  for (const q of questions) {
    let last = "";
    const cells = interviews.map((interview): GridCell => {
      const answer = byCell.get(`${interview.id}:${q.id}`) ?? null;
      const skipped = !answer && (interview.skippedIds ?? []).includes(q.id);
      const asked = (interview.questionIds ?? []).includes(q.id) || !!answer;
      let trend: GridCell["trend"] = "";
      if (answer) {
        const norm = normalizeAnswer(answer.description);
        trend = !last ? "first" : norm === last ? "same" : "changed";
        last = norm;
      }
      return { answer, skipped, asked, trend };
    });
    if (!cells.some(c => c.answer || c.skipped)) continue;
    sectionFor(q.oneOff ? ALSO_ASKED : q.section).rows.push({ question: q, cells });
  }
  const also = sections.findIndex(s => s.name === ALSO_ASKED);
  if (also >= 0) sections.push(sections.splice(also, 1)[0]);
  return { interviews, sections };
}

export type QuestionTimeline = {
  question: server.InterviewQuestion;
  points: { interview: server.Interview; answer: server.Milestone; age: number }[];
  streak: number;
  distinct: number;
};

export function questionTimelines(bundle: server.GetPersonInterviewsResponse): QuestionTimeline[] {
  const { interviews, sections } = interviewGrid(bundle);
  const birthday = bundle.person.birthday;
  return sections.flatMap(section =>
    section.rows.map(row => {
      const points = row.cells.flatMap((cell, i) =>
        cell.answer
          ? [
              {
                interview: interviews[i],
                answer: cell.answer,
                age: ageYearsAt(birthday, interviews[i].date),
              },
            ]
          : []
      );
      let streak = 0;
      let run = 0;
      let prev = "";
      const seen = new Set<string>();
      for (const p of points) {
        const norm = normalizeAnswer(p.answer.description);
        run = norm === prev ? run + 1 : 1;
        prev = norm;
        streak = Math.max(streak, run);
        seen.add(norm);
      }
      return { question: row.question, points, streak, distinct: seen.size };
    })
  );
}

export type TimelineSort = "list" | "consistent" | "changed";

export function sortTimelines(
  timelines: QuestionTimeline[],
  sort: TimelineSort
): QuestionTimeline[] {
  const multi = (t: QuestionTimeline) => (t.points.length > 1 ? 1 : 0);
  if (sort === "consistent") {
    return [...timelines].sort((a, b) => multi(b) - multi(a) || b.streak - a.streak);
  }
  if (sort === "changed") {
    return [...timelines].sort(
      (a, b) =>
        multi(b) - multi(a) ||
        b.distinct / b.points.length - a.distinct / a.points.length ||
        b.distinct - a.distinct
    );
  }
  return timelines;
}

export function filterTimelines(timelines: QuestionTimeline[], query: string): QuestionTimeline[] {
  const q = normalizeAnswer(query);
  if (!q) return timelines;
  return timelines.filter(
    t =>
      normalizeAnswer(t.question.text).includes(q) ||
      t.points.some(p => normalizeAnswer(p.answer.description).includes(q))
  );
}

export function columnLabel(interview: server.Interview, person: server.Person): string {
  const year = new Date(interview.date).getUTCFullYear();
  if (!person.birthday) return String(year);
  return `Age ${ageYearsAt(person.birthday, interview.date)} · ${year}`;
}

export function rowLabel(row: GridRow): string {
  for (let i = row.cells.length - 1; i >= 0; i--) {
    const context = row.cells[i].answer?.context;
    if (context) return context;
  }
  return row.question.text.replace("{name}", "you").replace("{age}", "your age");
}

export function questionRef(q: server.InterviewQuestion): string {
  return q.id ? `id:${q.id}` : `key:${q.defaultKey}`;
}

export type ImportDraftRow = {
  raw: string;
  answer: string;
  choice: string;
  confidence: string;
  keep: boolean;
};

export type ImportDraft = {
  header: string;
  date: string;
  skip: boolean;
  rows: ImportDraftRow[];
};

export function importDrafts(
  proposals: server.ImportProposal[],
  fallbackDate: string
): ImportDraft[] {
  return proposals.map(p => ({
    header: p.header,
    date: p.date || (p.header ? "" : fallbackDate),
    skip: false,
    rows: (p.rows ?? []).map(r => ({
      raw: r.raw,
      answer: r.answer,
      choice: !r.answer.trim() ? "ignore" : r.ref ? (r.confidence === "exact" ? r.ref : "") : "new",
      confidence: r.confidence,
      keep: true,
    })),
  }));
}

export function duplicateRefs(draft: ImportDraft): Set<string> {
  const seen = new Set<string>();
  const dups = new Set<string>();
  for (const row of draft.rows) {
    if (!row.choice || row.choice === "new" || row.choice === "ignore" || !row.answer.trim())
      continue;
    if (seen.has(row.choice)) dups.add(row.choice);
    seen.add(row.choice);
  }
  return dups;
}

export function importRequest(
  personId: number,
  drafts: ImportDraft[],
  requestId = ""
): server.ImportInterviewsRequest {
  return {
    personId,
    requestId,
    interviews: drafts
      .filter(d => !d.skip)
      .map(d => ({
        inputType: "date",
        date: d.date,
        ageYears: 0,
        ageMonths: 0,
        title: "",
        notes: "",
        answers: d.rows
          .filter(r => r.choice !== "ignore" && r.answer.trim())
          .map(r =>
            r.choice === "new"
              ? { ref: "", answer: r.answer, newText: r.raw, keep: r.keep, rawQuestion: r.raw }
              : { ref: r.choice, answer: r.answer, newText: "", keep: false, rawQuestion: r.raw }
          ),
      })),
  };
}

export type InterviewGroup = {
  interviewId: number;
  personId: number;
  date: string;
  answers: server.Milestone[];
};

// Answers of one interview fold into a single card wherever moments are listed.
export function groupInterviewAnswers(milestones: server.Milestone[]): {
  groups: InterviewGroup[];
  loose: server.Milestone[];
} {
  const groups: InterviewGroup[] = [];
  const byId = new Map<number, InterviewGroup>();
  const loose: server.Milestone[] = [];
  for (const m of milestones) {
    if (!m.interviewId) {
      loose.push(m);
      continue;
    }
    let group = byId.get(m.interviewId);
    if (!group) {
      group = {
        interviewId: m.interviewId,
        personId: m.personId,
        date: m.milestoneDate,
        answers: [],
      };
      byId.set(m.interviewId, group);
      groups.push(group);
    }
    group.answers.push(m);
  }
  for (const g of groups) g.answers.sort((a, b) => a.id - b.id);
  return { groups, loose };
}

export function shortQuestion(context: string): string {
  return context
    .trim()
    .replace(/^(what's|what is|who's|who is)\s+(your\s+)?/i, "")
    .replace(/\?$/, "")
    .trim();
}

export function interviewsById(interviews: server.Interview[] | null | undefined) {
  return new Map((interviews ?? []).map(i => [i.id, i]));
}

export type AgeGroup = {
  age: number;
  answers: { person: server.Person; text: string; date: string; milestoneId: number }[];
};

export function compareByAge(rows: server.CompareRow[]): AgeGroup[] {
  const groups = new Map<number, AgeGroup>();
  for (const row of rows) {
    for (const a of row.answers ?? []) {
      let group = groups.get(a.ageYears);
      if (!group) {
        group = { age: a.ageYears, answers: [] };
        groups.set(a.ageYears, group);
      }
      group.answers.push({
        person: row.person,
        text: a.text,
        date: a.date,
        milestoneId: a.milestoneId,
      });
    }
  }
  return [...groups.values()].sort((a, b) => a.age - b.age);
}
