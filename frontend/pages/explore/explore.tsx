import {
  Baby,
  BookOpen,
  CalendarDays,
  ChartNoAxesCombined,
  Images,
  MessageCircle,
  Mic,
  Trophy,
} from "lucide-preact";
import type { LucideIcon } from "lucide-preact";
import * as preact from "preact";
import * as rpc from "vlens/rpc";
import * as server from "../../server";
import * as auth from "../../lib/authCache";
import { Header, Footer } from "../../layout";
import { Icon } from "../../components/Icon";
import { FaceCrop } from "../../components/FaceCrop";
import { ThumbnailImage } from "../../components/ResponsiveImage";
import { MemoryTiles } from "../../components/MemoryCard";
import { ensureAuthInFetch, requireAuthInView } from "../../lib/authHelpers";
import type { Destination } from "../../lib/appNav";
import { chipLabels, chipOrder } from "../../lib/familyGroups";
import { localDateString } from "../../lib/dateUtils";
import { firstName } from "../../lib/interviews";
import { currentVisit, memoryHref } from "../../lib/memories";
import { isExactRecord, recordTimeLabel } from "../../lib/recordTime";
import { ageHeading, invitingAge, portraitOrder, sameAgePath } from "../../lib/sameAge";
import { copy } from "../../lib/copy";
import "./explore-styles";
import { PageHeader } from "../../components/PageHeader";

const FEATURES: { key: Destination; href: string; icon: LucideIcon }[] = [
  { key: "photos", href: "/photos", icon: Images },
  { key: "history", href: "/history", icon: CalendarDays },
  { key: "growth", href: "/growth", icon: ChartNoAxesCombined },
  { key: "activities", href: "/activities", icon: Trophy },
  { key: "interviews", href: "/interviews", icon: Mic },
  { key: "books", href: "/books", icon: BookOpen },
  { key: "sameAge", href: "/same-age", icon: Baby },
  { key: "chat", href: "/chat", icon: MessageCircle },
];

type ExploreData = {
  people: server.Person[];
  sameAge: server.GetSameAgeResponse | null;
  said: server.Memory[];
  years: { person: server.Person; memories: server.Memory[] } | null;
};

const SAID_COUNT = 4;
const MIN_YEARS_OLD = 3;

const yearsOld = (birthday: string) =>
  (Date.now() - new Date(birthday).getTime()) / (365.25 * 24 * 60 * 60 * 1000);
const YEARS_COUNT = 6;

function memoryRequest(fields: Partial<server.GetMemoriesRequest>): server.GetMemoriesRequest {
  const visit = currentVisit();
  return {
    today: localDateString(new Date()),
    seed: visit.seed,
    seen: [],
    limit: 12,
    kinds: [],
    personId: 0,
    recentlyAdded: false,
    ...fields,
  };
}

const yearLabel = (memory: server.Memory) =>
  isExactRecord(memory.recordTime)
    ? String(new Date(memory.date).getUTCFullYear())
    : recordTimeLabel(memory.recordTime);

const pictured = (sameAge: server.GetSameAgeResponse) =>
  (sameAge.rows ?? []).filter(row => (row.portraits ?? []).length > 0);

const pick = <T,>(list: T[], seed: number): T | undefined =>
  list.length > 0 ? list[seed % list.length] : undefined;

export async function fetch(route: string, prefix: string): Promise<rpc.Response<ExploreData>> {
  const empty: ExploreData = { people: [], sameAge: null, said: [], years: null };
  if (!(await ensureAuthInFetch())) return rpc.ok(empty);

  const seed = currentVisit().seed;
  const today = localDateString(new Date());
  const [[roster], [said], [ages]] = await Promise.all([
    server.ListPeople({}),
    server.GetMemories(memoryRequest({ kinds: ["quote"], limit: SAID_COUNT })),
    server.GetSameAge({
      includeAvailableAges: true,
      details: false,
      ageMonths: null,
      fromPersonId: 0,
      today,
    }),
  ]);

  const people = chipOrder(
    roster?.people ?? [],
    roster?.relations ?? [],
    auth.getAuth()?.familyId ?? 0
  ).filter(p => !p.isPregnancy && p.birthday && yearsOld(p.birthday) >= MIN_YEARS_OLD);

  const shared = (options: server.SameAgeOption[] | null | undefined) =>
    (options ?? []).filter(a => a.peopleCount >= 2 && invitingAge(a.ageMonths));
  const age = pick(shared(ages?.portraitAges), seed);
  const person = pick(people, seed);

  const [[sameAge], [years]] = await Promise.all([
    !age
      ? Promise.resolve([null] as const)
      : age.ageMonths === ages?.ageMonths
        ? Promise.resolve([ages] as const)
        : server.GetSameAge({
            includeAvailableAges: false,
            details: false,
            ageMonths: age.ageMonths,
            fromPersonId: 0,
            today,
          }),
    person
      ? server.GetMemories(memoryRequest({ personId: person.id, limit: 24 }))
      : Promise.resolve([null] as const),
  ]);

  const byYear = new Map<string, server.Memory>();
  for (const memory of years?.memories ?? []) {
    const year = yearLabel(memory);
    if ((memory.photoIds ?? []).length > 0 && !byYear.has(year)) byYear.set(year, memory);
  }
  const throughYears = [...byYear.values()]
    .slice(0, YEARS_COUNT)
    .sort((a, b) => (a.recordTime.start < b.recordTime.start ? -1 : 1));

  return rpc.ok({
    people: roster?.people ?? [],
    sameAge: sameAge && pictured(sameAge).length >= 2 ? sameAge : null,
    said: said?.memories ?? [],
    years: person && throughYears.length >= 2 ? { person, memories: throughYears } : null,
  });
}

export function view(route: string, prefix: string, data: ExploreData): preact.ComponentChild {
  if (!requireAuthInView()) return;
  const people = new Map(data.people.map(p => [p.id, p]));
  const hasPreviews = data.sameAge || data.said.length > 0 || data.years;

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="explore-page">
        <PageHeader title={copy.explore.title} lede={copy.explore.lede} />
        <FeatureDirectory />
        {hasPreviews && (
          <div className="explore-previews">
            <h2 className="explore-previews-title">{copy.explore.previews}</h2>
            {data.sameAge && <SameAgePreview sameAge={data.sameAge} />}
            {data.said.length > 0 && (
              <section className="explore-preview">
                <div className="explore-preview-head">
                  <h3>{copy.explore.said}</h3>
                </div>
                <MemoryTiles memories={data.said} people={people} />
              </section>
            )}
            {data.years && <YearsPreview {...data.years} />}
          </div>
        )}
      </main>
      <Footer />
    </div>
  );
}

const FeatureDirectory = () => (
  <nav className="explore-directory" aria-label={copy.explore.title}>
    {FEATURES.map(feature => (
      <a key={feature.key} href={feature.href}>
        <span className="explore-directory-icon" aria-hidden="true">
          <Icon symbol={feature.icon} />
        </span>
        <span className="explore-directory-label">
          <strong>{copy.nav[feature.key]}</strong>
          <span>{copy.explore.blurbs[feature.key]}</span>
        </span>
      </a>
    ))}
  </nav>
);

const SameAgePreview = ({ sameAge }: { sameAge: server.GetSameAgeResponse }) => {
  const rows = portraitOrder(pictured(sameAge));
  const names = chipLabels(rows.map(r => r.person));
  const href = sameAgePath(sameAge.ageMonths, 0);
  return (
    <section className="explore-preview">
      <div className="explore-preview-head">
        <h3>{ageHeading(sameAge.ageMonths)}</h3>
        <a href={href}>{copy.explore.compare}</a>
      </div>
      <a href={href} className="explore-faces">
        {rows.map(row => (
          <span key={row.person.id} className="explore-face">
            <FaceCrop
              photoId={row.portraits[0].photoId}
              box={row.portraits[0].box}
              size={84}
              alt=""
            />
            <span>{names.get(row.person.id) ?? firstName(row.person.name)}</span>
          </span>
        ))}
      </a>
    </section>
  );
};

const YearsPreview = ({
  person,
  memories,
}: {
  person: server.Person;
  memories: server.Memory[];
}) => (
  <section className="explore-preview">
    <div className="explore-preview-head">
      <h3>{copy.explore.throughTheYears(firstName(person.name))}</h3>
      <a href={`/profile/${person.id}`}>{copy.explore.seeStory}</a>
    </div>
    <div className="explore-years">
      {memories.map(memory => (
        <a key={memory.key} href={memoryHref(memory)} className="explore-year">
          <ThumbnailImage photoId={memory.photoIds[0]} alt={memory.text} />
          <span>{yearLabel(memory)}</span>
        </a>
      ))}
    </div>
  </section>
);
