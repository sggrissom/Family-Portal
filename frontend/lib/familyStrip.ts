const MS_PER_DAY = 24 * 60 * 60 * 1000;

function utcDay(date: Date): number {
  return Date.UTC(date.getUTCFullYear(), date.getUTCMonth(), date.getUTCDate());
}

function localDay(date: Date): number {
  return Date.UTC(date.getFullYear(), date.getMonth(), date.getDate());
}

export function compactAge(birthday: string, today: Date): string {
  const born = new Date(birthday);
  if (isNaN(born.getTime())) return "";
  const days = Math.floor((localDay(today) - utcDay(born)) / MS_PER_DAY);
  if (days < 0) return "";

  let months =
    (today.getFullYear() - born.getUTCFullYear()) * 12 + today.getMonth() - born.getUTCMonth();
  if (today.getDate() < born.getUTCDate()) months--;

  if (months < 1) return days < 7 ? `${days}d` : `${Math.floor(days / 7)}w`;
  if (months < 24) return `${months}m`;
  const years = Math.floor(months / 12);
  const rest = months % 12;
  if (years >= 18 || rest === 0) return `${years}y`;
  return `${years}y ${rest}m`;
}

export function dueSummary(dueDate: string, today: Date): string {
  const due = new Date(dueDate);
  if (isNaN(due.getTime())) return "";
  const days = Math.round((utcDay(due) - localDay(today)) / MS_PER_DAY);

  if (days < 0) return `Due date passed ${-days} day${days === -1 ? "" : "s"} ago`;
  if (days === 0) return "Due today";
  const weeks = Math.floor(days / 7);
  if (weeks > 0) return days % 7 ? `Due in ${weeks}w ${days % 7}d` : `Due in ${weeks}w`;
  return `Due in ${days} day${days === 1 ? "" : "s"}`;
}
