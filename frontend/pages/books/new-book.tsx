import * as preact from "preact";
import * as vlens from "vlens";
import * as core from "vlens/core";
import * as rpc from "vlens/rpc";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView } from "../../lib/authHelpers";
import { SegmentedControl } from "../../components/SegmentedControl";
import {
  CATEGORIES,
  Category,
<<<<<<< HEAD
  DENSITIES,
  Density,
=======
  DENSITY_OPTIONS,
  Density,
  MATCH_OPTIONS,
>>>>>>> main
  PRESETS,
  addDays,
  bookDates,
  dayOf,
  draftSelection,
  firstBirthday,
<<<<<<< HEAD
=======
  toggled,
>>>>>>> main
} from "../../lib/book";
import {
  Period,
  YEAR_KINDS,
  YearKind,
  ageNow,
  bornBy,
  calendarYear,
  defaultTitle,
  hasBirthday,
  pastYear,
  yearOfAge,
} from "../../lib/bookPlans";
import { localDateString } from "../../lib/when";
import "./book-styles";
import "./edit-book-styles";

export const PRESET_CHOICES = [
  {
    value: PRESETS.firstYear,
    label: "First year",
    blurb: "Birth to the first birthday, month by month.",
  },
  {
    value: PRESETS.year,
    label: "A year of one person",
    blurb: "A calendar year, the past twelve months, or a year of age.",
  },
  {
    value: PRESETS.familyYear,
    label: "Family yearbook",
    blurb: "Shared moments through the year, then a section for each person.",
  },
  {
    value: PRESETS.custom,
    label: "Custom",
    blurb: "Any people, any dates.",
  },
];

type Preset = (typeof PRESET_CHOICES)[number]["value"];

const isPreset = (value: string | null): value is Preset =>
  PRESET_CHOICES.some(p => p.value === value);

type NewBookData = { people: server.Person[]; preset: Preset };

export async function fetch(route: string, prefix: string): Promise<rpc.Response<NewBookData>> {
  const params = new URLSearchParams(route.split("?")[1] ?? "");
  const preset = params.get("preset");
  const [resp, err] = await server.ListPeople({});
  if (!resp) return [null, err || "Failed to load people"];
  return rpc.ok({
    people: resp.people ?? [],
    preset: isPreset(preset) ? preset : PRESETS.firstYear,
  });
}

type Form = {
  personIds: number[];
  yearKind: YearKind;
  year: number;
  age: number;
  start: string;
  lastDay: string;
  match: string;
  categories: Category[];
  showGrowth: boolean;
  density: Density;
  title: string;
  titleTouched: boolean;
  busy: boolean;
  error: string;
};

const useForm = vlens.declareHook((data: NewBookData, familyId: number, today: string): Form => {
  const preset = data.preset;
  const people = eligiblePeople(data, familyId, today);
  const year = parseInt(today.slice(0, 4));
  const withBirthdays = people.filter(hasBirthday);
  const youngest = [...withBirthdays].sort((a, b) => b.birthday.localeCompare(a.birthday))[0];
  const familyYear = today.slice(5, 7) === "12" ? year : year - 1;
  return {
    personIds:
      preset === PRESETS.familyYear
        ? withBirthdays.filter(p => bornBy(p, `${familyYear + 1}-01-01`)).map(p => p.id)
        : youngest
          ? [youngest.id]
          : [],
    yearKind: "calendar",
    year: preset === PRESETS.familyYear ? familyYear : year,
    age: 0,
    start: addDays(today, -90),
    lastDay: today,
    match: "any",
    categories: CATEGORIES.map(c => c.value),
    showGrowth: preset !== PRESETS.familyYear,
    density: "balanced",
    title: "",
    titleTouched: false,
    busy: false,
    error: "",
  };
});

function update(form: Form, patch: Partial<Form>) {
  Object.assign(form, patch);
  form.error = "";
  vlens.scheduleRedraw();
}

function periodOf(
  preset: Preset,
  form: Form,
  chosen: server.Person[],
  today: string
): Period | null {
  const first = chosen[0];
  switch (preset) {
    case PRESETS.firstYear:
      return first ? { start: dayOf(first.birthday), end: firstBirthday(first.birthday) } : null;
    case PRESETS.year:
      if (!first) return null;
      if (form.yearKind === "past") return pastYear(today);
      if (form.yearKind === "age") return yearOfAge(first.birthday, form.age);
      return calendarYear(form.year);
    case PRESETS.familyYear:
      return calendarYear(form.year);
    default:
      if (!form.start || !form.lastDay || form.lastDay < form.start) return null;
      return { start: form.start, end: addDays(form.lastDay, 1) };
  }
}

async function create(
  form: Form,
  preset: Preset,
  chosen: server.Person[],
  period: Period,
  title: string
) {
  update(form, { busy: true });
  const request = {
    personIds: chosen.map(p => p.id),
    preset,
    startDate: period.start,
    endDate: period.end,
  };
  const [sources, err] = await server.GetBookSources(request);
  if (!sources) {
    update(form, { busy: false });
    form.error = err || "Could not gather the records";
    return;
  }
  const categories = form.categories.length === CATEGORIES.length ? [] : form.categories;
  const draft = draftSelection(sources.sources, {
    preset,
    title,
    startDate: period.start,
    endDate: period.end,
    categories,
    match: form.match,
    showGrowth: form.showGrowth,
    density: form.density,
  });
  const [created, createErr] = await server.CreateBook({
    ...request,
    content: {
      title,
      coverPhotoId: draft.coverPhotoId,
      density: form.density,
      categories,
      match: form.match,
      introduction: "",
      letter: "",
      signature: "",
      showGrowth: form.showGrowth,
      items: draft.items,
      excluded: [],
      reviewedAt: new Date().toISOString(),
    },
  });
  if (!created) {
    update(form, { busy: false });
    form.error = createErr || "Could not create the book";
    return;
  }
  core.setRoute(`/book/${created.book.id}`);
}

export function view(route: string, prefix: string, data: NewBookData): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth) return;
  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="book-editor">
        <NewBook data={data} familyId={currentAuth.familyId} />
      </main>
      <Footer />
    </div>
  );
}

const eligiblePeople = (data: NewBookData, familyId: number, today: string) =>
  data.people.filter(p => p.familyId === familyId && hasBirthday(p) && dayOf(p.birthday) <= today);

const NewBook = ({ data, familyId }: { data: NewBookData; familyId: number }) => {
  const today = localDateString(new Date());
  const people = eligiblePeople(data, familyId, today);
  const form = useForm(data, familyId, today);
  const preset = data.preset;
  const single = preset === PRESETS.firstYear || preset === PRESETS.year;
  const chosen = people.filter(p => form.personIds.includes(p.id));
  const period = periodOf(preset, form, chosen, today);
  const title =
    form.titleTouched || !period || !chosen.length
      ? form.title
      : defaultTitle(preset, chosen, period, form.yearKind, form.age);
  const choice = PRESET_CHOICES.find(p => p.value === preset)!;
  const firstYear = new Date(
    Math.min(...people.map(p => Date.parse(p.birthday)), Date.now())
  ).getUTCFullYear();
  const years = Array.from(
    { length: parseInt(today.slice(0, 4)) - firstYear + 1 },
    (_, i) => parseInt(today.slice(0, 4)) - i
  );
  const notBornYet = chosen.filter(p => period && dayOf(p.birthday) >= period.end);
  const ready = !!period && chosen.length > 0 && !form.busy && notBornYet.length === 0;

  const togglePerson = (id: number) => {
    if (single) update(form, { personIds: [id], age: 0 });
<<<<<<< HEAD
    else
      update(form, {
        personIds: form.personIds.includes(id)
          ? form.personIds.filter(p => p !== id)
          : [...form.personIds, id],
      });
=======
    else update(form, { personIds: toggled(form.personIds, id) });
>>>>>>> main
  };

  return (
    <>
      <div className="book-editor-bar">
        <a href="/books">← Books</a>
      </div>
      <section className="book-editor-section">
        <h1>{choice.label}</h1>
        <p className="book-editor-hint">{choice.blurb}</p>
      </section>

      <section className="book-editor-section">
        <h2>{single ? "Whose book?" : "Who's in it?"}</h2>
        {people.length === 0 ? (
          <p className="book-editor-hint">Add someone with a birthday first.</p>
        ) : (
          <div className="book-new-people" role={single ? "radiogroup" : "group"}>
            {people.map(p => (
              <label key={p.id} className="book-editor-check">
                <input
                  type={single ? "radio" : "checkbox"}
                  name="book-people"
                  checked={form.personIds.includes(p.id)}
                  onChange={() => togglePerson(p.id)}
                />
                {p.name}
              </label>
            ))}
          </div>
        )}
        {!single && chosen.length > 1 && (
          <div className="book-editor-row">
            <SegmentedControl
              label="Which photos"
<<<<<<< HEAD
              options={[
                { value: "any", label: "Photos of any of them" },
                { value: "all", label: "Only photos of all of them" },
              ]}
=======
              options={MATCH_OPTIONS}
>>>>>>> main
              value={form.match}
              onChange={match => update(form, { match })}
            />
          </div>
        )}
      </section>

      {preset !== PRESETS.firstYear && (
        <section className="book-editor-section">
          <h2>When</h2>
          {preset === PRESETS.year && (
            <div className="book-editor-row">
              <SegmentedControl
                label="Which year"
                options={YEAR_KINDS}
                value={form.yearKind}
                onChange={yearKind => update(form, { yearKind })}
              />
            </div>
          )}
          {(preset === PRESETS.familyYear ||
            (preset === PRESETS.year && form.yearKind === "calendar")) && (
            <label className="book-editor-field">
              <span>Year</span>
              <select
                value={form.year}
                onChange={(e: Event) =>
                  update(form, { year: parseInt((e.target as HTMLSelectElement).value) })
                }
              >
                {years.map(y => (
                  <option key={y} value={y}>
                    {y}
                  </option>
                ))}
              </select>
            </label>
          )}
          {preset === PRESETS.year && form.yearKind === "age" && chosen[0] && (
            <label className="book-editor-field">
              <span>Age</span>
              <select
                value={form.age}
                onChange={(e: Event) =>
                  update(form, { age: parseInt((e.target as HTMLSelectElement).value) })
                }
              >
                {Array.from({ length: ageNow(chosen[0].birthday, today) + 1 }, (_, age) => (
                  <option key={age} value={age}>
                    {age === 0 ? "Birth to one" : `${age} to ${age + 1}`}
                  </option>
                ))}
              </select>
            </label>
          )}
          {preset === PRESETS.custom && (
            <div className="book-editor-row">
              <label className="book-editor-field">
                <span>From</span>
                <input
                  type="date"
                  value={form.start}
                  onInput={(e: Event) =>
                    update(form, { start: (e.target as HTMLInputElement).value })
                  }
                />
              </label>
              <label className="book-editor-field">
                <span>Through</span>
                <input
                  type="date"
                  value={form.lastDay}
                  onInput={(e: Event) =>
                    update(form, { lastDay: (e.target as HTMLInputElement).value })
                  }
                />
              </label>
            </div>
          )}
        </section>
      )}

      <p className="book-new-dates" aria-live="polite">
        {period ? `Covers ${bookDates(period.start, period.end)}` : "Choose who and when."}
        {notBornYet.length > 0 &&
          ` — ${notBornYet.map(p => p.name).join(" and ")} ${notBornYet.length > 1 ? "were" : "was"} not born yet.`}
      </p>

      <section className="book-editor-section">
        <h2>What to include</h2>
        <div className="book-new-people">
          {CATEGORIES.map(c => (
            <label key={c.value} className="book-editor-check">
              <input
                type="checkbox"
                checked={form.categories.includes(c.value)}
<<<<<<< HEAD
                onChange={() =>
                  update(form, {
                    categories: form.categories.includes(c.value)
                      ? form.categories.filter(x => x !== c.value)
                      : [...form.categories, c.value],
                  })
                }
=======
                onChange={() => update(form, { categories: toggled(form.categories, c.value) })}
>>>>>>> main
              />
              {c.label}
            </label>
          ))}
          <label className="book-editor-check">
            <input
              type="checkbox"
              checked={form.showGrowth}
              onChange={() => update(form, { showGrowth: !form.showGrowth })}
            />
            Growth
          </label>
        </div>
        <div className="book-editor-row">
          <SegmentedControl
            label="How many photos"
<<<<<<< HEAD
            options={DENSITIES.map(d => ({ value: d, label: d[0].toUpperCase() + d.slice(1) }))}
=======
            options={DENSITY_OPTIONS}
>>>>>>> main
            value={form.density}
            onChange={density => update(form, { density })}
          />
        </div>
      </section>

      <section className="book-editor-section">
        <label className="book-editor-field">
          <span>Title</span>
          <input
            type="text"
            value={title}
            maxLength={120}
            onInput={(e: Event) =>
              update(form, { title: (e.target as HTMLInputElement).value, titleTouched: true })
            }
          />
        </label>
        {form.error && (
          <p className="error-message" role="alert">
            {form.error}
          </p>
        )}
        <button
          type="button"
          className="btn btn-primary"
          disabled={!ready || form.categories.length === 0}
          onClick={() => period && create(form, preset, chosen, period, title)}
        >
          {form.busy ? "Gathering…" : "Make the draft"}
        </button>
      </section>
    </>
  );
};
