import { block } from "vlens/css";

block(`
.explore-page {
  max-width: 1040px;
  margin: 0 auto;
  padding: 24px 16px 48px;
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

block(`
.explore-previews {
  display: grid;
  gap: 20px;
  margin-top: 32px;
}
`);

block(`
.explore-previews-title {
  margin: 0;
  font-size: 1.2rem;
}
`);

block(`
.explore-preview {
  display: grid;
  gap: 10px;
}
`);

block(`
.explore-preview-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
}
`);

block(`
.explore-preview-head h3 {
  margin: 0;
  font-size: 1rem;
}
`);

block(`
.explore-preview-head a {
  flex: none;
  color: var(--accent);
  font-weight: 600;
  text-decoration: none;
}
`);

block(`
.explore-faces {
  display: flex;
  gap: 14px;
  overflow-x: auto;
  padding-bottom: 4px;
  color: var(--text);
  text-decoration: none;
}
`);

block(`
.explore-face {
  display: grid;
  flex: none;
  justify-items: center;
  gap: 4px;
  font-size: 0.85rem;
  font-weight: 600;
}
`);

block(`
.explore-face .face-crop {
  border-radius: 50%;
}
`);

block(`
.explore-years {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(120px, 1fr));
  gap: 8px;
}
`);

block(`
.explore-year {
  position: relative;
  display: block;
  overflow: hidden;
  border-radius: 10px;
  aspect-ratio: 1;
  background: var(--surface);
}
`);

block(`
.explore-year img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
`);

block(`
.explore-year span {
  position: absolute;
  left: 6px;
  bottom: 6px;
  padding: 1px 8px;
  border-radius: 999px;
  background: rgba(15, 23, 42, 0.7);
  color: #fff;
  font-size: 0.8rem;
  font-weight: 700;
}
`);
