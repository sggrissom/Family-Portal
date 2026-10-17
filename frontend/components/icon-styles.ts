import { block } from "vlens/css";

block(`
.ui-icon {
  display: inline-block;
  vertical-align: -0.2em;
  flex-shrink: 0;
}
`);

block(`
.app-bottombar-icon .ui-icon,
.add-sheet-icon .ui-icon {
  width: 24px;
  height: 24px;
}
`);

block(`
.empty-icon .ui-icon {
  width: 32px;
  height: 32px;
  color: var(--muted);
}
`);
