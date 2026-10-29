import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as auth_ from "../../lib/authCache";
import { navigate } from "../../lib/navigate";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView } from "../../lib/authHelpers";
import { FamilySelect } from "../../components/FamilySelect";
import { RELATION_OPTIONS } from "../../lib/routeHelpers";
import { CoAnchorState, CoAnchorSuggestion, syncCoAnchors } from "../../lib/relations";
import { CoAnchorPicker } from "../../components/CoAnchorPicker";
import "./add-person-styles";

type Data = {
  people: server.Person[];
  relations: server.Relation[];
  families: server.FamilyLabel[];
  selfPersonId: number;
};

type AddPersonForm = {
  name: string;
  relationIndex: number;
  anchorId: number;
  gender: number;
  birthdate: string;
  isPregnancy: boolean;
  familyId: number;
  coAnchors: CoAnchorState;
  error: string;
  loading: boolean;
  success: boolean;
};

const useAddPersonForm = vlens.declareHook(
  (): AddPersonForm => ({
    name: "",
    relationIndex: -1,
    anchorId: 0,
    gender: 2,
    birthdate: "",
    isPregnancy: false,
    familyId: 0,
    coAnchors: { key: "", ids: [] },
    error: "",
    loading: false,
    success: false,
  })
);

export async function fetch(route: string, prefix: string) {
  const [resp, err] = await server.ListPeople({});
  if (err) return [null, err] as rpc.Response<Data>;
  const currentAuth = auth_.getAuth();
  return rpc.ok<Data>({
    people: resp?.people ?? [],
    relations: resp?.relations ?? [],
    families: resp?.families ?? [],
    selfPersonId: currentAuth?.personId ?? 0,
  });
}

export function view(route: string, prefix: string, data: Data): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth) {
    return null;
  }

  const form = useAddPersonForm();
  if (form.anchorId === 0) {
    const familyId = targetFamilyId(form);
    const inFamily = data.people.filter(p => p.familyId === familyId);
    const preferred =
      inFamily.find(p => p.id === data.selfPersonId) ?? inFamily[0] ?? data.people[0];
    form.anchorId = preferred ? preferred.id : 0;
  }

  const relation = RELATION_OPTIONS[form.relationIndex];
  const suggestions = relation
    ? syncCoAnchors(form.coAnchors, data.relations, relation.value, 0, form.anchorId)
    : [];

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="add-person-container">
        <AddPersonPage
          form={form}
          people={data.people}
          families={data.families}
          suggestions={suggestions}
          relationLabel={relation ? relation.label : ""}
        />
      </main>
      <Footer />
    </div>
  );
}

function targetFamilyId(form: AddPersonForm): number {
  if (form.familyId) return form.familyId;
  const families = auth_.contributableFamilies();
  return (families.find(f => f.isPrimary) ?? families[0])?.id ?? 0;
}

function familyName(families: server.FamilyLabel[], familyId: number): string {
  return families.find(f => f.id === familyId)?.name || "Family";
}

function anchorGroups(people: server.Person[], familyId: number) {
  const ids = [familyId, ...new Set(people.map(p => p.familyId))].filter(
    (id, i, all) => all.indexOf(id) === i
  );
  return ids
    .map(id => ({ familyId: id, people: people.filter(p => p.familyId === id) }))
    .filter(group => group.people.length > 0);
}

async function onAddPersonClicked(form: AddPersonForm, event: Event) {
  event.preventDefault();
  form.loading = true;
  form.error = "";
  form.success = false;

  try {
    const relation = RELATION_OPTIONS[form.relationIndex];
    let [resp, err] = await server.AddPerson({
      name: form.name,
      gender: form.gender,
      birthdate: form.birthdate,
      isPregnancy: form.isPregnancy,
      familyId: targetFamilyId(form),
      stated: relation ? relation.value : server.StatedNone,
      anchorId: relation ? form.anchorId : 0,
      additionalAnchorIds: relation ? form.coAnchors.ids : [],
    });

    form.loading = false;

    if (resp) {
      form.success = true;
      form.name = "";
      form.relationIndex = -1;
      form.coAnchors = { key: "", ids: [] };
      form.gender = 2;
      form.birthdate = "";
      form.isPregnancy = false;
      form.familyId = 0;

      navigate("/dashboard");
    } else {
      form.error = err || "Failed to add family member";
    }
  } catch (error) {
    form.loading = false;
    form.error = "Network error. Please try again.";
  }

  vlens.scheduleRedraw();
}

function onRelationPicked(form: AddPersonForm, event: Event) {
  const index = Number((event.currentTarget as HTMLSelectElement).value);
  form.relationIndex = index;
  const relation = RELATION_OPTIONS[index];
  if (relation && relation.gender !== null) {
    form.gender = relation.gender;
  }
  vlens.scheduleRedraw();
}

interface AddPersonPageProps {
  form: AddPersonForm;
  people: server.Person[];
  families: server.FamilyLabel[];
  suggestions: CoAnchorSuggestion[];
  relationLabel: string;
}

const AddPersonPage = ({
  form,
  people,
  families,
  suggestions,
  relationLabel,
}: AddPersonPageProps) => (
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

      {form.error && (
        <div className="error-message" role="alert">
          {form.error}
        </div>
      )}

      <form className="auth-form" onSubmit={vlens.cachePartial(onAddPersonClicked, form)}>
        <FamilySelect
          id="familyId"
          value={form.familyId}
          onChange={familyId => {
            form.familyId = familyId;
            form.anchorId = 0;
          }}
          disabled={form.loading}
        />
        {families.length > 1 && auth_.contributableFamilies().length < 2 && (
          <p className="form-hint">Adds to {familyName(families, targetFamilyId(form))}.</p>
        )}

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

        {people.length > 0 && (
          <div className="form-group">
            <label htmlFor="relation">Relationship (optional)</label>
            <div className="relation-row">
              <select
                id="relation"
                value={String(form.relationIndex)}
                onInput={vlens.cachePartial(onRelationPicked, form)}
                disabled={form.loading}
              >
                <option value="-1">Not saying yet</option>
                {RELATION_OPTIONS.map((option, index) => (
                  <option key={`${option.value}-${option.label}`} value={String(index)}>
                    {option.label}
                  </option>
                ))}
              </select>
              <span className="relation-joiner">of</span>
              <select
                id="relationAnchor"
                value={String(form.anchorId)}
                onInput={event => {
                  form.anchorId = Number((event.currentTarget as HTMLSelectElement).value);
                  vlens.scheduleRedraw();
                }}
                disabled={form.loading || form.relationIndex < 0}
              >
                {anchorGroups(people, targetFamilyId(form)).map((group, _, groups) =>
                  groups.length > 1 ? (
                    <optgroup key={group.familyId} label={familyName(families, group.familyId)}>
                      {group.people.map(person => (
                        <option key={person.id} value={String(person.id)}>
                          {person.name}
                        </option>
                      ))}
                    </optgroup>
                  ) : (
                    group.people.map(person => (
                      <option key={person.id} value={String(person.id)}>
                        {person.name}
                      </option>
                    ))
                  )
                )}
              </select>
            </div>
            <CoAnchorPicker
              suggestions={suggestions}
              state={form.coAnchors}
              people={people}
              relationLabel={relationLabel}
              disabled={form.loading}
            />
            <p className="form-hint">
              Everyone else's relationship is worked out from this — a grandchild is a daughter or
              son of one of your children.
            </p>
          </div>
        )}

        <div className="form-group">
          <label htmlFor="gender">Gender</label>
          <select
            id="gender"
            {...vlens.attrsBindInput(vlens.ref(form, "gender"))}
            disabled={form.loading}
          >
            <option value={2}>Not specified</option>
            <option value={0}>Male</option>
            <option value={1}>Female</option>
          </select>
        </div>

        <label className="checkbox-option">
          <input
            type="checkbox"
            checked={form.isPregnancy}
            onInput={event => {
              form.isPregnancy = (event.currentTarget as HTMLInputElement).checked;
            }}
            disabled={form.loading}
          />
          <span>Baby isn’t born yet</span>
        </label>

        <div className="form-group">
          <label htmlFor="birthdate">{form.isPregnancy ? "Due Date" : "Birthday"}</label>
          <input
            type="date"
            id="birthdate"
            {...vlens.attrsBindInput(vlens.ref(form, "birthdate"))}
            required
            disabled={form.loading}
          />
          <small>
            {form.isPregnancy
              ? "This date stays a due date until you mark the baby as born."
              : "Use the pregnancy option for an expected baby."}
          </small>
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
