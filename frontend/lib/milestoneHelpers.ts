import * as preact from "preact";
import {
  Sprout,
  Smile,
  HeartPulse,
  Trophy,
  Star,
  MessageCircle,
  Palette,
  Mic,
  NotebookPen,
} from "lucide-preact";
import { Icon } from "../components/Icon";
import * as server from "../server";

export const MILESTONE_CATEGORIES = [
  { value: "development", label: "Development", icon: Sprout },
  { value: "behavior", label: "Behavior", icon: Smile },
  { value: "health", label: "Health", icon: HeartPulse },
  { value: "achievement", label: "Achievement", icon: Trophy },
  { value: "first", label: "First Time", icon: Star },
  { value: "quote", label: "Quote", icon: MessageCircle },
  { value: "artwork", label: "Artwork", icon: Palette },
  { value: "interview", label: "Interview", icon: Mic },
  { value: "other", label: "Other", icon: NotebookPen },
] as const;

export type MilestoneCategory = (typeof MILESTONE_CATEGORIES)[number]["value"];

export function getCategoryIcon(category: string): preact.ComponentChild {
  const found = MILESTONE_CATEGORIES.find(c => c.value === category);
  return preact.h(Icon, { symbol: found ? found.icon : NotebookPen });
}

export function getCategoryLabel(category: string): string {
  const found = MILESTONE_CATEGORIES.find(c => c.value === category);
  return found ? found.label : "Other";
}

export function getMeasurementTypeLabel(type: server.MeasurementType): string {
  return type === server.Height ? "Height" : "Weight";
}
