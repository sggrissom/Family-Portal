import { block } from "vlens/css";

block(`
.media-upload {
  position: relative;
  cursor: pointer;
}
`);

block(`
.media-upload.disabled {
  cursor: default;
  opacity: 0.6;
}
`);

block(`
.media-upload input {
  position: absolute;
  width: 1px;
  height: 1px;
  opacity: 0;
}
`);

block(`
.media-upload:focus-within {
  outline: 2px solid var(--primary-accent);
  outline-offset: 2px;
}
`);

block(`
.media-upload-tile {
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
}
`);

block(`
.media-upload-error {
  margin-top: 8px;
}
`);
