import * as preact from "preact";
import * as rpc from "vlens/rpc";
import { Header } from "./header"

type Data = {};

export async function fetch(route: string, prefix: string) {
  return rpc.ok<Data>({});
}

export function view(
  route: string,
  prefix: string,
  data: Data,
): preact.ComponentChild {
  return (
    <div>
      <Header isHome={true} />
      <main id="app" class="landing-container">
        <LandingPage />
      </main>
      <Footer />
    </div>
  );
}

const LandingPage = () => (
  <div class="landing-page">
    <section class="landing-hero">
      <div class="hero-content">
        <h1 class="hero-title">Family Portal</h1>
        <p class="hero-subtitle">
          A private space for your family to share photos, coordinate schedules, and stay connected.
        </p>
        <div class="hero-actions">
          <button class="btn btn-primary btn-large">
            Create Account
          </button>
          <button class="btn btn-secondary btn-large">
            Log In
          </button>
        </div>
      </div>
      <div class="hero-visual">
        <div class="floating-card card-1">
          <div class="card-icon">📸</div>
          <div class="card-text">Photos</div>
        </div>
        <div class="floating-card card-2">
          <div class="card-icon">📅</div>
          <div class="card-text">Calendar</div>
        </div>
        <div class="floating-card card-3">
          <div class="card-icon">💬</div>
          <div class="card-text">Messages</div>
        </div>
      </div>
    </section>

    <section class="features-section">
      <h2 class="section-title">What you can do</h2>
      <div class="features-grid">
        <div class="feature-card">
          <div class="feature-icon">🏠</div>
          <h3>Family Space</h3>
          <p>Create a private group just for your family. Invite members with a simple link.</p>
        </div>
        <div class="feature-card">
          <div class="feature-icon">📸</div>
          <h3>Photo Albums</h3>
          <p>Share photos in organized albums. Everyone can contribute and download.</p>
        </div>
        <div class="feature-card">
          <div class="feature-icon">📅</div>
          <h3>Shared Calendar</h3>
          <p>Keep track of birthdays, events, and family plans in one place.</p>
        </div>
        <div class="feature-card">
          <div class="feature-icon">💬</div>
          <h3>Group Chat</h3>
          <p>Simple messaging to stay in touch. No ads, no algorithms, just family.</p>
        </div>
      </div>
    </section>

    <section class="cta-section">
      <div class="cta-content">
        <h2>Getting started is simple</h2>
        <p>Create an account, set up your family group, and invite members.</p>
        <div class="cta-actions">
          <button class="btn btn-primary btn-large">
            Create Account
          </button>
        </div>
      </div>
    </section>
  </div>
);

const Footer = () => (
  <footer className="site-footer">
    <p>
      © <span id="year">2025</span> Family Portal. All rights reserved.
    </p>
  </footer>
);
