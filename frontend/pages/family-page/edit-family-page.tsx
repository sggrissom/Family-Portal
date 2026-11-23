import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView, ensureAuthInFetch } from "../../lib/authHelpers";
import { ROLE_ADMIN } from "../../lib/authCache";
import { FamilySelect } from "../../components/FamilySelect";
import { PhotoPicker } from "../../components/PhotoPicker";
import {
  hasMorePhotos,
  loadMorePhotos,
  syncPhotoPages,
  usePhotoPages,
} from "../../hooks/usePhotoPages";
import { copy } from "../../lib/copy";
import { FamilyPageView } from "./public-page";
import { maxPhotos, previewPage, toggleMember, togglePhoto } from "../../lib/familyPage";
import "./family-page-styles";

const text = copy.familyPage;

type Data = { settings: server.FamilyPageSettingsResponse | null; error: string };

export async function fetch(route: string, prefix: string): Promise<rpc.Response<Data>> {
  if (!(await ensureAuthInFetch())) return rpc.ok<Data>({ settings: null, error: "" });
  const [resp, err] = await server.GetFamilyPageSettings({ familyId: 0 });
  return rpc.ok<Data>({ settings: resp, error: resp ? "" : err || text.loadFailed });
}

type EditorState = {
  seed: server.FamilyPageSettingsResponse | null;
  current: server.FamilyPageSettingsResponse | null;
  form: server.FamilyPageSettings;
  saving: boolean;
  copied: boolean;
  error: string;
};

const emptyForm = (): server.FamilyPageSettings => ({
  slug: "",
  indexable: false,
  title: "",
  intro: "",
  members: [],
  photoIds: [],
});

const useEditor = vlens.declareHook(
  (): EditorState => ({
    seed: null,
    current: null,
    form: emptyForm(),
    saving: false,
    copied: false,
    error: "",
  })
);

function load(state: EditorState, settings: server.FamilyPageSettingsResponse) {
  state.current = settings;
  state.form = {
    ...settings.settings,
    members: settings.settings.members.map(member => ({ ...member })),
    photoIds: [...settings.settings.photoIds],
  };
}

async function switchFamily(state: EditorState, familyId: number) {
  const [resp, err] = await server.GetFamilyPageSettings({ familyId });
  state.error = resp ? "" : err || text.loadFailed;
  if (resp) load(state, resp);
  vlens.scheduleRedraw();
}

const pageAddress = (slug: string) => `${location.origin}/f/${slug.trim().toLowerCase()}`;

async function publish(state: EditorState) {
  if (!state.current || state.saving) return;
  if (!confirm(text.confirmPublish(pageAddress(state.form.slug)))) return;
  state.saving = true;
  state.error = "";
  vlens.scheduleRedraw();
  const [resp, err] = await server.PublishFamilyPage({
    familyId: state.current.familyId,
    settings: state.form,
  });
  state.saving = false;
  if (resp) load(state, resp);
  else state.error = err || text.loadFailed;
  vlens.scheduleRedraw();
}

async function unpublish(state: EditorState) {
  if (!state.current || state.saving) return;
  if (!confirm(text.confirmUnpublish)) return;
  state.saving = true;
  state.error = "";
  vlens.scheduleRedraw();
  const [resp, err] = await server.UnpublishFamilyPage({ familyId: state.current.familyId });
  state.saving = false;
  if (resp) load(state, resp);
  else state.error = err || text.loadFailed;
  vlens.scheduleRedraw();
}

async function copyAddress(state: EditorState, url: string) {
  try {
    await navigator.clipboard.writeText(url);
    state.copied = true;
  } catch {
    state.copied = false;
  }
  vlens.scheduleRedraw();
}

const localDay = (value: string | null) => {
  const date = value ? new Date(value) : null;
  return date && !isNaN(date.getTime()) ? date.toLocaleDateString() : "";
};

export function view(route: string, prefix: string, data: Data): preact.ComponentChild {
  if (!requireAuthInView()) return;
  const state = useEditor();
  if (state.seed !== data.settings) {
    state.seed = data.settings;
    state.error = data.error;
    if (data.settings) load(state, data.settings);
  }

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="family-page-editor">
        <h1>{text.title}</h1>
        <p className="section-description">{text.intro}</p>
        <FamilySelect
          id="familyPageFamilyId"
          value={state.current?.familyId ?? 0}
          minRole={ROLE_ADMIN}
          onChange={familyId => switchFamily(state, familyId)}
          disabled={state.saving}
        />
        {state.error && (
          <div className="error-message" role="alert">
            {state.error}
          </div>
        )}
        {state.current && <Editor state={state} current={state.current} />}
      </main>
      <Footer />
    </div>
  );
}

const Editor = ({
  state,
  current,
}: {
  state: EditorState;
  current: server.FamilyPageSettingsResponse;
}) => {
  const form = state.form;
  const disabled = state.saving || !current.canEdit;
  return (
    <>
      <Status state={state} current={current} />
      {!current.canEdit && <p className="section-description">{text.adminsOnly}</p>}

      <div className="form-group">
        <label htmlFor="familyPageSlug">{text.address}</label>
        <div className="family-page-address">
          <span className="family-page-address-prefix">{location.host}/f/</span>
          <input
            id="familyPageSlug"
            type="text"
            autoCapitalize="none"
            autoComplete="off"
            spellcheck={false}
            maxLength={40}
            value={form.slug}
            disabled={disabled}
            onInput={e => {
              form.slug = e.currentTarget.value;
              vlens.scheduleRedraw();
            }}
          />
        </div>
        <small className="section-description">{text.addressHint}</small>
      </div>

      <div className="form-group">
        <label htmlFor="familyPageTitle">{text.pageTitle}</label>
        <input
          id="familyPageTitle"
          type="text"
          maxLength={80}
          value={form.title}
          disabled={disabled}
          onInput={e => {
            form.title = e.currentTarget.value;
            vlens.scheduleRedraw();
          }}
        />
      </div>

      <div className="form-group">
        <label htmlFor="familyPageIntro">{text.introLabel}</label>
        <textarea
          id="familyPageIntro"
          rows={4}
          maxLength={1000}
          placeholder={text.introPlaceholder}
          value={form.intro}
          disabled={disabled}
          onInput={e => {
            form.intro = e.currentTarget.value;
            vlens.scheduleRedraw();
          }}
        />
      </div>

      <h2>{text.people}</h2>
      <p className="section-description">{text.peopleHint}</p>
      {current.people.length === 0 && <p className="section-description">{text.noPeople}</p>}
      {current.people.map(person => (
        <MemberRow
          key={person.id}
          state={state}
          current={current}
          person={person}
          disabled={disabled}
        />
      ))}

      <h2>{text.photos(maxPhotos)}</h2>
      <p className="section-description">{text.photosHint}</p>
      <Photos state={state} familyId={current.familyId} disabled={disabled} />

      <div className="family-page-indexable">
        <label className="family-page-check">
          <input
            type="checkbox"
            checked={form.indexable}
            disabled={disabled}
            onChange={e => {
              form.indexable = e.currentTarget.checked;
              vlens.scheduleRedraw();
            }}
          />
          {text.indexable}
        </label>
        <small className="section-description">{text.indexableHint}</small>
      </div>

      {current.canEdit && (
        <div className="family-page-actions">
          <button
            className="btn btn-primary"
            disabled={state.saving || !form.slug.trim()}
            onClick={() => publish(state)}
          >
            {state.saving ? text.publishing : current.published ? text.republish : text.publish}
          </button>
          {current.published && (
            <button
              className="btn btn-secondary"
              disabled={state.saving}
              onClick={() => unpublish(state)}
            >
              {text.unpublish}
            </button>
          )}
        </div>
      )}
      <p className="section-description">{text.caching}</p>

      <section className="family-page-preview" aria-label="Preview">
        <h2>Preview</h2>
        <FamilyPageView page={previewPage(form, current.people)} />
      </section>
    </>
  );
};

const Status = ({
  state,
  current,
}: {
  state: EditorState;
  current: server.FamilyPageSettingsResponse;
}) => {
  if (!current.published) {
    return <p className="family-page-status">{text.notPublished}</p>;
  }
  return (
    <div className="family-page-status">
      <span>{text.published}</span>
      <a href={current.url} target="_blank" rel="noreferrer">
        {current.url}
      </a>
      <button className="btn btn-secondary" onClick={() => copyAddress(state, current.url)}>
        {state.copied ? text.copied : text.copyLink}
      </button>
      <small className="section-description">
        {text.publishedBy(localDay(current.publishedAt), current.publishedByName)}
      </small>
    </div>
  );
};

const MemberRow = ({
  state,
  current,
  person,
  disabled,
}: {
  state: EditorState;
  current: server.FamilyPageSettingsResponse;
  person: server.FamilyPagePerson;
  disabled: boolean;
}) => {
  const member = state.form.members.find(m => m.personId === person.id);
  return (
    <div className="family-page-member-row">
      <label className="family-page-check">
        <input
          type="checkbox"
          checked={!!member}
          disabled={disabled}
          onChange={() => {
            toggleMember(state.form, current.people, person);
            vlens.scheduleRedraw();
          }}
        />
        {person.name}
      </label>
      {member && (
        <div className="family-page-member-fields">
          <div className="form-group">
            <input
              type="text"
              aria-label={`${text.shownName} for ${person.name}`}
              placeholder={text.shownName}
              maxLength={60}
              value={member.name}
              disabled={disabled}
              onInput={e => {
                member.name = e.currentTarget.value;
                vlens.scheduleRedraw();
              }}
            />
            <input
              type="text"
              aria-label={`${text.line} for ${person.name}`}
              placeholder={text.line}
              maxLength={160}
              value={member.line}
              disabled={disabled}
              onInput={e => {
                member.line = e.currentTarget.value;
                vlens.scheduleRedraw();
              }}
            />
          </div>
          <div className="family-page-member-options">
            {person.ageYears !== null && person.ageYears !== undefined && (
              <label className="family-page-check">
                <input
                  type="checkbox"
                  checked={member.showAge}
                  disabled={disabled}
                  onChange={e => {
                    member.showAge = e.currentTarget.checked;
                    vlens.scheduleRedraw();
                  }}
                />
                {text.showAge}
              </label>
            )}
            {person.profilePhotoId > 0 ? (
              <label className="family-page-check">
                <input
                  type="checkbox"
                  checked={member.showPhoto}
                  disabled={disabled}
                  onChange={e => {
                    member.showPhoto = e.currentTarget.checked;
                    vlens.scheduleRedraw();
                  }}
                />
                {text.showPhoto}
              </label>
            ) : (
              <small className="section-description">{text.noProfilePhoto}</small>
            )}
          </div>
        </div>
      )}
    </div>
  );
};

const Photos = ({
  state,
  familyId,
  disabled,
}: {
  state: EditorState;
  familyId: number;
  disabled: boolean;
}) => {
  const pages = usePhotoPages("family-page-photos");
  syncPhotoPages(pages, {});
  const photos = pages.photos.filter(
    photo =>
      photo.image.familyId === familyId &&
      photo.image.mediaKind === server.MediaPhoto &&
      photo.image.status === 0
  );
  return (
    <PhotoPicker
      photos={photos}
      selectedIds={state.form.photoIds}
      onToggle={photoId => {
        togglePhoto(state.form, photoId);
        vlens.scheduleRedraw();
      }}
      disabled={disabled}
      emptyText={text.noPhotos}
      loading={!pages.started || pages.loading}
      onLoadMore={pages.started && hasMorePhotos(pages) ? () => loadMorePhotos(pages) : undefined}
    />
  );
};
