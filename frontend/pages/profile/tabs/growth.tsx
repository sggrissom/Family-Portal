import * as preact from "preact";
import * as server from "../../../server";
import { GrowthChart } from "../../../components/chart/chart";
import {
  ageInMonths,
  computePercentileLabel,
  formatAgeAtMeasurement,
  isValidBirthday,
} from "../../../lib/growthPercentiles";

interface GrowthTabProps {
  person: server.Person;
  growthData: server.GrowthData[];
}

const formatDate = (dateString: string) => {
  if (!dateString) return "";
  if (dateString.includes("T") && dateString.endsWith("Z")) {
    const dateParts = dateString.split("T")[0].split("-");
    const year = parseInt(dateParts[0]);
    const month = parseInt(dateParts[1]) - 1; // Month is 0-indexed
    const day = parseInt(dateParts[2]);
    return new Date(year, month, day).toLocaleDateString();
  }
  return new Date(dateString).toLocaleDateString();
};

const handleDeleteGrowthData = async (
  id: number,
  type: server.MeasurementType,
  value: number,
  unit: string
) => {
  const typeLabel = type === server.Height ? "Height" : "Weight";
  const confirmed = confirm(
    `Are you sure you want to delete this ${typeLabel.toLowerCase()} measurement of ${value} ${unit}?`
  );

  if (confirmed) {
    try {
      let [resp, err] = await server.DeleteGrowthData({ id });

      if (resp && resp.success) {
        // Refresh the page to update the growth data
        window.location.reload();
      } else {
        alert(err || "Failed to delete growth measurement");
      }
    } catch (error) {
      alert("Network error. Please try again.");
    }
  }
};

export const GrowthTab = ({ person, growthData }: GrowthTabProps) => {
  const getMeasurementTypeLabel = (type: server.MeasurementType) => {
    return type === server.Height ? "Height" : "Weight";
  };

  // Sort growth data by measurement date (newest first) for table
  const sortedGrowthData = (growthData || [])
    .slice()
    .sort((a, b) => new Date(b.measurementDate).getTime() - new Date(a.measurementDate).getTime());

  const hasBirthday = isValidBirthday(person.birthday);

  const getAgeLabel = (record: server.GrowthData): string => {
    if (!hasBirthday) return "—";
    const months = ageInMonths(person.birthday, record.measurementDate);
    return formatAgeAtMeasurement(months);
  };

  const getPercentile = (record: server.GrowthData): string | null => {
    if (!hasBirthday) return null;
    const months = ageInMonths(person.birthday, record.measurementDate);
    if (months > 240) return null;
    const type = record.measurementType === server.Height ? "height" : "weight";
    return computePercentileLabel(record.value, record.unit, months, person.gender, type);
  };

  return (
    <div className="growth-tab">
      <h2>Growth Tracking for {person.name}</h2>
      <div className="growth-content">
        <div className="growth-chart-container">
          <h3>Growth Chart</h3>
          <GrowthChart growthData={growthData} birthday={person.birthday} gender={person.gender} />
        </div>

        <div className="growth-table">
          <h3>Growth Records</h3>
          {sortedGrowthData.length === 0 ? (
            <div className="empty-state">
              <p>No growth records yet.</p>
              <a href={`/add-growth/${person.id}`} className="btn btn-primary">
                Add First Measurement
              </a>
            </div>
          ) : (
            <div className="growth-records">
              <table className="data-table">
                <thead>
                  <tr>
                    <th>Type</th>
                    <th>Value</th>
                    {hasBirthday && <th>Age</th>}
                    {hasBirthday && <th>Percentile</th>}
                    <th>Date</th>
                    <th>Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {sortedGrowthData.map(record => {
                    const pctLabel = getPercentile(record);
                    return (
                      <tr key={record.id}>
                        <td>{getMeasurementTypeLabel(record.measurementType)}</td>
                        <td>
                          {record.value} {record.unit}
                        </td>
                        {hasBirthday && <td>{getAgeLabel(record)}</td>}
                        {hasBirthday && (
                          <td>
                            {pctLabel ? <span className="percentile-badge">{pctLabel}</span> : "—"}
                          </td>
                        )}
                        <td>{formatDate(record.measurementDate)}</td>
                        <td>
                          <div className="table-actions">
                            <a
                              href={`/edit-growth/${record.id}`}
                              className="btn-action btn-edit"
                              title="Edit"
                            >
                              ✏️
                            </a>
                            <button
                              className="btn-action btn-delete"
                              title="Delete"
                              onClick={() =>
                                handleDeleteGrowthData(
                                  record.id,
                                  record.measurementType,
                                  record.value,
                                  record.unit
                                )
                              }
                            >
                              🗑️
                            </button>
                          </div>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
              <div className="table-actions">
                <a href={`/add-growth/${person.id}`} className="btn btn-primary">
                  Add New Measurement
                </a>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};
