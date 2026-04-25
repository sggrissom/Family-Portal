/**
 * Calculate age between two dates, formatted as a human-readable string
 * @param birthdayString - ISO date string of birthday
 * @param targetDateString - ISO date string of target date (e.g., milestone date)
 * @returns Formatted age string like "2 years 3 months" or "5 months" or "Newborn"
 */
export const calculateAge = (birthdayString: string, targetDateString: string): string => {
  if (!birthdayString || !targetDateString) return "";

  const birthday = new Date(birthdayString);
  const targetDate = new Date(targetDateString);

  // Treat future birthdates as due dates and show gestational weeks.
  if (targetDate < birthday) {
    const birthdayUtc = Date.UTC(birthday.getUTCFullYear(), birthday.getUTCMonth(), birthday.getUTCDate());
    const targetDateUtc = Date.UTC(
      targetDate.getUTCFullYear(),
      targetDate.getUTCMonth(),
      targetDate.getUTCDate()
    );
    const msPerDay = 1000 * 60 * 60 * 24;
    const daysUntilDue = Math.max(0, Math.floor((birthdayUtc - targetDateUtc) / msPerDay));
    const weeksPregnant = Math.max(0, 40 - Math.ceil(daysUntilDue / 7));
    return weeksPregnant === 1 ? "1 week" : `${weeksPregnant} weeks`;
  }

  // Calculate the difference
  let years = targetDate.getFullYear() - birthday.getFullYear();
  let months = targetDate.getMonth() - birthday.getMonth();

  // Adjust if target month is before birthday month
  if (months < 0) {
    years--;
    months += 12;
  }

  // Format the age string
  if (years === 0 && months === 0) {
    return "Newborn";
  } else if (years === 0) {
    return months === 1 ? "1 month" : `${months} months`;
  } else if (months === 0) {
    return years === 1 ? "1 year" : `${years} years`;
  } else {
    const yearStr = years === 1 ? "1 year" : `${years} years`;
    const monthStr = months === 1 ? "1 month" : `${months} months`;
    return `${yearStr} ${monthStr}`;
  }
};

/**
 * Format a date string for display
 * @param dateString - ISO date string
 * @returns Localized date string
 */
export const formatDate = (dateString: string): string => {
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
