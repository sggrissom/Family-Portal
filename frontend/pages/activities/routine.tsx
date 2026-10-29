import * as preact from "preact";
import * as rpc from "vlens/rpc";
import * as server from "../../server";
import * as auth from "../../lib/authCache";
import { Header, Footer } from "../../layout";
import { requireAuthInView, ensureAuthInFetch } from "../../lib/authHelpers";
import { getIdFromRoute } from "../../lib/routeHelpers";
import { labelsForKind } from "./labels";
import { PerformanceCard, SeasonStats, groupByEvent } from "./performance-card";
import "./activities-styles";
import "./season-styles";

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

  const [people] = await server.ListPeople({});
  return rpc.ok<RoutinePageData>({ history, people: people?.people ?? [] });
}

export function view(route: string, prefix: string, data: RoutinePageData): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth) return;

  const entry = data.history.entry.entry;
  const labels = labelsForKind(data.history.season.kind);
  const appearances = data.history.appearances ?? [];
  const traits = [entry.format, entry.style, entry.division, entry.level]
    .filter(part => part)
    .join(" · ");
  const roster = (data.history.entry.personIds ?? [])
    .map(id => data.people.find(person => person.id === id))
    .filter((person): person is server.Person => !!person);
  const editable = auth.canContribute(entry.familyId);
  const groups = groupByEvent(appearances);

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
          {roster.length > 0 && (
            <p className="routine-roster">
              {roster.map(person => (
                <a key={person.id} href={`/profile/${person.id}`}>
                  {person.name}
                </a>
              ))}
            </p>
          )}
          {entry.notes && <p className="season-notes">{entry.notes}</p>}
        </div>

        <SeasonStats appearances={appearances} labels={labels} />

        <section className="activities-section">
          <h2 className="activities-heading">The season so far</h2>
          {groups.length === 0 ? (
            <div className="empty-state">
              <p>
                This {labels.entry.toLowerCase()} has not been to any {labels.event.toLowerCase()}{" "}
                yet.
              </p>
            </div>
          ) : (
            <div className="story-cards">
              {groups.map(group => (
                <PerformanceCard
                  key={group[0].appearance.id}
                  group={group}
                  people={data.people}
                  showEntry={false}
                  linkEvent={true}
                  editable={editable}
                />
              ))}
            </div>
          )}
        </section>
      </main>
      <Footer />
    </div>
  );
}
