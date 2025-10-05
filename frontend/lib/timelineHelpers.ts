import * as server from "../server";

/**
 * Extract numeric age in years from an age string
 * Examples: "2 years 3 months" -> 2, "5 months" -> 0, "Newborn" -> 0
 */
export function getAgeInYears(ageString: string): number {
  if (!ageString || ageString === "Newborn") return 0;

  // Extract year number from strings like "2 years 3 months" or "1 year"
  const yearMatch = ageString.match(/(\d+)\s+years?/);
  if (yearMatch) {
    return parseInt(yearMatch[1]);
  }

  // If it's only months (e.g., "5 months"), return 0
  if (ageString.includes("month")) {
    return 0;
  }

  return 0;
}

/**
 * Delete a milestone with user confirmation
 * Reloads the page on success
 */
export async function handleDeleteMilestone(id: number, description: string): Promise<void> {
  const confirmed = confirm(`Are you sure you want to delete this milestone: "${description}"?`);

  if (confirmed) {
    try {
      let [resp, err] = await server.DeleteMilestone({ id });

      if (resp && resp.success) {
        // Refresh the page to update the milestone data
        window.location.reload();
      } else {
        alert(err || "Failed to delete milestone");
      }
    } catch (error) {
      alert("Network error. Please try again.");
    }
  }
}

/**
 * Delete a growth data measurement with user confirmation
 * Reloads the page on success
 */
export async function handleDeleteGrowthData(
  id: number,
  type: server.MeasurementType,
  value: number,
  unit: string
): Promise<void> {
  const typeLabel = type === server.Height ? "Height" : "Weight";
  const confirmed = confirm(
    `Are you sure you want to delete this ${typeLabel.toLowerCase()} measurement of ${value} ${unit}?`
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
