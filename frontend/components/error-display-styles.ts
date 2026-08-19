import { block } from "vlens/css";

block(`
.error-display {
  max-width: 560px;
  margin: 60px auto;
  padding: 0 20px;
  text-align: center;
}
`);

block(`
.error-display-title {
  font-size: clamp(1.5rem, 4vw, 2rem);
  margin: 0 0 12px;
  color: var(--hero);
}
`);

block(`
.error-display-message {
  color: var(--muted);
  line-height: 1.6;
  margin: 0 0 24px;
}
`);

block(`
.error-reference {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  justify-content: center;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 10px 14px;
  margin-bottom: 24px;
}
`);

block(`
.error-reference-label {
  font-size: 0.75rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--muted);
}
`);

block(`
.error-reference-code {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 1rem;
  letter-spacing: 0.05em;
  color: var(--text);
  user-select: all;
}
`);

block(`
.error-reference-copy {
  background: none;
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 4px 10px;
  font-size: 0.8rem;
  color: var(--text);
  cursor: pointer;
}
`);

block(`
.error-reference-copy:hover {
  background: var(--hover-bg);
}
`);

block(`
.error-display-actions {
  display: flex;
  gap: 12px;
  justify-content: center;
  flex-wrap: wrap;
  margin-bottom: 20px;
}
`);

block(`
.error-display-support {
  font-size: 0.9rem;
  color: var(--muted);
  line-height: 1.6;
}
`);
