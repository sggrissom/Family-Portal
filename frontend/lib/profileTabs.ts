export type ProfileTab =
  | "story"
  | "quotes"
  | "artwork"
  | "interviews"
  | "photos"
  | "growth"
  | "activities";

export const PROFILE_TABS: ProfileTab[] = [
  "story",
  "quotes",
  "artwork",
  "interviews",
  "photos",
  "growth",
  "activities",
];

export type ProfileContent = {
  isPregnancy: boolean;
  hasBirthday: boolean;
  quotes: number;
  artwork: number;
  interviews: number;
  seasons: number;
};

export function profileTabs(content: ProfileContent): ProfileTab[] {
  if (content.isPregnancy) return ["story", "photos"];
  return PROFILE_TABS.filter(tab => {
    switch (tab) {
      case "quotes":
        return content.quotes > 0;
      case "artwork":
        return content.artwork > 0;
      case "interviews":
        return content.interviews > 0;
      case "growth":
        return content.hasBirthday;
      case "activities":
        return content.seasons > 0;
      default:
        return true;
    }
  });
}

export function shownTab(requested: string | null, tabs: ProfileTab[]): ProfileTab {
  return tabs.find(tab => tab === requested) ?? "story";
}
