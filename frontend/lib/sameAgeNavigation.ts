import type { GetSameAgeResponse } from "../server";
import type { SameAgeView } from "./sameAge";

export interface SameAgeNavigation {
  data: GetSameAgeResponse;
  view: SameAgeView;
  selectedAge: number;
  loading: boolean;
  error: string;
  request: number;
}

export function sameAgeNavigation(
  data: GetSameAgeResponse,
  view: SameAgeView = "portraits"
): SameAgeNavigation {
  return {
    data,
    view,
    selectedAge: data.ageMonths,
    loading: false,
    error: "",
    request: 0,
  };
}

type LoadAge = (age: number) => Promise<[GetSameAgeResponse | null, string]>;

// Resolve the latest selection even if an earlier request finishes last.
// Navigation within this page does not use the router, which drops changes
// while a route fetch is in progress.
export async function selectSameAge(
  state: SameAgeNavigation,
  age: number,
  load: LoadAge,
  updateURL: (age: number) => void,
  redraw: () => void
): Promise<void> {
  if (age === state.selectedAge && state.loading) return;
  if (age === state.data.ageMonths && !state.loading && !state.error) {
    state.selectedAge = age;
    redraw();
    return;
  }
  const request = ++state.request;
  state.selectedAge = age;
  state.loading = true;
  state.error = "";
  updateURL(age);
  redraw();
  try {
    const [data, error] = await load(age);
    if (request !== state.request) return;
    if (data) {
      // Discovery is needed only on the first load, not on every age change.
      state.data = {
        ...data,
        availableAges: state.data.availableAges,
        portraitAges: state.data.portraitAges,
      };
    } else {
      state.error = error || "Could not load records. Please try again.";
    }
  } catch {
    if (request !== state.request) return;
    state.error = "Could not load records. Please try again.";
  }
  if (request === state.request) {
    state.loading = false;
    redraw();
  }
}
