import * as server from "../server";

export const MILESTONE_CATEGORIES = [
  { value: "development", label: "Development", icon: "🌱" },
  { value: "behavior", label: "Behavior", icon: "😊" },
  { value: "health", label: "Health", icon: "🏥" },
  { value: "achievement", label: "Achievement", icon: "🏆" },
  { value: "first", label: "First Time", icon: "⭐" },
  { value: "other", label: "Other", icon: "📝" },
] as const;

export type MilestoneCategory = (typeof MILESTONE_CATEGORIES)[number]["value"];

export function getCategoryIcon(category: string): string {
  const found = MILESTONE_CATEGORIES.find(c => c.value === category);
  return found ? found.icon : "📝";
}

export function getCategoryLabel(category: string): string {
  const found = MILESTONE_CATEGORIES.find(c => c.value === category);
  return found ? found.label : "Other";
}

export function getMeasurementTypeLabel(type: server.MeasurementType): string {
  return type === server.Height ? "Height" : "Weight";
}
