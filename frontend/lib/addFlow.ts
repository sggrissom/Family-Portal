import { UnitPrefs, newUnitPrefs } from "./checkup";

let handedOffPhotos: File[] = [];

export function handOffPhotos(files: File[]) {
  handedOffPhotos = files;
}

export function takeHandedOffPhotos(): File[] {
  const files = handedOffPhotos;
  handedOffPhotos = [];
  return files;
}

const RETURN_KEY = "add-return-to";

export function rememberReturn(path: string) {
  try {
    sessionStorage.setItem(RETURN_KEY, path);
  } catch {}
}

export function returnPath(fallback: string): string {
  try {
    return sessionStorage.getItem(RETURN_KEY) || fallback;
  } catch {
    return fallback;
  }
}

export function takeReturnPath(fallback: string): string {
  const path = returnPath(fallback);
  try {
    sessionStorage.removeItem(RETURN_KEY);
  } catch {}
  return path;
}

const LAST_PERSON_KEY = "last-person-id";

export function readLastPerson(): number | null {
  try {
    const id = parseInt(localStorage.getItem(LAST_PERSON_KEY) ?? "");
    return isNaN(id) ? null : id;
  } catch {
    return null;
  }
}

export function writeLastPerson(id: number | null) {
  try {
    if (id) {
      localStorage.setItem(LAST_PERSON_KEY, String(id));
    } else {
      localStorage.removeItem(LAST_PERSON_KEY);
    }
  } catch {}
}

const UNIT_PREFS_KEY = "unit-prefs";

export function loadUnitPrefs(): UnitPrefs {
  try {
    const stored = JSON.parse(localStorage.getItem(UNIT_PREFS_KEY) ?? "null");
    return stored ? { ...newUnitPrefs(), ...stored } : newUnitPrefs();
  } catch {
    return newUnitPrefs();
  }
}

export function saveUnitPrefs(prefs: UnitPrefs) {
  try {
    localStorage.setItem(UNIT_PREFS_KEY, JSON.stringify(prefs));
  } catch {}
}
