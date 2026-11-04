import { block } from "vlens/css";

block(`
.edit-photo-preview {
  display: flex;
  gap: 1rem;
  align-items: center;
  margin-bottom: 1rem;
}
`);

block(`
.edit-photo-preview .preview-image {
  width: 96px;
  height: 96px;
  object-fit: cover;
  border-radius: 8px;
}
`);

block(`
.edit-photo-preview .photo-info {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  font-size: 0.875rem;
  color: var(--muted);
}
`);
