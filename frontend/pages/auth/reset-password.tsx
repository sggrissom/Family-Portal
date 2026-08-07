import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as core from "vlens/core";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { ensureNoAuthInFetch } from "../../lib/authHelpers";
import "./create-account-styles";
import "./login-styles";

type Data = {
  token: string;
  valid: boolean;
};

type ResetPasswordForm = {
  password: string;
  confirmPassword: string;
  error: string;
  loading: boolean;
  done: boolean;
};

const useResetPasswordForm = vlens.declareHook(
  (): ResetPasswordForm => ({
    password: "",
    confirmPassword: "",
    error: "",
    loading: false,
    done: false,
  })
);

function tokenFromLocation(): string {
  if (typeof window === "undefined") return "";
  return new URLSearchParams(window.location.search).get("token") ?? "";
}

export async function fetch(route: string, prefix: string) {
  await ensureNoAuthInFetch();

  const token = tokenFromLocation();
  if (!token) {
    return rpc.ok<Data>({ token: "", valid: false });
  }

  // Check the link up front so an expired one is reported before the user
  // bothers choosing a password.
  let [resp, err] = await server.ValidatePasswordResetToken({ token });
  return rpc.ok<Data>({ token, valid: resp?.valid ?? false });
}

export function view(route: string, prefix: string, data: Data): preact.ComponentChild {
  const form = useResetPasswordForm();
  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="login-container">
        <ResetPasswordPage data={data} form={form} />
      </main>
      <Footer />
    </div>
  );
}

async function onResetPasswordClicked(data: Data, form: ResetPasswordForm, event: Event) {
  event.preventDefault();
  form.loading = true;
  form.error = "";
  vlens.scheduleRedraw();

  let [resp, err] = await server.ResetPassword({
    token: data.token,
    password: form.password,
    confirmPassword: form.confirmPassword,
  });

  form.loading = false;

  if (resp && resp.success) {
    form.done = true;
    form.password = "";
    form.confirmPassword = "";
  } else {
    form.error = resp?.error || err || "Could not reset your password. Please try again.";
  }

  vlens.scheduleRedraw();
}

interface ResetPasswordPageProps {
  data: Data;
  form: ResetPasswordForm;
}

const ResetPasswordPage = ({ data, form }: ResetPasswordPageProps) => (
  <div className="login-page">
    <div className="auth-card">
      {form.done ? (
        <PasswordChanged />
      ) : data.valid ? (
        <ResetForm data={data} form={form} />
      ) : (
        <InvalidLink />
      )}
    </div>
  </div>
);

const PasswordChanged = () => (
  <div>
    <div className="auth-header">
      <h1>Password Updated</h1>
      <p>You've been signed out on your other devices</p>
    </div>
    <div className="success-message">Your new password is ready to use.</div>
    <button
      className="btn btn-primary btn-large auth-submit"
      onClick={() => core.setRoute("/login")}
    >
      Sign In
    </button>
  </div>
);

const InvalidLink = () => (
  <div>
    <div className="auth-header">
      <h1>Link Expired</h1>
      <p>This password reset link is invalid or has already been used</p>
    </div>
    <div className="error-message">Reset links are good for one hour and one use.</div>
    <a href="/forgot-password" className="btn btn-primary btn-large auth-submit">
      Request a New Link
    </a>
  </div>
);

const ResetForm = ({ data, form }: ResetPasswordPageProps) => (
  <div>
    <div className="auth-header">
      <h1>Choose a New Password</h1>
      <p>Pick something at least 8 characters long</p>
    </div>

    {form.error && <div className="error-message">{form.error}</div>}

    <form className="auth-form" onSubmit={vlens.cachePartial(onResetPasswordClicked, data, form)}>
      <div className="form-group">
        <label htmlFor="password">New Password</label>
        <input
          type="password"
          id="password"
          placeholder="Enter a new password"
          autoComplete="new-password"
          {...vlens.attrsBindInput(vlens.ref(form, "password"))}
          required
          disabled={form.loading}
        />
      </div>

      <div className="form-group">
        <label htmlFor="confirmPassword">Confirm New Password</label>
        <input
          type="password"
          id="confirmPassword"
          placeholder="Re-enter the new password"
          autoComplete="new-password"
          {...vlens.attrsBindInput(vlens.ref(form, "confirmPassword"))}
          required
          disabled={form.loading}
        />
      </div>

      <button
        type="submit"
        className="btn btn-primary btn-large auth-submit"
        disabled={form.loading}
      >
        {form.loading ? "Updating..." : "Update Password"}
      </button>
    </form>
  </div>
);
