import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { ensureNoAuthInFetch } from "../../lib/authHelpers";
import "./create-account-styles";
import "./login-styles";

type Data = {};

type ForgotPasswordForm = {
  email: string;
  error: string;
  loading: boolean;
  submitted: boolean;
};

const useForgotPasswordForm = vlens.declareHook(
  (): ForgotPasswordForm => ({
    email: "",
    error: "",
    loading: false,
    submitted: false,
  })
);

export async function fetch(route: string, prefix: string) {
  if (!(await ensureNoAuthInFetch())) {
    return rpc.ok<Data>({});
  }

  return rpc.ok<Data>({});
}

export function view(route: string, prefix: string, data: Data): preact.ComponentChild {
  const form = useForgotPasswordForm();
  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="login-container">
        <ForgotPasswordPage form={form} />
      </main>
      <Footer />
    </div>
  );
}

async function onRequestResetClicked(form: ForgotPasswordForm, event: Event) {
  event.preventDefault();
  form.loading = true;
  form.error = "";
  vlens.scheduleRedraw();

  let [resp, err] = await server.RequestPasswordReset({ email: form.email });

  form.loading = false;

  if (resp && resp.success) {
    form.submitted = true;
  } else {
    form.error = resp?.error || err || "Could not send the reset email. Please try again.";
  }

  vlens.scheduleRedraw();
}

interface ForgotPasswordPageProps {
  form: ForgotPasswordForm;
}

const ForgotPasswordPage = ({ form }: ForgotPasswordPageProps) => (
  <div className="login-page">
    <div className="auth-card">
      {form.submitted ? <ResetRequested email={form.email} /> : <ResetRequestForm form={form} />}

      <div className="auth-footer">
        <p>
          Remembered it?
          <a href="/login" className="auth-link">
            Back to sign in
          </a>
        </p>
      </div>
    </div>
  </div>
);

// The confirmation deliberately does not say whether an account exists, so the
// page cannot be used to discover which addresses are registered.
const ResetRequested = ({ email }: { email: string }) => (
  <div>
    <div className="auth-header">
      <h1>Check Your Email</h1>
      <p>
        If an account exists for <strong>{email}</strong>, a password reset link is on its way.
      </p>
    </div>
    <div className="success-message">
      The link expires in one hour. If it does not arrive, check your spam folder.
    </div>
  </div>
);

const ResetRequestForm = ({ form }: { form: ForgotPasswordForm }) => (
  <div>
    <div className="auth-header">
      <h1>Forgot Password</h1>
      <p>Enter your email and we'll send you a reset link</p>
    </div>

    {form.error && (
      <div className="error-message" role="alert">
        {form.error}
      </div>
    )}

    <form className="auth-form" onSubmit={vlens.cachePartial(onRequestResetClicked, form)}>
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

      <button
        type="submit"
        className="btn btn-primary btn-large auth-submit"
        disabled={form.loading}
      >
        {form.loading ? "Sending..." : "Send Reset Link"}
      </button>
    </form>
  </div>
);
