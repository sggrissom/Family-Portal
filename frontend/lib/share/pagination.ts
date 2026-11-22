export interface Placement {
  group: number;
  item: number;
}

/** How a recap falls onto pages. Each group (the header, then each routine) starts a new page rather than split, unless it is taller than a page on its own; then its items run on across pages. */
export function paginate(groups: number[][], pageHeight: number, spacing: number): Placement[][] {
  const pages: Placement[][] = [];
  let current: Placement[] = [];
  let used = 0;

  const place = (placement: Placement, height: number) => {
    const needed = current.length === 0 ? height : used + spacing + height;
    if (current.length > 0 && needed > pageHeight) {
      pages.push(current);
      current = [placement];
      used = height;
    } else {
      current.push(placement);
      used = needed;
    }
  };

  groups.forEach((items, group) => {
    const total = items.reduce((sum, h) => sum + h, 0) + spacing * Math.max(items.length - 1, 0);
    if (current.length > 0 && total <= pageHeight && used + spacing + total > pageHeight) {
      pages.push(current);
      current = [];
      used = 0;
    }
    items.forEach((height, item) => place({ group, item }, height));
  });
  if (current.length > 0) pages.push(current);
  return pages;
}
