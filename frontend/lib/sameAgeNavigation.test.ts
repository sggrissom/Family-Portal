import { describe, expect, it, vi } from "vitest";
import type { GetSameAgeResponse } from "../server";
import { nearbyRecordedAge } from "./sameAge";
import { recordedAgeIndex, sameAgeNavigation, selectSameAge } from "./sameAgeNavigation";

const response = (ageMonths: number): GetSameAgeResponse => ({
  ageMonths,
  fromPersonId: 7,
  maxAgeMonths: 400,
  peopleCount: 2,
  rows: [],
  availableAges: [6, 12, 18].map(ageMonths => ({ ageMonths, peopleCount: 2 })),
});

function deferred() {
  let resolve!: (value: [GetSameAgeResponse | null, string]) => void;
  const promise = new Promise<[GetSameAgeResponse | null, string]>(done => {
    resolve = done;
  });
  return { promise, resolve };
}

describe("Same Age navigation", () => {
  it("advances from the latest selection during fast taps and ignores stale responses", async () => {
    const state = sameAgeNavigation(response(6));
    const first = deferred();
    const second = deferred();
    const load = vi.fn().mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise);
    const url = vi.fn();
    const redraw = vi.fn();
    const ages = [6, 12, 18];

    const one = selectSameAge(
      state,
      nearbyRecordedAge(ages, state.selectedAge, 1)!,
      load,
      url,
      redraw
    );
    const two = selectSameAge(
      state,
      nearbyRecordedAge(ages, state.selectedAge, 1)!,
      load,
      url,
      redraw
    );
    expect(load.mock.calls).toEqual([[12], [18]]);
    expect(state.selectedAge).toBe(18);
    expect(state.loading).toBe(true);
    second.resolve([{ ...response(18), availableAges: [] }, ""]);
    await two;
    first.resolve([response(12), ""]);
    await one;
    expect(state.data.ageMonths).toBe(18);
    expect(state.data.availableAges.map(a => a.ageMonths)).toEqual(ages);
    expect(state.loading).toBe(false);
    expect(url.mock.calls).toEqual([[12], [18]]);
  });

  it("does not let a stale failure interrupt the latest request", async () => {
    const state = sameAgeNavigation(response(6));
    const first = deferred();
    const second = deferred();
    const one = selectSameAge(state, 12, () => first.promise, vi.fn(), vi.fn());
    const two = selectSameAge(state, 18, () => second.promise, vi.fn(), vi.fn());
    first.resolve([null, "Old error"]);
    await one;
    expect(state.error).toBe("");
    expect(state.loading).toBe(true);
    second.resolve([response(18), ""]);
    await two;
    expect(state.data.ageMonths).toBe(18);
  });

  it("shows a failed load and permits retrying the selected age", async () => {
    const state = sameAgeNavigation(response(6));
    await selectSameAge(
      state,
      12,
      async () => {
        throw new Error("offline");
      },
      vi.fn(),
      vi.fn()
    );
    expect(state.loading).toBe(false);
    expect(state.error).toContain("try again");
    expect(state.data.ageMonths).toBe(6);
    await selectSameAge(state, 12, async () => [response(12), ""], vi.fn(), vi.fn());
    expect(state.error).toBe("");
    expect(state.data.ageMonths).toBe(12);
  });

  it("positions contextual ages on the slider without offering empty years", () => {
    const ages = [0, 6, 12, 72];
    expect(recordedAgeIndex(ages, 6)).toBe(1);
    expect(recordedAgeIndex(ages, 11)).toBe(2);
    expect(recordedAgeIndex(ages, 400)).toBe(3);
    expect(recordedAgeIndex([], 400)).toBe(0);
  });
});
