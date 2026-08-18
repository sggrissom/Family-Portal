// One routine across the whole season: every competition it went to, and how
// it did at each. The other direction off the same hinge as the competition
// page — same performances, read entry-first instead of event-first.
//
// See docs/activities-plan.md, phase 6.

import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView, ensureAuthInFetch } from "../../lib/authHelpers";
import { getIdFromRoute } from "../../lib/routeHelpers";
import { formatDate, formatDateRange, isRealDate } from "../../lib/dateUtils";
import { labelsForKind } from "./labels";
import { ResultKindAdjudication, ResultList } from "./results";
import "./activities-styles";
import "./season-styles";
import "./routine-styles";

// RoutinePageData is the history plus the names a roster and a
// narrowed-to-one-person result need. There is no vocabulary here: this page
// has no form on it.
export type RoutinePageData = {
  history: server.GetEntryHistoryResponse;
  people: server.Person[];
};

const emptyHistory: server.GetEntryHistoryResponse = {
  entry: {
    entry: {
      id: 0,
      seasonId: 0,
      familyId: 0,
      name: "",
      format: "",
      style: "",
      division: "",
      level: "",
      notes: "",
      createdAt: "",
    },
    personIds: [],
  },
  season: { id: 0, name: "", kind: "", startDate: "", endDate: "" },
  appearances: [],
};

export async function fetch(route: string, prefix: string): Promise<rpc.Response<RoutinePageData>> {
  if (!(await ensureAuthInFetch())) {
    return rpc.ok<RoutinePageData>({ history: emptyHistory, people: [] });
  }

  const [history, historyErr] = await server.GetEntryHistory({
    entryId: getIdFromRoute(route) || 0,
  });
  if (historyErr || !history) {
    return [null, historyErr || "Failed to load routine"];
  }

  // Names are decoration here, not the page. A viewer who reached this routine
  // through a link may not see every person on it.
  const [people] = await server.ListPeople({});
  return rpc.ok<RoutinePageData>({ history, people: people?.people ?? [] });
}

// countLabels is what free-text adjudications allow and no more: an exact
// count per label. "3× Diamond, 5× High Gold" is honest; ranking them is not,
// because Diamond and Gold are strings and every host uses its own scale.
function countLabels(appearances: server.AppearanceDetail[]): { label: string; count: number }[] {
  const counts = new Map<string, number>();
  for (const detail of appearances) {
    for (const result of detail.results ?? []) {
      if (result.kind !== ResultKindAdjudication) continue;
      const label = result.label.trim();
      if (!label) continue;
      counts.set(label, (counts.get(label) ?? 0) + 1);
    }
  }
  return [...counts.entries()]
    .map(([label, count]) => ({ label, count }))
    .sort((a, b) => (a.count !== b.count ? b.count - a.count : a.label < b.label ? -1 : 1));
}

export function view(route: string, prefix: string, data: RoutinePageData): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth) return;

  const entry = data.history.entry.entry;
  // SeasonSummary carries the activity's kind so this page can name things the
  // way the rest of the UI does — nothing here should say "competition" when
  // the season is soccer.
  const labels = labelsForKind(data.history.season.kind);
  const appearances = data.history.appearances ?? [];
  const traits = [entry.format, entry.style, entry.division, entry.level]
    .filter(part => part)
    .join(" · ");
  const roster = (data.history.entry.personIds ?? [])
    .map(id => data.people.find(person => person.id === id)?.name)
    .filter((name): name is string => !!name);
  const adjudications = countLabels(appearances);

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="activities-container">
        <a className="back-link" href={`/season/${data.history.season.id}`}>
          ← {data.history.season.name || "Season"}
        </a>

        <div className="season-header">
          <span className="season-eyebrow">{traits || labels.entry}</span>
          <h1>{entry.name}</h1>
          {roster.length > 0 && <p className="season-dates">{roster.join(", ")}</p>}
          {entry.notes && <p className="season-notes">{entry.notes}</p>}
        </div>

        {adjudications.length > 0 && (
          <div className="routine-tally">
            {adjudications.map(row => (
              <span key={row.label} className="routine-tally-item">
                <strong>{row.count}×</strong> {row.label}
              </span>
            ))}
          </div>
        )}

        <section className="activities-section">
          <h2>
            {appearances.length === 0
              ? "This season"
              : `${appearances.length} ${
                  appearances.length === 1
                    ? labels.event.toLowerCase()
                    : labels.eventPlural.toLowerCase()
                }`}
          </h2>

          {appearances.length === 0 ? (
            <div className="empty-state">
              <p>
                This {labels.entry.toLowerCase()} has not been to a {labels.event.toLowerCase()}{" "}
                yet.
              </p>
            </div>
          ) : (
            <ul className="event-list">
              {appearances.map(detail => (
                <li key={detail.appearance.id} className="event-item">
                  <div className="event-item-main">
                    <a className="event-name" href={`/competition/${detail.event.id}`}>
                      {detail.event.name}
                    </a>
                    <span className="event-meta">{appearanceWhen(detail)}</span>
                    {(detail.results ?? []).length === 0 ? (
                      <span className="event-count">No results recorded</span>
                    ) : (
                      <ResultList results={detail.results} people={data.people} />
                    )}
                    {detail.appearance.notes && (
                      <p className="event-notes">{detail.appearance.notes}</p>
                    )}
                  </div>
                </li>
              ))}
            </ul>
          )}
        </section>
      </main>
      <Footer />
    </div>
  );
}

// appearanceWhen prefers the performance's own time and falls back to the
// competition's dates, which is the same fallback the server sorts on.
function appearanceWhen(detail: server.AppearanceDetail): string {
  if (isRealDate(detail.appearance.occurredAt)) {
    return formatDate(detail.appearance.occurredAt);
  }
  const range = formatDateRange(detail.event.startDate, detail.event.endDate);
  const where = [detail.event.host, detail.event.location].filter(part => part).join(" · ");
  return [range, where].filter(part => part).join(" — ");
}
