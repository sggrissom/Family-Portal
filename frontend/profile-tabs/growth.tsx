import * as preact from "preact";
import * as server from "../server";
import { GrowthChart } from "../chart";

interface GrowthTabProps {
  person: server.Person;
  growthData: server.GrowthData[];
}

const formatDate = (dateString: string) => {
  if (!dateString) return '';
  if (dateString.includes('T') && dateString.endsWith('Z')) {
    const dateParts = dateString.split('T')[0].split('-');
    const year = parseInt(dateParts[0]);
    const month = parseInt(dateParts[1]) - 1; // Month is 0-indexed
    const day = parseInt(dateParts[2]);
    return new Date(year, month, day).toLocaleDateString();
  }
  return new Date(dateString).toLocaleDateString();
};

export const GrowthTab = ({ person, growthData }: GrowthTabProps) => {
  const getMeasurementTypeLabel = (type: server.MeasurementType) => {
    return type === server.Height ? 'Height' : 'Weight';
  };

  // Sort growth data by measurement date (newest first) for table
  const sortedGrowthData = (growthData || []).slice().sort((a, b) =>
    new Date(b.measurementDate).getTime() - new Date(a.measurementDate).getTime()
  );

  return (
    <div className="growth-tab">
      <h2>Growth Tracking for {person.name}</h2>
      <div className="growth-content">
        <div className="growth-chart-container">
          <h3>Growth Chart</h3>
          <GrowthChart growthData={growthData} />
        </div>

        <div className="growth-table">
          <h3>Growth Records</h3>
          {sortedGrowthData.length === 0 ? (
            <div className="empty-state">
              <p>No growth records yet.</p>
              <a href={`/add-growth/${person.id}`} className="btn btn-primary">Add First Measurement</a>
            </div>
          ) : (
            <div className="growth-records">
              <table className="data-table">
                <thead>
                  <tr>
                    <th>Type</th>
                    <th>Value</th>
                    <th>Date</th>
                    <th>Added</th>
                  </tr>
                </thead>
                <tbody>
                  {sortedGrowthData.map(record => (
                    <tr key={record.id}>
                      <td>{getMeasurementTypeLabel(record.measurementType)}</td>
                      <td>{record.value} {record.unit}</td>
                      <td>{formatDate(record.measurementDate)}</td>
                      <td>{formatDate(record.createdAt)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
              <div className="table-actions">
                <a href={`/add-growth/${person.id}`} className="btn btn-primary">Add New Measurement</a>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};