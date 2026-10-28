import * as preact from "preact";
import * as server from "../../server";
import { isRealDate } from "../../lib/dateUtils";
import { hasMatchup, recordLabel, recordText, runSummary, tallyOutcomes } from "../../lib/matchups";
import { adjudicationCounts, bestFinish, resultText } from "../../lib/activityResults";
import { ActivityLabels } from "./labels";
import { MatchupLine, MediaGrid, ResultBadges } from "./results";
import "./performance-card-styles";

const MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

export const DateBlock = ({ date }: { date: string }) => {
  if (!isRealDate(date)) return <span className="date-block date-block-empty">—</span>;
  const d = new Date(date);
  return (
    <span className="date-block">
      <small>{MONTHS[d.getUTCMonth()]}</small>
      <strong>{d.getUTCDate()}</strong>
    </span>
  );
};

export function groupByEvent(appearances: server.AppearanceDetail[]): server.AppearanceDetail[][] {
  const groups = new Map<string, server.AppearanceDetail[]>();
  for (const detail of appearances) {
    const key = `${detail.event.id}:${detail.entry.id}`;
    const group = groups.get(key);
    if (group) group.push(detail);
    else groups.set(key, [detail]);
  }
  return [...groups.values()];
}

export const PerformanceCard = ({
  group,
  people,
  showEntry,
  linkEvent,
  editable,
}: {
  group: server.AppearanceDetail[];
  people: server.Person[];
  showEntry: boolean;
  linkEvent: boolean;
  editable: boolean;
}): preact.ComponentChild => {
  const first = group[0];
  const event = first.event;
  const where = [event.location, event.host].filter(part => part).join(" · ");
  const summary = group.length > 1 ? runSummary(group.map(detail => detail.appearance)) : "";
  const photos = group.flatMap(detail => detail.photoIds ?? []);
  const blank = group.every(
    detail => (detail.results ?? []).length === 0 && !hasMatchup(detail.appearance)
  );

  return (
    <article className="story-card">
      <DateBlock date={event.startDate} />
      <div className="story-card-body">
        <header className="story-card-head">
          {linkEvent ? (
            <a className="story-card-event" href={`/competition/${event.id}`}>
              {event.name}
            </a>
          ) : (
            <span className="story-card-event">{event.name}</span>
          )}
          {where && <span className="story-card-where">{where}</span>}
          {showEntry && (
            <a className="story-card-entry" href={`/routine/${first.entry.id}`}>
              {first.entry.name}
            </a>
          )}
        </header>

        {summary && <p className="perf-run-summary">{summary}</p>}

        {group.map(detail => (
          <div key={detail.appearance.id} className="story-card-performance">
            <MatchupLine appearance={detail.appearance} />
            <ResultBadges results={detail.results} people={people} />
            {detail.appearance.notes && <p className="perf-notes">{detail.appearance.notes}</p>}
          </div>
        ))}

        {blank && <span className="perf-date">No results recorded</span>}

        <MediaGrid photoIds={photos} size="small" />

        {editable && linkEvent && (
          <a className="link-action" href={`/competition/${event.id}`}>
            {blank ? "Add results" : "Edit results"}
          </a>
        )}
      </div>
    </article>
  );
};

export const SeasonStats = ({
  appearances,
  labels,
}: {
  appearances: server.AppearanceDetail[];
  labels: ActivityLabels;
}): preact.ComponentChild => {
  const events = new Set(appearances.map(detail => detail.event.id)).size;
  const best = bestFinish(appearances);
  const tally = tallyOutcomes(appearances.map(detail => detail.appearance));
  const record = recordText(tally);
  const adjudications = adjudicationCounts(appearances.flatMap(detail => detail.results ?? []));
  if (events === 0) return null;

  return (
    <div className="stat-tiles">
      <div className="stat-tile">
        <small>{events === 1 ? labels.event : labels.eventPlural}</small>
        <strong>{events}</strong>
      </div>
      {best && (
        <a className="stat-tile" href={`/competition/${best.view.event.id}`}>
          <small>Best placement</small>
          <strong>{resultText(best.result)}</strong>
          <span>{best.view.event.name}</span>
        </a>
      )}
      {record && (
        <div className="stat-tile">
          <small>Record</small>
          <strong>{record}</strong>
          <span>{recordLabel(tally)}</span>
        </div>
      )}
      {adjudications.length > 0 && (
        <div className="stat-tile">
          <small>Adjudications</small>
          <strong>
            {adjudications[0].count}× {adjudications[0].label}
          </strong>
          {adjudications.length > 1 && (
            <span>
              {adjudications
                .slice(1)
                .map(row => `${row.count}× ${row.label}`)
                .join(" · ")}
            </span>
          )}
        </div>
      )}
    </div>
  );
};
