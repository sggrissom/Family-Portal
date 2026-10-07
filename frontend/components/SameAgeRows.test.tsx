import { describe, expect, it, vi } from "vitest";
import type { VNode } from "preact";
import type { SameAgeRow } from "../server";
import { SameAgeRows } from "./SameAgeRows";

vi.mock("vlens/css", () => ({ block: () => {} }));

const emptyRow = (id: number, name: string): SameAgeRow =>
  ({
    person: {
      id,
      name,
      birthday: "2020-06-15T00:00:00Z",
      familyId: 1,
      gender: 0,
      age: "",
      profilePhotoId: 0,
      profileCropX: 0,
      profileCropY: 0,
      profileCropScale: 1,
      isPregnancy: false,
      relationship: "",
    },
    date: "2024-01-15T00:00:00Z",
    height: null,
    weight: null,
    photoIds: [],
    milestones: [],
    portraits: [],
  }) as SameAgeRow;

describe("SameAgeRows", () => {
  it("keeps names unambiguous when people without records are hidden", () => {
    const child = { ...emptyRow(1, "UI Child"), photoIds: [7] };
    const parent = emptyRow(2, "UI Runner");
    const view = SameAgeRows({
      rows: [child, parent],
      ageMonths: 43,
      today: "2024-01-15",
      hideEmpty: true,
    });
    const rows = view.props.children as VNode<any>[];
    expect(rows).toHaveLength(1);
    const header = rows[0].props.children[0];
    const name = header.props.children[0];
    expect(name.props.href).toBe("/profile/1");
    expect(name.props.children).toBe("UI Child");
  });

  it("renders no person cards when none have records", () => {
    const view = SameAgeRows({
      rows: [emptyRow(1, "Ann"), emptyRow(2, "Ben")],
      ageMonths: 43,
      today: "2024-01-15",
      hideEmpty: true,
    });
    expect(view.props.children).toEqual([]);
  });
});
