import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import { Ref } from "vlens/refs";
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
import { SegmentedControl } from "../../components/SegmentedControl";
import { chipOrder } from "../../lib/familyGroups";
import { copy } from "../../lib/copy";
import { When, newWhen, whenProblem, whenRequest } from "../../lib/when";
import {
  CheckupEntry,
  CheckupMeasurement,
  HEIGHT_UNITS,
  HeightUnit,
  WEIGHT_UNITS,
  WeightUnit,
  checkupMeasurements,
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
    resp.people.map(item => item.person),
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
    entry: {
      heightUnit: "in",
      height: "",
      feet: "",
      inches: "",
      weightUnit: "lb",
      weight: "",
      pounds: "",
      ounces: "",
    },
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

function setHeightUnit(form: CheckupForm, unit: HeightUnit) {
  form.entry.heightUnit = unit;
  form.entry.height = "";
  form.entry.feet = "";
  form.entry.inches = "";
  vlens.scheduleRedraw();
}

function setWeightUnit(form: CheckupForm, unit: WeightUnit) {
  form.entry.weightUnit = unit;
  form.entry.weight = "";
  form.entry.pounds = "";
  form.entry.ounces = "";
  vlens.scheduleRedraw();
}

function clearSaved(entry: CheckupEntry, type: "height" | "weight") {
  if (type === "height") {
    entry.height = entry.feet = entry.inches = "";
  } else {
    entry.weight = entry.pounds = entry.ounces = "";
  }
}

async function saveCheckup(form: CheckupForm, event: Event) {
  event.preventDefault();
  if (form.saving) return;

  if (form.personId === null) {
    form.error = copy.measurement.pickPerson;
    vlens.scheduleRedraw();
    return;
  }
  const { measurements, error } = checkupMeasurements(form.entry);
  const problem = error || whenProblem(form.when);
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
  const savedIds: number[] = [];
  const saved: CheckupMeasurement[] = [];

  for (const m of measurements) {
    const [resp, err] = await server.AddGrowthData({
      personId,
      measurementType: m.measurementType,
      value: m.value,
      unit: m.unit,
      inputType: when.inputType,
      measurementDate: when.date,
      ageYears: when.ageYears,
      ageMonths: when.ageMonths,
    });
    if (!resp) {
      saved.forEach(s => clearSaved(form.entry, s.measurementType));
      const kept = saved.map(s => copy.measurement[s.measurementType].toLowerCase());
      form.error =
        (kept.length ? `The ${kept.join(" and ")} was saved. ` : "") +
        (err || "That measurement could not be saved.");
      form.saving = false;
      vlens.scheduleRedraw();
      return;
    }
    savedIds.push(resp.growthData.id);
    saved.push(m);
  }

  saveUnitPrefs(rememberUnits(loadUnitPrefs(), personId, measurements, form.entry));
  writeLastPerson(personId);
  takeReturnPath("");
  core.setRoute(`/view-growth/${savedIds.join(",")}`);
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

        <div className="checkup-field">
          <div className="checkup-field-head">
            <label htmlFor={form.entry.heightUnit === "ft-in" ? "feet" : "height"}>
              {copy.measurement.height}
            </label>
            <SegmentedControl
              label={copy.measurement.heightUnit}
              options={HEIGHT_UNITS}
              value={form.entry.heightUnit}
              onChange={vlens.cachePartial(setHeightUnit, form)}
              disabled={disabled}
            />
          </div>
          {form.entry.heightUnit === "ft-in" ? (
            <div className="checkup-inputs">
              <UnitInput
                id="feet"
                suffix="ft"
                label={copy.measurement.feet}
                bind={vlens.ref(form.entry, "feet")}
                disabled={disabled}
              />
              <UnitInput
                id="inches"
                suffix="in"
                label={copy.measurement.inches}
                bind={vlens.ref(form.entry, "inches")}
                disabled={disabled}
              />
            </div>
          ) : (
            <div className="checkup-inputs">
              <UnitInput
                id="height"
                suffix={form.entry.heightUnit}
                bind={vlens.ref(form.entry, "height")}
                disabled={disabled}
              />
            </div>
          )}
          {lastHeight && (
            <small className="checkup-last">
              {describeLast(lastHeight, personAgeMonths(person, lastHeight.measurementDate), now)}
            </small>
          )}
        </div>

        <div className="checkup-field">
          <div className="checkup-field-head">
            <label htmlFor={form.entry.weightUnit === "lb-oz" ? "pounds" : "weight"}>
              {copy.measurement.weight}
            </label>
            <SegmentedControl
              label={copy.measurement.weightUnit}
              options={WEIGHT_UNITS}
              value={form.entry.weightUnit}
              onChange={vlens.cachePartial(setWeightUnit, form)}
              disabled={disabled}
            />
          </div>
          {form.entry.weightUnit === "lb-oz" ? (
            <div className="checkup-inputs">
              <UnitInput
                id="pounds"
                suffix="lb"
                label={copy.measurement.pounds}
                bind={vlens.ref(form.entry, "pounds")}
                disabled={disabled}
              />
              <UnitInput
                id="ounces"
                suffix="oz"
                label={copy.measurement.ounces}
                bind={vlens.ref(form.entry, "ounces")}
                disabled={disabled}
              />
            </div>
          ) : (
            <div className="checkup-inputs">
              <UnitInput
                id="weight"
                suffix="lb"
                bind={vlens.ref(form.entry, "weight")}
                disabled={disabled}
              />
            </div>
          )}
          {lastWeight && (
            <small className="checkup-last">
              {describeLast(lastWeight, personAgeMonths(person, lastWeight.measurementDate), now)}
            </small>
          )}
        </div>

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

interface UnitInputProps {
  id: string;
  suffix: string;
  label?: string;
  bind: Ref;
  disabled: boolean;
}

const UnitInput = ({ id, suffix, label, bind, disabled }: UnitInputProps) => (
  <span className="unit-input">
    <input
      id={id}
      type="text"
      inputmode="decimal"
      autocomplete="off"
      aria-label={label}
      disabled={disabled}
      {...vlens.attrsBindInput(bind)}
    />
    <span className="unit-input-suffix" aria-hidden="true">
      {suffix}
    </span>
  </span>
);
