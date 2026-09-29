import * as preact from "preact";
import * as vlens from "vlens";
import * as server from "../server";
import "./family-places-styles";

type FamilyPlacesState = {
  places: server.FamilyPlaceWithCount[] | null;
  loading: boolean;
  names: Record<number, string>;
  error: string;
};

const useFamilyPlaces = vlens.declareHook(
  (): FamilyPlacesState => ({ places: null, loading: false, names: {}, error: "" })
);

async function refresh(state: FamilyPlacesState) {
  state.loading = true;
  const [resp, err] = await server.ListFamilyPlaces({ familyId: 0 });
  state.loading = false;
  state.places = resp?.places ?? [];
  state.names = {};
  state.error = err || "";
  vlens.scheduleRedraw();
}

async function rename(state: FamilyPlacesState, place: server.FamilyPlace) {
  const name = (state.names[place.id] ?? place.name).trim();
  if (!name || name === place.name) return;
  const [, err] = await server.SaveFamilyPlace({
    id: place.id,
    photoId: 0,
    name,
    radiusMeters: place.radiusMeters,
  });
  state.error = err || "";
  await refresh(state);
}

async function remove(state: FamilyPlacesState, place: server.FamilyPlace) {
  if (!confirm(`Forget "${place.name}"? Photos taken there keep their location.`)) return;
  const [, err] = await server.DeleteFamilyPlace({ id: place.id });
  state.error = err || "";
  await refresh(state);
}

const formatRadius = (meters: number) =>
  meters >= 1000 ? `${(meters / 1000).toFixed(meters % 1000 ? 1 : 0)} km` : `${meters} m`;

export const FamilyPlacesSection = () => {
  const state = useFamilyPlaces();
  if (state.places === null && !state.loading) refresh(state);

  return (
    <div className="settings-section">
      <h2>Places</h2>
      <div className="settings-card">
        <p className="section-description">
          Named places replace the nearest town on photos taken inside them. Name a place from any
          photo that has a location.
        </p>
        {state.error && (
          <div className="error-message" role="alert">
            {state.error}
          </div>
        )}
        {state.places && state.places.length === 0 && (
          <p className="family-places-empty">No named places yet.</p>
        )}
        <div className="family-places">
          {(state.places ?? []).map(({ place, photoCount }) => (
            <div key={place.id} className="family-place-row">
              <input
                type="text"
                maxLength={80}
                aria-label="Place name"
                value={state.names[place.id] ?? place.name}
                onInput={e => (state.names[place.id] = e.currentTarget.value)}
                onBlur={() => rename(state, place)}
              />
              <span className="family-place-meta">
                within {formatRadius(place.radiusMeters)} ·{" "}
                <a href={`/photos?place=f${place.id}`}>
                  {photoCount} photo{photoCount === 1 ? "" : "s"}
                </a>
              </span>
              <button className="btn btn-outline btn-small" onClick={() => remove(state, place)}>
                Forget
              </button>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
};
