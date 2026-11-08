import { block } from "vlens/css";

block(`
.explore-page {
  max-width: 1040px;
  margin: 0 auto;
  padding: 24px 16px 48px;
}
`);

block(`
.explore-page h1 {
  margin: 0;
}
`);

block(`
.explore-lede {
  margin: 4px 0 20px;
  color: var(--muted);
}
`);

block(`
.explore-directory {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 8px;
}
`);

block(`
.explore-directory a {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--surface);
  color: var(--text);
  text-decoration: none;
}
`);

block(`
.explore-directory a:hover {
  border-color: var(--accent);
}
`);

block(`
.explore-directory-icon {
  display: grid;
  flex: none;
  place-items: center;
  width: 40px;
  height: 40px;
  border-radius: 10px;
  background: var(--accent-soft);
  font-size: 1.2rem;
}
`);

block(`
.explore-directory-label {
  display: grid;
  gap: 2px;
  min-width: 0;
}
`);

block(`
.explore-directory-label span {
  color: var(--muted);
  font-size: 0.85rem;
}
`);

block(`
@media (max-width: 519px) {
  .explore-directory {
    grid-template-columns: 1fr 1fr;
  }

  .explore-directory a {
    gap: 10px;
    padding: 10px;
  }

  .explore-directory-icon {
    width: 34px;
    height: 34px;
  }

  .explore-directory-label span {
    display: none;
  }
}
`);
