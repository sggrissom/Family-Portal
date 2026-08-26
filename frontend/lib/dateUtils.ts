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

  let years = targetDate.getFullYear() - birthday.getFullYear();
  let months = targetDate.getMonth() - birthday.getMonth();

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
