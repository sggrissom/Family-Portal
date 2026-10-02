import * as preact from "preact";
import * as rpc from "vlens/rpc";
import * as server from "../../server";
import { timelineRequest } from "../../lib/photoPages";
import { Header, Footer } from "../../layout";
import { requireAuthInView } from "../../lib/authHelpers";
import { canContribute } from "../../lib/authCache";
import { formatDate } from "../../lib/dateUtils";
import { formatMeasurement } from "../../lib/weightFormat";
import { ErrorPage } from "../../components/ErrorPage";
import { handleDeleteGrowthData } from "../../lib/timelineHelpers";
import {
  ageInMonths,
  computePercentileLabel,
  formatAgeAtMeasurement,
  isValidBirthday,
} from "../../lib/growthPercentiles";
import {
  computeFamilyComparisons,
  describeAgeComparison,
  describeValueComparison,
  FamilyComparisonEntry,
  ComparisonPoint,
} from "../../lib/growthComparison";
import { sameAgePath } from "../../lib/sameAge";
import { copy } from "../../lib/copy";
import "./view-growth-styles";

type ViewGrowthData = {
  measurements: server.GrowthData[];
  targetPerson: server.Person | null;
  familyMembers: server.FamilyTimelineItem[];
  relationGroups: Map<number, string>;
};

export async function fetch(route: string, prefix: string): Promise<rpc.Response<ViewGrowthData>> {
  const ids = growthIdsFromRoute(route);
  const measurements: server.GrowthData[] = [];
  for (const id of ids.length ? ids : [0]) {
    const [growthResp, growthErr] = await server.GetGrowthData({ id });
    if (!growthResp) return [null, growthErr];
    measurements.push(growthResp.growthData);
  }

  const [timelineResp, timelineErr] = await server.GetFamilyTimeline(
    timelineRequest({ skipMilestones: true, skipPhotos: true })
  );
  if (timelineErr) return [null, timelineErr];

  const growthData = measurements[0] ?? null;
  const familyMembers = timelineResp?.people ?? [];
  const targetPerson =
    familyMembers.find(item => item.person.id === growthData?.personId)?.person ?? null;

  const relationGroups = new Map<number, string>();
  if (targetPerson) {
    const [labelResp] = await server.GetRelationLabels({ subjectId: targetPerson.id });
    for (const entry of labelResp?.labels ?? []) {
      relationGroups.set(entry.personId, entry.group);
    }
  }

  return [{ measurements, targetPerson, familyMembers, relationGroups }, ""];
}

function growthIdsFromRoute(route: string): number[] {
  const segment = route.split("?")[0].split("/")[2] ?? "";
  return segment
    .split(",")
    .map(part => parseInt(part))
    .filter(id => !isNaN(id));
}

export function view(route: string, prefix: string, data: ViewGrowthData): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth) {
    return;
  }

  const person = data.targetPerson;
  if (data.measurements.length === 0 || !person) {
    return (
      <ErrorPage
        title="Measurement Not Found"
        message="The measurement you're looking for could not be found"
        containerClass="view-growth-container"
      />
    );
  }

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="view-growth-container">
        <div className="view-growth-page">
          <div className="view-growth-header">
            <a href={`/profile/${person.id}`} className="back-link">
              ← Back to {person.name}'s Profile
            </a>
          </div>
          {data.measurements
            .filter(growthData => growthData.personId === person.id)
            .map(growthData => (
              <ViewGrowthPage
                key={growthData.id}
                growthData={growthData}
                person={person}
                familyMembers={data.familyMembers}
                relationGroups={data.relationGroups}
              />
            ))}
        </div>
      </main>
      <Footer />
    </div>
  );
}

interface ViewGrowthPageProps {
  growthData: server.GrowthData;
  person: server.Person;
  familyMembers: server.FamilyTimelineItem[];
  relationGroups: Map<number, string>;
}

const getMeasurementTypeLabel = (type: server.MeasurementType) =>
  type === server.Height ? "Height" : "Weight";

const ViewGrowthPage = ({
  growthData,
  person,
  familyMembers,
  relationGroups,
}: ViewGrowthPageProps) => {
  const hasBirthday = isValidBirthday(person.birthday);
  const ageMonths = hasBirthday ? ageInMonths(person.birthday, growthData.measurementDate) : null;
  const ageLabel = ageMonths !== null ? formatAgeAtMeasurement(ageMonths) : null;
  const percentileLabel =
    ageMonths !== null && ageMonths <= 240
      ? computePercentileLabel(
          growthData.value,
          growthData.unit,
          ageMonths,
          person.gender,
          growthData.measurementType === server.Height ? "height" : "weight"
        )
      : null;

  const comparisons = computeFamilyComparisons(
    growthData,
    person,
    familyMembers.map(item => ({ person: item.person, growthData: item.growthData }))
  );

  const { siblings, parents, others } = splitComparisonsByRelation(comparisons, relationGroups);

  return (
    <section className="view-growth-measurement">
      <div className="growth-detail-card">
        <div className="growth-detail-icon">
          {growthData.measurementType === server.Height ? "📏" : "⚖️"}
        </div>
        <div className="growth-detail-main">
          <div className="growth-detail-type">
            {getMeasurementTypeLabel(growthData.measurementType)}
          </div>
          <div className="growth-detail-value">
            {formatMeasurement(growthData.value, growthData.unit, ageMonths)}
          </div>
          <div className="growth-detail-meta">
            <span>{person.name}</span>
            {ageLabel && <span>• Age {ageLabel}</span>}
            <span>• {formatDate(growthData.measurementDate)}</span>
            {percentileLabel && <span className="percentile-badge">{percentileLabel}</span>}
          </div>
        </div>
        {canContribute(growthData.familyId) && (
          <div className="growth-detail-actions">
            <a href={`/edit-growth/${growthData.id}`} className="btn btn-secondary">
              ✏️ Edit
            </a>
            <button
              className="btn btn-danger"
              onClick={() =>
                handleDeleteGrowthData(
                  growthData.id,
                  growthData.measurementType,
                  growthData.value,
                  growthData.unit
                )
              }
            >
              🗑️ Delete
            </button>
          </div>
        )}
      </div>

      {!hasBirthday ? (
        <div className="empty-state">
          <p>Add a birthday for {person.name} to see how they compare to the rest of the family.</p>
        </div>
      ) : (
        <div className="family-comparison">
          <div className="family-comparison-head">
            <h2>Compared to Family</h2>
            {ageMonths !== null && ageMonths >= 0 && (
              <a href={sameAgePath(Math.floor(ageMonths), person.id)}>{copy.sameAge.seeAll}</a>
            )}
          </div>
          {comparisons.length === 0 ? (
            <div className="empty-state">
              <p>No other family members have a birthday set yet.</p>
            </div>
          ) : (
            <>
              {siblings.length > 0 && (
                <ComparisonGroup
                  title="Siblings"
                  entries={siblings}
                  measurementType={growthData.measurementType}
                />
              )}
              {parents.length > 0 && (
                <ComparisonGroup
                  title="Parents"
                  entries={parents}
                  measurementType={growthData.measurementType}
                />
              )}
              {others.length > 0 && (
                <ComparisonGroup
                  title="Rest of the family"
                  entries={others}
                  measurementType={growthData.measurementType}
                />
              )}
            </>
          )}
        </div>
      )}
    </section>
  );
};

function splitComparisonsByRelation(entries: FamilyComparisonEntry[], groups: Map<number, string>) {
  return {
    siblings: entries.filter(e => groups.get(e.person.id) === "sibling"),
    parents: entries.filter(e => groups.get(e.person.id) === "parent"),
    others: entries.filter(e => {
      const group = groups.get(e.person.id);
      return group !== "sibling" && group !== "parent";
    }),
  };
}

interface ComparisonGroupProps {
  title: string;
  entries: FamilyComparisonEntry[];
  measurementType: server.MeasurementType;
}

const ComparisonGroup = ({ title, entries, measurementType }: ComparisonGroupProps) => {
  const typeLabel = getMeasurementTypeLabel(measurementType).toLowerCase();
  return (
    <div className="comparison-group">
      <h3>{title}</h3>
      <div className="comparison-cards">
        {entries.map(entry => (
          <div key={entry.person.id} className="comparison-card">
            <a href={`/profile/${entry.person.id}`} className="comparison-person-name">
              {entry.person.name}
            </a>
            {!entry.atSameAge && !entry.atSameValue ? (
              <p className="comparison-empty">No {typeLabel} data recorded yet.</p>
            ) : (
              <div className="comparison-rows">
                <ComparisonRow
                  icon="📅"
                  label="At the same age"
                  point={entry.atSameAge}
                  renderText={p => (
                    <>
                      At <strong>{p.ageLabel}</strong> old, {entry.person.name} measured{" "}
                      <strong>{formatMeasurement(p.value, p.unit, p.ageMonths)}</strong> —{" "}
                      {describeValueComparison(p, measurementType)}
                    </>
                  )}
                />
                <ComparisonRow
                  icon="🎯"
                  label="Reached this measurement"
                  point={entry.atSameValue}
                  renderText={p => (
                    <>
                      {entry.person.name} reached{" "}
                      <strong>{formatMeasurement(p.value, p.unit, p.ageMonths)}</strong> at{" "}
                      <strong>{p.ageLabel}</strong> old (on {formatDate(p.date)}) —{" "}
                      {describeAgeComparison(p)}
                    </>
                  )}
                />
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  );
};

interface ComparisonRowProps {
  icon: string;
  label: string;
  point: ComparisonPoint | null;
  renderText: (point: ComparisonPoint) => preact.ComponentChild;
}

const ComparisonRow = ({ icon, point, renderText }: ComparisonRowProps) => {
  if (!point) return null;
  return (
    <div className="comparison-row">
      <span className="comparison-row-icon">{icon}</span>
      <span className="comparison-row-text">{renderText(point)}</span>
    </div>
  );
};
