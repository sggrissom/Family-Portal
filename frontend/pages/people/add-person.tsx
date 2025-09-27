import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as auth from "../../lib/authCache";
import * as core from "vlens/core";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView } from "../../lib/authHelpers";
import "./add-person-styles";

type Data = {};

type AddPersonForm = {
  name: string;
  personType: number; // 0 = Parent, 1 = Child
  gender: number; // 0 = Male, 1 = Female, 2 = Unknown
  birthdate: string;
  error: string;
  loading: boolean;
  success: boolean;
};

const useAddPersonForm = vlens.declareHook(
  (): AddPersonForm => ({
    name: "",
    personType: 0,
    gender: 0,
    birthdate: "",
    error: "",
    loading: false,
    success: false,
  })
);

export async function fetch(route: string, prefix: string) {
  return rpc.ok<Data>({});
}

export function view(route: string, prefix: string, data: Data): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth) {
    return null;
  }

  const form = useAddPersonForm();

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="add-person-container">
        <AddPersonPage form={form} />
      </main>
      <Footer />
    </div>
  );
}

async function onAddPersonClicked(form: AddPersonForm, event: Event) {
  event.preventDefault();
  form.loading = true;
  form.error = "";
  form.success = false;

  try {
    let [resp, err] = await server.AddPerson({
      name: form.name,
      personType: form.personType,
      gender: form.gender,
      birthdate: form.birthdate,
    });

    form.loading = false;

    if (resp) {
      form.success = true;
      // Clear form
      form.name = "";
      form.personType = 0;
      form.gender = 0;
      form.birthdate = "";

      core.setRoute("/dashboard");
    } else {
      form.error = err || "Failed to add family member";
    }
  } catch (error) {
    form.loading = false;
    form.error = "Network error. Please try again.";
  }

  vlens.scheduleRedraw();
}

interface AddPersonPageProps {
  form: AddPersonForm;
}

const AddPersonPage = ({ form }: AddPersonPageProps) => (
  <div className="add-person-page">
    <div className="auth-card">
      <div className="auth-header">
        <h1>Add Family Member</h1>
        <p>Add a new person to your family</p>
      </div>

      {form.success && (
        <div className="success-message">
          Family member added successfully! Redirecting to dashboard...
        </div>
      )}

      {form.error && <div className="error-message">{form.error}</div>}

      <form className="auth-form" onSubmit={vlens.cachePartial(onAddPersonClicked, form)}>
        <div className="form-group">
          <label htmlFor="name">Name</label>
          <input
            type="text"
            id="name"
            placeholder="Enter full name"
            {...vlens.attrsBindInput(vlens.ref(form, "name"))}
            required
            disabled={form.loading}
          />
        </div>

        <div className="form-group">
          <label htmlFor="personType">Relationship</label>
          <select
            id="personType"
            {...vlens.attrsBindInput(vlens.ref(form, "personType"))}
            disabled={form.loading}
          >
            <option value={0}>Parent</option>
            <option value={1}>Child</option>
          </select>
        </div>

        <div className="form-group">
          <label htmlFor="gender">Gender</label>
          <select
            id="gender"
            {...vlens.attrsBindInput(vlens.ref(form, "gender"))}
            disabled={form.loading}
          >
            <option value={0}>Male</option>
            <option value={1}>Female</option>
            <option value={2}>Unknown</option>
          </select>
        </div>

        <div className="form-group">
          <label htmlFor="birthdate">Birthdate</label>
          <input
            type="date"
            id="birthdate"
            {...vlens.attrsBindInput(vlens.ref(form, "birthdate"))}
            required
            disabled={form.loading}
          />
        </div>

        <div className="form-actions">
          <button
            type="submit"
            className="btn btn-primary btn-large auth-submit"
            disabled={form.loading}
          >
            {form.loading ? "Adding..." : "Add Family Member"}
          </button>

          <a href="/dashboard" className="btn btn-secondary btn-large">
            Cancel
          </a>
        </div>
      </form>
    </div>
  </div>
);
