import { block } from "vlens/css";

block(`
.add-milestone-container {
  max-width: 580px;
  padding: 40px 20px;
  margin: 0 auto;
  background: var(--bg);
  min-height: calc(100vh - 200px);
  display: flex;
  align-items: center;
  justify-content: center;
}
`);

block(`
@media (max-width: 580px) {
  .add-milestone-container {
    padding: 30px 16px;
  }
}
`);

block(`
.entry-suggested {
  font-weight: 400;
  color: var(--muted);
  font-size: 0.85rem;
}
`);

block(`
.entry-hint-button {
  justify-self: start;
  margin-top: 8px;
  padding: 6px 10px;
  border: 1px dashed var(--border);
  border-radius: 8px;
  background: none;
  color: var(--primary-accent);
  font: inherit;
  font-size: 0.9rem;
  cursor: pointer;
}
`);

block(`
.suggested-photos {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
`);

block(`
.suggested-photo {
  width: 72px;
  height: 72px;
  padding: 0;
  border: 3px solid transparent;
  border-radius: 8px;
  overflow: hidden;
  background: var(--bg);
  cursor: pointer;
  opacity: 0.75;
}
`);

block(`
.suggested-photo.selected {
  border-color: var(--primary-accent);
  opacity: 1;
}
`);

block(`
.suggested-photo img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}
`);
