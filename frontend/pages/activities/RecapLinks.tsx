import * as preact from "preact";
import * as vlens from "vlens";
import { Copy, Link } from "lucide-preact";
import { Icon } from "../../components/Icon";
import * as server from "../../server";
import { copy } from "../../lib/copy";
import { CompetitionShare, recapLinkInput } from "../../lib/share/competitionShare";
import { ActivityLabels } from "./labels";
import "./recap-links-styles";

const text = copy.share.recapLink;

export type RecapLinksState = {
  eventId: number;
  links: server.RecapLinkView[];
  loadError: string;
  expiresInDays: number;
  confirming: boolean;
  publishing: boolean;
  created: string;
  copied: boolean;
  error: string;
  busyId: number;
};

export function newRecapLinks(): RecapLinksState {
  return {
    eventId: 0,
    links: [],
    loadError: "",
    expiresInDays: 30,
    confirming: false,
    publishing: false,
    created: "",
    copied: false,
    error: "",
    busyId: 0,
  };
}

/** Starts the panel fresh for this event and fetches its existing links. */
export async function openRecapLinks(state: RecapLinksState, eventId: number) {
  state.eventId = eventId;
  state.confirming = false;
  state.publishing = false;
  state.created = "";
  state.copied = false;
  state.error = "";
  state.loadError = "";
  const [resp, err] = await server.ListRecapLinks({ eventId });
  if (state.eventId !== eventId) return;
  if (err || !resp) state.loadError = text.loadFailed;
  else state.links = resp.links ?? [];
  vlens.scheduleRedraw();
}

function onExpiryChange(state: RecapLinksState, event: Event) {
  state.expiresInDays = Number((event.target as HTMLSelectElement).value) || 0;
  vlens.scheduleRedraw();
}

function setConfirming(state: RecapLinksState, confirming: boolean) {
  state.confirming = confirming;
  state.error = "";
  vlens.scheduleRedraw();
}

async function onPublish(
  state: RecapLinksState,
  recap: CompetitionShare,
  appearances: server.AppearanceDetail[]
) {
  if (state.publishing) return;
  state.publishing = true;
  state.error = "";
  vlens.scheduleRedraw();

  const eventId = state.eventId;
  const [resp, err] = await server.CreateRecapLink({
    eventId,
    expiresInDays: state.expiresInDays,
    recap: recapLinkInput(recap, appearances),
  });
  state.publishing = false;
  if (state.eventId !== eventId) return;
  if (err || !resp) {
    state.error = err || "Couldn't create the link.";
  } else {
    state.created = resp.url;
    state.copied = false;
    state.confirming = false;
    state.links = [resp.link, ...state.links];
  }
  vlens.scheduleRedraw();
}

async function onCopy(state: RecapLinksState) {
  try {
    await navigator.clipboard.writeText(state.created);
    state.copied = true;
  } catch {
    state.copied = false;
  }
  vlens.scheduleRedraw();
}

async function onRevoke(state: RecapLinksState, id: number) {
  if (state.busyId) return;
  state.busyId = id;
  state.error = "";
  vlens.scheduleRedraw();

  const [, err] = await server.RevokeRecapLink({ id });
  state.busyId = 0;
  if (err) {
    state.error = err;
  } else {
    state.links = state.links.filter(link => link.id !== id);
  }
  vlens.scheduleRedraw();
}

function selectAll(event: FocusEvent) {
  (event.target as HTMLInputElement).select();
}

function localDay(timestamp: string): string {
  const date = new Date(timestamp);
  return isNaN(date.getTime()) ? "" : date.toLocaleDateString();
}

function lastsText(days: number): string {
  return days > 0 ? text.forDays(days) : text.untilTurnedOff;
}

export const RecapLinks = ({
  state,
  recap,
  appearances,
  labels,
}: {
  state: RecapLinksState;
  recap: CompetitionShare;
  appearances: server.AppearanceDetail[];
  labels: ActivityLabels;
}) => {
  const count = recap.routines.length;
  const included = `${count} ${(count === 1 ? labels.entry : labels.entryPlural).toLowerCase()}`;
  const showsNames = recap.routines.some(routine => routine.roster);

  return (
    <section className="recap-links" aria-labelledby="recapLinksTitle">
      <h3 id="recapLinksTitle">{text.title}</h3>
      <p className="share-dialog-note">{text.explain}</p>

      {state.created ? (
        <div className="recap-link-created">
          <p className="share-dialog-note">{text.created}</p>
          <div className="recap-link-row">
            <input
              className="recap-link-url"
              type="text"
              readOnly
              value={state.created}
              aria-label={text.title}
              onFocus={selectAll}
            />
            <button
              type="button"
              className="btn btn-secondary"
              onClick={vlens.cachePartial(onCopy, state)}
            >
              <Icon symbol={Copy} /> {state.copied ? copy.share.copied : text.copy}
            </button>
          </div>
        </div>
      ) : state.confirming ? (
        <div className="recap-link-confirm">
          <p>{text.confirm(included, showsNames, lastsText(state.expiresInDays))}</p>
          <div className="recap-link-row">
            <button
              type="button"
              className="btn btn-primary"
              disabled={state.publishing || count === 0}
              onClick={vlens.cachePartial(onPublish, state, recap, appearances)}
            >
              {state.publishing ? text.publishing : text.publish}
            </button>
            <button
              type="button"
              className="btn btn-secondary"
              disabled={state.publishing}
              onClick={vlens.cachePartial(setConfirming, state, false)}
            >
              {text.cancel}
            </button>
          </div>
        </div>
      ) : (
        <div className="recap-link-row">
          <label className="recap-link-expiry">
            <span>{text.lasts}</span>
            <select
              value={String(state.expiresInDays)}
              onChange={vlens.cachePartial(onExpiryChange, state)}
            >
              {text.expiryOptions.map(option => (
                <option key={option.days} value={String(option.days)}>
                  {option.label}
                </option>
              ))}
            </select>
          </label>
          <button
            type="button"
            className="btn btn-secondary"
            disabled={count === 0}
            onClick={vlens.cachePartial(setConfirming, state, true)}
          >
            <Icon symbol={Link} /> {text.create}
          </button>
        </div>
      )}

      {state.error && (
        <p className="error-message" role="alert">
          {state.error}
        </p>
      )}
      <p className="share-dialog-note">{text.turnOffNote}</p>

      {state.loadError && <p className="share-dialog-note">{state.loadError}</p>}
      {state.links.length > 0 && (
        <div className="recap-link-list">
          <h4>{text.existing}</h4>
          <ul>
            {state.links.map(link => (
              <RecapLinkRow key={link.id} state={state} link={link} labels={labels} />
            ))}
          </ul>
        </div>
      )}
    </section>
  );
};

const RecapLinkRow = ({
  state,
  link,
  labels,
}: {
  state: RecapLinksState;
  link: server.RecapLinkView;
  labels: ActivityLabels;
}) => {
  const routines = `${link.routineCount} ${(link.routineCount === 1 ? labels.entry : labels.entryPlural).toLowerCase()}`;
  const expiry = link.expired
    ? text.expired
    : link.expiresAt
      ? text.expires(localDay(link.expiresAt))
      : text.noExpiry;
  return (
    <li className={link.expired ? "recap-link-item is-expired" : "recap-link-item"}>
      <div>
        <p>{text.createdBy(localDay(link.createdAt), link.createdByName)}</p>
        <p className="share-dialog-note">
          {[routines, link.snapshotLabel, expiry].filter(Boolean).join(" · ")}
        </p>
      </div>
      {link.canRevoke && (
        <button
          type="button"
          className="btn btn-secondary btn-small"
          disabled={state.busyId !== 0}
          onClick={vlens.cachePartial(onRevoke, state, link.id)}
        >
          {link.expired ? text.remove : text.turnOff}
        </button>
      )}
    </li>
  );
};
