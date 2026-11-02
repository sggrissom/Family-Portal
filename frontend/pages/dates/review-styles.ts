import { block } from "vlens/css";

block(`
.date-review {
  max-width: 760px;
  margin: 0 auto;
  padding: 24px 16px 48px;
}
`);

block(`
.date-review-intro,
.date-review-empty {
  color: var(--muted);
}
`);

block(`
.date-review-notice {
  margin: 12px 0;
  padding: 10px 14px;
  border-radius: 10px;
  background: var(--surface);
  border: 1px solid var(--border);
}
`);

block(`
.date-review-cluster {
  margin-top: 24px;
  padding: 16px;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--surface);
}
`);

block(`
.date-review-cluster h2 {
  margin: 0 0 4px;
  font-size: 1.1rem;
}
`);

block(`
.date-review-records {
  list-style: none;
  margin: 12px 0;
  padding: 0;
  display: grid;
  gap: 8px;
}
`);

block(`
.date-review-records label {
  display: grid;
  grid-template-columns: auto auto 1fr;
  align-items: center;
  gap: 10px;
}
`);

block(`
.date-review-thumb {
  width: 40px;
  height: 40px;
}
`);

block(`
.date-review-records img {
  width: 40px;
  height: 40px;
  object-fit: cover;
  border-radius: 6px;
}
`);

block(`
.date-review-change {
  grid-column: 3;
  color: var(--muted);
  font-size: 0.9rem;
}
`);

block(`
.date-review-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
}
`);
