import * as server from "../../server";

export interface ActivityLabels {
  event: string;
  eventPlural: string;
  entry: string;
  entryPlural: string;
  appearance: string;
  appearancePlural: string;
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
