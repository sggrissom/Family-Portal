import * as server from "../server";

export type QuestionRow = {
  key: number;
  id: number;
  text: string;
  section: string;
  minAgeYears: number;
  maxAgeYears: number;
  defaultKey: string;
  archived: boolean;
};

export type QuestionDraft = {
  rows: QuestionRow[];
  sections: string[];
  nextKey: number;
};

export function draftFrom(questions: server.InterviewQuestion[]): QuestionDraft {
  const draft: QuestionDraft = { rows: [], sections: [], nextKey: 1 };
  for (const q of questions) addRow(draft, q);
  return draft;
}

export function addRow(
  draft: QuestionDraft,
  q: Pick<server.InterviewQuestion, "text" | "section"> & Partial<server.InterviewQuestion>,
  index = draft.rows.length
): QuestionRow {
  const row: QuestionRow = {
    key: draft.nextKey++,
    id: q.id ?? 0,
    text: q.text,
    section: q.section,
    minAgeYears: q.minAgeYears ?? 0,
    maxAgeYears: q.maxAgeYears ?? 0,
    defaultKey: q.defaultKey ?? "",
    archived: q.archived ?? false,
  };
  draft.rows.splice(index, 0, row);
  if (!draft.sections.includes(row.section)) draft.sections.push(row.section);
  return row;
}

export function activeIn(draft: QuestionDraft, section: string): QuestionRow[] {
  return draft.rows.filter(r => r.section === section && !r.archived);
}

export function retired(draft: QuestionDraft): QuestionRow[] {
  return draft.rows.filter(r => r.archived);
}

function take(draft: QuestionDraft, key: number): QuestionRow | null {
  const i = draft.rows.findIndex(r => r.key === key);
  return i < 0 ? null : draft.rows.splice(i, 1)[0];
}

function endOf(draft: QuestionDraft, section: string): number {
  let at = -1;
  draft.rows.forEach((r, i) => {
    if (r.section === section) at = i;
  });
  if (at >= 0) return at + 1;
  const later = draft.sections.slice(draft.sections.indexOf(section) + 1);
  const next = draft.rows.findIndex(r => later.includes(r.section));
  return next < 0 ? draft.rows.length : next;
}

export function moveBefore(draft: QuestionDraft, key: number, targetKey: number) {
  if (key === targetKey) return;
  const row = take(draft, key);
  if (!row) return;
  const at = draft.rows.findIndex(r => r.key === targetKey);
  if (at < 0) {
    draft.rows.push(row);
    return;
  }
  row.section = draft.rows[at].section;
  draft.rows.splice(at, 0, row);
}

export function moveToSection(draft: QuestionDraft, key: number, section: string) {
  const row = take(draft, key);
  if (!row) return;
  if (!draft.sections.includes(section)) draft.sections.push(section);
  row.section = section;
  draft.rows.splice(endOf(draft, section), 0, row);
}

// Up and down step past neighbours in the same section, then across the
// section edge, so a question can travel the whole list with two buttons.
export function moveStep(draft: QuestionDraft, key: number, step: -1 | 1) {
  const row = draft.rows.find(r => r.key === key);
  if (!row || row.archived) return;
  const siblings = activeIn(draft, row.section);
  const neighbour = siblings[siblings.indexOf(row) + step];
  if (neighbour) {
    if (step < 0) moveBefore(draft, key, neighbour.key);
    else moveBefore(draft, neighbour.key, key);
    return;
  }
  const sections = draft.sections;
  const next = sections[sections.indexOf(row.section) + step];
  if (next === undefined) return;
  if (step > 0) {
    const first = activeIn(draft, next)[0];
    if (first) moveBefore(draft, key, first.key);
    else moveToSection(draft, key, next);
  } else {
    moveToSection(draft, key, next);
  }
}

export function renameSection(draft: QuestionDraft, from: string, to: string) {
  to = to.trim();
  if (!to || to === from) return;
  for (const r of draft.rows) if (r.section === from) r.section = to;
  const merged = draft.sections.includes(to);
  draft.sections = merged
    ? draft.sections.filter(s => s !== from)
    : draft.sections.map(s => (s === from ? to : s));
}

export function addSection(draft: QuestionDraft, name: string): boolean {
  name = name.trim();
  if (!name || draft.sections.includes(name)) return false;
  draft.sections.push(name);
  return true;
}

export function resetToDefaults(draft: QuestionDraft, defaults: server.InterviewQuestion[]) {
  const have = new Set(draft.rows.map(r => r.defaultKey));
  for (const r of draft.rows) if (r.defaultKey) r.archived = false;
  for (const d of defaults) {
    if (have.has(d.defaultKey)) continue;
    if (!draft.sections.includes(d.section)) draft.sections.push(d.section);
    addRow(draft, d, endOf(draft, d.section));
  }
}

export function questionsForSave(draft: QuestionDraft): server.InterviewQuestionInput[] {
  return draft.sections.flatMap(section =>
    draft.rows
      .filter(r => r.section === section)
      .map(r => ({
        id: r.id,
        text: r.text,
        section: r.section,
        minAgeYears: r.minAgeYears,
        maxAgeYears: r.maxAgeYears,
        defaultKey: r.defaultKey,
        archived: r.archived,
      }))
  );
}
