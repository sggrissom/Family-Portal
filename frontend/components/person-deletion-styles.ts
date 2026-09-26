import { block } from "vlens/css";

block(`
.person-deletion {
  margin-top: 1.5rem;
  padding-top: 1.25rem;
  border-top: 1px solid var(--border);
}
`);

block(`
.person-deletion p {
  margin: 0 0 0.75rem;
  color: var(--muted);
}
`);

block(`
.person-deletion-confirm {
  padding: 0.75rem 1rem;
  border: 1px solid var(--danger);
  border-radius: 8px;
}
`);

block(`
.person-deletion-confirm ul {
  margin: 0 0 0.75rem;
  padding-left: 1.25rem;
}
`);

block(`
.person-deletion-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
}
`);
