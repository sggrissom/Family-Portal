import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as core from "vlens/core";
import * as auth from "../../lib/authCache";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { ensureNoAuthInFetch } from "../../lib/authHelpers";
import {
  OAuthButtons,
  AuthProviders,
  allProvidersOff,
  loadProviders,
  anyProvider,
} from "../../components/OAuthButtons";
import "./login-styles";

type Data = {
  providers: AuthProviders;
};

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
    return rpc.ok<Data>({ providers: allProvidersOff });
  }

  return rpc.ok<Data>({ providers: await loadProviders() });
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
        <LoginPage form={form} providers={data.providers} />
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
      rpc.setAuthHeaders({ "x-auth-token": result.token });
      auth.setAuth(result.auth);
      core.setRoute("/dashboard");
    } else {
      form.error = result.error || "Login failed";
    }
  } catch (error) {
    form.loading = false;
    form.error = "Network error. Please try again.";
  }

  vlens.scheduleRedraw();

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
  providers: AuthProviders;
}

const LoginPage = ({ form, providers }: LoginPageProps) => (
  <div className="login-page">
    <div className="auth-card">
      <div className="auth-header">
        <h1>Welcome Back</h1>
        <p>Sign in to your family portal</p>
      </div>

      {form.error && (
        <div className="error-message" role="alert">
          {form.error}
        </div>
      )}

      <div className="auth-methods">
        <OAuthButtons providers={providers} disabled={form.loading} />

        {anyProvider(providers) && (
          <div className="auth-divider">
            <span>or</span>
          </div>
        )}

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
