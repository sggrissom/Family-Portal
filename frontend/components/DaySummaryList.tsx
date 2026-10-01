import * as preact from "preact";
import * as server from "../server";
import { Checkup, DayEvent, DaySummary, PhotoGroup, dayLabel } from "../lib/daySummary";
import { chipLabels } from "../lib/familyGroups";
import { getCategoryIcon } from "../lib/milestoneHelpers";
import { MilestoneText } from "./MilestoneText";
import { formatMeasurement } from "../lib/weightFormat";
import { ageInMonths, computePercentileLabel, isValidBirthday } from "../lib/growthPercentiles";
import { copy } from "../lib/copy";
import "./day-summary-styles";

interface DaySummaryListProps {
  days: DaySummary[];
  people: server.Person[];
  today: string;
  subjectId?: number;
}

export const DaySummaryList = ({ days, people, today, subjectId }: DaySummaryListProps) => {
  const byId = new Map(people.map(p => [p.id, p]));
  const names = chipLabels(people);
  const name = (id: number) => names.get(id) ?? "Someone";
  const who = (id: number) => (id === subjectId ? null : <strong>{name(id)} </strong>);

  return (
    <div className="day-summaries">
      {days.map(day => (
        <section key={day.day} className="day-summary">
          <h3 className="day-summary-label">{dayLabel(day.day, today)}</h3>
          {day.birthdays.map(b => (
            <div key={`b${b.personId}`} className="day-birthday">
              🎂 {copy.home.turned(name(b.personId), b.age)}
            </div>
          ))}
          {day.milestones.map(m => (
            <a key={`m${m.id}`} href={`/milestone/${m.id}`} className="day-milestone">
              <span className="day-milestone-icon" aria-hidden="true">
                {getCategoryIcon(m.category)}
              </span>
              <span>
                {who(m.personId)}
                <MilestoneText milestone={m} />
              </span>
            </a>
          ))}
          {day.events.map(e => (
            <EventCard key={`e${e.event.id}`} dayEvent={e} name={name} subjectId={subjectId} />
          ))}
          {day.checkups.map(c => (
            <CheckupRow
              key={`c${c.personId}`}
              checkup={c}
              person={byId.get(c.personId)}
              name={c.personId === subjectId ? null : name(c.personId)}
            />
          ))}
          {day.photos && <PhotoMosaic day={day.day} photos={day.photos} name={name} />}
        </section>
      ))}
    </div>
  );
};

function percentile(g: server.GrowthData, person: server.Person | undefined): string {
  if (!person || !isValidBirthday(person.birthday)) return "";
  const months = ageInMonths(person.birthday, g.measurementDate);
  if (months > 240) return "";
  return (
    computePercentileLabel(
      g.value,
      g.unit,
      months,
      person.gender,
      g.measurementType === server.Height ? "height" : "weight"
    ) ?? ""
  );
}

const EVENT_PHOTOS = 4;

const EventCard = ({
  dayEvent,
  name,
  subjectId,
}: {
  dayEvent: DayEvent;
  name: (id: number) => string;
  subjectId?: number;
}) => {
  const photoIds = dayEvent.appearances
    .flatMap(a => a.detail.photoIds ?? [])
    .slice(0, EVENT_PHOTOS);
  return (
    <a href={`/competition/${dayEvent.event.id}`} className="day-event">
      <span className="day-event-head">
        <span className="day-milestone-icon" aria-hidden="true">
          🏆
        </span>
        <strong>{dayEvent.event.name}</strong>
      </span>
      {dayEvent.appearances.map(({ detail, personIds }) => {
        const who = personIds.filter(id => id !== subjectId).map(name);
        const results = (detail.results ?? []).map(r => r.label).filter(l => l.trim());
        return (
          <span key={detail.appearance.id} className="day-event-line">
            {detail.entry.name}
            {who.length > 0 && ` (${who.join(", ")})`}
            {results.length > 0 && <strong> · {results.join(" · ")}</strong>}
          </span>
        );
      })}
      {photoIds.length > 0 && (
        <span className="day-event-photos">
          {photoIds.map(id => (
            <img key={id} src={`/api/photo/${id}/thumb`} alt="" loading="lazy" />
          ))}
        </span>
      )}
    </a>
  );
};

const CheckupRow = ({
  checkup,
  person,
  name,
}: {
  checkup: Checkup;
  person: server.Person | undefined;
  name: string | null;
}) => {
  const ids = [checkup.height, checkup.weight].filter(Boolean).map(g => g!.id);
  const months = (g: server.GrowthData) =>
    person && isValidBirthday(person.birthday)
      ? ageInMonths(person.birthday, g.measurementDate)
      : null;
  const parts = [checkup.height, checkup.weight]
    .filter((g): g is server.GrowthData => !!g)
    .map(g => {
      const pct = percentile(g, person);
      return `${formatMeasurement(g.value, g.unit, months(g))}${pct ? ` (${pct})` : ""}`;
    });

  return (
    <a href={`/view-growth/${ids.join(",")}`} className="day-checkup">
      <span className="day-milestone-icon" aria-hidden="true">
        📏
      </span>
      <span>
        {name ? (
          <>
            <strong>{name}</strong> {copy.home.checkup}
          </>
        ) : (
          copy.home.checkupTitle
        )}{" "}
        · {parts.join(" · ")}
      </span>
    </a>
  );
};

const PhotoMosaic = ({
  day,
  photos,
  name,
}: {
  day: string;
  photos: PhotoGroup;
  name: (id: number) => string;
}) => {
  const who = photos.personIds.map(name).join(", ");
  return (
    <a href={`/photos?from=${day}&to=${day}`} className="day-mosaic">
      <span className={`day-mosaic-grid count-${Math.min(photos.photoIds.length, 4)}`}>
        {photos.photoIds.map(id => (
          <img key={id} src={`/api/photo/${id}/thumb`} alt="" loading="lazy" />
        ))}
      </span>
      <span className="day-mosaic-caption">
        {copy.home.photoCount(photos.count)}
        {who && ` · ${who}`}
      </span>
    </a>
  );
};
