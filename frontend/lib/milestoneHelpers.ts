import * as server from "../server";

// Milestone categories with their display labels and icons
export const MILESTONE_CATEGORIES = [
  { value: "development", label: "Development", icon: "🌱" },
  { value: "behavior", label: "Behavior", icon: "😊" },
  { value: "health", label: "Health", icon: "🏥" },
  { value: "achievement", label: "Achievement", icon: "🏆" },
  { value: "first", label: "First Time", icon: "⭐" },
  { value: "other", label: "Other", icon: "📝" },
] as const;

export type MilestoneCategory = (typeof MILESTONE_CATEGORIES)[number]["value"];

/**
 * Get the emoji icon for a milestone category
 */
export function getCategoryIcon(category: string): string {
  const found = MILESTONE_CATEGORIES.find(c => c.value === category);
  return found ? found.icon : "📝";
}

/**
 * Get the display label for a milestone category
 */
export function getCategoryLabel(category: string): string {
  const found = MILESTONE_CATEGORIES.find(c => c.value === category);
  return found ? found.label : "Other";
}

/**
 * Get the display label for a measurement type
 */
export function getMeasurementTypeLabel(type: server.MeasurementType): string {
  return type === server.Height ? "Height" : "Weight";
}
