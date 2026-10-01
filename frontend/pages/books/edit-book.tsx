import * as preact from "preact";
import * as vlens from "vlens";
import * as core from "vlens/core";
import * as rpc from "vlens/rpc";
import * as server from "../../server";
import { Header } from "../../layout";
import { requireAuthInView } from "../../lib/authHelpers";
import { getIdFromRoute } from "../../lib/routeHelpers";
import { ThumbnailImage } from "../../components/ResponsiveImage";
import { SegmentedControl } from "../../components/SegmentedControl";
import { getCategoryIcon } from "../../lib/milestoneHelpers";
import { photoAge } from "../../lib/sameAge";
import {
  CATEGORIES,
  DENSITIES,
  Density,
  Resolved,
  Selection,
  assembleBook,
  bookDates,
  candidatesIn,
  dayOf,
  insertByDay,
  isDensity,
  itemDay,
  itemKey,
  originalCaption,
  additionsSince,
  isMulti,
  joinNames,
  photoDetail,
  resolveSource,
  shortDay,
  suggestItems,
} from "../../lib/book";
import { EditorNotes } from "./book";
import "./book-styles";
import "./edit-book-styles";

export async function fetch(route: string, prefix: string) {
  return server.GetBook({ id: getIdFromRoute(route) || 0 });
}

type EditorState = {
  revision: number;
  title: string;
  introduction: string;
  letter: string;
  signature: string;
  coverPhotoId: number;
  density: Density;
  showGrowth: boolean;
  categories: string[];
  match: string;
  items: server.BookItem[];
  excluded: server.BookItem[];
  dirty: boolean;
  saving: boolean;
  saved: boolean;
  error: string;
  conflict: boolean;
  pickingCover: boolean;
  openMonths: Record<string, boolean>;
  confirmDelete: boolean;
};

const useEditor = vlens.declareHook(
  (resp: server.GetBookResponse): EditorState => ({
    revision: resp.book.revision,
    title: resp.book.title,
    introduction: resp.book.introduction,
    letter: resp.book.letter,
    signature: resp.book.signature,
    coverPhotoId: resp.book.coverPhotoId,
    density: isDensity(resp.book.density) ? resp.book.density : "balanced",
    showGrowth: resp.book.showGrowth,
    categories: resp.book.categories?.length
      ? [...resp.book.categories]
      : CATEGORIES.map(c => c.value),
    match: resp.book.match || "any",
    items: [...(resp.book.items ?? [])],
    excluded: [...(resp.book.excluded ?? [])],
    dirty: false,
    saving: false,
    saved: false,
    error: "",
    conflict: false,
    pickingCover: false,
    openMonths: {},
    confirmDelete: false,
  })
);

function selectionOf(resp: server.GetBookResponse, state: EditorState): Selection {
  return {
    preset: resp.book.preset,
    categories: state.categories,
    match: state.match,
    title: state.title,
    startDate: resp.book.startDate,
    endDate: resp.book.endDate,
    coverPhotoId: state.coverPhotoId,
    introduction: state.introduction,
    letter: state.letter,
    signature: state.signature,
    showGrowth: state.showGrowth,
    items: state.items,
  };
}

function changed(state: EditorState) {
  state.dirty = true;
  state.saved = false;
  vlens.scheduleRedraw();
}

function setText(
  state: EditorState,
  field: "title" | "introduction" | "letter" | "signature",
  event: Event
) {
  state[field] = (event.target as HTMLInputElement).value;
  changed(state);
}

function moveItem(state: EditorState, chapterItems: number[], position: number, delta: number) {
  const a = chapterItems[position];
  const b = chapterItems[position + delta];
  if (b === undefined) return;
  const items = [...state.items];
  [items[a], items[b]] = [items[b], items[a]];
  state.items = items;
  changed(state);
}

function updateItem(state: EditorState, index: number, patch: Partial<server.BookItem>) {
  state.items = state.items.map((item, i) => (i === index ? { ...item, ...patch } : item));
  changed(state);
}

function removeItem(state: EditorState, index: number) {
  const item = state.items[index];
  state.items = state.items.filter((_, i) => i !== index);
  state.excluded = [...state.excluded, { ...item, caption: "", pinned: false, photoId: 0 }];
  changed(state);
}

function addItem(state: EditorState, r: Resolved, item: server.BookItem) {
  const items = [...state.items];
  insertByDay(r, items, item);
  state.items = items;
  state.excluded = state.excluded.filter(e => itemKey(e) !== itemKey(item));
  changed(state);
}

function resuggest(state: EditorState, resp: server.GetBookResponse) {
  const r = resolveSource(resp.sources, selectionOf(resp, state));
  const next = suggestItems(r, {
    density: state.density,
    current: state.items,
    excluded: state.excluded,
    coverPhotoId: state.coverPhotoId,
  });
  state.items = next.items;
  changed(state);
}

async function save(state: EditorState, resp: server.GetBookResponse, r: Resolved) {
  state.saving = true;
  state.error = "";
  vlens.scheduleRedraw();
  const items = state.items.filter(item => itemDay(r, item) !== null);
  const [result, err] = await server.UpdateBook({
    id: resp.book.id,
    revision: state.revision,
    content: {
      title: state.title,
      coverPhotoId: r.photos.has(state.coverPhotoId) ? state.coverPhotoId : 0,
      density: state.density,
      categories: state.categories.length === CATEGORIES.length ? [] : state.categories,
      match: state.match,
      introduction: state.introduction,
      letter: state.letter,
      signature: state.signature,
      showGrowth: state.showGrowth,
      items,
      excluded: state.excluded,
      reviewedAt: resp.now,
    },
  });
  state.saving = false;
  if (!result) {
    state.error = err || "Could not save the book";
    state.conflict = err === server.ErrBookChanged;
  } else {
    state.revision = result.book.revision;
    state.items = result.book.items ?? [];
    state.excluded = result.book.excluded ?? [];
    state.title = result.book.title;
    state.dirty = false;
    state.saved = true;
  }
  vlens.scheduleRedraw();
}

async function deleteBook(state: EditorState, bookId: number) {
  const [, err] = await server.DeleteBook({ id: bookId });
  if (err) {
    state.error = err;
    vlens.scheduleRedraw();
    return;
  }
  core.setRoute("/books");
}

export function view(
  route: string,
  prefix: string,
  data: server.GetBookResponse
): preact.ComponentChild {
  if (!requireAuthInView()) return;
  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="book-editor">
        {data.canEdit ? <Editor resp={data} /> : <p>You can read this book, but not change it.</p>}
      </main>
    </div>
  );
}

const Editor = ({ resp }: { resp: server.GetBookResponse }) => {
  const state = useEditor(resp);
  const r = resolveSource(resp.sources, selectionOf(resp, state));
  const book = assembleBook(resp.sources, selectionOf(resp, state));
  const additions = state.saved
    ? []
    : additionsSince(r, resp.book.reviewedAt, state.items, state.excluded);

  return (
    <>
      <div className="book-editor-bar">
        <a href={`/book/${resp.book.id}`}>{state.dirty ? "← Back without saving" : "← Read"}</a>
        <span className="book-editor-status" aria-live="polite">
          {state.saving ? "Saving…" : state.dirty ? "Unsaved changes" : state.saved ? "Saved" : ""}
        </span>
        <button
          type="button"
          className="btn btn-primary"
          disabled={!state.dirty || state.saving}
          onClick={() => save(state, resp, r)}
        >
          Save
        </button>
      </div>
      {state.error && (
        <p className="error-message" role="alert">
          {state.error}{" "}
          {state.conflict && (
            <button type="button" className="btn btn-secondary" onClick={() => location.reload()}>
              Reload
            </button>
          )}
        </p>
      )}

      <section className="book-editor-section">
        <h1>Edit book</h1>
        <label className="book-editor-field">
          <span>Title</span>
          <input
            type="text"
            value={state.title}
            maxLength={120}
            onInput={vlens.cachePartial(setText, state, "title")}
          />
        </label>
        <p className="book-editor-hint">{bookDates(resp.book.startDate, resp.book.endDate)}</p>
        <CoverPicker state={state} r={r} />
      </section>

      <section className="book-editor-section">
        <h2>Your words</h2>
        <p className="book-editor-hint">
          These live only in this book. Memories worth keeping are better added as milestones, so
          they appear everywhere.
        </p>
        <label className="book-editor-field">
          <span>Introduction</span>
          <textarea
            rows={3}
            value={state.introduction}
            placeholder="A few lines to open the book"
            onInput={vlens.cachePartial(setText, state, "introduction")}
          />
        </label>
        <label className="book-editor-field">
          <span>Closing letter</span>
          <textarea
            rows={4}
            value={state.letter}
            placeholder="Something to say to them at the end of the year"
            onInput={vlens.cachePartial(setText, state, "letter")}
          />
        </label>
        <label className="book-editor-field">
          <span>Signed</span>
          <input
            type="text"
            value={state.signature}
            placeholder="Love, Mom and Dad"
            onInput={vlens.cachePartial(setText, state, "signature")}
          />
        </label>
        <label className="book-editor-check">
          <input
            type="checkbox"
            checked={state.showGrowth}
            onChange={() => {
              state.showGrowth = !state.showGrowth;
              changed(state);
            }}
          />
          Include how they grew
        </label>
      </section>

      <section className="book-editor-section">
        <h2>How many photos</h2>
        <div className="book-editor-row">
          <SegmentedControl
            label="Length"
            options={DENSITIES.map(d => ({ value: d, label: d[0].toUpperCase() + d.slice(1) }))}
            value={state.density}
            onChange={density => {
              state.density = density;
              resuggest(state, resp);
            }}
          />
        </div>
        <div className="book-new-people" role="group" aria-label="What to include">
          {CATEGORIES.map(c => (
            <label key={c.value} className="book-editor-check">
              <input
                type="checkbox"
                checked={state.categories.includes(c.value)}
                disabled={state.categories.length === 1 && state.categories.includes(c.value)}
                onChange={() => {
                  state.categories = state.categories.includes(c.value)
                    ? state.categories.filter(x => x !== c.value)
                    : [...state.categories, c.value];
                  resuggest(state, resp);
                }}
              />
              {c.label}
            </label>
          ))}
        </div>
        {isMulti(r) && (
          <div className="book-editor-row">
            <SegmentedControl
              label="Which photos"
              options={[
                { value: "any", label: "Photos of any of them" },
                { value: "all", label: "Only photos of all of them" },
              ]}
              value={state.match}
              onChange={match => {
                state.match = match;
                resuggest(state, resp);
              }}
            />
          </div>
        )}
        <p className="book-editor-hint">
          Changing these picks again. Anything you've kept stays, and anything you've removed stays
          out.
        </p>
      </section>

      {additions.length > 0 && (
        <section className="book-editor-section book-editor-additions">
          <h2>New since you last saved</h2>
          <p className="book-editor-hint">
            Added to the family record after this book was last saved. Nothing joins the book until
            you add it.
          </p>
          {additions.map(item => (
            <div key={itemKey(item)} className="book-editor-leftout">
              <span className="book-editor-addition">
                {item.kind === server.BookItemPhoto ? (
                  <span className="book-editor-item-thumb">
                    <ThumbnailImage photoId={item.sourceId} alt="" />
                  </span>
                ) : (
                  <span>{r.milestones.get(item.sourceId)?.description}</span>
                )}
                <span className="book-editor-hint">{detailOf(r, item)}</span>
              </span>
              <span className="book-editor-row">
                <button type="button" onClick={() => addItem(state, r, item)}>
                  Add
                </button>
                <button
                  type="button"
                  onClick={() => {
                    state.excluded = [...state.excluded, { ...item, photoId: 0 }];
                    changed(state);
                  }}
                >
                  Leave out
                </button>
              </span>
            </div>
          ))}
        </section>
      )}

      {book.notes.missing > 0 && (
        <p className="book-editor-warning">
          {book.notes.missing === 1 ? "One item was" : `${book.notes.missing} items were`} deleted
          from the family record or can no longer be seen. Saving will drop{" "}
          {book.notes.missing === 1 ? "it" : "them"} from the book.
        </p>
      )}

      <section className="book-editor-section">
        <h2>Chapters</h2>
        {book.chapters
          .filter(chapter => chapter.items.length > 0)
          .map(chapter => (
            <div key={chapter.id} className="book-editor-chapter">
              <h3>
                {chapter.title}
                {chapter.dates && <span>{chapter.dates}</span>}
              </h3>
              <ol className="book-editor-items">
                {chapter.items.map((index, position) => (
                  <ItemRow
                    key={itemKey(state.items[index])}
                    state={state}
                    r={r}
                    index={index}
                    position={position}
                    chapterItems={chapter.items}
                    opening={position === 0 && chapter.blocks.some(b => b.kind === "hero")}
                  />
                ))}
              </ol>
            </div>
          ))}
      </section>

      <LeftOut state={state} r={r} />

      <EditorNotes book={book} />

      <section className="book-editor-section book-editor-danger">
        {state.confirmDelete ? (
          <p>
            Delete this book? The photos and milestones stay in the family record.{" "}
            <button
              type="button"
              className="btn btn-danger"
              onClick={() => deleteBook(state, resp.book.id)}
            >
              Delete book
            </button>{" "}
            <button
              type="button"
              className="btn btn-secondary"
              onClick={() => {
                state.confirmDelete = false;
                vlens.scheduleRedraw();
              }}
            >
              Cancel
            </button>
          </p>
        ) : (
          <button
            type="button"
            className="btn btn-secondary"
            onClick={() => {
              state.confirmDelete = true;
              vlens.scheduleRedraw();
            }}
          >
            Delete this book…
          </button>
        )}
      </section>
    </>
  );
};

const CoverPicker = ({ state, r }: { state: EditorState; r: Resolved }) => {
  const photos = candidatesIn(r).photos;
  return (
    <div className="book-editor-cover">
      <div className="book-editor-cover-current">
        {r.photos.has(state.coverPhotoId) ? (
          <ThumbnailImage photoId={state.coverPhotoId} alt="Cover photo" />
        ) : (
          <span>No cover</span>
        )}
      </div>
      <button
        type="button"
        className="btn btn-secondary"
        aria-expanded={state.pickingCover ? "true" : "false"}
        onClick={() => {
          state.pickingCover = !state.pickingCover;
          vlens.scheduleRedraw();
        }}
      >
        {state.pickingCover ? "Done choosing" : "Change cover"}
      </button>
      {state.pickingCover && (
        <ul className="book-editor-thumbs">
          {photos.map(photo => (
            <li key={photo.id}>
              <button
                type="button"
                aria-pressed={photo.id === state.coverPhotoId ? "true" : "false"}
                aria-label={`Use the photo from ${shortDay(dayOf(photo.photoDate))} as the cover`}
                onClick={() => {
                  state.coverPhotoId = photo.id;
                  state.pickingCover = false;
                  changed(state);
                }}
              >
                <ThumbnailImage photoId={photo.id} alt="" />
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
};

function detailOf(r: Resolved, item: server.BookItem): string {
  const day = itemDay(r, item) ?? "";
  const parts = [shortDay(day)];
  if (item.kind === server.BookItemMilestone) {
    const m = r.milestones.get(item.sourceId);
    const person = m ? r.person.get(m.personId) : undefined;
    if (person && isMulti(r)) parts.push(person.name);
    if (person && dayOf(person.birthday) <= day) {
      parts.push(photoAge(person.birthday, day + "T00:00:00Z"));
    }
  } else {
    const photo = r.photos.get(item.sourceId);
    const tagged = (r.photoPeople.get(item.sourceId) ?? []).map(id => r.person.get(id)?.name ?? "");
    const detail = photo ? photoDetail(r, photo) : "";
    if (detail) parts.push(detail);
    else if (isMulti(r)) parts.push(tagged.length ? joinNames(tagged) : "Nobody tagged");
  }
  return parts.join(" · ");
}

const ItemRow = ({
  state,
  r,
  index,
  position,
  chapterItems,
  opening,
}: {
  state: EditorState;
  r: Resolved;
  index: number;
  position: number;
  chapterItems: number[];
  opening: boolean;
}) => {
  const item = state.items[index];
  const day = itemDay(r, item) ?? "";
  const when = detailOf(r, item);
  const milestone = item.kind === server.BookItemMilestone ? r.milestones.get(item.sourceId) : null;
  const photo = item.kind === server.BookItemPhoto ? r.photos.get(item.sourceId) : null;
  const attached = (milestone?.photoIds ?? []).filter(id => r.photos.has(id));
  const shownPhoto = photo?.id ?? item.photoId;

  return (
    <li className={item.pinned ? "book-editor-item is-pinned" : "book-editor-item"}>
      <div className="book-editor-item-thumb">
        {shownPhoto ? (
          <ThumbnailImage photoId={shownPhoto} alt="" />
        ) : (
          <span aria-hidden="true">{milestone ? getCategoryIcon(milestone.category) : ""}</span>
        )}
      </div>
      <div className="book-editor-item-body">
        {opening && <span className="book-editor-badge">Opens the chapter</span>}
        {milestone && <p className="book-editor-item-text">{milestone.description}</p>}
        <p className="book-editor-hint">{when}</p>
        {photo && (
          <input
            type="text"
            className="book-editor-caption"
            aria-label="Caption in this book"
            value={item.caption}
            maxLength={300}
            placeholder={originalCaption(photo) || "Add a caption for this book"}
            onInput={(e: Event) =>
              updateItem(state, index, { caption: (e.target as HTMLInputElement).value })
            }
          />
        )}
        {milestone && attached.length > 0 && (
          <div className="book-editor-attached" role="group" aria-label="Photo shown with this">
            <button
              type="button"
              aria-pressed={item.photoId === 0 ? "true" : "false"}
              onClick={() => updateItem(state, index, { photoId: 0 })}
            >
              No photo
            </button>
            {attached.map(id => (
              <button
                key={id}
                type="button"
                className="book-editor-attached-thumb"
                aria-pressed={item.photoId === id ? "true" : "false"}
                aria-label="Show this photo"
                onClick={() => updateItem(state, index, { photoId: id })}
              >
                <ThumbnailImage photoId={id} alt="" />
              </button>
            ))}
          </div>
        )}
        {milestone && (
          <a className="book-editor-original" href={`/edit-milestone/${milestone.id}`}>
            Edit the original milestone
          </a>
        )}
      </div>
      <div className="book-editor-item-actions">
        <button
          type="button"
          aria-label="Move earlier"
          disabled={position === 0}
          onClick={() => moveItem(state, chapterItems, position, -1)}
        >
          ↑
        </button>
        <button
          type="button"
          aria-label="Move later"
          disabled={position === chapterItems.length - 1}
          onClick={() => moveItem(state, chapterItems, position, 1)}
        >
          ↓
        </button>
        <button
          type="button"
          aria-pressed={item.pinned ? "true" : "false"}
          title="Kept items stay when photos are picked again"
          onClick={() => updateItem(state, index, { pinned: !item.pinned })}
        >
          {item.pinned ? "Kept" : "Keep"}
        </button>
        <button type="button" onClick={() => removeItem(state, index)}>
          Remove
        </button>
      </div>
    </li>
  );
};

const monthLabel = (day: string) =>
  new Date(day + "T00:00:00Z").toLocaleDateString("en-US", {
    month: "long",
    year: "numeric",
    timeZone: "UTC",
  });

const LeftOut = ({ state, r }: { state: EditorState; r: Resolved }) => {
  const included = new Set(state.items.map(itemKey));
  const usedPhotos = new Set(
    state.items.filter(i => i.kind === server.BookItemMilestone && i.photoId).map(i => i.photoId)
  );
  usedPhotos.add(state.coverPhotoId);
  const { milestones, photos } = candidatesIn(r);
  const months = new Map<string, { milestones: server.Milestone[]; photos: server.Image[] }>();
  const month = (day: string) => {
    const key = day.slice(0, 7);
    if (!months.has(key)) months.set(key, { milestones: [], photos: [] });
    return months.get(key)!;
  };
  for (const m of milestones) {
    if (!included.has(`${server.BookItemMilestone}:${m.id}`))
      month(dayOf(m.milestoneDate)).milestones.push(m);
  }
  for (const p of photos) {
    if (!included.has(`${server.BookItemPhoto}:${p.id}`) && !usedPhotos.has(p.id)) {
      month(dayOf(p.photoDate)).photos.push(p);
    }
  }
  if (months.size === 0) return null;

  return (
    <section className="book-editor-section">
      <h2>Not in the book</h2>
      <p className="book-editor-hint">Everything else from the year. Add anything you miss.</p>
      {[...months.entries()].map(([key, group]) => {
        const open = !!state.openMonths[key];
        const parts = [
          group.milestones.length &&
            `${group.milestones.length} milestone${group.milestones.length === 1 ? "" : "s"}`,
          group.photos.length &&
            `${group.photos.length} photo${group.photos.length === 1 ? "" : "s"}`,
        ].filter(Boolean);
        return (
          <div key={key} className="book-editor-month">
            <button
              type="button"
              className="book-editor-month-toggle"
              aria-expanded={open ? "true" : "false"}
              onClick={() => {
                state.openMonths = { ...state.openMonths, [key]: !open };
                vlens.scheduleRedraw();
              }}
            >
              <span>{monthLabel(key + "-01")}</span>
              <span className="book-editor-hint">{parts.join(" · ")}</span>
            </button>
            {open && (
              <div className="book-editor-month-body">
                {group.milestones.map(m => (
                  <div key={m.id} className="book-editor-leftout">
                    <span>
                      {getCategoryIcon(m.category)} {m.description}
                      <span className="book-editor-hint">
                        {" "}
                        · {shortDay(dayOf(m.milestoneDate))}
                      </span>
                    </span>
                    <button
                      type="button"
                      onClick={() =>
                        addItem(state, r, {
                          kind: server.BookItemMilestone,
                          sourceId: m.id,
                          photoId: (m.photoIds ?? []).find(id => r.photos.has(id)) ?? 0,
                          caption: "",
                          pinned: false,
                        })
                      }
                    >
                      Add
                    </button>
                  </div>
                ))}
                {group.photos.length > 0 && (
                  <ul className="book-editor-thumbs">
                    {group.photos.map(p => (
                      <li key={p.id}>
                        <button
                          type="button"
                          aria-label={`Add the photo from ${detailOf(r, { kind: server.BookItemPhoto, sourceId: p.id, photoId: 0, caption: "", pinned: false })}`}
                          onClick={() =>
                            addItem(state, r, {
                              kind: server.BookItemPhoto,
                              sourceId: p.id,
                              photoId: 0,
                              caption: "",
                              pinned: false,
                            })
                          }
                        >
                          <ThumbnailImage photoId={p.id} alt="" />
                          <span className="book-editor-add">+</span>
                        </button>
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            )}
          </div>
        );
      })}
    </section>
  );
};
