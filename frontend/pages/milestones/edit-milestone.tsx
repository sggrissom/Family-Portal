import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as core from "vlens/core";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView } from "../../lib/authHelpers";
import { getIdFromRoute } from "../../lib/routeHelpers";
import { copy } from "../../lib/copy";
import { When, whenProblem, whenRequest } from "../../lib/when";
import { ErrorPage } from "../../components/ErrorPage";
import { PagedPhotoPicker } from "../../components/PhotoPicker";
import { WhenControl } from "../../components/WhenControl";
import {
  CategoryChips,
  SuggestedPhotos,
  TagPicker,
  suggestPhotos,
  toggleId,
} from "./MilestoneFields";

type EditMilestoneForm = {
  description: string;
  category: string;
  when: When;
  photoIds: number[];
  tagIds: number[];
  error: string;
  saving: boolean;
  suggestedPhotoIds: number[];
  lookupKey: string;
  lookupTimer: number;
};

const useEditMilestoneForm = vlens.declareHook(
  (milestone: server.Milestone): EditMilestoneForm => ({
    description: milestone.description,
    category: milestone.category,
    when: {
      mode: "date",
      date: milestone.milestoneDate.split("T")[0],
      ageYears: "",
      ageMonths: "",
    },
    photoIds: [...(milestone.photoIds ?? [])],
    tagIds: [...(milestone.tagIds ?? [])],
    error: "",
    saving: false,
    suggestedPhotoIds: [],
    lookupKey: "",
    lookupTimer: 0,
  })
);

// Photos from around the milestone's date that aren't attached yet, refreshed
// a moment after the description or date stop changing.
function scheduleLookup(form: EditMilestoneForm, milestone: server.Milestone) {
  const text = form.description.trim();
  const when = whenProblem(form.when) ? null : whenRequest(form.when, new Date());
  const key = JSON.stringify([text, when]);
  if (key === form.lookupKey) return;
  form.lookupKey = key;
  window.clearTimeout(form.lookupTimer);
  form.lookupTimer = window.setTimeout(async () => {
    const photoIds =
      when && text.length >= 3
        ? await suggestPhotos(milestone.personId, text, when, milestone.photoIds ?? [])
        : [];
    if (form.lookupKey !== key) return;
    form.suggestedPhotoIds = photoIds;
    vlens.scheduleRedraw();
  }, 600);
}

type EditMilestoneData = {
  milestone: server.GetMilestoneResponse;
  tags: server.Tag[];
};

export async function fetch(
  route: string,
  prefix: string
): Promise<rpc.Response<EditMilestoneData>> {
  const milestoneId = getIdFromRoute(route);

  if (!milestoneId) {
    return [null, "Milestone ID is required"];
  }

  const [milestone, milestoneErr] = await server.GetMilestone({ id: milestoneId });
  if (milestoneErr) return [null, milestoneErr];

  const [tagsResp] = await server.ListTags({});

  return [{ milestone: milestone!, tags: tagsResp?.tags ?? [] }, ""];
}

export function view(
  route: string,
  prefix: string,
  data: EditMilestoneData
): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth) {
    return;
  }

  const milestone = data.milestone.milestone;

  if (!milestone) {
    return (
      <ErrorPage
        title="Milestone Not Found"
        message="The milestone you're trying to edit could not be found"
        containerClass="entry-container"
      />
    );
  }

  const form = useEditMilestoneForm(milestone);

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="entry-container">
        <EditMilestonePage form={form} milestone={milestone} tags={data.tags} />
      </main>
      <Footer />
    </div>
  );
}

function chooseCategory(form: EditMilestoneForm, category: string) {
  form.category = category;
  vlens.scheduleRedraw();
}

async function save(form: EditMilestoneForm, milestone: server.Milestone, event: Event) {
  event.preventDefault();
  if (form.saving) return;

  const problem = !form.description.trim() ? copy.milestone.needsText : whenProblem(form.when);
  if (problem) {
    form.error = problem;
    vlens.scheduleRedraw();
    return;
  }

  form.saving = true;
  form.error = "";
  vlens.scheduleRedraw();

  const when = whenRequest(form.when, new Date());
  const [resp, err] = await server.UpdateMilestone({
    id: milestone.id,
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
    form.error = err || "That milestone could not be saved. Please try again.";
    form.saving = false;
    vlens.scheduleRedraw();
    return;
  }
  core.setRoute(`/profile/${milestone.personId}`);
}

interface EditMilestonePageProps {
  form: EditMilestoneForm;
  milestone: server.Milestone;
  tags: server.Tag[];
}

const EditMilestonePage = ({ form, milestone, tags }: EditMilestonePageProps) => {
  const disabled = form.saving;
  if (BROWSER) scheduleLookup(form, milestone);
  return (
    <div className="entry-card">
      <h1 className="entry-title">Edit milestone</h1>

      <form className="entry-form" onSubmit={vlens.cachePartial(save, form, milestone)} noValidate>
        {form.error && (
          <div className="error-message" role="alert">
            {form.error}
          </div>
        )}

        <div className="entry-field">
          <label htmlFor="description">{copy.milestone.whatHappened}</label>
          <textarea
            id="description"
            rows={3}
            placeholder={copy.milestone.placeholder}
            disabled={disabled}
            {...vlens.attrsBindInput(vlens.ref(form, "description"))}
          />
        </div>

        <CategoryChips
          value={form.category}
          onChange={vlens.cachePartial(chooseCategory, form)}
          disabled={disabled}
        />

        <div className="entry-subject">
          <span className="entry-label">{copy.when.label}</span>
          <WhenControl when={form.when} disabled={disabled} />
        </div>

        <SuggestedPhotos
          photoIds={form.suggestedPhotoIds}
          selected={form.photoIds}
          disabled={disabled}
        />

        <div className="entry-field">
          <span className="entry-label">{copy.milestone.photos}</span>
          <PagedPhotoPicker
            pageKey="milestone-photos"
            filters={{ personIds: [milestone.personId] }}
            selectedIds={form.photoIds}
            onToggle={vlens.cachePartial(toggleId, form.photoIds)}
            disabled={disabled}
            emptyText="No photos found for this person"
          />
        </div>

        <TagPicker tags={tags} selected={form.tagIds} disabled={disabled} />

        <div className="entry-actions">
          <a href={`/profile/${milestone.personId}`} className="btn btn-secondary">
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
