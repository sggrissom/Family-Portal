import { block } from "vlens/css";

block(`
.face-crop {
  position: relative;
  overflow: hidden;
  border-radius: 10px;
  background: var(--surface);
  border: 1px solid var(--border);
  flex-shrink: 0;
}
`);

block(`
.face-crop img {
  position: absolute;
  max-width: none;
  display: block;
}
`);

block(`
.face-crop img.face-crop-whole {
  position: static;
  width: 100%;
  height: 100%;
  object-fit: cover;
}
`);
