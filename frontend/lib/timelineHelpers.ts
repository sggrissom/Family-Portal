import * as server from "../server";
import { formatMeasurement } from "./weightFormat";

export function getAgeInYears(ageString: string): number {
  if (!ageString || ageString === "Newborn") return 0;

  const yearMatch = ageString.match(/(\d+)\s+years?/);
  if (yearMatch) {
    return parseInt(yearMatch[1]);
  }

  if (ageString.includes("month")) {
    return 0;
  }

  return 0;
}

export async function handleDeleteMilestone(id: number, description: string): Promise<void> {
  const confirmed = confirm(`Are you sure you want to delete this moment: "${description}"?`);

  if (confirmed) {
    try {
      const [current] = await server.GetMilestone({ id });
      if (!current) {
        alert("Reload this page before deleting the moment.");
        return;
      }
      if (current.milestone.interviewId && current.milestone.description !== description) {
        alert("This answer changed. Reload it before deleting.");
        return;
      }
      let [resp, err] = await server.DeleteMilestone({
        id,
        interviewVersion: current.interviewVersion,
      });

      if (resp && resp.success) {
        window.location.reload();
      } else {
        alert(err || "Failed to delete moment");
      }
    } catch (error) {
      alert("Network error. Please try again.");
    }
  }
}

export async function handleDeleteGrowthData(
  id: number,
  type: server.MeasurementType,
  value: number,
  unit: string
): Promise<void> {
  const typeLabel = type === server.Height ? "Height" : "Weight";
  const confirmed = confirm(
    `Are you sure you want to delete this ${typeLabel.toLowerCase()} measurement of ${formatMeasurement(value, unit)}?`
  );

  if (confirmed) {
    try {
      let [resp, err] = await server.DeleteGrowthData({ id });

      if (resp && resp.success) {
        window.location.reload();
      } else {
        alert(err || "Failed to delete growth measurement");
      }
    } catch (error) {
      alert("Network error. Please try again.");
    }
  }
}
