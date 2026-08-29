import { block } from "vlens/css";

block(`
.legal-container {
  max-width: 760px;
  margin: 0 auto;
  padding: 40px 20px 80px;
}
`);

block(`
.legal-page h1 {
  font-size: clamp(1.75rem, 4vw, 2.5rem);
  margin: 0 0 8px;
  color: var(--hero);
}
`);

block(`
.legal-updated {
  color: var(--muted);
  font-size: 0.9rem;
  margin: 0 0 32px;
}
`);

block(`
.legal-page h2 {
  font-size: 1.25rem;
  margin: 36px 0 12px;
  color: var(--hero);
}
`);

block(`
.legal-page h3 {
  font-size: 1rem;
  margin: 24px 0 8px;
  color: var(--text);
}
`);

block(`
.legal-page p {
  line-height: 1.7;
  margin: 0 0 14px;
  color: var(--text);
}
`);

block(`
.legal-page ul {
  line-height: 1.7;
  margin: 0 0 14px;
  padding-left: 22px;
  color: var(--text);
}
`);

block(`
.legal-page li {
  margin-bottom: 6px;
}
`);

block(`
.legal-lead {
  font-size: 1.05rem;
  color: var(--muted);
  margin-bottom: 28px;
}
`);

block(`
.legal-callout {
  background: var(--surface);
  border: 1px solid var(--border);
  border-left: 4px solid var(--accent);
  border-radius: 8px;
  padding: 16px 20px;
  margin: 24px 0;
}
`);

block(`
.legal-table {
  width: 100%;
  border-collapse: collapse;
  margin: 0 0 20px;
  font-size: 0.95rem;
}
`);

block(`
.legal-table th {
  text-align: left;
  padding: 10px 12px;
  border-bottom: 2px solid var(--border);
  color: var(--hero);
}
`);

block(`
.legal-table td {
  padding: 10px 12px;
  border-bottom: 1px solid var(--border);
  vertical-align: top;
  color: var(--text);
}
`);

block(`
.legal-table-wrap {
  overflow-x: auto;
}
`);

block(`
.legal-contact {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 20px 24px;
  margin: 24px 0;
}
`);

block(`
.legal-contact-address {
  font-size: 1.15rem;
  font-weight: 600;
  word-break: break-all;
}
`);

block(`
.legal-nav {
  display: flex;
  gap: 16px;
  flex-wrap: wrap;
  margin-top: 48px;
  padding-top: 20px;
  border-top: 1px solid var(--border);
  font-size: 0.95rem;
}
`);
