import { block } from "vlens/css";

block(`
.view-milestone-container {
  max-width: 760px;
  margin: 0 auto;
  padding: 20px 16px 40px;
}
`);

block(`
.view-milestone-page {
  display: grid;
  gap: 20px;
}
`);

block(`
.milestone-detail {
  display: grid;
  gap: 10px;
  padding: 20px;
  border: 1px solid var(--accent-soft-border);
  border-radius: 16px;
  background: var(--accent-soft);
}
`);

block(`
.milestone-detail h1 {
  margin: 0;
  font-size: 1.5rem;
}
`);

block(`
.milestone-detail-category {
  color: var(--muted);
  font-size: 0.85rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}
`);

block(`
.milestone-detail-meta {
  margin: 0;
  color: var(--muted);
}
`);

block(`
.milestone-detail-meta a {
  color: var(--text);
  font-weight: 600;
}
`);

block(`
.milestone-detail-actions {
  display: flex;
  gap: 8px;
}
`);

block(`
.view-milestone-page .back-link {
  color: var(--accent);
  font-weight: 600;
  text-decoration: none;
}
`);
