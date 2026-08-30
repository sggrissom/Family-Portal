export const calculateAge = (birthdayString: string, targetDateString: string): string => {
  if (!birthdayString || !targetDateString) return "";

  const birthday = new Date(birthdayString);
  const targetDate = new Date(targetDateString);

  if (targetDate < birthday) {
    const birthdayUtc = Date.UTC(
      birthday.getUTCFullYear(),
      birthday.getUTCMonth(),
      birthday.getUTCDate()
    );
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

  // Read in UTC, as the pregnancy branch above does. A date-only or Z-suffixed
  // string parses to UTC midnight, so local getters shift it to the previous
  // day west of UTC and report an age up to a month out.
  let years = targetDate.getUTCFullYear() - birthday.getUTCFullYear();
  let months = targetDate.getUTCMonth() - birthday.getUTCMonth();

  // The month is only complete once the day of the month has come round again,
  // so borrow from it first. Someone born on the 20th is not a month older on
  // the 1st. A birthday later in the month than the target's last day (born the
  // 31st, viewed in February) simply lands on the 1st of the month after.
  if (targetDate.getUTCDate() < birthday.getUTCDate()) {
    months--;
  }

  if (months < 0) {
    years--;
    months += 12;
  }

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

export const formatDate = (dateString: string): string => {
  if (!dateString) return "";
  if (dateString.includes("T") && dateString.endsWith("Z")) {
    const dateParts = dateString.split("T")[0].split("-");
    const year = parseInt(dateParts[0]);
    const month = parseInt(dateParts[1]) - 1;
    const day = parseInt(dateParts[2]);
    return new Date(year, month, day).toLocaleDateString();
  }
  return new Date(dateString).toLocaleDateString();
};

export const isRealDate = (dateString: string | null | undefined): dateString is string => {
  if (!dateString) return false;
  const year = new Date(dateString).getFullYear();
  return !isNaN(year) && year > 1000;
};

export const toDateInputValue = (dateString: string | null | undefined): string => {
  if (!isRealDate(dateString)) return "";
  return dateString.split("T")[0];
};

export const formatDateRange = (startDate: string, endDate: string): string => {
  const start = isRealDate(startDate) ? formatDate(startDate) : "";
  const end = isRealDate(endDate) ? formatDate(endDate) : "";
  if (start && end && start !== end) return `${start} – ${end}`;
  return start || end;
};

export const formatDateTime = (dateString: string): string => {
  if (!dateString) return "";
  const date = new Date(dateString);
  return (
    date.toLocaleDateString() +
    " " +
    date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })
  );
};

export const formatRelativeTime = (timestamp: string, fallback?: string): string => {
  const then = new Date(timestamp).getTime();
  if (!Number.isFinite(then) || then <= 0) return fallback ?? timestamp;

  const seconds = Math.max(0, Math.round((Date.now() - then) / 1000));
  if (seconds < 60) return "just now";

  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;

  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;

  return `${Math.floor(hours / 24)}d ago`;
};
