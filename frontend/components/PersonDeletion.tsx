import * as preact from "preact";
import * as vlens from "vlens";
import * as core from "vlens/core";
import * as server from "../server";
import "./person-deletion-styles";

type PersonDeletionState = {
  summary: server.PersonDeletionSummary | null;
  loadedFor: number;
  confirming: boolean;
  error: string;
  busy: boolean;
};

const usePersonDeletion = vlens.declareHook(
  (): PersonDeletionState => ({
    summary: null,
    loadedFor: 0,
    confirming: false,
    error: "",
    busy: false,
  })
);

async function load(state: PersonDeletionState, personId: number) {
  state.loadedFor = personId;
  state.summary = null;
  state.confirming = false;
  state.error = "";
  const [resp] = await server.GetPersonDeletionSummary({ personId });
  state.summary = resp;
  vlens.scheduleRedraw();
}

async function onReviewClicked(state: PersonDeletionState, personId: number) {
  state.busy = true;
  vlens.scheduleRedraw();
  const [resp, err] = await server.GetPersonDeletionSummary({ personId });
  state.busy = false;
  if (resp) {
    state.summary = resp;
    state.confirming = true;
    state.error = "";
  } else {
    state.error = err || "Could not load what would be deleted";
  }
  vlens.scheduleRedraw();
}

function onCancelClicked(state: PersonDeletionState) {
  state.confirming = false;
  state.error = "";
  vlens.scheduleRedraw();
}

async function onDeleteClicked(state: PersonDeletionState, personId: number) {
  state.busy = true;
  state.error = "";
  vlens.scheduleRedraw();
  const [resp, err] = await server.DeletePerson({ personId });
  state.busy = false;
  if (resp && resp.success) {
    state.loadedFor = 0;
    core.setRoute("/dashboard");
  } else {
    state.error = err || "Could not delete this person";
  }
  vlens.scheduleRedraw();
}

function plural(count: number, one: string, many: string): string {
  return `${count} ${count === 1 ? one : many}`;
}

function removalLines(summary: server.PersonDeletionSummary): string[] {
  const lines: string[] = [];
  if (summary.milestones > 0) {
    lines.push(plural(summary.milestones, "milestone", "milestones"));
  }
  if (summary.growthRecords > 0) {
    lines.push(plural(summary.growthRecords, "measurement", "measurements"));
  }
  if (summary.photoTags > 0) {
    lines.push(`their tag on ${plural(summary.photoTags, "photo", "photos")}`);
  }
  if (summary.faces > 0) {
    lines.push(`their name on ${plural(summary.faces, "recognised face", "recognised faces")}`);
  }
  if (summary.activityRoles > 0) {
    lines.push(
      `their place on ${plural(summary.activityRoles, "activity entry", "activity entries")}`
    );
  }
  if (summary.results > 0) {
    lines.push(`their name on ${plural(summary.results, "result", "results")}`);
  }
  if (summary.relations > 0) {
    lines.push(plural(summary.relations, "relationship", "relationships"));
  }
  if (summary.sharedFamilies > 0) {
    lines.push(`sharing with ${plural(summary.sharedFamilies, "other family", "other families")}`);
  }
  return lines;
}

interface PersonDeletionProps {
  personId: number;
  personName: string;
}

export const PersonDeletionSection = ({
  personId,
  personName,
}: PersonDeletionProps): preact.ComponentChild => {
  const state = usePersonDeletion();

  if (state.loadedFor !== personId) {
    load(state, personId);
    return null;
  }

  if (!state.summary) {
    return null;
  }

  const lines = removalLines(state.summary);

  return (
    <div className="form-group person-deletion">
      <label>Delete {personName}</label>

      {state.error && (
        <div className="error-message" role="alert">
          {state.error}
        </div>
      )}

      {!state.confirming && (
        <div className="person-deletion-intro">
          <p>
            Removes {personName} and everything recorded about them. Photos stay; only their tags
            are removed.
          </p>
          <button
            type="button"
            className="btn btn-secondary"
            disabled={state.busy}
            onClick={vlens.cachePartial(onReviewClicked, state, personId)}
          >
            Delete {personName}…
          </button>
        </div>
      )}

      {state.confirming && (
        <div className="person-deletion-confirm">
          {lines.length > 0 ? (
            <>
              <p>This permanently deletes {personName} and:</p>
              <ul>
                {lines.map(line => (
                  <li key={line}>{line}</li>
                ))}
              </ul>
            </>
          ) : (
            <p>This permanently deletes {personName}. Nothing else is recorded about them.</p>
          )}
          <p className="person-deletion-note">
            This can't be undone. If {personName} is a duplicate, merge them in Settings instead so
            their records are kept.
          </p>
          <div className="person-deletion-actions">
            <button
              type="button"
              className="btn btn-danger"
              disabled={state.busy}
              onClick={vlens.cachePartial(onDeleteClicked, state, personId)}
            >
              {state.busy ? "Deleting..." : `Delete ${personName}`}
            </button>
            <button
              type="button"
              className="btn btn-secondary"
              disabled={state.busy}
              onClick={vlens.cachePartial(onCancelClicked, state)}
            >
              Cancel
            </button>
          </div>
        </div>
      )}
    </div>
  );
};
