import { block } from "vlens/css";

block(`
.recap-links {
  display: grid;
  gap: 8px;
  padding-top: 12px;
  border-top: 1px solid var(--border);
}
`);

block(`
.recap-links h3 {
  margin: 0;
  font-size: 0.95rem;
}
`);

block(`
.recap-links h4 {
  margin: 4px 0;
  font-size: 0.9rem;
}
`);

block(`
.recap-links p {
  margin: 0;
}
`);

block(`
.recap-link-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}
`);

block(`
.recap-link-expiry {
  display: flex;
  align-items: center;
  gap: 8px;
}
`);

block(`
.recap-link-created,
.recap-link-confirm {
  display: grid;
  gap: 8px;
}
`);

block(`
.recap-link-url {
  flex: 1 1 220px;
  min-width: 0;
  padding: 8px 10px;
  font-family: ui-monospace, monospace;
  font-size: 0.85rem;
}
`);

block(`
.recap-link-list ul {
  display: grid;
  gap: 6px;
  margin: 0;
  padding: 0;
  list-style: none;
}
`);

block(`
.recap-link-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 10px;
  border: 1px solid var(--border);
  border-radius: 10px;
}
`);

block(`
.recap-link-item.is-expired {
  opacity: 0.7;
}
`);

block(`
.recap-link-item > div {
  flex: 1 1 auto;
  min-width: 0;
}
`);

block(`
.recap-link-item > .btn {
  flex: 0 0 auto;
  width: auto;
}
`);
