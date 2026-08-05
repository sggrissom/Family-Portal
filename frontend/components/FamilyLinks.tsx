import * as preact from "preact";
import * as vlens from "vlens";
import * as server from "../server";
import { FamilySelect } from "./FamilySelect";
import "./family-links-styles";

// SCOPE_LABELS drives both the create form and the per-link editor, so the two
// can never drift apart on what a link is able to share.
const SCOPE_LABELS: { key: keyof server.LinkScopes; label: string; hint: string }[] = [
  { key: "people", label: "People", hint: "Required — the people you choose to share" },
  { key: "milestones", label: "Milestones", hint: "Their milestones and the tags on them" },
  { key: "photos", label: "Photos", hint: "Photos they appear in" },
  { key: "growth", label: "Measurements", hint: "Height and weight history" },
];

function defaultScopes(): server.LinkScopes {
  return { people: true, milestones: true, photos: true, growth: false };
}

type FamilyLinksState = {
  links: server.FamilyLinkView[] | null;
  familyId: number;
  inviteCode: string;
  kind: string;
  scopes: server.LinkScopes;
  // Pending scope edits per link id, so editing one card does not disturb
  // another that is mid-change.
  edits: Record<number, server.LinkScopes>;
  error: string;
  busy: boolean;
};

const useFamilyLinks = vlens.declareHook(
  (): FamilyLinksState => ({
    links: null,
    familyId: 0,
    inviteCode: "",
    kind: "",
    scopes: defaultScopes(),
    edits: {},
    error: "",
    busy: false,
  })
);

async function refresh(state: FamilyLinksState) {
  const [resp, err] = await server.ListFamilyLinks({ familyId: 0 });
  if (resp) {
    state.links = resp.links;
    state.edits = {};
  } else if (err) {
    state.error = err;
  }
  vlens.scheduleRedraw();
}

// Sharing people is what every other scope hangs off, so it is implied rather
// than separately selectable. The backend normalizes the same way; doing it here
// too keeps the checkboxes honest about what will be saved.
function withImpliedPeople(scopes: server.LinkScopes): server.LinkScopes {
  const needsPeople = scopes.milestones || scopes.photos || scopes.growth;
  return { ...scopes, people: scopes.people || needsPeople };
}

async function onCreateLink(state: FamilyLinksState, event: Event) {
  event.preventDefault();
  state.busy = true;
  state.error = "";
  vlens.scheduleRedraw();

  const [resp, err] = await server.CreateFamilyLink({
    familyId: state.familyId,
    inviteCode: state.inviteCode.trim(),
    kind: state.kind.trim(),
    scopes: withImpliedPeople(state.scopes),
  });

  state.busy = false;
  if (resp && resp.success) {
    state.inviteCode = "";
    state.kind = "";
    state.scopes = defaultScopes();
    await refresh(state);
    return;
  }
  state.error = resp?.error || err || "Could not create the link";
  vlens.scheduleRedraw();
}

async function onAcceptLink(state: FamilyLinksState, linkId: number) {
  state.busy = true;
  state.error = "";
  vlens.scheduleRedraw();

  const [resp, err] = await server.AcceptFamilyLink({ id: linkId });
  state.busy = false;
  if (resp && resp.success) {
    await refresh(state);
    return;
  }
  state.error = resp?.error || err || "Could not accept the link";
  vlens.scheduleRedraw();
}

async function onSaveScopes(state: FamilyLinksState, link: server.FamilyLinkView) {
  const scopes = withImpliedPeople(state.edits[link.id] ?? link.scopes);
  state.busy = true;
  state.error = "";
  vlens.scheduleRedraw();

  const [resp, err] = await server.UpdateFamilyLink({
    id: link.id,
    kind: link.kind,
    scopes,
  });

  state.busy = false;
  if (resp && resp.success) {
    await refresh(state);
    return;
  }
  state.error = resp?.error || err || "Could not update what this link shares";
  vlens.scheduleRedraw();
}

async function onRevokeLink(state: FamilyLinksState, link: server.FamilyLinkView) {
  const other = link.outgoing ? link.toFamilyName : link.fromFamilyName;
  if (
    !confirm(
      `Disconnect from ${other}? Anyone shared through this connection is removed from the other family's list.`
    )
  ) {
    return;
  }

  state.busy = true;
  state.error = "";
  vlens.scheduleRedraw();

  const [resp, err] = await server.RevokeFamilyLink({ id: link.id });
  state.busy = false;
  if (resp && resp.success) {
    await refresh(state);
    return;
  }
  state.error = resp?.error || err || "Could not disconnect";
  vlens.scheduleRedraw();
}

function scopeSummary(scopes: server.LinkScopes): string {
  const shared = SCOPE_LABELS.filter(scope => scopes[scope.key]).map(scope => scope.label);
  return shared.length > 0 ? shared.join(", ") : "nothing";
}

interface ScopeCheckboxesProps {
  idPrefix: string;
  scopes: server.LinkScopes;
  disabled: boolean;
  onChange: (scopes: server.LinkScopes) => void;
}

const ScopeCheckboxes = ({ idPrefix, scopes, disabled, onChange }: ScopeCheckboxesProps) => (
  <div className="family-link-scopes">
    {SCOPE_LABELS.map(scope => (
      <label key={scope.key} htmlFor={`${idPrefix}-${scope.key}`} title={scope.hint}>
        <input
          type="checkbox"
          id={`${idPrefix}-${scope.key}`}
          checked={scopes[scope.key]}
          disabled={disabled || scope.key === "people"}
          onInput={event => {
            const checked = (event.currentTarget as HTMLInputElement).checked;
            onChange(withImpliedPeople({ ...scopes, [scope.key]: checked }));
            vlens.scheduleRedraw();
          }}
        />
        <span>{scope.label}</span>
      </label>
    ))}
  </div>
);

interface FamilyLinkCardProps {
  state: FamilyLinksState;
  link: server.FamilyLinkView;
}

const FamilyLinkCard = ({ state, link }: FamilyLinkCardProps) => {
  const other = link.outgoing ? link.toFamilyName : link.fromFamilyName;
  const mine = link.outgoing ? link.fromFamilyName : link.toFamilyName;
  const scopes = state.edits[link.id] ?? link.scopes;
  const dirty = state.edits[link.id] !== undefined;
  const pending = link.status === server.LinkPending;

  return (
    <div className="family-link-card">
      <div className="family-link-heading">
        <strong>{other || `Family ${link.outgoing ? link.toFamilyId : link.fromFamilyId}`}</strong>
        <span
          className={pending ? "family-link-status is-pending" : "family-link-status is-accepted"}
        >
          {pending ? "Awaiting acceptance" : "Connected"}
        </span>
      </div>

      <p className="family-link-direction">
        {link.outgoing
          ? `${mine} shares with them${link.kind ? ` · ${link.kind}` : ""}`
          : `They share with ${mine}${link.kind ? ` · ${link.kind}` : ""}`}
      </p>

      {link.outgoing ? (
        <>
          <ScopeCheckboxes
            idPrefix={`link-${link.id}`}
            scopes={scopes}
            disabled={state.busy}
            onChange={next => {
              state.edits = { ...state.edits, [link.id]: next };
            }}
          />
          <p className="family-link-note">
            {link.sharedCount === 0
              ? "Nobody shared yet — choose who from each person's page."
              : `${link.sharedCount} ${link.sharedCount === 1 ? "person" : "people"} shared.`}
          </p>
        </>
      ) : (
        <p className="family-link-note">They share: {scopeSummary(link.scopes)}.</p>
      )}

      <div className="family-link-actions">
        {pending && !link.outgoing && (
          <button
            type="button"
            className="btn btn-primary"
            disabled={state.busy}
            onClick={vlens.cachePartial(onAcceptLink, state, link.id)}
          >
            Accept
          </button>
        )}
        {pending && link.outgoing && (
          <span className="family-link-note">
            Waiting for {other} to accept. Nothing is shared until they do.
          </span>
        )}
        {link.outgoing && dirty && (
          <button
            type="button"
            className="btn btn-primary"
            disabled={state.busy}
            onClick={vlens.cachePartial(onSaveScopes, state, link)}
          >
            Save changes
          </button>
        )}
        <button
          type="button"
          className="btn btn-secondary"
          disabled={state.busy}
          onClick={vlens.cachePartial(onRevokeLink, state, link)}
        >
          {pending && !link.outgoing ? "Decline" : "Disconnect"}
        </button>
      </div>
    </div>
  );
};

interface FamilyLinksSectionProps {
  initialLinks: server.FamilyLinkView[];
}

// FamilyLinksSection is the settings surface for relating one household to
// another: who we share with, who shares with us, and exactly what travels.
export const FamilyLinksSection = ({
  initialLinks,
}: FamilyLinksSectionProps): preact.ComponentChild => {
  const state = useFamilyLinks();
  const links = state.links ?? initialLinks;

  return (
    <div className="settings-section">
      <h2>Connected Families</h2>
      <div className="settings-card">
        <p className="section-description">
          Connect to another household — grandparents, a co-parent — to share specific people with
          them. They see only the people you share and only the kinds of record you allow. This is
          not the same as joining a family: a connection never lets the other household edit
          anything.
        </p>

        {state.error && <div className="error-message">{state.error}</div>}

        {links.length > 0 && (
          <div className="family-links">
            {links.map(link => (
              <FamilyLinkCard key={link.id} state={state} link={link} />
            ))}
          </div>
        )}

        <form onSubmit={vlens.cachePartial(onCreateLink, state)}>
          <h3>Connect to another family</h3>

          <FamilySelect
            id="linkFromFamily"
            label="Share from"
            value={state.familyId}
            disabled={state.busy}
            onChange={familyId => {
              state.familyId = familyId;
            }}
          />

          <div className="form-group">
            <label htmlFor="linkInviteCode">Their family invite code</label>
            <input
              type="text"
              id="linkInviteCode"
              placeholder="Ask them for their code"
              {...vlens.attrsBindInput(vlens.ref(state, "inviteCode"))}
              disabled={state.busy}
              required
            />
          </div>

          <div className="form-group">
            <label htmlFor="linkKind">What to call this (optional)</label>
            <input
              type="text"
              id="linkKind"
              placeholder="Grandparents"
              {...vlens.attrsBindInput(vlens.ref(state, "kind"))}
              disabled={state.busy}
            />
          </div>

          <div className="form-group">
            <label>What they will be able to see</label>
            <ScopeCheckboxes
              idPrefix="new-link"
              scopes={state.scopes}
              disabled={state.busy}
              onChange={scopes => {
                state.scopes = scopes;
              }}
            />
          </div>

          <button
            type="submit"
            className="btn btn-primary"
            disabled={state.busy || !state.inviteCode}
          >
            {state.busy ? "Working..." : "Send connection request"}
          </button>
        </form>
      </div>
    </div>
  );
};
