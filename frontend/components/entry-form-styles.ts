import { block } from "vlens/css";

block(`
.entry-container {
  max-width: 560px;
  margin: 0 auto;
  padding: 24px 16px 40px;
}
`);

block(`
.entry-card {
  padding: 20px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 16px;
}
`);

block(`
.entry-title {
  margin: 0 0 14px;
  font-size: 1.4rem;
}
`);

block(`
.entry-form {
  display: grid;
  gap: 18px;
}
`);

block(`
.entry-subject {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding-top: 4px;
}
`);

block(`
.entry-subject-age,
.entry-subject-missing {
  color: var(--muted);
}
`);

block(`
.entry-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}
`);

block(`
.entry-form label,
.entry-label {
  font-weight: 600;
}
`);

block(`
.entry-field {
  display: grid;
  gap: 8px;
}
`);

block(`
.entry-field textarea,
.entry-field input[type="text"] {
  width: 100%;
  padding: 10px 12px;
  border: 1px solid var(--control-border);
  border-radius: 10px;
  background: var(--bg);
  color: var(--text);
  font: inherit;
}
`);

block(`
.checkup-field {
  display: grid;
  gap: 8px;
}
`);

block(`
.checkup-field-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}
`);

block(`
.checkup-inputs {
  display: flex;
  gap: 10px;
}
`);

block(`
.checkup-last {
  color: var(--muted);
}
`);

block(`
.unit-input {
  position: relative;
  flex: 1;
  min-width: 0;
}
`);

block(`
.unit-input input {
  width: 100%;
  min-height: 48px;
  padding: 10px 44px 10px 14px;
  border: 1px solid var(--control-border);
  border-radius: 10px;
  background: var(--bg);
  color: var(--text);
  font: inherit;
  font-size: 1.2rem;
}
`);

block(`
.unit-input-suffix {
  position: absolute;
  right: 14px;
  top: 50%;
  transform: translateY(-50%);
  color: var(--muted);
  pointer-events: none;
}
`);

block(`
.category-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
`);

block(`
.category-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 36px;
  padding: 6px 12px;
  border: 1px solid var(--border);
  border-radius: 999px;
  background: var(--bg);
  color: var(--text);
  font: inherit;
  font-size: 0.9rem;
  cursor: pointer;
}
`);

block(`
.category-chip.selected {
  border-color: var(--accent);
  background: var(--accent-soft);
  font-weight: 600;
}
`);

block(`
.entry-more > summary {
  cursor: pointer;
  color: var(--accent);
  font-weight: 600;
}
`);

block(`
.entry-more[open] > summary {
  margin-bottom: 12px;
}
`);

block(`
.entry-field .person-chips {
  margin-bottom: 0;
  padding-bottom: 4px;
}
`);
