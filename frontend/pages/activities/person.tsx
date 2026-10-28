import { ChevronRight } from "lucide-preact";
import { Icon } from "../../components/Icon";
import * as preact from "preact";
import * as server from "../../server";
import * as auth from "../../lib/authCache";
import { formatDateRange, toDateInputValue } from "../../lib/dateUtils";
import { localDateString } from "../../lib/when";
import { seasonPhase } from "../../lib/activityHome";
import { activityKindIcon, activityKindName, labelsForKind } from "./labels";
import { PerformanceCard, SeasonStats, groupByEvent } from "./performance-card";
import "./activities-styles";
import "./season-styles";
import "./person-styles";

export type PersonActivitiesData = {
  season: server.GetPersonSeasonResponse;
  people: server.Person[];
};

function entryTraits(entry: server.Entry): string {
  return [entry.format, entry.style, entry.division, entry.level].filter(part => part).join(" · ");
}

export const PersonActivities = ({ data }: { data: PersonActivitiesData }) => {
  const personId = data.season.personId;
  const person = data.people.find(p => p.id === personId);
  const name = person?.name ?? "This person";

  const ownsPerson = !!person && auth.canContribute(person.familyId);

  const seasons = data.season.seasons ?? [];
  const entries = data.season.entries ?? [];
  const appearances = data.season.appearances ?? [];
  const today = localDateString(new Date());
  const isPast = (season: server.SeasonSummary) =>
    seasonPhase(toDateInputValue(season.startDate), toDateInputValue(season.endDate), today) ===
    "past";
  const current = seasons.filter(season => !isPast(season));
  const past = seasons.filter(isPast);

  const group = (season: server.SeasonSummary, open: boolean) => (
    <SeasonGroup
      key={season.id}
      season={season}
      entries={entries.filter(entry => entry.entry.seasonId === season.id)}
      appearances={appearances.filter(detail => detail.entry.seasonId === season.id)}
      people={data.people}
      ownsPerson={ownsPerson}
      open={open}
    />
  );

  return seasons.length === 0 ? (
    <div className="empty-state">
      <p>{name} is not on any roster yet.</p>
      {ownsPerson && (
        <a className="btn btn-primary" href="/activities">
          Go to activities
        </a>
      )}
    </div>
  ) : (
    <>
      {current.map(season => group(season, true))}
      {past.length > 0 && (
        <>
          <h2 className="activities-heading person-past-heading">Past seasons</h2>
          {past.map(season => group(season, false))}
        </>
      )}
    </>
  );
};

const SeasonGroup = ({
  season,
  entries,
  appearances,
  people,
  ownsPerson,
  open,
}: {
  season: server.SeasonSummary;
  entries: server.EntryView[];
  appearances: server.AppearanceDetail[];
  people: server.Person[];
  ownsPerson: boolean;
  open: boolean;
}): preact.ComponentChild => {
  const labels = labelsForKind(season.kind);
  const dates = formatDateRange(season.startDate, season.endDate);
  const groups = groupByEvent(appearances);

  const body = (
    <>
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

      <SeasonStats appearances={appearances} labels={labels} />

      {groups.length === 0 ? (
        <p className="form-hint">
          No {labels.appearancePlural.toLowerCase()} recorded this season yet.
        </p>
      ) : (
        <div className="story-cards">
          {groups.map(rows => (
            <PerformanceCard
              key={rows[0].appearance.id}
              group={rows}
              people={people}
              showEntry={entries.length > 1}
              linkEvent={ownsPerson}
              editable={ownsPerson}
            />
          ))}
        </div>
      )}
    </>
  );

  const title = (
    <span className="person-season-title">
      <span className="home-season-program">
        <Icon symbol={activityKindIcon(season.kind)} /> {activityKindName(season.kind)}
      </span>
      <strong>{season.name}</strong>
      {dates && <span className="person-season-when">{dates}</span>}
    </span>
  );

  if (!open) {
    return (
      <details className="person-season-group person-season-past">
        <summary>
          {title}
          <Icon symbol={ChevronRight} />
        </summary>
        {ownsPerson && (
          <a className="link-action" href={`/season/${season.id}`}>
            Open the season
          </a>
        )}
        {body}
      </details>
    );
  }

  return (
    <section className="person-season-group">
      <div className="person-season-head">
        {ownsPerson ? (
          <a className="season-link" href={`/season/${season.id}`}>
            {title}
          </a>
        ) : (
          title
        )}
      </div>
      {body}
    </section>
  );
};
