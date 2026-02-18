import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as core from "vlens/core";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView } from "../../lib/authHelpers";
import { getIdFromRoute } from "../../lib/routeHelpers";
import "./add-person-styles";

type EditPersonForm = {
  name: string;
  personType: number;
  gender: number;
  birthdate: string;
  error: string;
  loading: boolean;
};

const useEditPersonForm = vlens.declareHook(
  (): EditPersonForm => ({
    name: "",
    personType: 0,
    gender: 0,
    birthdate: "",
    error: "",
    loading: false,
  })
);

export async function fetch(route: string, prefix: string) {
  const personId = getIdFromRoute(route) || 0;
  return server.GetPerson({ id: personId });
}

export function view(
  route: string,
  prefix: string,
  data: server.GetPersonResponse
): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth) {
    return null;
  }

  if (!data.person) {
    return (
      <div>
        <Header isHome={false} />
        <main id="app" className="add-person-container">
          <div className="error-page">
            <h1>Error</h1>
            <p>Person not found</p>
            <a href="/dashboard" className="btn btn-primary">
              Back to Dashboard
            </a>
          </div>
        </main>
        <Footer />
      </div>
    );
  }

  const form = useEditPersonForm();

  // Initialize form with current values on first render
  if (!form.name && data.person.name) {
    form.name = data.person.name;
    form.personType = data.person.type;
    form.gender = data.person.gender;
    // Format birthday from ISO to YYYY-MM-DD
    if (data.person.birthday) {
      const d = new Date(data.person.birthday);
      form.birthdate = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
    }
  }

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="add-person-container">
        <EditPersonPage form={form} personId={data.person.id} />
      </main>
      <Footer />
    </div>
  );
}

async function onUpdatePersonClicked(form: EditPersonForm, personId: number, event: Event) {
  event.preventDefault();
  form.loading = true;
  form.error = "";

  try {
    let [resp, err] = await server.UpdatePerson({
      id: personId,
      name: form.name,
      personType: form.personType,
      gender: form.gender,
      birthdate: form.birthdate,
    });

    form.loading = false;

    if (resp) {
      core.setRoute(`/profile/${personId}`);
    } else {
      form.error = err || "Failed to update family member";
    }
  } catch (error) {
    form.loading = false;
    form.error = "Network error. Please try again.";
  }

  vlens.scheduleRedraw();
}

interface EditPersonPageProps {
  form: EditPersonForm;
  personId: number;
}

const EditPersonPage = ({ form, personId }: EditPersonPageProps) => (
  <div className="add-person-page">
    <div className="auth-card">
      <div className="auth-header">
        <h1>Edit Family Member</h1>
        <p>Update person details</p>
      </div>

      {form.error && <div className="error-message">{form.error}</div>}

      <form
        className="auth-form"
        onSubmit={vlens.cachePartial(onUpdatePersonClicked, form, personId)}
      >
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
            {form.loading ? "Saving..." : "Save Changes"}
          </button>

          <a href={`/profile/${personId}`} className="btn btn-secondary btn-large">
            Cancel
          </a>
        </div>
      </form>
    </div>
  </div>
);
