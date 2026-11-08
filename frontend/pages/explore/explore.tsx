import {
  Baby,
  BookOpen,
  CalendarDays,
  ChartNoAxesCombined,
  Images,
  MessageCircle,
  Mic,
  Trophy,
} from "lucide-preact";
import type { LucideIcon } from "lucide-preact";
import * as preact from "preact";
import * as rpc from "vlens/rpc";
import { Header, Footer } from "../../layout";
import { Icon } from "../../components/Icon";
import { requireAuthInView } from "../../lib/authHelpers";
import type { Destination } from "../../lib/appNav";
import { copy } from "../../lib/copy";
import "./explore-styles";

const FEATURES: { key: Destination; href: string; icon: LucideIcon }[] = [
  { key: "photos", href: "/photos", icon: Images },
  { key: "history", href: "/history", icon: CalendarDays },
  { key: "growth", href: "/growth", icon: ChartNoAxesCombined },
  { key: "activities", href: "/activities", icon: Trophy },
  { key: "interviews", href: "/interviews", icon: Mic },
  { key: "books", href: "/books", icon: BookOpen },
  { key: "sameAge", href: "/same-age", icon: Baby },
  { key: "chat", href: "/chat", icon: MessageCircle },
];

export async function fetch(route: string, prefix: string) {
  return rpc.ok({});
}

export function view(route: string, prefix: string, data: {}): preact.ComponentChild {
  if (!requireAuthInView()) return;

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="explore-page">
        <h1>{copy.explore.title}</h1>
        <p className="explore-lede">{copy.explore.lede}</p>
        <FeatureDirectory />
      </main>
      <Footer />
    </div>
  );
}

const FeatureDirectory = () => (
  <nav className="explore-directory" aria-label={copy.explore.title}>
    {FEATURES.map(feature => (
      <a key={feature.key} href={feature.href}>
        <span className="explore-directory-icon" aria-hidden="true">
          <Icon symbol={feature.icon} />
        </span>
        <span className="explore-directory-label">
          <strong>{copy.nav[feature.key]}</strong>
          <span>{copy.explore.blurbs[feature.key]}</span>
        </span>
      </a>
    ))}
  </nav>
);
