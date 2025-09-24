import { block } from "vlens/css";

// Landing Page Styles
block(`
.landing-container {
  max-width: 1200px;
  padding: 0;
  margin: 0 auto;
}
`);

block(`
.landing-page {
  width: 100%;
}
`);

block(`
.landing-hero {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 60px;
  align-items: center;
  padding: 80px 20px;
  min-height: 60vh;
  background: linear-gradient(135deg, var(--surface) 0%, var(--bg) 100%);
}
`);

block(`
.hero-content {
  max-width: 600px;
}
`);

block(`
.hero-title {
  font-size: clamp(2rem, 5vw, 3.5rem);
  font-weight: 800;
  margin: 0 0 20px;
  background: linear-gradient(135deg, var(--text) 0%, var(--accent) 100%);
  background-clip: text;
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  line-height: 1.2;
}
`);

block(`
.hero-subtitle {
  font-size: clamp(1.1rem, 2vw, 1.4rem);
  color: var(--muted);
  margin: 0 0 40px;
  line-height: 1.5;
}
`);

block(`
.hero-actions {
  display: flex;
  gap: 16px;
  flex-wrap: wrap;
}
`);

block(`
.btn-large {
  padding: 16px 32px;
  font-size: 1.1rem;
  font-weight: 600;
  border-radius: 12px;
  transition: all 0.3s ease;
  cursor: pointer;
  border: none;
  min-width: 140px;
}
`);

block(`
.btn-secondary {
  background: transparent;
  color: var(--text);
  border: 2px solid var(--border);
}
`);

block(`
.btn-secondary:hover {
  background: var(--surface);
  border-color: var(--accent);
  transform: translateY(-2px);
}
`);

block(`
.btn-primary.btn-large:hover {
  transform: translateY(-2px);
  box-shadow: 0 10px 30px rgba(105, 219, 124, 0.3);
}
`);

block(`
.hero-visual {
  position: relative;
  height: 400px;
  display: flex;
  align-items: center;
  justify-content: center;
}
`);

block(`
.floating-card {
  position: absolute;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 16px;
  padding: 24px;
  box-shadow: 0 10px 40px rgba(0, 0, 0, 0.1);
  transition: all 0.3s ease;
  display: flex;
  align-items: center;
  gap: 12px;
}
`);

block(`
.floating-card:hover {
  transform: translateY(-5px);
  box-shadow: 0 15px 50px rgba(0, 0, 0, 0.15);
}
`);

block(`
.card-1 {
  top: 20px;
  left: 20px;
  animation: float 6s ease-in-out infinite;
}
`);

block(`
.card-2 {
  top: 140px;
  right: 40px;
  animation: float 6s ease-in-out infinite 2s;
}
`);

block(`
.card-3 {
  bottom: 40px;
  left: 60px;
  animation: float 6s ease-in-out infinite 4s;
}
`);

block(`
@keyframes float {
  0%, 100% {
    transform: translateY(0px);
  }
  50% {
    transform: translateY(-20px);
  }
}
`);

block(`
.card-icon {
  font-size: 2rem;
}
`);

block(`
.card-text {
  font-weight: 600;
  color: var(--text);
}
`);

block(`
.features-section {
  padding: 80px 20px;
  background: var(--bg);
}
`);

block(`
.section-title {
  text-align: center;
  font-size: clamp(1.8rem, 4vw, 2.5rem);
  font-weight: 700;
  margin: 0 0 60px;
  color: var(--text);
}
`);

block(`
.features-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
  gap: 30px;
  max-width: 1000px;
  margin: 0 auto;
}
`);

block(`
.feature-card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 16px;
  padding: 32px;
  text-align: center;
  transition: all 0.3s ease;
}
`);

block(`
.feature-card:hover {
  transform: translateY(-5px);
  border-color: var(--accent);
  box-shadow: 0 10px 40px rgba(0, 0, 0, 0.1);
}
`);

block(`
.feature-icon {
  font-size: 3rem;
  margin-bottom: 20px;
}
`);

block(`
.feature-card h3 {
  font-size: 1.3rem;
  margin: 0 0 12px;
  color: var(--text);
}
`);

block(`
.feature-card p {
  color: var(--muted);
  line-height: 1.6;
  margin: 0;
}
`);

block(`
.cta-section {
  padding: 100px 20px;
  background: linear-gradient(135deg, var(--surface) 0%, var(--bg) 100%);
  text-align: center;
}
`);

block(`
.cta-content h2 {
  font-size: clamp(1.8rem, 4vw, 2.8rem);
  margin: 0 0 16px;
  color: var(--text);
}
`);

block(`
.cta-content p {
  font-size: 1.2rem;
  color: var(--muted);
  margin: 0 0 40px;
}
`);

block(`
.cta-actions {
  display: flex;
  justify-content: center;
  gap: 20px;
}
`);

block(`
@media (max-width: 768px) {
  .landing-hero {
    grid-template-columns: 1fr;
    gap: 40px;
    padding: 60px 20px;
    text-align: center;
  }

  .hero-visual {
    display: none;
  }

  .hero-actions {
    justify-content: center;
  }

  .btn-large {
    width: 100%;
    max-width: 300px;
  }

  .features-grid {
    grid-template-columns: 1fr;
    gap: 20px;
  }

  .features-section {
    padding: 60px 20px;
  }

  .cta-section {
    padding: 60px 20px;
  }

  .cta-actions {
    flex-direction: column;
    align-items: center;
  }
}
`);

block(`
@media (max-width: 480px) {
  .hero-title {
    font-size: 2rem;
  }

  .hero-subtitle {
    font-size: 1rem;
  }

  .section-title {
    font-size: 1.5rem;
    margin: 0 0 40px;
  }

  .feature-card {
    padding: 24px;
  }

  .feature-icon {
    font-size: 2.5rem;
  }

  .cta-content h2 {
    font-size: 1.8rem;
  }

  .cta-content p {
    font-size: 1rem;
  }
}
`);