import { test, expect } from "@playwright/test";

let pageErrors: string[] = [];
let clients = 0;

test.beforeEach(async ({ page }, testInfo) => {
  pageErrors = [];
  page.on("pageerror", error => pageErrors.push(error.message));
  clients++;
  await page.setExtraHTTPHeaders({
    "X-Forwarded-For": `10.${testInfo.workerIndex}.4.${clients}`,
  });
});

test.afterEach(() => {
  expect(pageErrors, "the page threw while the test ran").toEqual([]);
});

test("a recap link opens signed out, keeps its snapshot, and stops once turned off", async ({
  page,
  browser,
}) => {
  const routines = page.locator("section").filter({
    has: page.getByRole("heading", { name: "Routines", exact: true }),
  });

  await test.step("sign up and record a result at a competition", async () => {
    await page.goto("/create-account");
    await page.getByLabel("Full Name").fill("Recap Parent");
    await page.getByLabel("Email Address").fill(`ui-recap-${Date.now()}@family-portal.invalid`);
    await page.getByLabel("Password", { exact: true }).fill("ui-portal-password");
    await page.getByLabel("Confirm Password").fill("ui-portal-password");
    await page.getByLabel("Birthday").fill("1988-02-01");
    await page.getByRole("button", { name: "Create Account" }).click();
    await expect(page).toHaveURL(/\/dashboard$/);

    await page.goto("/activities");
    await page.getByRole("button", { name: "New program" }).click();
    await page.locator("#newActivityName").fill("Dance");
    await page.getByRole("button", { name: "Add program" }).click();
    await page.getByLabel("Season name").fill("2026");
    await page.getByRole("button", { name: "Add season" }).click();
    await page.getByRole("link", { name: "2026" }).click();
    await expect(page).toHaveURL(/\/season\/\d+$/);

    await page.getByRole("button", { name: "Add competition" }).click();
    await page.locator("#eventName").fill("Recap Classic");
    await page.locator("#eventStart").fill("2026-10-17");
    await page.getByRole("button", { name: "Add competition" }).click();
    await expect(page.getByRole("link", { name: "Recap Classic" })).toBeVisible();

    await page.getByRole("button", { name: "Add routine" }).click();
    await page.locator("#entryName").fill("Crew Battle");
    await page.getByRole("button", { name: "Add routine" }).click();
    await expect(routines.getByRole("link", { name: "Crew Battle", exact: true })).toBeVisible();

    await page.locator(".schedule").getByRole("link", { name: "Recap Classic" }).click();
    await expect(page.getByRole("heading", { name: "Recap Classic", level: 1 })).toBeVisible();

    await page.getByRole("button", { name: "Record results" }).click();
    await page.getByRole("button", { name: "+ Head-to-head" }).click();
    await page.getByLabel("Round").fill("Final");
    await page.getByLabel("Against").fill("Crew Z");
    await page.getByLabel("Outcome").selectOption({ label: "Won" });
    await page.getByRole("button", { name: "Save", exact: true }).click();
    await expect(
      page.locator(".matchup-line", { hasText: "Final · vs Crew Z · Won" })
    ).toBeVisible();
  });

  let recapPath = "";
  await test.step("publish a recap link from the share preview", async () => {
    await page.getByRole("button", { name: "Share" }).click();
    const dialog = page.getByRole("dialog");
    await expect(dialog.getByRole("heading", { name: "Recap link" })).toBeVisible();
    await dialog.getByLabel("Link works for").selectOption({ label: "7 days" });
    await dialog.getByRole("button", { name: "Create recap link" }).click();
    await expect(
      dialog.getByText("Publish 1 routine without names, working for 7 days?")
    ).toBeVisible();
    await dialog.getByRole("button", { name: "Publish link" }).click();

    const url = await dialog.getByRole("textbox", { name: "Recap link" }).inputValue();
    recapPath = new URL(url).pathname;
    expect(recapPath).toMatch(/^\/recap\/[0-9a-f]{32}$/);
    await expect(dialog.locator(".recap-link-item")).toHaveCount(1);
    await expect(dialog.locator(".recap-link-item")).toContainText("Created");
    await expect(dialog.locator(".recap-link-item")).toContainText("Expires");
  });

  const outsider = await browser.newContext({
    viewport: { width: 390, height: 844 },
    extraHTTPHeaders: { "X-Forwarded-For": `10.99.4.${clients}` },
  });
  const phone = await outsider.newPage();

  await test.step("someone without an account opens it on a phone", async () => {
    const response = await phone.goto(recapPath);
    expect(response?.status()).toBe(200);
    expect(response?.headers()["referrer-policy"]).toBe("no-referrer");
    await expect(phone.getByRole("heading", { name: "Recap Classic", level: 1 })).toBeVisible();
    await expect(phone.getByRole("heading", { name: "Crew Battle" })).toBeVisible();
    await expect(phone.getByText("Final · vs Crew Z · Won")).toBeVisible();
    expect(await phone.title()).not.toContain("Recap Classic");
  });

  await test.step("a path below a live token cannot show the recap", async () => {
    const asset = await phone.request.get(`${recapPath}/recap.css`);
    expect(asset.status()).toBe(404);
    await phone.goto(`${recapPath}/extra`);
    await expect(phone.getByRole("heading", { name: "This link isn't available" })).toBeVisible();
    await expect(phone.getByRole("heading", { name: "Crew Battle" })).toHaveCount(0);
    await phone.goto(recapPath);
  });

  await test.step("an edit to the event leaves the published recap as it was", async () => {
    const final = page.locator(".perf", { hasText: "vs Crew Z" });
    await page.getByRole("dialog").getByRole("button", { name: "Close" }).click();
    await final.getByRole("button", { name: "Edit", exact: true }).click();
    await page.getByLabel("Against").fill("Crew Q");
    await page.getByRole("button", { name: "Save", exact: true }).click();
    await expect(page.locator(".matchup-line", { hasText: "vs Crew Q" })).toBeVisible();

    await phone.reload();
    await expect(phone.getByText("Final · vs Crew Z · Won")).toBeVisible();
    await expect(phone.getByText("Crew Q")).toHaveCount(0);
  });

  await test.step("turning it off ends access to the recap", async () => {
    await page.getByRole("button", { name: "Share" }).click();
    const dialog = page.getByRole("dialog");
    await dialog.locator(".recap-link-item").getByRole("button", { name: "Turn off" }).click();
    await expect(dialog.locator(".recap-link-item")).toHaveCount(0);

    await phone.goto(recapPath);
    await expect(phone.getByRole("heading", { name: "This link isn't available" })).toBeVisible();
    await expect(phone.getByRole("heading", { name: "Crew Battle" })).toHaveCount(0);
  });

  await outsider.close();
});
