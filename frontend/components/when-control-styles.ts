import { block } from "vlens/css";

block(`
.when-control {
  display: inline-flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
}
`);

block(`
.when-select {
  min-height: 36px;
  padding: 6px 10px;
  border: 1px solid var(--border);
  border-radius: 999px;
  background: var(--bg);
  color: var(--text);
  font: inherit;
  font-weight: 600;
  cursor: pointer;
}
`);

block(`
.when-date {
  min-height: 36px;
  padding: 6px 10px;
  border: 1px solid var(--control-border);
  border-radius: 8px;
  background: var(--bg);
  color: var(--text);
  font: inherit;
}
`);

block(`
.when-age {
  display: inline-flex;
  gap: 6px;
}
`);

block(`
.when-age input {
  width: 64px;
  min-height: 36px;
  padding: 6px 8px;
  border: 1px solid var(--control-border);
  border-radius: 8px;
  background: var(--bg);
  color: var(--text);
  font: inherit;
}
`);
