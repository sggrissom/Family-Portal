import { block } from "vlens/css";

block(`
.birth-details {
  display: grid;
  gap: 14px;
  padding: 12px 14px;
  border: 1px solid var(--border);
  border-radius: 8px;
}
`);

block(`
.birth-details summary {
  cursor: pointer;
  font-weight: 600;
}
`);

block(`
.birth-details[open] {
  padding-bottom: 16px;
}
`);

block(`
.birth-details small {
  color: var(--muted);
}
`);

block(`
.birth-details-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 2fr);
  gap: 12px;
}
`);

block(`
@media (max-width: 480px) {
  .birth-details-row {
    grid-template-columns: 1fr;
  }
}
`);
