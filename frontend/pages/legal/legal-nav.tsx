import * as preact from "preact";

export const LegalNav = ({ current }: { current: "privacy" | "terms" | "support" }) => (
  <nav className="legal-nav" aria-label="Policies">
    {current === "privacy" ? <span>Privacy</span> : <a href="/privacy">Privacy</a>}
    {current === "terms" ? <span>Terms</span> : <a href="/terms">Terms</a>}
    {current === "support" ? <span>Support</span> : <a href="/support">Support</a>}
  </nav>
);
