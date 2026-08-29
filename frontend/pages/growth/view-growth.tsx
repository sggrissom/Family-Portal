import * as preact from "preact";
import * as rpc from "vlens/rpc";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView } from "../../lib/authHelpers";
import { getIdFromRoute } from "../../lib/routeHelpers";
import { formatDate } from "../../lib/dateUtils";
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
import "./view-growth-styles";

type ViewGrowthData = {
  growthData: server.GrowthData | null;
  targetPerson: server.Person | null;
  familyMembers: server.FamilyTimelineItem[];
  relationGroups: Map<number, string>;
};

export async function fetch(route: string, prefix: string): Promise<rpc.Response<ViewGrowthData>> {
  const growthId = getIdFromRoute(route) || 0;
  const [growthResp, growthErr] = await server.GetGrowthData({ id: growthId });
  if (growthErr) return [null, growthErr];

  const [timelineResp, timelineErr] = await server.GetFamilyTimeline({});
  if (timelineErr) return [null, timelineErr];

  const growthData = growthResp?.growthData ?? null;
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

  return [{ growthData, targetPerson, familyMembers, relationGroups }, ""];
}

export function view(route: string, prefix: string, data: ViewGrowthData): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth) {
    return;
  }

  if (!data.growthData || !data.targetPerson) {
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
        <ViewGrowthPage
          growthData={data.growthData}
          person={data.targetPerson}
          familyMembers={data.familyMembers}
          relationGroups={data.relationGroups}
        />
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
    <div className="view-growth-page">
      <div className="view-growth-header">
        <a href={`/profile/${person.id}`} className="back-link">
          ← Back to {person.name}'s Profile
        </a>
      </div>

      <div className="growth-detail-card">
        <div className="growth-detail-icon">
          {growthData.measurementType === server.Height ? "📏" : "⚖️"}
        </div>
        <div className="growth-detail-main">
          <div className="growth-detail-type">
            {getMeasurementTypeLabel(growthData.measurementType)}
          </div>
          <div className="growth-detail-value">
            {growthData.value} {growthData.unit}
          </div>
          <div className="growth-detail-meta">
            <span>{person.name}</span>
            {ageLabel && <span>• Age {ageLabel}</span>}
            <span>• {formatDate(growthData.measurementDate)}</span>
            {percentileLabel && <span className="percentile-badge">{percentileLabel}</span>}
          </div>
        </div>
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
      </div>

      {!hasBirthday ? (
        <div className="empty-state">
          <p>Add a birthday for {person.name} to see how they compare to the rest of the family.</p>
        </div>
      ) : (
        <div className="family-comparison">
          <h2>Compared to Family</h2>
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
    </div>
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
                      <strong>
                        {p.value} {p.unit}
                      </strong>{" "}
                      — {describeValueComparison(p, measurementType)}
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
                      <strong>
                        {p.value} {p.unit}
                      </strong>{" "}
                      at <strong>{p.ageLabel}</strong> old (on {formatDate(p.date)}) —{" "}
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
