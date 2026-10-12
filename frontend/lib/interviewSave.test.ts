import { describe, it, expect } from "vitest";
// Vite loads the same JSON fixture used by the Go regression test.
// @ts-expect-error raw imports are provided by the test bundler
import fixtureText from "../../backend/testdata/interview-normalization.json?raw";
import {
  acknowledgeInterviewRequest,
  interviewRequestId,
  InterviewSaveQueue,
} from "./interviewSave";
import { normalizeAnswer } from "./interviews";

describe("interview writes", () => {
  it("reuses a create ID until acknowledged and then allows an identical new sitting", () => {
    const first = interviewRequestId("test-start", { personId: 3 });
    expect(interviewRequestId("test-start", { personId: 3 })).toBe(first);
    acknowledgeInterviewRequest("test-start");
    expect(interviewRequestId("test-start", { personId: 3 })).not.toBe(first);
  });
  it("completion waits for all acknowledgements and writes cannot race", async () => {
    const queue = new InterviewSaveQueue();
    const order: string[] = [];
    let acknowledge!: () => void;
    const gate = new Promise<void>(resolve => {
      acknowledge = resolve;
    });
    const answer = queue.run(async () => {
      order.push("answer sent");
      await gate;
      order.push("answer saved");
    });
    const skip = queue.run(async () => {
      order.push("skip saved");
    });
    let finished = false;
    const finish = queue.flush().then(() => {
      finished = true;
    });
    await Promise.resolve();
    await Promise.resolve();
    expect(order).toEqual(["answer sent"]);
    expect(finished).toBe(false);
    acknowledge();
    await Promise.all([answer, skip, finish]);
    expect(order).toEqual(["answer sent", "answer saved", "skip saved"]);
    expect(queue.pending).toBe(0);
  });
  it("a failed write does not strand the queue", async () => {
    const queue = new InterviewSaveQueue();
    await expect(
      queue.run(async () => {
        throw new Error("offline");
      })
    ).rejects.toThrow("offline");
    expect(await queue.run(async () => "retry")).toBe("retry");
    await queue.flush();
    expect(queue.pending).toBe(0);
  });
  it("uses the same comparison fixtures as Go", () => {
    const cases = JSON.parse(fixtureText) as { input: string; expected: string }[];
    for (const c of cases) expect(normalizeAnswer(c.input)).toBe(c.expected);
  });
});
