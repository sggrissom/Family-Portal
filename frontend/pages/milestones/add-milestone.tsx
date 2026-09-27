import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as auth from "../../lib/authCache";
import * as core from "vlens/core";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView } from "../../lib/authHelpers";
import { MILESTONE_CATEGORIES } from "../../lib/milestoneHelpers";
import { getIdFromRoute } from "../../lib/routeHelpers";
import { chipOrder } from "../../lib/familyGroups";
import { copy } from "../../lib/copy";
import { When, newWhen, whenProblem, whenRequest } from "../../lib/when";
import { readLastPerson, returnPath, takeReturnPath, writeLastPerson } from "../../lib/addFlow";
import { NoFamilyMembersPage } from "../../components/NoFamilyMembersPage";
import { PagedPhotoPicker, PhotoPicker } from "../../components/PhotoPicker";
import { PersonChips, scrollSelectedChipIntoView } from "../../components/PersonChips";
import { WhenControl } from "../../components/WhenControl";
import "./add-milestone-styles";
import "../../components/entry-form-styles";

type AddMilestoneForm = {
  personId: number | null;
  description: string;
  category: string;
  when: When;
  photoIds: number[];
  tagIds: number[];
  error: string;
  saving: boolean;
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
  };
});

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
  });

  if (!resp) {
    form.saving = false;
    form.error = err || "That milestone could not be saved. Please try again.";
    vlens.scheduleRedraw();
    return;
  }

  if (form.tagIds.length > 0) {
    await server.UpdateMilestoneTags({ milestoneId: resp.milestone.id, tagIds: form.tagIds });
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
  vlens.scheduleRedraw();
}

function onToggleTag(form: AddMilestoneForm, tagId: number) {
  const idx = form.tagIds.indexOf(tagId);
  if (idx >= 0) form.tagIds.splice(idx, 1);
  else form.tagIds.push(tagId);
  vlens.scheduleRedraw();
}

function onTogglePhoto(form: AddMilestoneForm, photoId: number) {
  const idx = form.photoIds.indexOf(photoId);
  if (idx >= 0) {
    form.photoIds.splice(idx, 1);
  } else {
    form.photoIds.push(photoId);
  }
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

const CATEGORY_ORDER = ["first", "development", "achievement", "behavior", "health", "other"];

interface AddMilestonePageProps {
  form: AddMilestoneForm;
  people: server.Person[];
  tags: server.Tag[];
}

const AddMilestonePage = ({ form, people, tags }: AddMilestonePageProps) => {
  const disabled = form.saving;
  const categories = CATEGORY_ORDER.map(
    value => MILESTONE_CATEGORIES.find(c => c.value === value)!
  );

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

        <div className="entry-field">
          <span className="entry-label" id="categoryLabel">
            {copy.milestone.category}
          </span>
          <div className="category-chips" role="group" aria-labelledby="categoryLabel">
            {categories.map(category => (
              <button
                key={category.value}
                type="button"
                className={
                  form.category === category.value ? "category-chip selected" : "category-chip"
                }
                aria-pressed={form.category === category.value ? "true" : "false"}
                disabled={disabled}
                onClick={vlens.cachePartial(chooseCategory, form, category.value)}
              >
                <span aria-hidden="true">{category.icon}</span>
                {category.label}
              </button>
            ))}
          </div>
        </div>

        <div className="entry-subject">
          <span className="entry-label">{copy.when.label}</span>
          <WhenControl when={form.when} disabled={disabled} />
        </div>

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
                  onToggle={photoId => onTogglePhoto(form, photoId)}
                  disabled={disabled}
                  emptyText="No photos found for this person"
                />
              ) : (
                <PhotoPicker
                  photos={[]}
                  selectedIds={form.photoIds}
                  onToggle={photoId => onTogglePhoto(form, photoId)}
                  disabled={disabled}
                  emptyText="Pick a person to see their photos"
                />
              )}
            </div>

            {tags.length > 0 && (
              <div className="entry-field">
                <span className="entry-label" id="tagPickerLabel">
                  {copy.milestone.tags}
                </span>
                <div className="tag-picker" role="group" aria-labelledby="tagPickerLabel">
                  {tags.map(tag => {
                    const selected = form.tagIds.includes(tag.id);
                    return (
                      <button
                        key={tag.id}
                        type="button"
                        className={`tag-pill${selected ? " selected" : ""}`}
                        style={{ borderColor: tag.color }}
                        aria-pressed={selected}
                        onClick={vlens.cachePartial(onToggleTag, form, tag.id)}
                      >
                        <span className="tag-color-dot" style={{ background: tag.color }} />
                        {tag.name}
                      </button>
                    );
                  })}
                </div>
              </div>
            )}
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
