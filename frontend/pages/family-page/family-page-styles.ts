import { block } from "vlens/css";

block(`
.family-page-container {
  max-width: 720px;
  margin: 0 auto;
  padding: 24px 16px 64px;
}
`);

block(`
.family-page h1 {
  margin: 0 0 12px;
  font-size: clamp(1.6rem, 6vw, 2.25rem);
  line-height: 1.2;
  text-align: center;
}
`);

block(`
.family-page-intro {
  margin: 0 auto 24px;
  max-width: 560px;
  white-space: pre-line;
  text-align: center;
  color: var(--text);
}
`);

block(`
.family-page-people {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 20px 16px;
  margin: 0 0 28px;
  padding: 0;
  list-style: none;
}
`);

block(`
.family-page-person {
  width: 150px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  text-align: center;
}
`);

block(`
.family-page-avatar {
  width: 96px;
  height: 96px;
  margin-bottom: 6px;
  overflow: hidden;
  border-radius: 50%;
  background: var(--surface);
  border: 1px solid var(--border);
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--muted);
  font-size: 2rem;
  font-weight: 600;
}
`);

block(`
.family-page-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
`);

block(`
.family-page-person-name {
  font-weight: 600;
}
`);

block(`
.family-page-person-meta {
  margin: 0;
  color: var(--muted);
  font-size: 0.9rem;
}
`);

block(`
.family-page-photos {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 8px;
}
`);

block(`
.family-page-photos img {
  width: 100%;
  aspect-ratio: 1;
  object-fit: cover;
  border-radius: 10px;
  background: var(--surface);
}
`);

block(`
.family-page .family-page-footer-note {
  margin-top: 32px;
  color: var(--muted);
  font-size: 0.8rem;
  text-align: center;
}
`);

block(`
.family-page-editor {
  max-width: 720px;
  margin: 0 auto;
  padding: 24px 16px 64px;
}
`);

block(`
.family-page-editor .section-description {
  color: var(--muted);
}
`);

block(`
.family-page-status {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin: 12px 0 20px;
  padding: 12px 16px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 12px;
}
`);

block(`
.family-page-status a {
  word-break: break-all;
}
`);

block(`
.family-page-editor .form-group {
  margin-bottom: 16px;
}
`);

block(`
.family-page-editor .family-page-member-fields .form-group {
  margin-bottom: 0;
}
`);

block(`
.family-page-member-options {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px 20px;
}
`);

block(`
.family-page-indexable {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-top: 20px;
}
`);

block(`
.family-page-address {
  display: flex;
  align-items: center;
  gap: 4px;
}
`);

block(`
.family-page-address-prefix {
  color: var(--muted);
  white-space: nowrap;
}
`);

block(`
.family-page-member-row {
  margin: 8px 0;
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: 10px;
}
`);

block(`
.family-page-member-fields {
  display: grid;
  gap: 8px;
  margin-top: 8px;
}
`);

block(`
.family-page-check {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: normal;
}
`);

block(`
.family-page-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin: 20px 0;
}
`);

block(`
.family-page-preview {
  margin-top: 32px;
  padding: 16px;
  border: 1px dashed var(--border);
  border-radius: 12px;
}
`);

block(`
.family-page-preview > h2 {
  margin: 0 0 12px;
  color: var(--muted);
  font-size: 0.9rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
`);
