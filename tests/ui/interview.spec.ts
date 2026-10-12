import { test, expect, type Page } from "@playwright/test";

let pageErrors: string[] = [];
let clients = 0;

test.beforeEach(async ({ page }, testInfo) => {
  pageErrors = [];
  page.on("pageerror", error => pageErrors.push(error.message));
  clients++;
  await page.setExtraHTTPHeaders({
    "X-Forwarded-For": `10.${testInfo.workerIndex}.1.${clients}`,
  });
});

test.afterEach(() => {
  expect(pageErrors, "the page threw while the test ran").toEqual([]);
});

async function signUpWithChild(page: Page, childName: string): Promise<string> {
  await page.goto("/create-account");
  await page.getByLabel("Full Name").fill("Interview Parent");
  await page.getByLabel("Email Address").fill(`ui-interview-${Date.now()}@family-portal.invalid`);
  await page.getByLabel("Password", { exact: true }).fill("ui-portal-password");
  await page.getByLabel("Confirm Password").fill("ui-portal-password");
  await page.getByLabel("Birthday").fill("1988-02-01");
  await page.getByRole("button", { name: "Create Account" }).click();
  await expect(page).toHaveURL(/\/dashboard$/);

  await page.getByRole("link", { name: "Add family member" }).click();
  await expect(page.getByRole("heading", { name: "Add Family Member" })).toBeVisible();
  await page.locator("#name").fill(childName);
  await page.locator("#gender").selectOption("1");
  await page.locator("#birthdate").fill("2018-03-04");
  await page.getByRole("button", { name: "Add Family Member" }).click();
  await expect(page).toHaveURL(/\/dashboard$/);

  const card = page
    .getByRole("navigation", { name: "Family" })
    .getByRole("link", { name: childName });
  const href = await card.getAttribute("href");
  return href!.split("/").pop()!;
}

test("a parent runs an interview, resumes it after a reload, and finishes", async ({ page }) => {
  const personId = await signUpWithChild(page, "Ivy");
  const answer = page.locator(".iv-answer");
  const progress = page.locator(".iv-progress");

  await test.step("set up and start", async () => {
    await page.goto(`/interview/new/${personId}`);
    await expect(page.getByRole("heading", { name: "Interview Ivy" })).toBeVisible();
    await page.getByRole("button", { name: /^Start · \d+ questions$/ }).click();
    await expect(page).toHaveURL(/\/interview\/\d+\/ask$/);
    await expect(page.getByRole("heading", { name: "What's your favorite color?" })).toBeVisible();
    await answer.focus();
  });

  await test.step("answer two, skip one, and leave a draft", async () => {
    await answer.fill("purple");
    await answer.press("Control+Enter");
    await expect(page.getByRole("heading", { name: "What's your favorite food?" })).toBeVisible();
    await answer.fill("spaghetti");
    await answer.press("Control+Enter");
    await page.getByRole("button", { name: "Skip", exact: true }).click();
    await expect(progress).toHaveText(/^4 of \d+$/);
    await answer.fill("chocolate cake");
  });

  await test.step("a reload picks up where it left off, draft and all", async () => {
    await page.reload();
    await expect(progress).toHaveText(/^4 of \d+$/);
    await expect(answer).toHaveValue("chocolate cake");
  });

  await test.step("jump back and change an answer", async () => {
    await page.getByRole("button", { name: "All questions" }).click();
    await page.getByRole("button", { name: "What's your favorite color?" }).click();
    await expect(answer).toHaveValue("purple");
    await answer.fill("purple and pink");
  });

  await test.step("finish from the last question", async () => {
    await page.getByRole("button", { name: "All questions" }).click();
    await page.locator(".iv-jump-item").last().click();
    let release!: () => void;
    let started!: () => void;
    const held = new Promise<void>(resolve => {
      release = resolve;
    });
    const sending = new Promise<void>(resolve => {
      started = resolve;
    });
    await page.route("**/rpc/SaveInterviewAnswer", async route => {
      started();
      await held;
      await route.continue();
    });
    await answer.fill("a made-up word");
    await page.getByRole("button", { name: "Next", exact: true }).click();
    await sending;
    await expect(page.getByRole("heading", { name: "That's everything for Ivy" })).toBeVisible();
    await page.getByRole("button", { name: "Finish interview" }).click();
    // Regression: the old flow completed after five seconds with a write in flight.
    await page.waitForTimeout(5500);
    await expect(page).toHaveURL(/\/interview\/\d+\/ask$/);
    await expect(page.getByRole("button", { name: "Finishing…" })).toBeDisabled();
    release();
    await expect(page).toHaveURL(/\/interview\/\d+$/);
  });

  await test.step("the interview page shows what was said", async () => {
    await expect(page.getByRole("heading", { name: /^Ivy's interview at \d+$/ })).toBeVisible();
    const qa = page.locator(".iv-qa");
    await expect(qa.filter({ hasText: "favorite color" })).toContainText("purple and pink");
    await expect(qa.filter({ hasText: "favorite food" })).toContainText("spaghetti");
    await expect(qa.filter({ hasText: "favorite snack" })).toContainText("skipped");
    await expect(qa.filter({ hasText: "dessert" })).toContainText("chocolate cake");
  });
});
