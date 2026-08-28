import { block } from "vlens/css";

block(`
.verify-banner {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
  padding: 10px 20px;
  background: var(--surface-alt, #fff8e1);
  border-bottom: 1px solid var(--border);
  color: var(--text);
  font-size: 0.9rem;
}
`);

block(`
.verify-banner-text {
  flex: 1 1 260px;
  margin: 0;
}
`);

block(`
.verify-banner-actions {
  display: flex;
  gap: 8px;
  align-items: center;
}
`);

block(`
.verify-banner-button {
  background: none;
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 4px 10px;
  color: inherit;
  cursor: pointer;
  font: inherit;
}
`);

block(`
.verify-banner-button:hover {
  border-color: var(--hero);
}
`);

block(`
.verify-banner-dismiss {
  background: none;
  border: none;
  color: var(--muted);
  cursor: pointer;
  font-size: 1.1rem;
  line-height: 1;
  padding: 4px 8px;
}
`);
