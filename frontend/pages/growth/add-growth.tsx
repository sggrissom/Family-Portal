import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as core from "vlens/core";
import * as server from "../../server";
import * as auth from "../../lib/authCache";
import { Header, Footer } from "../../layout";
import { requireAuthInView } from "../../lib/authHelpers";
import { getIdFromRoute } from "../../lib/routeHelpers";
import { timelineRequest } from "../../lib/photoPages";
import { NoFamilyMembersPage } from "../../components/NoFamilyMembersPage";
import { PersonChips, scrollSelectedChipIntoView } from "../../components/PersonChips";
import { WhenControl } from "../../components/WhenControl";
import { HeightField, WeightField } from "../../components/MeasurementFields";
import { chipOrder } from "../../lib/familyGroups";
import { copy } from "../../lib/copy";
import { When, newWhen, whenProblem, whenRequest } from "../../lib/when";
import {
  CheckupEntry,
  checkupValues,
  newEntry,
  defaultHeightUnit,
  defaultWeightUnit,
  describeLast,
  latestOf,
  rememberUnits,
} from "../../lib/checkup";
import { ageInMonths, formatAgeAtMeasurement, isValidBirthday } from "../../lib/growthPercentiles";
import {
  loadUnitPrefs,
  readLastPerson,
  returnPath,
  takeReturnPath,
  saveUnitPrefs,
  writeLastPerson,
} from "../../lib/addFlow";
import "../../components/entry-form-styles";

type AddGrowthData = {
  people: server.Person[];
  growth: Map<number, server.GrowthData[]>;
};

export async function fetch(route: string, prefix: string): Promise<rpc.Response<AddGrowthData>> {
  const [resp, err] = await server.GetFamilyTimeline(
    timelineRequest({ skipMilestones: true, skipPhotos: true })
  );
  if (!resp) return [null, err];

  const people = chipOrder(
    resp.people.map(item => item.person).filter(person => auth.canContribute(person.familyId)),
    resp.relations || [],
    auth.getAuth()?.familyId ?? 0
  );
  const growth = new Map(resp.people.map(item => [item.person.id, item.growthData || []]));
  return [{ people, growth }, ""];
}

type CheckupForm = {
  personId: number | null;
  entry: CheckupEntry;
  when: When;
  defaultsApplied: boolean;
  error: string;
  saving: boolean;
};

const useCheckupForm = vlens.declareHook(
  (initialPersonId: number | null): CheckupForm => ({
    personId: initialPersonId,
    entry: newEntry(),
    when: newWhen(),
    defaultsApplied: false,
    error: "",
    saving: false,
  })
);

export function view(route: string, prefix: string, data: AddGrowthData): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth) {
    return;
  }

  if (data.people.length === 0) {
    return (
      <NoFamilyMembersPage
        message="Please add family members before tracking growth data"
        containerClass="entry-container"
      />
    );
  }

  const fromRoute = getIdFromRoute(route);
  const initial = [fromRoute, readLastPerson()].find(
    id => id !== null && data.people.some(p => p.id === id)
  );
  const form = useCheckupForm(initial ?? null);
  if (!form.defaultsApplied) {
    form.defaultsApplied = true;
    applyDefaultUnits(form, data);
    scrollSelectedChipIntoView();
  }

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="entry-container">
        <CheckupPage form={form} data={data} />
      </main>
      <Footer />
    </div>
  );
}

function personAgeMonths(person: server.Person | undefined, at: string | Date): number | null {
  if (!person || !isValidBirthday(person.birthday)) return null;
  return ageInMonths(person.birthday, at);
}

function applyDefaultUnits(form: CheckupForm, data: AddGrowthData) {
  if (form.personId === null) return;
  const prefs = loadUnitPrefs();
  const person = data.people.find(p => p.id === form.personId);
  form.entry.heightUnit = defaultHeightUnit(
    prefs,
    form.personId,
    data.growth.get(form.personId) ?? []
  );
  form.entry.weightUnit = defaultWeightUnit(
    prefs,
    form.personId,
    personAgeMonths(person, new Date())
  );
}

function choosePerson(form: CheckupForm, data: AddGrowthData, personId: number) {
  form.personId = form.personId === personId ? null : personId;
  form.error = "";
  applyDefaultUnits(form, data);
  vlens.scheduleRedraw();
}

async function saveCheckup(form: CheckupForm, event: Event) {
  event.preventDefault();
  if (form.saving) return;

  if (form.personId === null) {
    form.error = copy.measurement.pickPerson;
    vlens.scheduleRedraw();
    return;
  }
  const values = checkupValues(form.entry);
  const problem = values.error || whenProblem(form.when);
  if (problem) {
    form.error = problem;
    vlens.scheduleRedraw();
    return;
  }

  form.saving = true;
  form.error = "";
  vlens.scheduleRedraw();

  const personId = form.personId;
  const when = whenRequest(form.when, new Date());
  const [resp, err] = await server.AddCheckup({
    personId,
    inputType: when.inputType,
    measurementDate: when.date,
    ageYears: when.ageYears,
    ageMonths: when.ageMonths,
    height: values.height,
    weight: values.weight,
  });
  if (!resp) {
    form.error = err || "That checkup could not be saved.";
    form.saving = false;
    vlens.scheduleRedraw();
    return;
  }

  saveUnitPrefs(rememberUnits(loadUnitPrefs(), personId, values, form.entry));
  writeLastPerson(personId);
  takeReturnPath("");
  core.setRoute(`/view-growth/${resp.growthData.map(g => g.id).join(",")}`);
}

function cancel(event: Event) {
  event.preventDefault();
  core.setRoute(takeReturnPath("/dashboard"));
}

const CheckupPage = ({ form, data }: { form: CheckupForm; data: AddGrowthData }) => {
  const person = data.people.find(p => p.id === form.personId);
  const growth = form.personId !== null ? (data.growth.get(form.personId) ?? []) : [];
  const now = new Date();
  const ageNow = personAgeMonths(person, now);
  const lastHeight = latestOf(growth, server.Height);
  const lastWeight = latestOf(growth, server.Weight);
  const disabled = form.saving;

  return (
    <div className="entry-card">
      <h1 className="entry-title">{copy.measurement.title}</h1>

      <PersonChips
        people={data.people}
        selected={form.personId !== null ? [form.personId] : []}
        onToggle={vlens.cachePartial(choosePerson, form, data)}
        label={copy.measurement.who}
        disabled={disabled}
      />

      <form className="entry-form" onSubmit={vlens.cachePartial(saveCheckup, form)} noValidate>
        <div className="entry-subject">
          <span className="entry-subject-name">
            {person ? (
              <>
                <strong>{person.name}</strong>
                {ageNow !== null && ageNow >= 0 && (
                  <span className="entry-subject-age"> · {formatAgeAtMeasurement(ageNow)}</span>
                )}
              </>
            ) : (
              <span className="entry-subject-missing">{copy.measurement.pickPerson}</span>
            )}
          </span>
          <WhenControl when={form.when} disabled={disabled} />
        </div>

        {form.error && (
          <div className="error-message" role="alert">
            {form.error}
          </div>
        )}

        <HeightField
          entry={form.entry}
          disabled={disabled}
          note={
            lastHeight
              ? describeLast(lastHeight, personAgeMonths(person, lastHeight.measurementDate), now)
              : ""
          }
        />
        <WeightField
          entry={form.entry}
          disabled={disabled}
          note={
            lastWeight
              ? describeLast(lastWeight, personAgeMonths(person, lastWeight.measurementDate), now)
              : ""
          }
        />

        <div className="entry-actions">
          <a href={returnPath("/dashboard")} className="btn btn-secondary" onClick={cancel}>
            {copy.measurement.cancel}
          </a>
          <button type="submit" className="btn btn-primary" disabled={disabled}>
            {form.saving ? copy.measurement.saving : copy.measurement.save}
          </button>
        </div>
      </form>
    </div>
  );
};
