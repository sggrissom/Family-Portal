import { block } from "vlens/css";

block(`
.page-head {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  justify-content: space-between;
  gap: 8px 16px;
  margin-bottom: 16px;
}
`);

block(`
.page-head-text {
  min-width: 0;
}
`);

block(`
.page-head h1 {
  margin: 0;
  font-size: 1.6rem;
  line-height: 1.2;
}
`);

block(`
.page-head-lede {
  margin: 4px 0 0;
  color: var(--muted);
  font-size: 0.95rem;
}
`);

block(`
.page-head-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}
`);

block(`
.page-head-actions .btn {
  width: auto;
  flex: 0 0 auto;
  white-space: nowrap;
}
`);

block(`
.history-page > .page-head,
.growth-page > .page-head,
.same-age-page > .page-head {
  margin-bottom: 0;
}
`);
