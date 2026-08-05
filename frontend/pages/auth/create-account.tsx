import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as core from "vlens/core";
import * as auth from "../../lib/authCache";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import "./create-account-styles";

type CreateAccountForm = {
  name: string;
  email: string;
  password: string;
  confirmPassword: string;
  familyCode: string;
  initialPersonName: string;
  initialPersonGender: number;
  initialPersonBirthdate: string;
  error: string;
  loading: boolean;
};

type Data = {};

const useCreateAccountForm = vlens.declareHook(
  (): CreateAccountForm => ({
    name: "",
    email: "",
    password: "",
    confirmPassword: "",
    familyCode: "",
    initialPersonName: "",
    initialPersonGender: 2,
    initialPersonBirthdate: "",
    error: "",
    loading: false,
  })
);

export async function fetch(route: string, prefix: string) {
  return vlens.rpcOk({});
}

export function view(route: string, prefix: string, data: Data): preact.ComponentChild {
  const form = useCreateAccountForm();

  // Check for family code in URL parameters and pre-fill if present
  if (typeof window !== "undefined" && !form.familyCode) {
    const urlParams = new URLSearchParams(window.location.search);
    const codeParam = urlParams.get("code");
    if (codeParam) {
      form.familyCode = codeParam;
      vlens.scheduleRedraw();
    }
  }

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="create-account-container">
        <CreateAccountPage form={form} />
      </main>
      <Footer />
    </div>
  );
}

async function onCreateAccountClicked(form: CreateAccountForm, event: Event) {
  event.preventDefault();
  form.loading = true;
  form.error = "";

  let [resp, err] = await server.CreateAccount({
    name: form.name,
    email: form.email,
    password: form.password,
    confirmPassword: form.confirmPassword,
    familyCode: form.familyCode,
    initialPersonName: form.initialPersonName || form.name,
    initialPersonGender: form.initialPersonGender,
    initialPersonBirthdate: form.initialPersonBirthdate,
  });

  form.loading = false;

  if (resp && resp.success) {
    rpc.setAuthHeaders({ "x-auth-token": resp.token });
    auth.setAuth(resp.auth);
    core.setRoute("/dashboard");
  } else {
    form.error = resp?.error || err || "Failed to create account";
  }
  vlens.scheduleRedraw();

  // Scroll to error message if there's an error
  if (form.error) {
    setTimeout(() => {
      const errorElement = document.querySelector(".error-message");
      if (errorElement) {
        errorElement.scrollIntoView({ behavior: "smooth", block: "center" });
      }
    }, 100);
  }
}

interface CreateAccountPageProps {
  form: CreateAccountForm;
}

const CreateAccountPage = ({ form }: CreateAccountPageProps) => (
  <div className="create-account-page">
    <div className="auth-card">
      <div className="auth-header">
        <h1>Create Account</h1>
        <p>{form.familyCode ? "Join Your Family" : "Start a Family"}</p>
      </div>

      {form.error && <div className="error-message">{form.error}</div>}

      <form className="auth-form" onSubmit={vlens.cachePartial(onCreateAccountClicked, form)}>
        <div className="form-group">
          <label htmlFor="name">Full Name</label>
          <input
            type="text"
            id="name"
            placeholder="Enter your full name"
            {...vlens.attrsBindInput(vlens.ref(form, "name"))}
            required
            disabled={form.loading}
          />
        </div>

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
            placeholder="Create a password"
            {...vlens.attrsBindInput(vlens.ref(form, "password"))}
            required
            disabled={form.loading}
          />
        </div>

        <div className="form-group">
          <label htmlFor="confirmPassword">Confirm Password</label>
          <input
            type="password"
            id="confirmPassword"
            placeholder="Confirm your password"
            {...vlens.attrsBindInput(vlens.ref(form, "confirmPassword"))}
            required
            disabled={form.loading}
          />
        </div>

        <fieldset className="form-section">
          <legend>Your family profile</legend>
          <p className="section-hint">
            We’ll add you as the first adult in this family so photos, milestones, and family
            timelines can include you too.
          </p>

          <div className="form-group">
            <label htmlFor="initialPersonName">Profile Name</label>
            <input
              type="text"
              id="initialPersonName"
              placeholder="Name to show in your family"
              {...vlens.attrsBindInput(vlens.ref(form, "initialPersonName"))}
              disabled={form.loading}
            />
            <small className="form-hint">Leave blank to use your account name.</small>
          </div>

          <div className="form-group">
            <label htmlFor="initialPersonGender">Gender</label>
            <select
              id="initialPersonGender"
              {...vlens.attrsBindInput(vlens.ref(form, "initialPersonGender"))}
              disabled={form.loading}
            >
              <option value={2}>Prefer not to say</option>
              <option value={0}>Male</option>
              <option value={1}>Female</option>
            </select>
          </div>

          <div className="form-group">
            <label htmlFor="initialPersonBirthdate">Birthday</label>
            <input
              type="date"
              id="initialPersonBirthdate"
              {...vlens.attrsBindInput(vlens.ref(form, "initialPersonBirthdate"))}
              required
              disabled={form.loading}
            />
          </div>
        </fieldset>

        <div className="form-group">
          <label htmlFor="familyCode">Family Code (Optional)</label>
          <input
            type="text"
            id="familyCode"
            placeholder="Enter family invite code"
            {...vlens.attrsBindInput(vlens.ref(form, "familyCode"))}
            disabled={form.loading}
          />
          <small className="form-hint">
            {form.familyCode
              ? "You're joining an existing family with this code"
              : "Leave blank to create a new family group"}
          </small>
        </div>

        <button
          type="submit"
          className="btn btn-primary btn-large auth-submit"
          disabled={form.loading}
        >
          {form.loading ? "Creating..." : "Create Account"}
        </button>
      </form>

      <div className="auth-footer">
        <p>
          Already have an account?
          <a href="/login" className="auth-link">
            Sign in
          </a>
        </p>
      </div>
    </div>
  </div>
);
