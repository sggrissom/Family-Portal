import { block } from "vlens/css";

block(`
.growth-page-container {
  max-width: 860px;
  margin: 0 auto;
  padding: 20px 16px 40px;
}
`);

block(`
.growth-page {
  display: grid;
  gap: 14px;
}
`);

block(`
.growth-page-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}
`);

block(`
.growth-page-head h1 {
  margin: 0;
  font-size: 1.4rem;
}
`);

block(`
.growth-page-controls {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}
`);

block(`
.growth-page-bands {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: var(--muted);
  font-size: 0.9rem;
}
`);

block(`
.growth-page-empty {
  margin: 0;
  color: var(--muted);
}
`);
