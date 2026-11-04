import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView, ensureAuthInFetch } from "../../lib/authHelpers";
import { formatLongDate } from "../../lib/dateUtils";
import { recordTimeLabel } from "../../lib/recordTime";
import { copy } from "../../lib/copy";
import "./review-styles";

const empty: server.ListBirthdayClustersResponse = { clusters: [], people: [] };

export async function fetch(
  route: string,
  prefix: string
): Promise<rpc.Response<server.ListBirthdayClustersResponse>> {
  if (!(await ensureAuthInFetch())) return rpc.ok(empty);
  return server.ListBirthdayClusters({});
}

type Meaning = "period" | "occurrence";

type ReviewState = {
  clusters: server.BirthdayCluster[] | null;
  excluded: Record<string, boolean>;
  meaning: Record<string, Meaning>;
  busy: boolean;
  error: string;
  undo: { correctionId: number; changed: number } | null;
};

const useReviewState = vlens.declareHook(
  (): ReviewState => ({
    clusters: null,
    excluded: {},
    meaning: {},
    busy: false,
    error: "",
    undo: null,
  })
);

const clusterKey = (c: server.BirthdayCluster) => `${c.personId}:${c.age}`;
const recordKey = (r: server.BirthdayClusterRecord) => `${r.kind}:${r.id}`;

function ageTime(
  cluster: server.BirthdayCluster,
  record: server.BirthdayClusterRecord,
  meaning: Meaning
): server.RecordTime {
  return {
    ...record.recordTime,
    meaning: record.kind === "photo" ? "occurrence" : meaning,
    basis: "age",
    precision: "year",
    estimated: false,
    from: "",
    to: "",
    around: "",
    anchorPersonId: cluster.personId,
    ageYears: cluster.age,
    ageMonths: 0,
    ageToYears: 0,
    source: "family",
    note: "",
    changedAt: "0001-01-01T00:00:00Z",
    start: "",
    end: "",
    resolved: false,
  };
}

async function apply(state: ReviewState, cluster: server.BirthdayCluster) {
  if (state.busy) return;
  const meaning = state.meaning[clusterKey(cluster)] ?? "period";
  const chosen = cluster.records.filter(r => !state.excluded[recordKey(r)]);
  if (chosen.length === 0) return;
  state.busy = true;
  state.error = "";
  vlens.scheduleRedraw();
  const [resp, err] = await server.CorrectRecordTimes({
    changes: chosen.map(r => ({
      kind: r.kind,
      id: r.id,
      recordTime: ageTime(cluster, r, meaning),
    })),
  });
  state.busy = false;
  if (!resp) {
    state.error = err || copy.dateReview.failed;
  } else {
    state.undo = { correctionId: resp.correctionId, changed: resp.changed };
    state.clusters = await reload();
  }
  vlens.scheduleRedraw();
}

async function undo(state: ReviewState) {
  if (!state.undo || state.busy) return;
  state.busy = true;
  vlens.scheduleRedraw();
  const [, err] = await server.UndoRecordTimeCorrection({ correctionId: state.undo.correctionId });
  state.busy = false;
  if (err) state.error = err;
  state.undo = null;
  state.clusters = await reload();
  vlens.scheduleRedraw();
}

async function reload(): Promise<server.BirthdayCluster[]> {
  const [resp] = await server.ListBirthdayClusters({});
  return resp?.clusters ?? [];
}

function toggle(state: ReviewState, key: string) {
  state.excluded[key] = !state.excluded[key];
  vlens.scheduleRedraw();
}

function choose(state: ReviewState, key: string, meaning: Meaning) {
  state.meaning[key] = meaning;
  vlens.scheduleRedraw();
}

export function view(
  route: string,
  prefix: string,
  data: server.ListBirthdayClustersResponse
): preact.ComponentChild {
  if (!requireAuthInView()) return;
  const state = useReviewState();
  if (state.clusters === null) state.clusters = data.clusters ?? [];
  const names = new Map((data.people ?? []).map(p => [p.id, p.name]));
  const t = copy.dateReview;

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="date-review">
        <h1>{t.title}</h1>
        <p className="date-review-intro">{t.intro}</p>

        {state.error && (
          <div className="error-message" role="alert">
            {state.error}
          </div>
        )}
        {state.undo && (
          <div className="date-review-notice" role="status">
            {t.changed(state.undo.changed)}{" "}
            <button
              type="button"
              className="link-button"
              disabled={state.busy}
              onClick={vlens.cachePartial(undo, state)}
            >
              {t.undo}
            </button>
          </div>
        )}
        {state.clusters.length === 0 && <p className="date-review-empty">{t.none}</p>}

        {state.clusters.map(cluster => {
          const key = clusterKey(cluster);
          const meaning = state.meaning[key] ?? "period";
          const name = names.get(cluster.personId) ?? "";
          const chosen = cluster.records.filter(r => !state.excluded[recordKey(r)]).length;
          return (
            <section key={key} className="date-review-cluster">
              <h2>{t.clusterTitle(name, cluster.age, formatLongDate(cluster.day))}</h2>
              <p className="date-review-intro">{t.clusterHint(cluster.age)}</p>
              <ul className="date-review-records">
                {cluster.records.map(record => {
                  const rk = recordKey(record);
                  const included = !state.excluded[rk];
                  const after = recordTimeLabel(ageTime(cluster, record, meaning));
                  return (
                    <li key={rk}>
                      <label>
                        <input
                          type="checkbox"
                          checked={included}
                          disabled={state.busy}
                          onChange={vlens.cachePartial(toggle, state, rk)}
                        />
                        <span className="date-review-thumb">
                          {record.photoId > 0 && (
                            <img src={`/api/photo/${record.photoId}/thumb`} alt="" loading="lazy" />
                          )}
                        </span>
                        <span className="date-review-text">{record.text}</span>
                        <span className="date-review-change">
                          {included ? `${formatLongDate(cluster.day)} → ${after}` : t.keep}
                        </span>
                      </label>
                    </li>
                  );
                })}
              </ul>
              <div className="date-review-actions" role="radiogroup" aria-label={t.meaning}>
                <label>
                  <input
                    type="radio"
                    name={`meaning-${key}`}
                    checked={meaning === "period"}
                    onChange={vlens.cachePartial(choose, state, key, "period")}
                  />{" "}
                  {t.describes(cluster.age)}
                </label>
                <label>
                  <input
                    type="radio"
                    name={`meaning-${key}`}
                    checked={meaning === "occurrence"}
                    onChange={vlens.cachePartial(choose, state, key, "occurrence")}
                  />{" "}
                  {t.happened(cluster.age)}
                </label>
                <button
                  type="button"
                  className="btn btn-primary"
                  disabled={state.busy || chosen === 0}
                  onClick={vlens.cachePartial(apply, state, cluster)}
                >
                  {t.apply(chosen)}
                </button>
              </div>
            </section>
          );
        })}
      </main>
      <Footer />
    </div>
  );
}
