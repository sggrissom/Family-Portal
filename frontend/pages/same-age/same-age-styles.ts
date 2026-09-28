import { block } from "vlens/css";

block(`
.same-age-container {
  max-width: 760px;
  margin: 0 auto;
  padding: 20px 16px 40px;
}
`);

block(`
.same-age-page {
  display: grid;
  gap: 14px;
}
`);

block(`
.same-age-page h1 {
  margin: 0;
  font-size: 1.4rem;
}
`);

block(`
.same-age-control {
  display: flex;
  align-items: center;
  gap: 10px;
}
`);

block(`
.same-age-at {
  color: var(--muted);
}
`);

block(`
.same-age-age {
  min-width: 10ch;
  text-align: center;
  font-size: 1.15rem;
}
`);

block(`
.same-age-step {
  width: 40px;
  height: 40px;
  border: 1px solid var(--border);
  border-radius: 50%;
  background: var(--surface);
  color: var(--text);
  cursor: pointer;
}
`);

block(`
.same-age-step:disabled {
  opacity: 0.4;
  cursor: default;
}
`);

block(`
.same-age-slider {
  width: 100%;
  accent-color: var(--primary-accent);
}
`);
