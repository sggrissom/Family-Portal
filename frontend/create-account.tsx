import * as preact from "preact";
import * as rpc from "vlens/rpc";
import { Header, Footer } from "./layout"

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
      <Header isHome={false} />
      <main id="app" className="create-account-container">
        <CreateAccountPage />
      </main>
      <Footer />
    </div>
  );
}

const CreateAccountPage = () => (
  <div className="create-account-page">
    <div className="auth-card">
      <div className="auth-header">
        <h1>Create Account</h1>
        <p>Join your family portal</p>
      </div>

      <form className="auth-form">
        <div className="form-group">
          <label htmlFor="name">Full Name</label>
          <input
            type="text"
            id="name"
            name="name"
            placeholder="Enter your full name"
            required
          />
        </div>

        <div className="form-group">
          <label htmlFor="email">Email Address</label>
          <input
            type="email"
            id="email"
            name="email"
            placeholder="Enter your email"
            required
          />
        </div>

        <div className="form-group">
          <label htmlFor="password">Password</label>
          <input
            type="password"
            id="password"
            name="password"
            placeholder="Create a password"
            required
          />
        </div>

        <div className="form-group">
          <label htmlFor="confirmPassword">Confirm Password</label>
          <input
            type="password"
            id="confirmPassword"
            name="confirmPassword"
            placeholder="Confirm your password"
            required
          />
        </div>

        <div className="form-group">
          <label htmlFor="familyCode">Family Code (Optional)</label>
          <input
            type="text"
            id="familyCode"
            name="familyCode"
            placeholder="Enter family invite code"
          />
          <small className="form-hint">
            Leave blank to create a new family group
          </small>
        </div>

        <button type="submit" className="btn btn-primary btn-large auth-submit">
          Create Account
        </button>
      </form>

      <div className="auth-footer">
        <p>
          Already have an account?
          <a href="/login" className="auth-link">Sign in</a>
        </p>
      </div>
    </div>
  </div>
);

