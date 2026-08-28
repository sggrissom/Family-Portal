const ENDPOINT = "/api/client-error";

// One page load should not be able to flood the log, and a reporting failure
// must never surface as another error.
const MAX_REPORTS_PER_LOAD = 5;
let reported = 0;

const seen = new Set<string>();

export interface ClientErrorReport {
  message: string;
  stack?: string;
  source: string;
}

export function reportClientError(report: ClientErrorReport): void {
  if (reported >= MAX_REPORTS_PER_LOAD) return;

  const key = `${report.source}:${report.message}`;
  if (seen.has(key)) return;
  seen.add(key);
  reported++;

  const body = JSON.stringify({
    message: report.message,
    stack: report.stack ?? "",
    route: window.location.pathname + window.location.search,
    source: report.source,
  });

  try {
    window
      .fetch(ENDPOINT, {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body,
        keepalive: true,
      })
      .catch(() => {});
  } catch {
    // Reporting is best-effort.
  }
}

function messageOf(value: unknown): string {
  if (value instanceof Error) return value.message || String(value);
  if (typeof value === "string") return value;
  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
}

function stackOf(value: unknown): string | undefined {
  return value instanceof Error ? value.stack : undefined;
}

export function installGlobalErrorHandlers(): void {
  window.addEventListener("error", event => {
    const detail = event.error ?? event.message;
    reportClientError({
      message: messageOf(detail),
      stack: stackOf(event.error),
      source: "window.onerror",
    });
  });

  window.addEventListener("unhandledrejection", event => {
    reportClientError({
      message: messageOf(event.reason),
      stack: stackOf(event.reason),
      source: "unhandledrejection",
    });
  });
}
