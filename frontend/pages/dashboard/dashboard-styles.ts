import { block } from "vlens/css";

// Enhanced Dashboard Styles - Making People the Focus

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
  margin-bottom: 60px;
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

// ENHANCED FAMILY SECTION
block(`
.family-section {
  margin-bottom: 100px;
  order: 1;
}
`);

block(`
.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
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

block(`
.people-grid {
  display: flex;
  flex-direction: column;
  gap: 60px;
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

// MASSIVE IMPROVEMENT TO PERSON CARDS - LARGER AND MORE PROMINENT
block(`
.people-list {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 24px;
  padding: 8px;
}
`);

block(`
.person-card {
  display: flex;
  align-items: center;
  gap: 24px;
  background: var(--surface);
  border: 2px solid var(--border);
  border-radius: 20px;
  padding: 32px;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  cursor: pointer;
  position: relative;
  min-height: 140px;
}
`);

block(`
.person-card:hover {
  transform: translateY(-4px) scale(1.01);
  border-color: var(--accent);
  box-shadow:
    0 12px 32px rgba(0, 0, 0, 0.08),
    0 0 0 1px rgba(105, 219, 124, 0.2),
    0 0 16px rgba(105, 219, 124, 0.08);
}
`);

// DRAMATICALLY LARGER PROFILE PHOTOS
block(`
.person-avatar {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100px;
  height: 100px;
  background: linear-gradient(135deg, var(--bg) 0%, var(--surface) 100%);
  border-radius: 50%;
  border: 4px solid var(--border);
  font-size: 3.5rem;
  flex-shrink: 0;
  transition: all 0.3s ease;
  position: relative;
  overflow: hidden;
}
`);

block(`
.person-card:hover .person-avatar {
  border-color: var(--accent);
  box-shadow: 0 0 0 2px rgba(105, 219, 124, 0.2);
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

// ENHANCED PERSON INFO WITH BETTER TYPOGRAPHY
block(`
.person-info {
  flex: 1;
  min-width: 0;
}
`);

block(`
.person-info h4 {
  font-size: 1.5rem;
  margin: 0 0 8px;
  color: var(--text);
  font-weight: 700;
  line-height: 1.2;
}
`);

block(`
.person-details {
  font-size: 1.1rem;
  color: var(--muted);
  margin: 0;
  font-weight: 500;
}
`);

// QUICK ACTIONS - DE-EMPHASIZED BUT STILL ACCESSIBLE
block(`
.dashboard-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 20px;
  max-width: 1000px;
  margin: 0 auto;
  order: 2;
}
`);

block(`
.dashboard-card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 24px;
  text-align: center;
  transition: all 0.3s ease;
}
`);

block(`
.dashboard-card:hover {
  transform: translateY(-2px);
  border-color: var(--accent);
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.08);
}
`);

block(`
.dashboard-card .card-icon {
  font-size: 2.5rem;
  margin-bottom: 12px;
  display: block;
}
`);

block(`
.dashboard-card h3 {
  font-size: 1.1rem;
  margin: 0 0 6px;
  color: var(--text);
}
`);

block(`
.dashboard-card p {
  color: var(--muted);
  margin: 0 0 16px;
  line-height: 1.4;
  font-size: 0.95rem;
}
`);

// EMPTY STATE WITH BETTER VISUAL HIERARCHY
block(`
.empty-state {
  text-align: center;
  padding: 80px 20px;
  background: var(--surface);
  border: 2px dashed var(--border);
  border-radius: 20px;
  background-image: radial-gradient(circle at 50% 50%, rgba(105, 219, 124, 0.05) 0%, transparent 70%);
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

// RESPONSIVE DESIGN - MOBILE FIRST
block(`
@media (max-width: 1200px) {
  .people-list {
    grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
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

  .family-section {
    margin-bottom: 50px;
  }

  .section-header {
    flex-direction: column;
    gap: 20px;
    align-items: stretch;
    margin-bottom: 30px;
  }

  .section-header h2 {
    font-size: 1.8rem;
    text-align: center;
  }

  .people-grid {
    gap: 30px;
  }

  .people-group h3 {
    font-size: 1.4rem;
    margin-bottom: 20px;
  }

  .people-list {
    grid-template-columns: 1fr;
    gap: 20px;
  }

  .person-card {
    padding: 24px;
    gap: 20px;
    min-height: 120px;
  }

  .person-avatar {
    width: 80px;
    height: 80px;
    font-size: 3rem;
  }

  .person-info h4 {
    font-size: 1.3rem;
  }

  .person-details {
    font-size: 1rem;
  }

  .dashboard-grid {
    grid-template-columns: 1fr;
    gap: 16px;
  }

  .dashboard-card {
    padding: 20px;
  }

  .dashboard-card .card-icon {
    font-size: 2rem;
    margin-bottom: 10px;
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

  .person-card {
    padding: 20px;
    gap: 16px;
  }

  .person-avatar {
    width: 70px;
    height: 70px;
    font-size: 2.5rem;
  }

  .person-info h4 {
    font-size: 1.2rem;
  }

  .person-details {
    font-size: 0.95rem;
  }
}
`);
