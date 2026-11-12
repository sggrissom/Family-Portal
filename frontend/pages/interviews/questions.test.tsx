import * as preact from "preact";
import { beforeEach, describe, expect, it, vi } from "vitest";
import * as server from "../../server";
import { FamilySelect } from "../../components/FamilySelect";
import { view } from "./questions";

const hooks = vi.hoisted(() => ({ state: null as Record<string, any> | null }));
vi.mock("vlens", () => ({
  declareHook: (create: () => Record<string, any>) => () => (hooks.state ??= create()),
  scheduleRedraw: vi.fn(),
  cachePartial: (callback: (...args: any[]) => any, ...args: any[]) => callback.bind(null, ...args),
}));
vi.mock("vlens/css", () => ({ block: vi.fn() }));
vi.mock("../../layout", () => ({ Header: () => null, Footer: () => null }));
vi.mock("../../components/FamilySelect", () => ({ FamilySelect: () => null }));
vi.mock("../../components/ErrorPage", () => ({ ViewOnlyPage: () => null }));
vi.mock("../../lib/authHelpers", () => ({ requireAuthInView: () => ({ id: 1 }) }));
vi.mock("../../lib/authCache", () => ({ canContribute: () => true }));
vi.mock("../../server", () => ({
  SetInterviewCadence: vi.fn(),
  SaveInterviewQuestions: vi.fn(),
  GetInterviewQuestions: vi.fn(),
}));

const data: server.InterviewQuestionsResponse = {
  familyId: 1,
  revision: 1,
  cadence: { kind: "birthday", month: 0 },
  questions: [],
  suggested: [],
  defaults: [],
  usage: {},
};

function find(
  node: preact.ComponentChild,
  matches: (node: preact.VNode<any>) => boolean
): preact.VNode<any> | undefined {
  if (Array.isArray(node)) {
    for (const child of node) {
      const found = find(child, matches);
      if (found) return found;
    }
  } else if (node && typeof node === "object" && "type" in node) {
    if (matches(node)) return node;
    if (typeof node.type === "function" && node.type.name === "Cadence") {
      return find((node.type as preact.FunctionComponent<any>)(node.props), matches);
    }
    return find(node.props.children, matches);
  }
}

function controls() {
  const tree = view("/interview-questions", "", structuredClone(data));
  return {
    cadence: find(tree, node => node.type === "select")!,
    save: find(tree, node => node.type === "button" && node.props.className === "btn btn-primary")!,
    family: find(tree, node => node.type === FamilySelect)!,
  };
}

beforeEach(() => {
  hooks.state = null;
  vi.resetAllMocks();
});

describe("interview editor saves", () => {
  it("blocks cadence changes until a question save has finished", async () => {
    controls();
    hooks.state!.dirty = true;
    let finish!: (response: [server.InterviewQuestionsResponse, string]) => void;
    vi.mocked(server.SaveInterviewQuestions).mockImplementationOnce(
      () => new Promise(resolve => (finish = resolve))
    );
    const saving = controls().save.props.onClick();
    const pending = controls();
    expect(pending.cadence.props.disabled).toBe(true);
    await pending.cadence.props.onChange({ currentTarget: { value: "school-start" } });
    expect(server.SetInterviewCadence).not.toHaveBeenCalled();
    finish([{ ...data, revision: 2 }, ""]);
    await saving;
    expect(controls().cadence.props.disabled).toBe(false);
  });

  it("blocks question saves and family switches until the cadence is saved", async () => {
    controls();
    hooks.state!.dirty = true;
    let finish!: (response: [server.SetInterviewCadenceResponse, string]) => void;
    vi.mocked(server.SetInterviewCadence).mockImplementationOnce(
      () => new Promise(resolve => (finish = resolve))
    );
    const saving = controls().cadence.props.onChange({ currentTarget: { value: "school-start" } });
    const pending = controls();
    expect(pending.save.props.disabled).toBe(true);
    expect(pending.family.props.disabled).toBe(true);
    await pending.save.props.onClick();
    await pending.family.props.onChange(2);
    expect(server.SaveInterviewQuestions).not.toHaveBeenCalled();
    expect(server.GetInterviewQuestions).not.toHaveBeenCalled();
    finish([{ cadence: { kind: "school-start", month: 8 } }, ""]);
    await saving;
    expect(controls().cadence.props.value).toBe("school-start");
    expect(controls().save.props.disabled).toBe(false);
  });

  it("blocks cadence changes while another family's questions are loading", async () => {
    let finish!: (response: [server.InterviewQuestionsResponse, string]) => void;
    vi.mocked(server.GetInterviewQuestions).mockImplementationOnce(
      () => new Promise(resolve => (finish = resolve))
    );
    const loading = controls().family.props.onChange(2);
    expect(controls().cadence.props.disabled).toBe(true);
    await controls().cadence.props.onChange({ currentTarget: { value: "off" } });
    expect(server.SetInterviewCadence).not.toHaveBeenCalled();
    finish([{ ...data, familyId: 2 }, ""]);
    await loading;
    expect(controls().cadence.props.disabled).toBe(false);
  });
});
