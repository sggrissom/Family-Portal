import { block } from "vlens/css";

block(`
.milestone-list {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  margin-bottom: 2rem;
}
`);

block(`
.milestone-item {
  display: flex;
  gap: 1rem;
  padding: 1rem;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  transition: all var(--transition-speed) ease;
}
`);

block(`
.milestone-item:hover {
  background: var(--hover-bg);
  border-color: var(--accent);
}
`);

block(`
.milestone-icon {
  font-size: 1.5rem;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  background: var(--accent);
  border-radius: 50%;
}
`);

block(`
.milestone-content {
  flex: 1;
  min-width: 0;
}
`);

block(`
.milestone-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 0.5rem;
  gap: 1rem;
}
`);

block(`
.milestone-category {
  font-weight: 600;
  color: var(--accent);
  font-size: 0.9rem;
}
`);

block(`
.milestone-date {
  color: var(--muted);
  font-size: 0.85rem;
  flex-shrink: 0;
}
`);

block(`
.milestone-description {
  color: var(--text);
  line-height: 1.4;
}
`);

block(`
.timeline-actions {
  text-align: center;
  margin-top: 1rem;
}
`);

block(`
@media (max-width: 600px) {
  .milestone-item {
    gap: 0.75rem;
    padding: 0.75rem;
  }

  .milestone-icon {
    width: 35px;
    height: 35px;
    font-size: 1.25rem;
  }

  .milestone-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 0.25rem;
  }

  .milestone-category {
    font-size: 0.85rem;
  }

  .milestone-date {
    font-size: 0.8rem;
  }
}
`);
