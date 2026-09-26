import { describe, expect, it } from "vitest";
import { filterQuery, parseFilterQuery } from "./photoFilterQuery";

describe("photo filter query", () => {
  it("is empty with no filters", () => {
    expect(filterQuery(parseFilterQuery(""))).toBe("");
  });

  it("round-trips every criterion", () => {
    const criteria = {
      selectedPeopleIds: [3, 12],
      selectedTagIds: [5],
      dateFrom: "2024-01-01",
      dateTo: "2024-12-31",
    };
    const query = filterQuery(criteria);
    expect(query).toBe("?people=3,12&tags=5&from=2024-01-01&to=2024-12-31");
    expect(parseFilterQuery(query)).toEqual(criteria);
  });

  it("drops values that aren't ids or dates", () => {
    expect(parseFilterQuery("?people=3,abc,-1,0&tags=&from=yesterday")).toEqual({
      selectedPeopleIds: [3],
      selectedTagIds: [],
      dateFrom: "",
      dateTo: "",
    });
  });
});
