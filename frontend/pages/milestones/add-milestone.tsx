import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as auth from "../../lib/authCache";
import * as core from "vlens/core";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView } from "../../lib/authHelpers";
import { getIdFromRoute } from "../../lib/routeHelpers";
import { chipOrder } from "../../lib/familyGroups";
import { copy } from "../../lib/copy";
import { When, newWhen, whenProblem, whenRequest } from "../../lib/when";
import { readLastPerson, returnPath, takeReturnPath, writeLastPerson } from "../../lib/addFlow";
import { NoFamilyMembersPage } from "../../components/NoFamilyMembersPage";
import { PagedPhotoPicker, PhotoPicker } from "../../components/PhotoPicker";
import { PersonChips, scrollSelectedChipIntoView } from "../../components/PersonChips";
import { WhenControl } from "../../components/WhenControl";
import { parseAgeFromText } from "../../lib/ageInText";
import { CategoryChips, TagPicker, toggleId } from "./MilestoneFields";

type AddMilestoneForm = {
  personId: number | null;
  description: string;
  category: string;
  when: When;
  photoIds: number[];
  tagIds: number[];
  error: string;
  saving: boolean;
  categoryTouched: boolean;
  categorySuggested: boolean;
  suggestedPhotoIds: number[];
  lookupKey: string;
  lookupTimer: number;
};

const useAddMilestoneForm = vlens.declareHook((personId: number | null): AddMilestoneForm => {
  scrollSelectedChipIntoView();
  return {
    personId,
    description: "",
    category: "first",
    when: newWhen(),
    photoIds: [],
    tagIds: [],
    error: "",
    saving: false,
    categoryTouched: false,
    categorySuggested: false,
    suggestedPhotoIds: [],
    lookupKey: "",
    lookupTimer: 0,
  };
});

// Suggestions refresh a moment after the description, person, or date stop
// changing: a category (unless one was picked by hand) and nearby photos.
function scheduleLookups(form: AddMilestoneForm) {
  const text = form.description.trim();
  const when = whenProblem(form.when) ? null : whenRequest(form.when, new Date());
  const key = JSON.stringify([form.personId, text, when]);
  if (key === form.lookupKey) return;
  form.lookupKey = key;
  window.clearTimeout(form.lookupTimer);
  form.lookupTimer = window.setTimeout(async () => {
    if (text.length >= 3 && !form.categoryTouched) {
      const [resp] = await server.SuggestMilestoneCategory({
        description: text,
        personId: form.personId ?? 0,
      });
      if (resp?.category && !form.categoryTouched && form.lookupKey === key) {
        form.category = resp.category;
        form.categorySuggested = true;
      }
    }
    if (form.personId !== null && when && text.length >= 3) {
      const [resp] = await server.SuggestMilestonePhotos({
        personId: form.personId,
        description: text,
        inputType: when.inputType,
        milestoneDate: when.date,
        ageYears: when.ageYears,
        ageMonths: when.ageMonths,
        excludeIds: [],
      });
      if (form.lookupKey === key) form.suggestedPhotoIds = resp?.photoIds ?? [];
    } else {
      form.suggestedPhotoIds = [];
    }
    vlens.scheduleRedraw();
  }, 600);
}

function applyAgeFromText(form: AddMilestoneForm, years: number, months: number) {
  form.when = { mode: "age", date: "", ageYears: String(years), ageMonths: String(months) };
  vlens.scheduleRedraw();
}

type AddMilestoneData = {
  people: server.Person[];
  tags: server.Tag[];
};

export async function fetch(
  route: string,
  prefix: string
): Promise<rpc.Response<AddMilestoneData>> {
  const [people, peopleErr] = await server.ListPeople({});
  if (!people) return [null, peopleErr];

  const [tags, tagsErr] = await server.ListTags({});
  if (!tags) return [null, tagsErr];

  const ordered = chipOrder(
    people.people || [],
    people.relations || [],
    auth.getAuth()?.familyId ?? 0
  );
  return [{ people: ordered, tags: tags.tags }, ""];
}

export function view(route: string, prefix: string, data: AddMilestoneData): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth) {
    return;
  }

  if (data.people.length === 0) {
    return (
      <NoFamilyMembersPage
        message="Please add family members before adding milestones"
        containerClass="entry-container"
      />
    );
  }

  const fromRoute = getIdFromRoute(route);
  const initial = [fromRoute, readLastPerson()].find(
    id => id !== null && data.people.some(p => p.id === id)
  );
  const form = useAddMilestoneForm(initial ?? null);

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="entry-container">
        <AddMilestonePage form={form} people={data.people} tags={data.tags} />
      </main>
      <Footer />
    </div>
  );
}

async function onSubmitMilestone(form: AddMilestoneForm, event: Event) {
  event.preventDefault();
  if (form.saving) return;

  const personId = form.personId;
  const problem =
    personId === null
      ? copy.milestone.pickPerson
      : !form.description.trim()
        ? copy.milestone.needsText
        : whenProblem(form.when);
  if (problem || personId === null) {
    form.error = problem;
    vlens.scheduleRedraw();
    return;
  }

  form.saving = true;
  form.error = "";
  vlens.scheduleRedraw();

  const when = whenRequest(form.when, new Date());
  const [resp, err] = await server.AddMilestone({
    personId,
    description: form.description.trim(),
    category: form.category,
    inputType: when.inputType,
    milestoneDate: when.date,
    ageYears: when.ageYears,
    ageMonths: when.ageMonths,
    photoIds: form.photoIds,
    tagIds: form.tagIds,
  });

  if (!resp) {
    form.saving = false;
    form.error = err || "That milestone could not be saved. Please try again.";
    vlens.scheduleRedraw();
    return;
  }

  writeLastPerson(personId);
  takeReturnPath("");
  core.setRoute(`/profile/${personId}`);
}

function choosePerson(form: AddMilestoneForm, personId: number) {
  form.personId = form.personId === personId ? null : personId;
  form.photoIds = [];
  form.error = "";
  vlens.scheduleRedraw();
}

function chooseCategory(form: AddMilestoneForm, category: string) {
  form.category = category;
  form.categoryTouched = true;
  form.categorySuggested = false;
  vlens.scheduleRedraw();
}

function cancel(event: Event) {
  event.preventDefault();
  core.setRoute(takeReturnPath("/dashboard"));
}

const focused = new WeakSet<HTMLElement>();

function focusOnMount(el: HTMLElement | null) {
  if (!el || focused.has(el)) return;
  focused.add(el);
  requestAnimationFrame(() => el.focus());
}

interface AddMilestonePageProps {
  form: AddMilestoneForm;
  people: server.Person[];
  tags: server.Tag[];
}

const AddMilestonePage = ({ form, people, tags }: AddMilestonePageProps) => {
  const disabled = form.saving;
  if (BROWSER) scheduleLookups(form);
  const statedAge = parseAgeFromText(form.description);
  const ageAlreadyUsed =
    statedAge &&
    form.when.mode === "age" &&
    form.when.ageYears === String(statedAge.years) &&
    (form.when.ageMonths || "0") === String(statedAge.months);

  return (
    <div className="entry-card">
      <h1 className="entry-title">{copy.milestone.title}</h1>

      <PersonChips
        people={people}
        selected={form.personId !== null ? [form.personId] : []}
        onToggle={vlens.cachePartial(choosePerson, form)}
        label={copy.milestone.who}
        disabled={disabled}
      />

      <form
        className="entry-form"
        onSubmit={vlens.cachePartial(onSubmitMilestone, form)}
        noValidate
      >
        {form.error && (
          <div className="error-message" role="alert">
            {form.error}
          </div>
        )}

        <div className="entry-field">
          <label htmlFor="description">{copy.milestone.whatHappened}</label>
          <textarea
            id="description"
            ref={focusOnMount}
            rows={3}
            placeholder={copy.milestone.placeholder}
            disabled={disabled}
            {...vlens.attrsBindInput(vlens.ref(form, "description"))}
          />
        </div>

        <CategoryChips
          value={form.category}
          suggested={form.categorySuggested}
          onChange={vlens.cachePartial(chooseCategory, form)}
          disabled={disabled}
        />

        <div className="entry-subject">
          <span className="entry-label">{copy.when.label}</span>
          <WhenControl when={form.when} disabled={disabled} />
          {statedAge && !ageAlreadyUsed && (
            <button
              type="button"
              className="entry-hint-button"
              disabled={disabled}
              onClick={vlens.cachePartial(
                applyAgeFromText,
                form,
                statedAge.years,
                statedAge.months
              )}
            >
              It says “{statedAge.text}”. Record it at that age?
            </button>
          )}
        </div>

        {form.suggestedPhotoIds.length > 0 && (
          <div className="entry-field">
            <span className="entry-label">Photos from around then. Attach any?</span>
            <div className="suggested-photos">
              {form.suggestedPhotoIds.map(id => {
                const selected = form.photoIds.includes(id);
                return (
                  <button
                    key={id}
                    type="button"
                    className={`suggested-photo${selected ? " selected" : ""}`}
                    aria-pressed={selected}
                    aria-label={selected ? "Don't attach this photo" : "Attach this photo"}
                    disabled={disabled}
                    onClick={vlens.cachePartial(toggleId, form.photoIds, id)}
                  >
                    <img src={`/api/photo/${id}/thumb`} alt="" loading="lazy" />
                  </button>
                );
              })}
            </div>
          </div>
        )}

        <details className="entry-more">
          <summary>{copy.milestone.more}</summary>
          <div className="entry-form">
            <div className="entry-field">
              <span className="entry-label">{copy.milestone.photos}</span>
              {form.personId !== null ? (
                <PagedPhotoPicker
                  pageKey="milestone-photos"
                  filters={{ personIds: [form.personId] }}
                  selectedIds={form.photoIds}
                  onToggle={vlens.cachePartial(toggleId, form.photoIds)}
                  disabled={disabled}
                  emptyText="No photos found for this person"
                />
              ) : (
                <PhotoPicker
                  photos={[]}
                  selectedIds={form.photoIds}
                  onToggle={vlens.cachePartial(toggleId, form.photoIds)}
                  disabled={disabled}
                  emptyText="Pick a person to see their photos"
                />
              )}
            </div>

            <TagPicker tags={tags} selected={form.tagIds} disabled={disabled} />
          </div>
        </details>

        <div className="entry-actions">
          <a href={returnPath("/dashboard")} className="btn btn-secondary" onClick={cancel}>
            {copy.milestone.cancel}
          </a>
          <button type="submit" className="btn btn-primary" disabled={disabled}>
            {form.saving ? copy.milestone.saving : copy.milestone.save}
          </button>
        </div>
      </form>
    </div>
  );
};
