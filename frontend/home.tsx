import * as preact from "preact";
import * as rpc from "vlens/rpc";
import * as auth from "./authCache";
import * as core from "vlens/core";
import * as server from "./server";
import { Header, Footer } from "./layout";
import { ensureNoAuthInFetch } from "./authHelpers";

type Data = {};

export async function fetch(route: string, prefix: string) {
  if (!await ensureNoAuthInFetch()) {
    return rpc.ok<Data>({});
  }

  return rpc.ok<Data>({});
}

export function view(
  route: string,
  prefix: string,
  data: Data,
): preact.ComponentChild {
  // Redirect authenticated users to dashboard
  const currentAuth = auth.getAuth();
  if (currentAuth && currentAuth.id > 0) {
    core.setRoute('/dashboard');
    return null;
  }

  return (
    <div>
      <Header isHome={true} />
      <main id="app" className="landing-container">
        <LandingPage />
      </main>
      <Footer />
    </div>
  );
}

const LandingPage = () => (
  <div className="landing-page">
    <section className="landing-hero">
      <div className="hero-content">
        <h1 className="hero-title">Family Portal</h1>
        <p className="hero-subtitle">
          A private space for your family to share photos, coordinate schedules, and stay connected.
        </p>
        <div className="hero-actions">
          <a href="/create-account" className="btn btn-primary btn-large">
            Create Account
          </a>
          <a href="/login" className="btn btn-secondary btn-large">
            Log In
          </a>
        </div>
      </div>
      <div className="hero-visual">
        <div className="floating-card card-1">
          <div className="card-icon">📸</div>
          <div className="card-text">Photos</div>
        </div>
        <div className="floating-card card-2">
          <div className="card-icon">📅</div>
          <div className="card-text">Calendar</div>
        </div>
        <div className="floating-card card-3">
          <div className="card-icon">💬</div>
          <div className="card-text">Messages</div>
        </div>
      </div>
    </section>

    <section className="features-section">
      <h2 className="section-title">What you can do</h2>
      <div className="features-grid">
        <div className="feature-card">
          <div className="feature-icon">🏠</div>
          <h3>Family Space</h3>
          <p>Create a private group just for your family. Invite members with a simple link.</p>
        </div>
        <div className="feature-card">
          <div className="feature-icon">📸</div>
          <h3>Photo Albums</h3>
          <p>Share photos in organized albums. Everyone can contribute and download.</p>
        </div>
        <div className="feature-card">
          <div className="feature-icon">📅</div>
          <h3>Shared Calendar</h3>
          <p>Keep track of birthdays, events, and family plans in one place.</p>
        </div>
        <div className="feature-card">
          <div className="feature-icon">💬</div>
          <h3>Group Chat</h3>
          <p>Simple messaging to stay in touch. No ads, no algorithms, just family.</p>
        </div>
      </div>
    </section>

    <section className="cta-section">
      <div className="cta-content">
        <h2>Getting started is simple</h2>
        <p>Create an account, set up your family group, and invite members.</p>
        <div className="cta-actions">
          <a href="/create-account" className="btn btn-primary btn-large">
            Create Account
          </a>
        </div>
      </div>
    </section>
  </div>
);

