const KEY = "browse-state";
const LIMIT = 20;

type Saved = { key: string; value: unknown; y: number };

function readAll(): Saved[] {
  try {
    const stored = JSON.parse(sessionStorage.getItem(KEY) ?? "[]");
    return Array.isArray(stored) ? stored : [];
  } catch {
    return [];
  }
}

function writeAll(entries: Saved[]) {
  try {
    sessionStorage.setItem(KEY, JSON.stringify(entries.slice(-LIMIT)));
  } catch {}
}

// browseKey names one visit to a page: the history entry it was opened in.
// Back returns to the same entry, a fresh click opens a new one.
export function browseKey(page: string, state: unknown = history.state): string {
  const ts = (state as { ts?: number } | null)?.ts ?? 0;
  return `${page}@${ts}`;
}

export function recallBrowse<T>(key: string): { value: T; y: number } | null {
  const found = readAll().find(entry => entry.key === key);
  return found ? { value: found.value as T, y: found.y } : null;
}

export function rememberBrowse(key: string, value: unknown) {
  const entries = readAll();
  const found = entries.find(entry => entry.key === key);
  writeAll([...entries.filter(entry => entry.key !== key), { key, value, y: found?.y ?? 0 }]);
}

export function rememberScroll(key: string, y: number) {
  const entries = readAll();
  const found = entries.find(entry => entry.key === key);
  if (!found) return;
  found.y = Math.round(y);
  writeAll(entries);
}
