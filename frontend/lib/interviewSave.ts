import { getAuth } from "./authCache";

// Serialize all writes to a sitting, including metadata and completion.
export class InterviewSaveQueue {
  private tail: Promise<void> = Promise.resolve();
  pending = 0;

  run<T>(write: () => Promise<T>): Promise<T> {
    this.pending++;
    const result = this.tail.then(write);
    this.tail = result
      .then(
        () => {},
        () => {}
      )
      .finally(() => {
        this.pending--;
      });
    return result;
  }

  async flush(): Promise<void> {
    // A write may append another write while it resolves.
    let tail: Promise<void>;
    do {
      tail = this.tail;
      await tail;
    } while (tail !== this.tail);
  }
}

export function acknowledgeInterviewRequest(operation: string) {
  requests.delete(`${getAuth()?.id ?? 0}:${operation}`);
}

const requests = new Map<string, { payload: string; id: string }>();
export function interviewRequestId(operation: string, payload: unknown): string {
  const key = `${getAuth()?.id ?? 0}:${operation}`;
  const text = JSON.stringify(payload);
  const old = requests.get(key);
  if (old?.payload === text) return old.id;
  const id = crypto.randomUUID();
  requests.set(key, { payload: text, id });
  return id;
}

export async function callInterview<T>(
  request: Promise<[T | null, string]>
): Promise<[T | null, string]> {
  let timer: ReturnType<typeof setTimeout> | undefined;
  try {
    return await Promise.race([
      request,
      new Promise<[null, string]>(resolve => {
        timer = setTimeout(
          () =>
            resolve([
              null,
              "The server has not confirmed this save. Your draft is still here; review the saved interview before retrying.",
            ]),
          15000
        );
      }),
    ]);
  } catch {
    return [null, "Could not reach the server. Your unsaved interview data is still on this page."];
  } finally {
    if (timer) clearTimeout(timer);
  }
}
