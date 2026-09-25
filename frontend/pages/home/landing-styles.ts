import { block } from "vlens/css";

block(`
.landing-container {
  max-width: 860px;
  padding: 0 20px;
  margin: 0 auto;
}
`);

block(`
.landing-page {
  width: 100%;
}
`);

block(`
.landing-intro {
  padding: 72px 0 56px;
  max-width: 660px;
}
`);

block(`
.landing-intro h1 {
  font-size: clamp(2rem, 5vw, 2.75rem);
  font-weight: 700;
  color: var(--text);
  margin: 0 0 24px;
  line-height: 1.15;
}
`);

block(`
.intro-lead {
  font-size: 1.15rem;
  line-height: 1.65;
  color: var(--text);
  margin: 0 0 32px;
}
`);

block(`
.intro-actions {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}
`);

block(`
.intro-actions-oauth {
  flex-direction: column;
  max-width: 340px;
}
`);

block(`
.landing-oauth {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
`);

block(`
.landing-email,
.landing-consent {
  margin: 0;
  color: var(--muted);
  line-height: 1.5;
}
`);

block(`
.landing-email {
  font-size: 0.95rem;
  margin-top: 4px;
}
`);

block(`
.landing-consent {
  font-size: 0.85rem;
}
`);

block(`
.landing-what {
  padding: 48px 0;
  border-top: 1px solid var(--border);
  max-width: 660px;
}
`);

block(`
.landing-what h2 {
  font-size: 1.4rem;
  font-weight: 600;
  color: var(--text);
  margin: 0 0 20px;
}
`);

block(`
.landing-what ul {
  margin: 0;
  padding: 0 0 0 20px;
  color: var(--muted);
  line-height: 1.7;
}
`);

block(`
.landing-what li {
  margin-bottom: 12px;
}
`);

block(`
.landing-what li:last-child {
  margin-bottom: 0;
}
`);

block(`
.landing-shots {
  display: flex;
  flex-direction: column;
  gap: 48px;
  padding: 48px 0;
  border-top: 1px solid var(--border);
}
`);

block(`
.shot {
  margin: 0;
}
`);

block(`
.shot-img {
  display: block;
  width: 100%;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--surface);
}
`);

block(`
.shot figcaption {
  color: var(--muted);
  font-size: 0.95rem;
  line-height: 1.6;
  margin-top: 12px;
}
`);

block(`
.landing-close {
  padding: 48px 0 80px;
  border-top: 1px solid var(--border);
}
`);

block(`
.landing-page a:not(.btn) {
  color: var(--accent);
  text-decoration: underline;
  text-underline-offset: 2px;
}
`);

block(`
.landing-page a:not(.btn):hover {
  color: var(--primary-accent);
}
`);

block(`
@media (max-width: 820px) {
  .landing-intro {
    padding: 48px 0 40px;
  }

  .landing-what,
  .landing-shots,
  .landing-close {
    padding: 40px 0;
  }

  .landing-shots {
    gap: 36px;
  }
}
`);
