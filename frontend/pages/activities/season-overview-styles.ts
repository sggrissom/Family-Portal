import { block } from "vlens/css";

block(`
.season-container { max-width: 900px; margin: 0 auto; padding: 2rem 1rem; }
`);

block(`
.season-breadcrumb { display: flex; gap: .55rem; color: var(--muted); margin-bottom: 1rem; }
`);

block(`
.season-breadcrumb a { color: var(--accent); }
`);

block(`
.season-title, .season-panel-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 1rem; }
`);

block(`
.season-title h1 { margin: .1rem 0; }
`);

block(`
.season-eyebrow { color: var(--accent); font-weight: 700; margin: 0; }
`);

block(`
.season-description { white-space: pre-wrap; color: var(--muted); }
`);

block(`
.season-panel { margin-top: 2rem; }
`);

block(`
.season-panel-head { align-items: center; margin-bottom: 1rem; }
`);

block(`
.season-panel-head h2, .season-panel-head p { margin: 0; }
`);

block(`
.season-panel-head p { color: var(--muted); margin-top: .2rem; }
`);

block(`
.season-card-list { display: grid; gap: .75rem; }
`);

block(`
.season-card { display: flex; justify-content: space-between; gap: 1rem; padding: 1rem; border: 1px solid var(--border); border-radius: 8px; background: var(--surface); }
`);

block(`
.season-card h3, .season-card p { margin: 0; }
`);

block(`
.season-card p, .season-card > span { color: var(--muted); font-size: .88rem; }
`);

block(`
.season-form { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: .8rem; border: 1px solid var(--border); border-radius: 8px; padding: 1rem; margin-bottom: 1rem; background: var(--surface); }
`);

block(`
.season-field { display: flex; flex-direction: column; gap: .3rem; font-weight: 600; }
`);

block(`
.season-field input, .season-field textarea { width: 100%; box-sizing: border-box; padding: .6rem .7rem; border: 1px solid var(--border); border-radius: 6px; background: var(--bg); color: var(--text); font: inherit; }
`);

block(`
.season-field.full, .form-hint.full { grid-column: 1 / -1; }
`);

block(`
.form-hint { color: var(--muted); font-size: .85rem; margin: 0; }
`);

block(`
@media (max-width: 600px) { .season-title, .season-panel-head, .season-card { flex-direction: column; } .season-form { grid-template-columns: 1fr; } }
`);
