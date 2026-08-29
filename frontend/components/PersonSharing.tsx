import * as preact from "preact";
import * as vlens from "vlens";
import * as server from "../server";
import "./family-links-styles";

type PersonSharingState = {
  sharing: server.GetPersonSharingResponse | null;
  loadedFor: number;
  targetFamilyId: number;
  relationship: string;
  error: string;
  busy: boolean;
};

const usePersonSharing = vlens.declareHook(
  (): PersonSharingState => ({
    sharing: null,
    loadedFor: 0,
    targetFamilyId: 0,
    relationship: "",
    error: "",
    busy: false,
  })
);

async function load(state: PersonSharingState, personId: number) {
  state.loadedFor = personId;
  const [resp, err] = await server.GetPersonSharing({ personId });
  if (resp) {
    state.sharing = resp;
    state.targetFamilyId = resp.canShare.length > 0 ? resp.canShare[0].familyId : 0;
  } else if (err) {
    state.error = err;
  }
  vlens.scheduleRedraw();
}

function applyResult(
  state: PersonSharingState,
  resp: server.PersonSharingActionResponse | null,
  err: string | null,
  fallback: string
) {
  state.busy = false;
  if (resp && resp.success) {
    state.sharing = resp.sharing;
    state.targetFamilyId = resp.sharing.canShare.length > 0 ? resp.sharing.canShare[0].familyId : 0;
    state.relationship = "";
    state.error = "";
  } else {
    state.error = resp?.error || err || fallback;
  }
  vlens.scheduleRedraw();
}

async function onShareClicked(state: PersonSharingState, personId: number, event: Event) {
  event.preventDefault();
  if (!state.targetFamilyId) {
    return;
  }
  state.busy = true;
  state.error = "";
  vlens.scheduleRedraw();

  const [resp, err] = await server.SharePersonWithFamily({
    personId,
    familyId: state.targetFamilyId,
    relationship: state.relationship.trim(),
  });
  applyResult(state, resp, err, "Could not share this person");
}

async function onUnshareClicked(
  state: PersonSharingState,
  personId: number,
  target: server.SharedRosterRef
) {
  if (!confirm(`Remove this person from ${target.familyName}'s list?`)) {
    return;
  }
  state.busy = true;
  state.error = "";
  vlens.scheduleRedraw();

  const [resp, err] = await server.UnsharePersonFromFamily({
    personId,
    familyId: target.familyId,
  });
  applyResult(state, resp, err, "Could not stop sharing this person");
}

interface PersonSharingProps {
  personId: number;
  personName: string;
}

export const PersonSharingSection = ({
  personId,
  personName,
}: PersonSharingProps): preact.ComponentChild => {
  const state = usePersonSharing();

  if (state.loadedFor !== personId) {
    load(state, personId);
    return null;
  }

  const sharing = state.sharing;
  if (!sharing || !sharing.manageable) {
    return null;
  }

  if (sharing.canShare.length === 0 && sharing.sharedWith.length === 0) {
    return (
      <div className="form-group">
        <label>Shared with</label>
        <p className="family-link-note">
          {personName} is only on your family's list. Connect to another family in Settings to be
          able to share them.
        </p>
      </div>
    );
  }

  return (
    <div className="form-group">
      <label>Shared with</label>

      {state.error && (
        <div className="error-message" role="alert">
          {state.error}
        </div>
      )}

      {sharing.sharedWith.length > 0 && (
        <div className="person-sharing-list">
          {sharing.sharedWith.map(target => (
            <div key={target.familyId} className="person-sharing-row">
              <span>
                {target.familyName}
                {target.relationship && (
                  <span className="person-sharing-role">
                    {" · "}
                    {target.relationship}
                  </span>
                )}
              </span>
              <button
                type="button"
                className="btn btn-secondary"
                disabled={state.busy}
                onClick={vlens.cachePartial(onUnshareClicked, state, personId, target)}
              >
                Stop sharing
              </button>
            </div>
          ))}
        </div>
      )}

      {sharing.canShare.length > 0 && (
        <div className="person-sharing-form">
          <div className="form-group">
            <label htmlFor="shareTargetFamily">Add to</label>
            <select
              id="shareTargetFamily"
              value={String(state.targetFamilyId)}
              disabled={state.busy}
              onInput={event => {
                state.targetFamilyId = Number((event.currentTarget as HTMLSelectElement).value);
                vlens.scheduleRedraw();
              }}
            >
              {sharing.canShare.map(target => (
                <option key={target.familyId} value={String(target.familyId)}>
                  {target.familyName}
                  {target.kind ? ` (${target.kind})` : ""}
                </option>
              ))}
            </select>
          </div>

          <div className="form-group">
            <label htmlFor="shareRelationship">Relationship (optional)</label>
            <input
              type="text"
              id="shareRelationship"
              placeholder="Grandchild, Niece…"
              maxLength={40}
              {...vlens.attrsBindInput(vlens.ref(state, "relationship"))}
              disabled={state.busy}
            />
          </div>

          <button
            type="button"
            className="btn btn-secondary"
            disabled={state.busy || !state.targetFamilyId}
            onClick={vlens.cachePartial(onShareClicked, state, personId)}
          >
            Share
          </button>
        </div>
      )}
    </div>
  );
};
