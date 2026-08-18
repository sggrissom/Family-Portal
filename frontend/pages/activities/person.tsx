// One child's activities: every routine they are on and every performance of
// those routines, grouped by season.
//
// This is the fourth question in docs/activities-plan.md, and the only one of
// the four with a screen that a linked household can reach. The other three
// pages all start from /activities, which resolves an acting family — a
// household that was shared one child has none, so without this page
// ScopeActivities is a checkbox that grants nothing anyone can see.
//
// See docs/activities-plan.md, phase 6.

import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as server from "../../server";
import * as auth from "../../lib/authCache";
import { Header, Footer } from "../../layout";
import { requireAuthInView, ensureAuthInFetch } from "../../lib/authHelpers";
import { getIdFromRoute } from "../../lib/routeHelpers";
import { formatDate, formatDateRange, isRealDate } from "../../lib/dateUtils";
import { labelsForKind } from "./labels";
import { ResultKindAdjudication, ResultList } from "./results";
import "./activities-styles";
import "./season-styles";
import "./routine-styles";
import "./person-styles";

export type PersonActivitiesData = {
  season: server.GetPersonSeasonResponse;
  people: server.Person[];
};

const emptySeason: server.GetPersonSeasonResponse = {
  personId: 0,
  seasonId: 0,
  seasons: [],
  entries: [],
  appearances: [],
};

export async function fetch(
  route: string,
  prefix: string
): Promise<rpc.Response<PersonActivitiesData>> {
  if (!(await ensureAuthInFetch())) {
    return rpc.ok<PersonActivitiesData>({ season: emptySeason, people: [] });
  }

  // seasonId 0 asks for every season the child has ever been in. A linked
  // household cannot list seasons — those have no person dimension — so this
  // page never has one to narrow by.
  const [season, seasonErr] = await server.GetPersonSeason({
    personId: getIdFromRoute(route) || 0,
    seasonId: 0,
  });
  if (seasonErr || !season) {
    return [null, seasonErr || "Failed to load activities"];
  }

  // Names for the subject of the page and for any result narrowed to one
  // performer. A viewer who reached this child through a link may not see
  // every co-performer, so a missing name is normal rather than an error.
  const [people] = await server.ListPeople({});
  return rpc.ok<PersonActivitiesData>({ season, people: people?.people ?? [] });
}

// countLabels is the same honest tally the routine page shows: an exact count
// per adjudication label, never a ranking. Diamond and Gold are strings, and
// every host uses its own scale.
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

function entryTraits(entry: server.Entry): string {
  return [entry.format, entry.style, entry.division, entry.level].filter(part => part).join(" · ");
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

export function view(
  route: string,
  prefix: string,
  data: PersonActivitiesData
): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth) return;

  const personId = data.season.personId;
  const person = data.people.find(p => p.id === personId);
  const name = person?.name ?? "This person";

  // Season and competition pages take plain family access, so a linked
  // household cannot open either — GetSeasonOverview and GetEventDetail have
  // no person dimension to reach through. Only link to them for a viewer whose
  // own family owns the child. The routine page is different: it resolves
  // through the roster, so everyone who can see this page can open it.
  const ownsPerson = !!person && auth.getFamilies().some(family => family.id === person.familyId);

  const seasons = data.season.seasons ?? [];
  const entries = data.season.entries ?? [];
  const appearances = data.season.appearances ?? [];

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="activities-container">
        <a className="back-link" href={`/profile/${personId}`}>
          ← {name}
        </a>

        <div className="season-header">
          <span className="season-eyebrow">Activities</span>
          <h1>{name}</h1>
        </div>

        {seasons.length === 0 ? (
          <div className="empty-state">
            <p>{name} is not on any roster yet.</p>
            {ownsPerson && (
              <a className="btn btn-primary" href="/activities">
                Go to activities
              </a>
            )}
          </div>
        ) : (
          seasons.map(season => (
            <SeasonGroup
              key={season.id}
              season={season}
              entries={entries.filter(entry => entry.entry.seasonId === season.id)}
              appearances={appearances.filter(detail => detail.entry.seasonId === season.id)}
              people={data.people}
              ownsPerson={ownsPerson}
            />
          ))
        )}
      </main>
      <Footer />
    </div>
  );
}

const SeasonGroup = ({
  season,
  entries,
  appearances,
  people,
  ownsPerson,
}: {
  season: server.SeasonSummary;
  entries: server.EntryView[];
  appearances: server.AppearanceDetail[];
  people: server.Person[];
  ownsPerson: boolean;
}): preact.ComponentChild => {
  // The label pack comes off the season's activity kind, which is why
  // SeasonSummary carries it — a page that spans a dance season and a soccer
  // season needs both vocabularies at once.
  const labels = labelsForKind(season.kind);
  const dates = formatDateRange(season.startDate, season.endDate);
  const adjudications = countLabels(appearances);

  return (
    <section className="person-season-group">
      <div className="person-season-head">
        <h2>
          {ownsPerson ? (
            <a className="season-link" href={`/season/${season.id}`}>
              {season.name}
            </a>
          ) : (
            season.name
          )}
        </h2>
        {dates && <span className="person-season-when">{dates}</span>}
      </div>

      <h3 className="person-season-subhead">
        {entries.length === 1 ? labels.entry : labels.entryPlural}
      </h3>
      <div className="person-entry-chips">
        {entries.map(entryView => {
          const traits = entryTraits(entryView.entry);
          return (
            <a
              key={entryView.entry.id}
              className="person-entry-chip"
              href={`/routine/${entryView.entry.id}`}
            >
              <span>{entryView.entry.name}</span>
              {traits && <span className="person-entry-chip-traits">{traits}</span>}
            </a>
          );
        })}
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

      <h3 className="person-season-subhead">
        {appearances.length === 1 ? labels.appearance : labels.appearancePlural}
      </h3>
      {appearances.length === 0 ? (
        <div className="empty-state">
          <p>
            No {labels.appearancePlural.toLowerCase()} recorded this season — the{" "}
            {labels.entryPlural.toLowerCase()} above have not been to a {labels.event.toLowerCase()}{" "}
            yet.
          </p>
        </div>
      ) : (
        <ul className="event-list">
          {appearances.map(detail => (
            <li key={detail.appearance.id} className="event-item">
              <div className="event-item-main">
                {ownsPerson ? (
                  <a className="event-name" href={`/competition/${detail.event.id}`}>
                    {detail.event.name}
                  </a>
                ) : (
                  <span className="event-name">{detail.event.name}</span>
                )}
                <span className="event-meta">{appearanceWhen(detail)}</span>
                <span className="person-appearance-entry">
                  <a href={`/routine/${detail.entry.id}`}>{detail.entry.name}</a>
                </span>
                {(detail.results ?? []).length === 0 ? (
                  <span className="event-count">No results recorded</span>
                ) : (
                  <ResultList results={detail.results} people={people} />
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
  );
};
