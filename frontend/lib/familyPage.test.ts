import { describe, expect, it } from "vitest";
import type * as server from "../server";
import { familyPageSlug, maxPhotos, previewPage, toggleMember, togglePhoto } from "./familyPage";

const person = (
  id: number,
  name: string,
  extra: Partial<server.FamilyPagePerson> = {}
): server.FamilyPagePerson => ({
  id,
  name,
  ageYears: null,
  profilePhotoId: 0,
  profileCropX: 0,
  profileCropY: 0,
  profileCropScale: 0,
  ...extra,
});

const emptyForm = (): server.FamilyPageSettings => ({
  slug: "smiths",
  indexable: false,
  title: "The Smiths",
  intro: "",
  members: [],
  photoIds: [],
});

const people = [
  person(1, "Ada Smith", {
    ageYears: 7,
    profilePhotoId: 40,
    profileCropX: 30,
    profileCropY: 60,
    profileCropScale: 2,
  }),
  person(2, "Ben Smith", { ageYears: 4 }),
  person(3, "Cy"),
];

describe("familyPageSlug", () => {
  it("takes exactly one slug segment after /f/", () => {
    expect(familyPageSlug("/f/smiths")).toBe("smiths");
    expect(familyPageSlug("/f/The-Smiths?ref=x#top")).toBe("the-smiths");
    expect(familyPageSlug("/f/smiths/photo")).toBe("");
    expect(familyPageSlug("/f/")).toBe("");
    expect(familyPageSlug("/f/two--hyphens")).toBe("");
    expect(familyPageSlug("/recap/smiths")).toBe("");
  });
});

describe("toggleMember", () => {
  it("adds people with their first name and keeps the roster order", () => {
    const form = emptyForm();
    toggleMember(form, people, people[1]);
    toggleMember(form, people, people[0]);
    expect(form.members.map(m => [m.personId, m.name])).toEqual([
      [1, "Ada"],
      [2, "Ben"],
    ]);
    expect(form.members.every(m => !m.showAge && !m.showPhoto && m.line === "")).toBe(true);
  });

  it("removes someone already on the page and keeps the others' edits", () => {
    const form = emptyForm();
    toggleMember(form, people, people[0]);
    toggleMember(form, people, people[1]);
    form.members[1].line = "Builds forts";
    toggleMember(form, people, people[0]);
    expect(form.members).toEqual([
      { personId: 2, name: "Ben", line: "Builds forts", showAge: false, showPhoto: false },
    ]);
  });
});

describe("togglePhoto", () => {
  it("stops at the photo limit and toggles off", () => {
    const form = emptyForm();
    for (let id = 1; id <= maxPhotos + 2; id++) togglePhoto(form, id);
    expect(form.photoIds).toHaveLength(maxPhotos);
    togglePhoto(form, 1);
    expect(form.photoIds).not.toContain(1);
  });
});

describe("previewPage", () => {
  it("shows only the choices on the form", () => {
    const form = emptyForm();
    form.intro = "  Hi there  ";
    toggleMember(form, people, people[0]);
    toggleMember(form, people, people[1]);
    toggleMember(form, people, people[2]);
    form.members[0].showAge = true;
    form.members[0].showPhoto = true;
    form.members[1].showPhoto = true;
    form.members[2].name = "  ";
    form.photoIds = [9];

    expect(previewPage(form, people)).toEqual({
      title: "The Smiths",
      intro: "Hi there",
      members: [
        {
          name: "Ada",
          line: "",
          ageYears: 7,
          photoUrl: "/api/photo/40/medium",
          cropX: 30,
          cropY: 60,
          cropScale: 2,
        },
        { name: "Ben", line: "", ageYears: null, photoUrl: "", cropX: 0, cropY: 0, cropScale: 0 },
        { name: "Cy", line: "", ageYears: null, photoUrl: "", cropX: 0, cropY: 0, cropScale: 0 },
      ],
      photoUrls: ["/api/photo/9/large"],
    });
  });

  it("drops members who are no longer on the roster", () => {
    const form = emptyForm();
    toggleMember(form, people, people[0]);
    expect(previewPage(form, people.slice(1)).members).toEqual([]);
  });
});
