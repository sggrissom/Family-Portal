import * as preact from "preact";
import * as vlens from "vlens";
import * as server from "../server";
import { RELATION_OPTIONS } from "../lib/routeHelpers";
import { CoAnchorState, CoAnchorSuggestion, syncCoAnchors } from "../lib/relations";
import { CoAnchorPicker } from "./CoAnchorPicker";
import "./family-links-styles";

type PersonRelationsState = {
  relations: server.GetPersonRelationsResponse | null;
  people: server.Person[];
  graph: server.Relation[];
  loadedFor: number;
  relationIndex: number;
  anchorId: number;
  coAnchors: CoAnchorState;
  error: string;
  busy: boolean;
};

const usePersonRelations = vlens.declareHook(
  (): PersonRelationsState => ({
    relations: null,
    people: [],
    graph: [],
    loadedFor: 0,
    relationIndex: -1,
    anchorId: 0,
    coAnchors: { key: "", ids: [] },
    error: "",
    busy: false,
  })
);

async function load(state: PersonRelationsState, personId: number) {
  state.loadedFor = personId;
  const [resp, err] = await server.GetPersonRelations({ personId });
  if (resp) {
    state.relations = resp;
  } else if (err) {
    state.error = err;
  }
  const [peopleResp] = await server.ListPeople({});
  state.graph = peopleResp?.relations ?? [];
  state.people = (peopleResp?.people ?? []).filter(p => p.id !== personId);
  if (state.anchorId === 0 && state.people.length > 0) {
    state.anchorId = state.people[0].id;
  }
  vlens.scheduleRedraw();
}

async function refreshGraph(state: PersonRelationsState) {
  const [peopleResp] = await server.ListPeople({});
  state.graph = peopleResp?.relations ?? [];
  vlens.scheduleRedraw();
}

function applyResult(
  state: PersonRelationsState,
  resp: server.RelationActionResponse | null,
  err: string | null,
  fallback: string
) {
  state.busy = false;
  if (resp && resp.success) {
    state.relations = resp.relations;
    state.relationIndex = -1;
    state.coAnchors = { key: "", ids: [] };
    state.error = "";
    refreshGraph(state);
  } else {
    state.error = resp?.error || err || fallback;
  }
  vlens.scheduleRedraw();
}

async function onAddClicked(state: PersonRelationsState, personId: number, event: Event) {
  event.preventDefault();
  const relation = RELATION_OPTIONS[state.relationIndex];
  if (!relation || !state.anchorId) {
    return;
  }
  state.busy = true;
  state.error = "";
  vlens.scheduleRedraw();

  const [resp, err] = await server.AddRelation({
    personId,
    anchorId: state.anchorId,
    stated: relation.value,
    additionalAnchorIds: state.coAnchors.ids,
  });
  applyResult(state, resp, err, "Could not save that relationship");
}

async function onRemoveClicked(
  state: PersonRelationsState,
  relation: server.RelationView,
  event: Event
) {
  event.preventDefault();
  state.busy = true;
  state.error = "";
  vlens.scheduleRedraw();

  const [resp, err] = await server.RemoveRelation({ relationId: relation.id });
  applyResult(state, resp, err, "Could not remove that relationship");
}

interface PersonRelationsProps {
  personId: number;
  personName: string;
}

export const PersonRelationsSection = ({
  personId,
  personName,
}: PersonRelationsProps): preact.ComponentChild => {
  const state = usePersonRelations();

  if (state.loadedFor !== personId) {
    load(state, personId);
    return null;
  }

  const relations = state.relations;
  if (!relations || !relations.manageable) {
    return null;
  }

  const stored = relations.relations.filter(r => r.stored);
  const derived = relations.relations.filter(r => !r.stored);
  const picked = RELATION_OPTIONS[state.relationIndex];
  const suggestions: CoAnchorSuggestion[] = picked
    ? syncCoAnchors(state.coAnchors, state.graph, picked.value, personId, state.anchorId)
    : [];

  return (
    <div className="form-group">
      <label>Relationships</label>

      {state.error && (
        <div className="error-message" role="alert">
          {state.error}
        </div>
      )}

      {stored.length > 0 ? (
        <div className="person-sharing-list">
          {stored.map(relation => (
            <div key={relation.id} className="person-sharing-row">
              <span>
                {relation.personName}
                <span className="person-sharing-role">
                  {" · "}
                  {personName}'s {relation.label}
                </span>
              </span>
              <button
                type="button"
                className="btn btn-secondary"
                disabled={state.busy}
                onClick={vlens.cachePartial(onRemoveClicked, state, relation)}
              >
                Remove
              </button>
            </div>
          ))}
        </div>
      ) : (
        <p className="family-link-note">
          Nobody is recorded as related to {personName} yet. Everyone else's relationship is worked
          out from the ones you add here.
        </p>
      )}

      {derived.length > 0 && (
        <>
          <p className="family-link-note">
            Worked out from the above — nothing to enter for these:
          </p>
          <div className="person-sharing-list">
            {derived.map(relation => (
              <div key={relation.personId} className="person-sharing-row is-derived">
                <span>
                  {relation.personName}
                  <span className="person-sharing-role">
                    {" · "}
                    {personName}'s {relation.label}
                  </span>
                </span>
                <span className="person-sharing-derived">implied</span>
              </div>
            ))}
          </div>
        </>
      )}

      {state.people.length > 0 && (
        <div className="person-sharing-form">
          <div className="form-group">
            <label htmlFor="relationKind">Add a relationship</label>
            <div className="relation-row">
              <select
                id="relationKind"
                value={String(state.relationIndex)}
                disabled={state.busy}
                onInput={event => {
                  state.relationIndex = Number((event.currentTarget as HTMLSelectElement).value);
                  vlens.scheduleRedraw();
                }}
              >
                <option value="-1">Pick one</option>
                {RELATION_OPTIONS.map((option, index) => (
                  <option key={`${option.value}-${option.label}`} value={String(index)}>
                    {option.label}
                  </option>
                ))}
              </select>
              <span className="relation-joiner">of</span>
              <select
                id="relationAnchor"
                value={String(state.anchorId)}
                disabled={state.busy}
                onInput={event => {
                  state.anchorId = Number((event.currentTarget as HTMLSelectElement).value);
                  vlens.scheduleRedraw();
                }}
              >
                {state.people.map(person => (
                  <option key={person.id} value={String(person.id)}>
                    {person.name}
                  </option>
                ))}
              </select>
            </div>
          </div>

          <CoAnchorPicker
            suggestions={suggestions}
            state={state.coAnchors}
            people={state.people}
            relationLabel={picked ? picked.label : ""}
            disabled={state.busy}
          />

          <button
            type="button"
            className="btn btn-secondary"
            disabled={state.busy || state.relationIndex < 0 || !state.anchorId}
            onClick={vlens.cachePartial(onAddClicked, state, personId)}
          >
            Add relationship
          </button>
        </div>
      )}
    </div>
  );
};
