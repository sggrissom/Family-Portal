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
.tag-picker {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}
`);

block(`
.tag-pill {
  display: inline-flex;
  font-family: inherit;
  color: inherit;
  align-items: center;
  gap: 0.4rem;
  padding: 0.3rem 0.75rem;
  border-radius: 999px;
  border: 2px solid transparent;
  background: var(--surface);
  cursor: pointer;
  font-size: 0.85rem;
  user-select: none;
  transition: background 0.15s;
}
`);

block(`
.tag-pill.selected {
  background: color-mix(in srgb, var(--surface) 60%, currentColor 40%);
}
`);

block(`
.tag-color-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
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

block(`
.artwork-upload {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 72px;
  height: 72px;
  padding: 6px;
  border: 2px dashed var(--border);
  border-radius: 8px;
  color: var(--muted);
  font-size: 0.75rem;
  text-align: center;
  cursor: pointer;
}
`);

block(`
.artwork-upload.disabled {
  cursor: default;
  opacity: 0.6;
}
`);

block(`
.artwork-upload input {
  position: absolute;
  width: 1px;
  height: 1px;
  opacity: 0;
}
`);

block(`
.artwork-upload:focus-within {
  outline: 2px solid var(--primary-accent);
  outline-offset: 2px;
}
`);
