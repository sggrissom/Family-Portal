// Vocabulary for the activities UI, keyed by Activity.Kind.
//
// Nothing in backend/ knows the word "routine" or "competition" — the schema
// is deliberately activity-agnostic (Event, Entry, Appearance). This map is
// the only place the domain word comes back, so shipping a sport label pack is
// a second entry here rather than a second set of screens.
//
// See docs/activities-plan.md, phase 6.

import * as server from "../../server";

export interface ActivityLabels {
  // A season's competitions / games / meets.
  event: string;
  eventPlural: string;
  // The recurring competitive unit: a routine, a team, a swim event.
  entry: string;
  entryPlural: string;
  // One entry at one event.
  appearance: string;
  appearancePlural: string;
  // What the people on an entry are called collectively.
  roster: string;
}

const dance: ActivityLabels = {
  event: "Competition",
  eventPlural: "Competitions",
  entry: "Routine",
  entryPlural: "Routines",
  appearance: "Performance",
  appearancePlural: "Performances",
  roster: "Dancers",
};

const sport: ActivityLabels = {
  event: "Game",
  eventPlural: "Games",
  entry: "Team",
  entryPlural: "Teams",
  appearance: "Game",
  appearancePlural: "Games",
  roster: "Players",
};

const generic: ActivityLabels = {
  event: "Event",
  eventPlural: "Events",
  entry: "Entry",
  entryPlural: "Entries",
  appearance: "Appearance",
  appearancePlural: "Appearances",
  roster: "Members",
};

// Kind values the backend normalizes to. Anything else it stores becomes
// "generic", so the default branch here is not dead code — it is the same
// fallback normalizeActivityKind applies on write.
export const ActivityKindDance = "dance";
export const ActivityKindSport = "sport";
export const ActivityKindGeneric = "generic";

export const activityKindOptions: { value: string; label: string }[] = [
  { value: ActivityKindDance, label: "Dance" },
  { value: ActivityKindSport, label: "Sport" },
  { value: ActivityKindGeneric, label: "Other" },
];

export function labelsForKind(kind: string): ActivityLabels {
  switch (kind) {
    case ActivityKindDance:
      return dance;
    case ActivityKindSport:
      return sport;
    default:
      return generic;
  }
}

export function labelsFor(activity: server.Activity | null | undefined): ActivityLabels {
  return labelsForKind(activity?.kind ?? ActivityKindGeneric);
}

export function activityKindName(kind: string): string {
  const option = activityKindOptions.find(o => o.value === kind);
  return option ? option.label : "Other";
}
