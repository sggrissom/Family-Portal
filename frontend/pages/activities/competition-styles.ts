import { block } from "vlens/css";

block(`
.appearance-item {
  align-items: flex-start;
}
`);

block(`
.appearance-item .event-item-main {
  flex: 1;
}
`);

block(`
.appearance-editing {
  flex: 1;
  min-width: 0;
}
`);

block(`
.photo-draft-upload {
  margin-bottom: 0.75rem;
}
`);

block(`
.matchup-summary {
  list-style: none;
  padding: 0.5rem 0 0.25rem;
  color: var(--muted);
  font-size: 0.9rem;
}
`);

block(`
.matchup-summary strong {
  color: var(--text);
}
`);

block(`
.matchup-fields {
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 0.75rem 1rem 0;
  margin: 1rem 0;
  min-width: 0;
}
`);

block(`
.matchup-fields legend {
  font-weight: 600;
  font-size: 0.9rem;
  padding: 0 0.25rem;
}
`);

block(`
.matchup-fields .form-row {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
  align-items: start;
  gap: 0 1rem;
}
`);

block(`
.matchup-fields .form-group {
  margin-bottom: 0.75rem;
  min-width: 0;
}
`);

block(`
.matchup-score {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}
`);

block(`
.matchup-score input {
  width: 5rem;
  min-width: 0;
}
`);
