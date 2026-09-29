export interface PhotoFilterCriteria {
  selectedPeopleIds: number[];
  selectedTagIds: number[];
  dateFrom: string;
  dateTo: string;
  placeKey: string;
  showSimilar: boolean;
  query: string;
}

const DATE_PATTERN = /^\d{4}-\d{2}-\d{2}$/;
const PLACE_PATTERN = /^[cf]\d+$/;

const parseIds = (value: string | null): number[] =>
  (value ?? "")
    .split(",")
    .map(Number)
    .filter(id => Number.isInteger(id) && id > 0);

const parseDate = (value: string | null): string =>
  value && DATE_PATTERN.test(value) ? value : "";

export const parseFilterQuery = (search: string): PhotoFilterCriteria => {
  const params = new URLSearchParams(search);
  return {
    selectedPeopleIds: parseIds(params.get("people")),
    selectedTagIds: parseIds(params.get("tags")),
    dateFrom: parseDate(params.get("from")),
    dateTo: parseDate(params.get("to")),
    placeKey: PLACE_PATTERN.test(params.get("place") ?? "") ? params.get("place")! : "",
    showSimilar: params.get("similar") === "all",
    query: (params.get("q") ?? "").trim().slice(0, 200),
  };
};

export const filterQuery = (criteria: PhotoFilterCriteria): string => {
  const params = new URLSearchParams();
  if (criteria.selectedPeopleIds.length > 0) {
    params.set("people", criteria.selectedPeopleIds.join(","));
  }
  if (criteria.selectedTagIds.length > 0) {
    params.set("tags", criteria.selectedTagIds.join(","));
  }
  if (criteria.dateFrom) params.set("from", criteria.dateFrom);
  if (criteria.dateTo) params.set("to", criteria.dateTo);
  if (criteria.placeKey) params.set("place", criteria.placeKey);
  if (criteria.showSimilar) params.set("similar", "all");
  if (criteria.query) params.set("q", criteria.query);
  const query = params.toString().replace(/%2C/g, ",");
  return query ? `?${query}` : "";
};

// The ListFamilyPhotos fields for a set of criteria. Key order is fixed so the
// same criteria always serialize the same way.
export const serverFilters = (criteria: PhotoFilterCriteria) => ({
  personIds: criteria.selectedPeopleIds,
  tagIds: criteria.selectedTagIds,
  dateFrom: criteria.dateFrom,
  dateTo: criteria.dateTo,
  placeKey: criteria.placeKey,
  collapseSimilar: !criteria.showSimilar,
  query: criteria.query,
});
