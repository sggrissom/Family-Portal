import { block } from "vlens/css";

block(`
.verify-banner {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
  padding: 10px 20px;
  background: var(--surface-alt);
  border-bottom: 1px solid var(--surface-alt-border);
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
  border: 1px solid var(--control-border);
  border-radius: 6px;
  padding: 4px 10px;
  color: inherit;
  cursor: pointer;
  font: inherit;
}
`);

block(`
.verify-banner-button:hover:not(:disabled) {
  border-color: var(--accent);
  background: var(--hover-bg);
}
`);

block(`
.verify-banner-button:disabled {
  cursor: default;
  opacity: 0.6;
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
  border-radius: 6px;
}
`);

block(`
.verify-banner-dismiss:hover {
  color: var(--text);
  background: var(--hover-bg);
}
`);
