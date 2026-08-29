import { block } from "vlens/css";

block(`
.timeline-items {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
`);

block(`
.timeline-item {
  display: flex;
  align-items: flex-start;
  gap: 16px;
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 20px;
  transition: all 0.2s ease;
}
`);

block(`
.timeline-item:hover {
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
  border-color: var(--accent);
}
`);

block(`
.timeline-item-icon {
  font-size: 2rem;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 48px;
  height: 48px;
  border-radius: 50%;
  flex-shrink: 0;
}
`);

block(`
.milestone-item .timeline-item-icon {
  background: rgba(99, 102, 241, 0.1);
}
`);

block(`
.measurement-item .timeline-item-icon {
  background: rgba(16, 185, 129, 0.1);
}
`);

block(`
.photo-item .timeline-item-icon {
  background: rgba(236, 72, 153, 0.1);
}
`);

block(`
.timeline-item-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
}
`);

block(`
.timeline-item-header {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}
`);

block(`
.timeline-item-type {
  font-size: 0.85rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  padding: 4px 10px;
  border-radius: 6px;
}
`);

block(`
.milestone-type {
  background: rgba(99, 102, 241, 0.15);
  color: rgb(99, 102, 241);
}
`);

block(`
.measurement-type {
  background: rgba(16, 185, 129, 0.15);
  color: rgb(16, 185, 129);
}
`);

block(`
.photo-type {
  background: rgba(236, 72, 153, 0.15);
  color: rgb(236, 72, 153);
}
`);

block(`
.timeline-item-age {
  font-size: 0.9rem;
  color: var(--muted);
  font-weight: 500;
}
`);

block(`
.timeline-item-date {
  font-size: 0.85rem;
  color: var(--muted);
  margin-left: auto;
}
`);

block(`
.timeline-item-description {
  font-size: 1rem;
  color: var(--text);
  line-height: 1.6;
  white-space: pre-wrap;
}
`);

block(`
.measurement-value {
  font-size: 1.2rem;
  font-weight: 600;
  color: rgb(16, 185, 129);
}
`);

block(`
.timeline-item-actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}
`);

block(`
.photo-item-details {
  display: flex;
  gap: 16px;
  align-items: flex-start;
}
`);

block(`
.photo-thumbnail {
  display: block;
  color: inherit;
  text-decoration: none;
  width: 120px;
  height: 120px;
  border-radius: 8px;
  overflow: hidden;
  cursor: pointer;
  transition: transform 0.2s ease;
  flex-shrink: 0;
  position: relative;
}
`);

block(`
.photo-thumbnail:hover {
  transform: scale(1.05);
}
`);

block(`
.timeline-photo-image {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
`);

block(`
.photo-info {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}
`);

block(`
.photo-title {
  font-size: 1rem;
  font-weight: 600;
  color: var(--text);
}
`);

block(`
.photo-description {
  font-size: 0.9rem;
  color: var(--muted);
  line-height: 1.5;
}
`);
