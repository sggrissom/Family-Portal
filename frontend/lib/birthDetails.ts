import * as server from "../server";
import { CheckupEntry, entryForRecord, heightValue, newEntry, weightValue } from "./checkup";

export interface BirthForm {
  time: string;
  place: string;
  entry: CheckupEntry;
  open: boolean;
}

export function newBirthForm(): BirthForm {
  return { time: "", place: "", entry: { ...newEntry(), weightUnit: "lb-oz" }, open: false };
}

function sameDay(a: string, b: string): boolean {
  return a.slice(0, 10) === b.slice(0, 10);
}

export function birthRecords(person: server.Person, growth: server.GrowthData[]) {
  const atBirth = growth.filter(g => sameDay(g.measurementDate, person.birthday));
  return {
    length: atBirth.find(g => g.measurementType === server.Height) ?? null,
    weight: atBirth.find(g => g.measurementType === server.Weight) ?? null,
  };
}

export function birthFormFor(person: server.Person, growth: server.GrowthData[]): BirthForm {
  const form = newBirthForm();
  form.time = person.birthTime;
  form.place = person.birthPlace;
  const { length, weight } = birthRecords(person, growth);
  if (length) {
    const e = entryForRecord(length);
    Object.assign(form.entry, { heightUnit: e.heightUnit, height: e.height });
  }
  if (weight) {
    const e = entryForRecord(weight);
    Object.assign(form.entry, {
      weightUnit: e.weightUnit,
      weight: e.weight,
      pounds: e.pounds,
      ounces: e.ounces,
    });
  }
  form.open = hasBirthDetails(form);
  return form;
}

export function hasBirthDetails(form: BirthForm): boolean {
  const e = form.entry;
  return [form.time, form.place, e.height, e.feet, e.inches, e.weight, e.pounds, e.ounces].some(
    v => v.trim() !== ""
  );
}

const CLEAR: server.CheckupValue = { value: 0, unit: "" };

// Every field is sent, so a cleared one is removed rather than left alone.
export function birthDetailsRequest(form: BirthForm): {
  details: server.BirthDetails;
  error: string;
} {
  const length = heightValue(form.entry);
  const weight = weightValue(form.entry);
  const error = [
    length.error && `Birth length: ${length.error}`,
    weight.error && `Birth weight: ${weight.error}`,
  ]
    .filter(Boolean)
    .join(". ");
  return {
    details: {
      birthTime: form.time,
      birthPlace: form.place.trim(),
      birthLength: length.value ?? CLEAR,
      birthWeight: weight.value ?? CLEAR,
    },
    error,
  };
}

export function formatBirthTime(time: string): string {
  const match = /^(\d{2}):(\d{2})$/.exec(time);
  if (!match) return "";
  const date = new Date(Date.UTC(2000, 0, 1, Number(match[1]), Number(match[2])));
  return date.toLocaleTimeString("en-US", { hour: "numeric", minute: "2-digit", timeZone: "UTC" });
}
