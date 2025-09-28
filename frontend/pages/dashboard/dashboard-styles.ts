import { block } from "vlens/css";

// People-First Dashboard Redesign - Massive Photos & Clean Navigation

block(`
.dashboard-container {
  max-width: 1400px;
  padding: 40px 20px;
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
  text-align: center;
  margin-bottom: 50px;
}
`);

block(`
.dashboard-header h1 {
  font-size: clamp(2.2rem, 5vw, 3rem);
  margin: 0 0 12px;
  color: var(--text);
  font-weight: 700;
}
`);

block(`
.dashboard-header p {
  font-size: 1.3rem;
  color: var(--muted);
  margin: 0;
}
`);

// NEW LAYOUT STRUCTURE - PEOPLE FIRST
block(`
.dashboard-content {
  display: flex;
  flex-direction: column;
  gap: 40px;
}
`);

// FAMILY SECTION - PRIMARY FOCUS
block(`
.family-section {
}
`);

block(`
.section-header {
  margin-bottom: 40px;
}
`);

block(`
.section-header h2 {
  font-size: 2.2rem;
  margin: 0;
  color: var(--text);
  font-weight: 700;
  background: linear-gradient(135deg, var(--text) 0%, var(--accent) 100%);
  background-clip: text;
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
}
`);

// PEOPLE GROUPS - ORGANIZED BY FAMILY STRUCTURE
block(`
.people-groups {
  display: flex;
  flex-direction: column;
  gap: 50px;
}
`);

block(`
.people-group h3 {
  font-size: 1.6rem;
  margin: 0 0 24px;
  color: var(--text);
  border-bottom: 3px solid var(--accent);
  padding-bottom: 12px;
  display: inline-block;
  font-weight: 600;
}
`);

// PEOPLE GRID - SIMPLIFIED AND SPACIOUS
block(`
.people-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(420px, 1fr));
  gap: 32px;
}
`);

// DRAMATICALLY LARGER PERSON CARDS
block(`
.person-card {
  display: flex;
  align-items: center;
  gap: 32px;
  background: var(--surface);
  border: 2px solid var(--border);
  border-radius: 24px;
  padding: 36px;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  cursor: pointer;
  position: relative;
  min-height: 180px;
}
`);

block(`
.person-card:hover {
  transform: translateY(-6px) scale(1.02);
  border-color: var(--accent);
  box-shadow:
    0 16px 40px rgba(0, 0, 0, 0.12),
    0 0 0 1px rgba(105, 219, 124, 0.2),
    0 0 20px rgba(105, 219, 124, 0.1);
}
`);

// MASSIVE PROFILE PHOTOS - 160px on desktop!
block(`
.person-avatar {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 160px;
  height: 160px;
  background: linear-gradient(135deg, var(--bg) 0%, var(--surface) 100%);
  border-radius: 50%;
  border: 5px solid var(--border);
  font-size: 5rem;
  flex-shrink: 0;
  transition: all 0.3s ease;
  position: relative;
  overflow: hidden;
}
`);

block(`
.person-card:hover .person-avatar {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px rgba(105, 219, 124, 0.2);
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

// ENHANCED PERSON INFO
block(`
.person-info {
  flex: 1;
  min-width: 0;
}
`);

block(`
.person-info h4 {
  font-size: 1.8rem;
  margin: 0 0 12px;
  color: var(--text);
  font-weight: 700;
  line-height: 1.2;
}
`);

block(`
.person-details {
  font-size: 1.2rem;
  color: var(--muted);
  margin: 0;
  font-weight: 500;
}
`);

// QUICK ACTIONS - COMPACT AND UNOBTRUSIVE
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
  transform: translateX(2px);
}
`);

// EMPTY STATE WITH BETTER VISUAL HIERARCHY
block(`
.empty-state {
  text-align: center;
  padding: 80px 20px;
  background: var(--surface);
  border: 2px dashed var(--border);
  border-radius: 24px;
  background-image: radial-gradient(circle at 50% 50%, rgba(105, 219, 124, 0.05) 0%, transparent 70%);
  grid-column: 1 / -1;
}
`);

block(`
.empty-state p {
  font-size: 1.3rem;
  color: var(--muted);
  margin: 0 0 32px;
  font-weight: 500;
}
`);

// CLICKABLE STYLING
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

// DESKTOP LAYOUT - HORIZONTAL QUICK ACTIONS
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

// RESPONSIVE DESIGN - MOBILE FIRST APPROACH
block(`
@media (max-width: 1200px) {
  .dashboard-content {
    gap: 50px;
  }

  .people-groups {
    gap: 40px;
  }

  .people-group h3 {
    font-size: 1.4rem;
    margin-bottom: 20px;
  }

  .people-grid {
    grid-template-columns: repeat(auto-fit, minmax(350px, 1fr));
    gap: 24px;
  }

  .person-avatar {
    width: 140px;
    height: 140px;
    font-size: 4.5rem;
  }

  .action-links {
    grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
    gap: 20px;
  }
}
`);

block(`
@media (max-width: 768px) {
  .dashboard-container {
    padding: 30px 16px;
  }

  .dashboard-header {
    margin-bottom: 40px;
  }

  .dashboard-header h1 {
    font-size: 2rem;
  }

  .dashboard-header p {
    font-size: 1.1rem;
  }

  .section-header h2 {
    font-size: 1.8rem;
  }

  .people-groups {
    gap: 30px;
  }

  .people-group h3 {
    font-size: 1.3rem;
    margin-bottom: 16px;
  }

  .people-grid {
    grid-template-columns: 1fr;
    gap: 20px;
  }

  .person-card {
    padding: 28px;
    gap: 24px;
    min-height: 140px;
  }

  .person-avatar {
    width: 120px;
    height: 120px;
    font-size: 4rem;
    border-width: 4px;
  }

  .person-info h4 {
    font-size: 1.5rem;
  }

  .person-details {
    font-size: 1.1rem;
  }


  .quick-actions {
    padding: 20px;
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
  .dashboard-header h1 {
    font-size: 1.8rem;
  }

  .section-header h2 {
    font-size: 1.6rem;
  }

  .people-group h3 {
    font-size: 1.2rem;
    margin-bottom: 14px;
  }

  .person-card {
    padding: 24px;
    gap: 20px;
    min-height: 120px;
  }

  .person-avatar {
    width: 100px;
    height: 100px;
    font-size: 3.5rem;
  }

  .person-info h4 {
    font-size: 1.3rem;
  }

  .person-details {
    font-size: 1rem;
  }

}
`);
