import { block } from "vlens/css";

block(`
.dashboard-container {
  max-width: 1000px;
  padding: 28px 16px 72px;
  margin: 0 auto;
}
`);

block(`
.dashboard-page {
  width: 100%;
}
`);

block(`
.dashboard-header {
  margin-bottom: 32px;
}
`);

block(`
.dashboard-header h1 {
  font-size: clamp(1.75rem, 4vw, 2.1rem);
  margin: 0 0 6px;
  color: var(--text);
  font-weight: 700;
}
`);

block(`
.dashboard-header p {
  font-size: 1.05rem;
  color: var(--muted);
  margin: 0;
}
`);

block(`
.dashboard-content {
  display: flex;
  flex-direction: column;
  gap: 28px;
}
`);

block(`
.family-section {
}
`);

block(`
.section-header {
  margin-bottom: 20px;
}
`);

block(`
.section-header h2 {
  font-size: 1.5rem;
  margin: 0;
  color: var(--text);
  font-weight: 700;
}
`);

block(`
.people-groups {
  display: flex;
  flex-direction: column;
  gap: 30px;
}
`);

block(`
.people-group h3 {
  font-size: 1.05rem;
  margin: 0 0 14px;
  color: var(--text);
  border-bottom: 2px solid var(--accent);
  padding-bottom: 6px;
  display: inline-block;
  font-weight: 600;
}
`);

block(`
.people-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 14px;
}
`);

block(`
.person-card {
  display: flex;
  align-items: center;
  gap: 16px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 16px 18px;
  transition:
    border-color 0.12s ease,
    background-color 0.12s ease;
  cursor: pointer;
  position: relative;
}
`);

block(`
.person-card:hover {
  border-color: var(--accent);
  background: var(--accent-soft);
}
`);

block(`
.person-avatar {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 64px;
  height: 64px;
  background: var(--bg);
  border-radius: 50%;
  border: 1px solid var(--border);
  font-size: 2rem;
  flex-shrink: 0;
  transition: border-color 0.12s ease;
  position: relative;
  overflow: hidden;
}
`);

block(`
.person-card:hover .person-avatar {
  border-color: var(--accent);
}
`);

block(`
.person-photo {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
`);

block(`
.person-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 100%;
}
`);

block(`
.person-info {
  flex: 1;
  min-width: 0;
}
`);

block(`
.person-info h4 {
  font-size: 1.05rem;
  margin: 0 0 3px;
  color: var(--text);
  font-weight: 700;
  line-height: 1.2;
}
`);

block(`
.dashboard-container .person-details {
  font-size: 0.9rem;
  color: var(--muted);
  margin: 0;
  font-weight: 500;
}
`);

block(`
.person-due-date {
  margin: 6px 0 0;
  font-size: 0.85rem;
  font-weight: 600;
  color: var(--accent);
}
`);

block(`
.person-trimester {
  margin: 4px 0 0;
  font-size: 0.8rem;
  font-weight: 500;
  color: var(--muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
`);

block(`
.quick-actions {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 16px;
  padding: 16px 20px;
  max-width: 1000px;
  margin: 0 auto;
  order: 2;
}
`);

block(`
.quick-actions h3 {
  font-size: 1.1rem;
  margin: 0 0 16px;
  color: var(--text);
  font-weight: 600;
  text-align: center;
}
`);

block(`
.action-links {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: 20px;
}
`);

block(`
.action-group h4 {
  font-size: 0.8rem;
  margin: 0 0 6px;
  color: var(--muted);
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}
`);

block(`
.action-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
`);

block(`
.action-link {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  border-radius: 6px;
  color: var(--text);
  text-decoration: none;
  font-size: 0.85rem;
  font-weight: 500;
  transition: all 0.2s ease;
}
`);

block(`
.action-link:hover {
  background: var(--hover-bg);
  color: var(--accent);
  text-decoration: none;
}
`);

block(`
.dashboard-container .empty-state {
  text-align: center;
  padding: 44px 20px;
  background: var(--surface);
  border: 1px dashed var(--control-border);
  border-radius: 12px;
  grid-column: 1 / -1;
}
`);

block(`
.empty-state p {
  font-size: 1.05rem;
  color: var(--muted);
  margin: 0 0 20px;
}
`);

block(`
.person-card.clickable {
  text-decoration: none;
  color: inherit;
}
`);

block(`
.person-card.clickable:hover {
  text-decoration: none;
  color: inherit;
}
`);

block(`
@media (min-width: 1201px) {
  .quick-actions {
    max-width: 1200px;
    padding: 20px 32px;
  }

  .quick-actions h3 {
    margin: 0 0 20px;
    font-size: 1rem;
  }

  .action-links {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    gap: 32px;
    align-items: flex-start;
  }

  .action-group {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 6px;
    min-width: 140px;
  }

  .action-group h4 {
    font-size: 0.75rem;
    text-align: center;
    margin: 0 0 8px;
  }

  .action-link {
    justify-content: center;
    padding: 8px 12px;
    font-size: 0.85rem;
    border-radius: 8px;
    min-width: 120px;
    text-align: center;
  }
}
`);

block(`
@media (max-width: 1200px) {
  .action-links {
    grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
    gap: 20px;
  }
}
`);

block(`
@media (max-width: 768px) {
  .dashboard-container {
    padding: 24px 16px 56px;
  }

  .dashboard-header {
    margin-bottom: 24px;
  }

  .section-header h2 {
    font-size: 1.35rem;
  }

  .people-grid {
    grid-template-columns: 1fr;
  }

  .quick-actions {
    padding: 16px;
  }

  .action-links {
    grid-template-columns: repeat(2, 1fr);
    gap: 16px;
  }

  .empty-state {
    padding: 60px 20px;
  }

  .empty-state p {
    font-size: 1.1rem;
    margin-bottom: 24px;
  }
}
`);

block(`
@media (max-width: 480px) {
  .person-card {
    padding: 14px;
    gap: 12px;
  }

  .person-avatar {
    width: 52px;
    height: 52px;
    font-size: 1.6rem;
  }
}
`);

block(`
.quick-actions {
  margin-top: 8px;
  padding: 20px;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--surface);
}
`);

block(`
.quick-actions-heading {
  display: flex;
  justify-content: space-between;
  gap: 24px;
  align-items: end;
  margin-bottom: 18px;
}
`);

block(`
.quick-actions .section-kicker {
  display: block;
  margin-bottom: 4px;
  color: var(--primary-accent);
  font-size: 0.75rem;
  font-weight: 750;
  letter-spacing: 0.09em;
  text-transform: uppercase;
}
`);

block(`
.quick-actions-heading h2 {
  margin: 0;
  color: var(--text);
  font-size: clamp(1.25rem, 3vw, 1.65rem);
}
`);

block(`
.quick-actions-heading p {
  max-width: 280px;
  margin: 0;
  color: var(--muted);
  font-size: 0.9rem;
  text-align: right;
}
`);

block(`
.quick-actions .action-links {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
}
`);

block(`
.action-card {
  display: grid;
  grid-template-columns: auto 1fr auto;
  align-items: center;
  gap: 12px;
  min-height: 84px;
  padding: 14px 16px;
  color: var(--text);
  text-decoration: none;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--bg);
  transition:
    border-color 0.12s ease,
    background-color 0.12s ease;
}
`);

block(`
.action-card:hover {
  border-color: var(--accent);
  background: var(--accent-soft);
}
`);

block(`
.action-card-featured {
  background: var(--accent-soft);
  border-color: var(--accent-soft-border);
}
`);

block(`
.action-icon {
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  border-radius: 12px;
  background: var(--surface);
  font-size: 1.4rem;
}
`);

block(`
.action-copy strong,
.action-copy small {
  display: block;
}
`);

block(`
.action-copy strong {
  margin-bottom: 4px;
  font-size: 0.98rem;
}
`);

block(`
.action-copy small {
  color: var(--muted);
  font-size: 0.79rem;
  line-height: 1.35;
}
`);

block(`
.action-arrow {
  color: var(--primary-accent);
  font-size: 1.2rem;
}
`);

block(`
.secondary-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 8px 22px;
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid var(--border);
}
`);

block(`
.secondary-actions a {
  color: var(--muted);
  font-size: 0.86rem;
  font-weight: 600;
  text-decoration: none;
}
`);

block(`
.secondary-actions a:hover {
  color: var(--primary-accent);
}
`);

block(`
@media (max-width: 900px) {
  .quick-actions .action-links {
    grid-template-columns: 1fr;
  }

  .action-card {
    min-height: 92px;
  }
}
`);

block(`
@media (max-width: 600px) {
  .quick-actions {
    padding: 18px;
  }

  .quick-actions-heading {
    display: block;
  }

  .quick-actions-heading p {
    margin-top: 8px;
    text-align: left;
  }

  .action-card {
    padding: 14px;
  }

  .secondary-actions {
    align-items: flex-start;
    flex-direction: column;
  }
}
`);

block(`
.person-born-action {
  margin: 4px 0 0;
  color: var(--accent);
  font-size: 0.85rem;
  font-weight: 700;
}
`);
