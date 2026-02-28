import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView, ensureAuthInFetch } from "../../lib/authHelpers";
import "./manage-tags-styles";

export async function fetch(route: string, prefix: string): Promise<rpc.Response<server.ListTagsResponse>> {
  if (!(await ensureAuthInFetch())) {
    return rpc.ok<server.ListTagsResponse>({ tags: [] });
  }
  return server.ListTags({});
}

type ManageTagsState = {
  initialized: boolean;
  tags: server.Tag[];
  newName: string;
  newColor: string;
  editingId: number | null;
  editName: string;
  editColor: string;
  error: string;
  saving: boolean;
};

const useManageTagsState = vlens.declareHook(
  (): ManageTagsState => ({
    initialized: false,
    tags: [],
    newName: "",
    newColor: "#6366f1",
    editingId: null,
    editName: "",
    editColor: "",
    error: "",
    saving: false,
  })
);

export function view(
  route: string,
  prefix: string,
  data: server.ListTagsResponse
): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth) return;

  const state = useManageTagsState();
  if (!state.initialized) {
    state.initialized = true;
    state.tags = data.tags ? [...data.tags] : [];
  }

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="manage-tags-container">
        <h1>Manage Tags</h1>
        <p>Create and manage tags to organize your family's milestones and photos.</p>

        {state.error && <div className="manage-tags-error">{state.error}</div>}

        {/* Create new tag */}
        <div className="tag-create-row">
          <input
            type="text"
            placeholder="Tag name"
            value={state.newName}
            onInput={e => {
              state.newName = e.currentTarget.value;
              vlens.scheduleRedraw();
            }}
            disabled={state.saving}
          />
          <input
            type="color"
            className="tag-color-input"
            value={state.newColor}
            onInput={e => {
              state.newColor = e.currentTarget.value;
              vlens.scheduleRedraw();
            }}
            disabled={state.saving}
          />
          <button
            className="btn btn-primary"
            onClick={vlens.cachePartial(onCreateTag, state)}
            disabled={state.saving || !state.newName.trim()}
          >
            Add Tag
          </button>
        </div>

        {/* Tag list */}
        {state.tags.length === 0 ? (
          <div className="manage-tags-empty">No tags yet. Create one above.</div>
        ) : (
          <div>
            {state.tags.map(tag => {
              const isEditing = state.editingId === tag.id;
              return (
                <div key={tag.id} className="tag-list-item">
                  {isEditing ? (
                    <>
                      <div className="tag-edit-row">
                        <div
                          className="tag-color-swatch"
                          style={{ background: state.editColor }}
                        />
                        <input
                          type="text"
                          value={state.editName}
                          onInput={e => {
                            state.editName = e.currentTarget.value;
                            vlens.scheduleRedraw();
                          }}
                          disabled={state.saving}
                        />
                        <input
                          type="color"
                          className="tag-color-input"
                          value={state.editColor}
                          onInput={e => {
                            state.editColor = e.currentTarget.value;
                            vlens.scheduleRedraw();
                          }}
                          disabled={state.saving}
                        />
                      </div>
                      <button
                        className="btn btn-primary"
                        onClick={vlens.cachePartial(onSaveTag, state, tag.id)}
                        disabled={state.saving || !state.editName.trim()}
                      >
                        Save
                      </button>
                      <button
                        className="btn btn-secondary"
                        onClick={vlens.cachePartial(onCancelEdit, state)}
                        disabled={state.saving}
                      >
                        Cancel
                      </button>
                    </>
                  ) : (
                    <>
                      <div className="tag-color-swatch" style={{ background: tag.color }} />
                      <span className="tag-name">{tag.name}</span>
                      <button
                        className="tag-action-btn"
                        title="Edit"
                        onClick={vlens.cachePartial(onStartEdit, state, tag)}
                        disabled={state.saving}
                      >
                        ✏️
                      </button>
                      <button
                        className="tag-action-btn"
                        title="Delete"
                        onClick={vlens.cachePartial(onDeleteTag, state, tag.id)}
                        disabled={state.saving}
                      >
                        🗑️
                      </button>
                    </>
                  )}
                </div>
              );
            })}
          </div>
        )}
      </main>
      <Footer />
    </div>
  );
}

async function onCreateTag(state: ManageTagsState) {
  const name = state.newName.trim();
  if (!name) return;
  state.saving = true;
  state.error = "";
  vlens.scheduleRedraw();

  const [resp, err] = await server.CreateTag({ name, color: state.newColor });
  if (err || !resp) {
    state.error = err || "Failed to create tag";
  } else {
    state.tags.push(resp.tag);
    state.newName = "";
  }
  state.saving = false;
  vlens.scheduleRedraw();
}

function onStartEdit(state: ManageTagsState, tag: server.Tag) {
  state.editingId = tag.id;
  state.editName = tag.name;
  state.editColor = tag.color;
  vlens.scheduleRedraw();
}

function onCancelEdit(state: ManageTagsState) {
  state.editingId = null;
  state.editName = "";
  state.editColor = "";
  vlens.scheduleRedraw();
}

async function onSaveTag(state: ManageTagsState, tagId: number) {
  const name = state.editName.trim();
  if (!name) return;
  state.saving = true;
  state.error = "";
  vlens.scheduleRedraw();

  const [resp, err] = await server.UpdateTag({ id: tagId, name, color: state.editColor });
  if (err || !resp) {
    state.error = err || "Failed to update tag";
  } else {
    const idx = state.tags.findIndex(t => t.id === tagId);
    if (idx >= 0) state.tags[idx] = resp.tag;
    state.editingId = null;
  }
  state.saving = false;
  vlens.scheduleRedraw();
}

async function onDeleteTag(state: ManageTagsState, tagId: number) {
  const tag = state.tags.find(t => t.id === tagId);
  const name = tag?.name || "this tag";
  if (!confirm(`Delete "${name}"? It will be removed from all milestones and photos.`)) return;
  state.saving = true;
  state.error = "";
  vlens.scheduleRedraw();

  const [, err] = await server.DeleteTag({ id: tagId });
  if (err) {
    state.error = err || "Failed to delete tag";
  } else {
    const idx = state.tags.findIndex(t => t.id === tagId);
    if (idx >= 0) state.tags.splice(idx, 1);
    if (state.editingId === tagId) state.editingId = null;
  }
  state.saving = false;
  vlens.scheduleRedraw();
}
