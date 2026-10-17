import * as preact from "preact";
import type { LucideIcon } from "lucide-preact";

// Icons accompany visible text or an accessible label on their control.
// Import individual Lucide components at each call site so unused icons are omitted.
export const Icon = ({
  symbol: Symbol,
  size = 20,
  label,
}: {
  symbol: LucideIcon;
  size?: number;
  label?: string;
}) => (
  <Symbol
    className="ui-icon"
    size={size}
    strokeWidth={1.75}
    aria-hidden={label ? undefined : "true"}
    role={label ? "img" : undefined}
    aria-label={label}
    focusable="false"
  />
);
