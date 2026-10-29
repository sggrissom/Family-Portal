import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as server from "../../server";
import { navigate } from "../../lib/navigate";
import * as auth from "../../lib/authCache";
import { Header, Footer } from "../../layout";
import {
  OAuthButtons,
  AuthProviders,
  loadProviders,
  anyProvider,
} from "../../components/OAuthButtons";
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
  claimsFor: string;
  claims: server.InviteClaim[];
  claimPersonId: number;
  error: string;
  loading: boolean;
};

type Data = {
  providers: AuthProviders;
};

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
    claimsFor: "",
    claims: [],
    claimPersonId: 0,
    error: "",
    loading: false,
  })
);

export async function fetch(route: string, prefix: string) {
  return vlens.rpcOk<Data>({ providers: await loadProviders() });
}

export function view(route: string, prefix: string, data: Data): preact.ComponentChild {
  const form = useCreateAccountForm();

  if (typeof window !== "undefined" && !form.familyCode) {
    const urlParams = new URLSearchParams(window.location.search);
    const codeParam = urlParams.get("code");
    if (codeParam) {
      form.familyCode = codeParam;
      loadClaims(form);
      vlens.scheduleRedraw();
    }
  }

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="create-account-container">
        <CreateAccountPage form={form} providers={data.providers} />
      </main>
      <Footer />
    </div>
  );
}

async function loadClaims(form: CreateAccountForm) {
  const code = form.familyCode.trim();
  if (code === form.claimsFor) return;
  form.claimsFor = code;
  form.claims = [];
  form.claimPersonId = 0;
  if (!code) return;
  const [resp] = await server.GetInviteClaims({ code });
  if (resp && form.claimsFor === code) {
    form.claims = resp.claims ?? [];
    vlens.scheduleRedraw();
  }
}

async function onCreateAccountClicked(form: CreateAccountForm, event: Event) {
  event.preventDefault();
  form.loading = true;
  form.error = "";

  const nativeFetch = window.fetch.bind(window);
  try {
    const res = await nativeFetch("/api/signup", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        name: form.name,
        email: form.email,
        password: form.password,
        confirmPassword: form.confirmPassword,
        familyCode: form.familyCode,
        initialPersonName: form.initialPersonName || form.name,
        initialPersonGender: form.initialPersonGender,
        initialPersonBirthdate: form.claimPersonId ? "" : form.initialPersonBirthdate,
        claimPersonId: form.familyCode ? form.claimPersonId : 0,
      }),
    });

    const result = await res.json();
    form.loading = false;

    if (result.success) {
      rpc.setAuthHeaders({ "x-auth-token": result.token });
      auth.setAuth(result.auth);
      auth.markNewAccount();
      navigate("/dashboard");
    } else {
      form.error = result.error || "Failed to create account";
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

interface CreateAccountPageProps {
  form: CreateAccountForm;
  providers: AuthProviders;
}

const CreateAccountPage = ({ form, providers }: CreateAccountPageProps) => (
  <div className="create-account-page">
    <div className="auth-card">
      <div className="auth-header">
        <h1>Create Account</h1>
        <p>{form.familyCode ? "Join Your Family" : "Start a Family"}</p>
      </div>

      {form.error && (
        <div className="error-message" role="alert">
          {form.error}
        </div>
      )}

      {anyProvider(providers) && (
        <div className="auth-methods">
          <OAuthButtons
            providers={providers}
            disabled={form.loading}
            inviteCode={form.familyCode.trim()}
          />
          <div className="auth-divider">
            <span>or sign up with email</span>
          </div>
        </div>
      )}

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
            minLength={8}
            aria-describedby="passwordHint"
            disabled={form.loading}
          />
          <small id="passwordHint" className="form-hint">
            At least 8 characters.
          </small>
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

        {form.claims.length > 0 && (
          <fieldset className="form-section">
            <legend>Are you already in this family?</legend>
            <p className="section-hint">
              If someone added you before inviting you, pick yourself so your account joins that
              profile instead of making a second one.
            </p>
            {[...form.claims, { personId: 0, name: "No, add me as someone new" }].map(claim => (
              <label key={claim.personId} className="checkbox-option">
                <input
                  type="radio"
                  name="claimPersonId"
                  checked={form.claimPersonId === claim.personId}
                  onChange={() => {
                    form.claimPersonId = claim.personId;
                    vlens.scheduleRedraw();
                  }}
                  disabled={form.loading}
                />
                <span>{claim.name}</span>
              </label>
            ))}
          </fieldset>
        )}

        {form.claimPersonId === 0 && (
          <fieldset className="form-section">
            <legend>Your family profile</legend>
            <p className="section-hint">
              {form.familyCode
                ? "We’ll add you to the family as an adult so photos, moments, and family timelines can include you too."
                : "We’ll add you as the first adult in this family so photos, moments, and family timelines can include you too."}
            </p>

            <div className="form-group">
              <label htmlFor="initialPersonName">Profile Name (Optional)</label>
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
                <option value={2}>Not specified</option>
                <option value={0}>Male</option>
                <option value={1}>Female</option>
              </select>
            </div>

            <div className="form-group">
              <label htmlFor="initialPersonBirthdate">Birthday (Required)</label>
              <input
                type="date"
                id="initialPersonBirthdate"
                {...vlens.attrsBindInput(vlens.ref(form, "initialPersonBirthdate"))}
                required
                aria-describedby="birthdayHint"
                disabled={form.loading}
              />
              <small id="birthdayHint" className="form-hint">
                Ages on the timeline are worked out from it.
              </small>
            </div>
          </fieldset>
        )}

        <div className="form-group">
          <label htmlFor="familyCode">Family Code (Optional)</label>
          <input
            type="text"
            id="familyCode"
            placeholder="Enter family invite code"
            {...vlens.attrsBindInput(vlens.ref(form, "familyCode"))}
            onBlur={() => loadClaims(form)}
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

        <p className="auth-consent">
          By creating an account you agree to the <a href="/terms">terms of use</a> and the{" "}
          <a href="/privacy">privacy page</a>, which covers what is stored, what leaves the server,
          and how to delete it.
        </p>
      </form>

      <div className="auth-footer">
        <p>
          Already have an account?
          <a href="/login" className="auth-link">
            Sign in
          </a>
        </p>
        <p>
          <a href="/support" className="auth-link">
            Need help?
          </a>
        </p>
      </div>
    </div>
  </div>
);
