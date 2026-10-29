export type WhenMode = "today" | "yesterday" | "date" | "age";

export interface When {
  mode: WhenMode;
  date: string;
  ageYears: string;
  ageMonths: string;
}

export interface WhenRequest {
  inputType: "date" | "age";
  date: string | null;
  ageYears: number | null;
  ageMonths: number | null;
}

export function newWhen(): When {
  return { mode: "today", date: "", ageYears: "", ageMonths: "" };
}

export function localDateString(date: Date): string {
  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, "0");
  const d = String(date.getDate()).padStart(2, "0");
  return `${y}-${m}-${d}`;
}

export function ageDate(birthday: string, when: When): string | null {
  const years = parseInt(when.ageYears);
  const months = when.ageMonths === "" ? 0 : parseInt(when.ageMonths);
  const born = new Date(birthday);
  if (isNaN(years) || isNaN(months) || isNaN(born.getTime()) || born.getUTCFullYear() < 1000) {
    return null;
  }
  const date = new Date(
    Date.UTC(born.getUTCFullYear() + years, born.getUTCMonth() + months, born.getUTCDate())
  );
  return date.toISOString().slice(0, 10);
}

export function whenProblem(when: When, birthday?: string): string {
  if (when.mode === "date" && !when.date) return "Pick a date";
  if (when.mode === "age") {
    const years = parseInt(when.ageYears);
    const months = when.ageMonths === "" ? 0 : parseInt(when.ageMonths);
    if (isNaN(years) || years < 0) return "Enter an age in years";
    if (isNaN(months) || months < 0 || months > 11) return "Months should be 0 to 11";
    const date = birthday ? ageDate(birthday, when) : null;
    if (date && date > localDateString(new Date())) return "That age hasn't happened yet";
  }
  return "";
}

export function whenRequest(when: When, now: Date): WhenRequest {
  switch (when.mode) {
    case "today":
      return { inputType: "date", date: localDateString(now), ageYears: null, ageMonths: null };
    case "yesterday": {
      const yesterday = new Date(now);
      yesterday.setDate(yesterday.getDate() - 1);
      return {
        inputType: "date",
        date: localDateString(yesterday),
        ageYears: null,
        ageMonths: null,
      };
    }
    case "date":
      return { inputType: "date", date: when.date, ageYears: null, ageMonths: null };
    case "age":
      return {
        inputType: "age",
        date: null,
        ageYears: parseInt(when.ageYears),
        ageMonths: when.ageMonths === "" ? 0 : parseInt(when.ageMonths),
      };
  }
}
