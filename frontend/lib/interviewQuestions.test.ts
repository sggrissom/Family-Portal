import { describe, expect, it } from "vitest";
import * as server from "@app/server";
import {
  activeIn,
  addSection,
  draftFrom,
  moveBefore,
  moveStep,
  moveToSection,
  questionsForSave,
  renameSection,
  resetToDefaults,
  retired,
} from "./interviewQuestions";

const q = (
  id: number,
  text: string,
  section: string,
  extra: Partial<server.InterviewQuestion> = {}
) => ({ id, text, section, defaultKey: "", archived: false, ...extra }) as server.InterviewQuestion;

const sample = () =>
  draftFrom([
    q(1, "Color", "Favorites"),
    q(2, "Food", "Favorites"),
    q(3, "Snack", "Favorites", { archived: true }),
    q(4, "Best friend", "People"),
    q(5, "Teacher", "People"),
  ]);

const texts = (rows: { text: string }[]) => rows.map(r => r.text);
const keyOf = (draft: ReturnType<typeof sample>, text: string) =>
  draft.rows.find(r => r.text === text)!.key;

describe("question draft", () => {
  it("groups by section in first-seen order and keeps retired questions apart", () => {
    const draft = sample();
    expect(draft.sections).toEqual(["Favorites", "People"]);
    expect(texts(activeIn(draft, "Favorites"))).toEqual(["Color", "Food"]);
    expect(texts(retired(draft))).toEqual(["Snack"]);
  });

  it("steps within a section, then across the section edge", () => {
    const draft = sample();
    moveStep(draft, keyOf(draft, "Food"), -1);
    expect(texts(activeIn(draft, "Favorites"))).toEqual(["Food", "Color"]);

    moveStep(draft, keyOf(draft, "Color"), 1);
    expect(texts(activeIn(draft, "Favorites"))).toEqual(["Food"]);
    expect(texts(activeIn(draft, "People"))).toEqual(["Color", "Best friend", "Teacher"]);

    moveStep(draft, keyOf(draft, "Color"), -1);
    expect(texts(activeIn(draft, "Favorites"))).toEqual(["Food", "Color"]);

    moveStep(draft, keyOf(draft, "Teacher"), 1);
    expect(texts(activeIn(draft, "People"))).toEqual(["Best friend", "Teacher"]);
  });

  it("drops a dragged question before its target, taking the target's section", () => {
    const draft = sample();
    moveBefore(draft, keyOf(draft, "Teacher"), keyOf(draft, "Food"));
    expect(texts(activeIn(draft, "Favorites"))).toEqual(["Color", "Teacher", "Food"]);
    expect(texts(activeIn(draft, "People"))).toEqual(["Best friend"]);
  });

  it("moves a question to the end of another section, even an empty one", () => {
    const draft = sample();
    addSection(draft, "Just for fun");
    moveToSection(draft, keyOf(draft, "Color"), "Just for fun");
    moveToSection(draft, keyOf(draft, "Food"), "People");
    expect(texts(activeIn(draft, "People"))).toEqual(["Best friend", "Teacher", "Food"]);
    expect(texts(activeIn(draft, "Just for fun"))).toEqual(["Color"]);
    expect(texts(questionsForSave(draft))).toEqual([
      "Snack",
      "Best friend",
      "Teacher",
      "Food",
      "Color",
    ]);
  });

  it("renames a section, merging into one that already has the name", () => {
    const draft = sample();
    renameSection(draft, "People", "Friends");
    expect(draft.sections).toEqual(["Favorites", "Friends"]);
    renameSection(draft, "Friends", "Favorites");
    expect(draft.sections).toEqual(["Favorites"]);
    expect(texts(activeIn(draft, "Favorites"))).toEqual([
      "Color",
      "Food",
      "Best friend",
      "Teacher",
    ]);
  });

  it("refuses a blank or duplicate section name", () => {
    const draft = sample();
    expect(addSection(draft, "  ")).toBe(false);
    expect(addSection(draft, "People")).toBe(false);
    expect(addSection(draft, "This year")).toBe(true);
  });

  it("resets by restoring retired built-ins and adding missing ones, keeping family questions", () => {
    const draft = draftFrom([
      q(1, "Color", "Favorites", { defaultKey: "fav-color", archived: true }),
      q(2, "Dinosaur", "Favorites", { archived: true }),
    ]);
    resetToDefaults(draft, [
      q(0, "Color", "Favorites", { defaultKey: "fav-color" }),
      q(0, "Food", "Favorites", { defaultKey: "fav-food" }),
      q(0, "Best friend", "People", { defaultKey: "best-friend" }),
    ]);
    expect(texts(activeIn(draft, "Favorites"))).toEqual(["Color", "Food"]);
    expect(texts(activeIn(draft, "People"))).toEqual(["Best friend"]);
    expect(texts(retired(draft))).toEqual(["Dinosaur"]);
  });

  it("saves in section order with every field", () => {
    const draft = sample();
    draft.sections.reverse();
    const saved = questionsForSave(draft);
    expect(texts(saved)).toEqual(["Best friend", "Teacher", "Color", "Food", "Snack"]);
    expect(saved[4]).toEqual({
      id: 3,
      text: "Snack",
      section: "Favorites",
      minAgeYears: 0,
      maxAgeYears: 0,
      defaultKey: "",
      archived: true,
    });
  });
});
