import { block } from "vlens/css";

block(`
.recap-container {
  max-width: 640px;
  margin: 0 auto;
  padding: 24px 16px 64px;
}
`);

block(`
.recap-page h1 {
  margin: 0 0 4px;
  font-size: clamp(1.5rem, 5vw, 2rem);
  line-height: 1.25;
}
`);

block(`
.recap-page p {
  margin: 0;
}
`);

block(`
.recap-meta {
  color: var(--muted);
}
`);

block(`
.recap-snapshot {
  display: inline-block;
  margin-top: 10px;
  padding: 2px 10px;
  border: 1px solid var(--border);
  border-radius: 999px;
  color: var(--muted);
  font-size: 0.875rem;
}
`);

block(`
.recap-routine {
  margin-top: 16px;
  padding: 16px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 12px;
}
`);

block(`
.recap-routine h2 {
  margin: 0 0 2px;
  font-size: 1.15rem;
}
`);

block(`
.recap-performance {
  margin-top: 10px;
}
`);

block(`
.recap-performance h3 {
  margin: 0 0 2px;
  color: var(--muted);
  font-size: 0.95rem;
  font-weight: 600;
}
`);

block(`
.recap-results {
  margin: 4px 0 0;
  padding-left: 20px;
}
`);

block(`
.recap-results li {
  margin: 2px 0;
}
`);

block(`
.recap-result {
  color: var(--accent);
  font-weight: 600;
}
`);

block(`
.recap-detail {
  color: var(--muted);
}
`);

block(`
.recap-page .recap-footer-note {
  margin-top: 32px;
  color: var(--muted);
  font-size: 0.8rem;
  text-align: center;
}
`);
