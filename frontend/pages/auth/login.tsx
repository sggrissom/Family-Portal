import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as core from "vlens/core";
import * as auth from "../../lib/authCache";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { ensureNoAuthInFetch } from "../../lib/authHelpers";
import "./login-styles";

type Data = {};

type LoginForm = {
  email: string;
  password: string;
  remember: boolean;
  error: string;
  loading: boolean;
};

const useLoginForm = vlens.declareHook(
  (): LoginForm => ({
    email: "",
    password: "",
    remember: false,
    error: "",
    loading: false,
  })
);

export async function fetch(route: string, prefix: string) {
  if (!(await ensureNoAuthInFetch())) {
    return rpc.ok<Data>({});
  }

  return rpc.ok<Data>({});
}

export function view(route: string, prefix: string, data: Data): preact.ComponentChild {
  const currentAuth = auth.getAuth();
  if (currentAuth && currentAuth.id > 0) {
    core.setRoute("/dashboard");
  }

  const form = useLoginForm();
  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="login-container">
        <LoginPage form={form} />
      </main>
      <Footer />
    </div>
  );
}

async function onLoginClicked(form: LoginForm, event: Event) {
  event.preventDefault();
  form.loading = true;
  form.error = "";
  vlens.scheduleRedraw();

  const nativeFetch = window.fetch.bind(window);
  try {
    const res = await nativeFetch("/api/login", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        email: form.email,
        password: form.password,
      }),
    });

    const result = await res.json();
    form.loading = false;

    if (result.success) {
      // Set auth token for future RPC calls
      rpc.setAuthHeaders({ "x-auth-token": result.token });
      // Cache auth data
      auth.setAuth(result.auth);
      // Redirect to dashboard
      core.setRoute("/dashboard");
    } else {
      form.error = result.error || "Login failed";
    }
  } catch (error) {
    form.loading = false;
    form.error = "Network error. Please try again.";
  }

  vlens.scheduleRedraw();

  // Scroll to error if there is one
  if (form.error) {
    setTimeout(() => {
      const errorElement = document.querySelector(".error-message");
      if (errorElement) {
        errorElement.scrollIntoView({ behavior: "smooth", block: "center" });
      }
    }, 100);
  }
}

interface LoginPageProps {
  form: LoginForm;
}

const LoginPage = ({ form }: LoginPageProps) => (
  <div className="login-page">
    <div className="auth-card">
      <div className="auth-header">
        <h1>Welcome Back</h1>
        <p>Sign in to your family portal</p>
      </div>

      {form.error && <div className="error-message">{form.error}</div>}

      <div className="auth-methods">
        <button
          className="btn btn-google"
          disabled={form.loading}
          onClick={() => (window.location.href = "/api/login/google")}
        >
          <GoogleIcon />
          Continue with Google
        </button>

        <div className="auth-divider">
          <span>or</span>
        </div>

        <form className="auth-form" onSubmit={vlens.cachePartial(onLoginClicked, form)}>
          <div className="form-group">
            <label htmlFor="email">Email Address</label>
            <input
              type="email"
              id="email"
              placeholder="Enter your email"
              {...vlens.attrsBindInput(vlens.ref(form, "email"))}
              required
              disabled={form.loading}
            />
          </div>

          <div className="form-group">
            <label htmlFor="password">Password</label>
            <input
              type="password"
              id="password"
              placeholder="Enter your password"
              {...vlens.attrsBindInput(vlens.ref(form, "password"))}
              required
              disabled={form.loading}
            />
          </div>

          <div className="form-options">
            <label className="checkbox-label">
              <input
                type="checkbox"
                {...vlens.attrsBindInput(vlens.ref(form, "remember"))}
                disabled={form.loading}
              />
              <span className="checkbox-text">Remember me</span>
            </label>
            <a href="/forgot-password" className="auth-link">
              Forgot password?
            </a>
          </div>

          <button
            type="submit"
            className="btn btn-primary btn-large auth-submit"
            disabled={form.loading}
          >
            {form.loading ? "Signing In..." : "Sign In"}
          </button>
        </form>
      </div>

      <div className="auth-footer">
        <p>
          Don't have an account?
          <a href="/create-account" className="auth-link">
            Create account
          </a>
        </p>
      </div>
    </div>
  </div>
);

const GoogleIcon = () => (
  <svg width="18" height="18" viewBox="0 0 24 24" fill="none">
    <path
      d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"
      fill="#4285F4"
    />
    <path
      d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"
      fill="#34A853"
    />
    <path
      d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"
      fill="#FBBC05"
    />
    <path
      d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"
      fill="#EA4335"
    />
  </svg>
);
