import { test, expect } from "@playwright/test";

let pageErrors: string[] = [];
let clients = 0;

test.beforeEach(async ({ page }, testInfo) => {
  pageErrors = [];
  page.on("pageerror", error => pageErrors.push(error.message));
  clients++;
  await page.setExtraHTTPHeaders({
    "X-Forwarded-For": `10.${testInfo.workerIndex}.3.${clients}`,
  });
});

test.afterEach(() => {
  expect(pageErrors, "the page threw while the test ran").toEqual([]);
});

test("a battle bracket records each round and sums up the run", async ({ page }) => {
  await test.step("sign up and set up a dance season with a competition and a routine", async () => {
    await page.goto("/create-account");
    await page.getByLabel("Full Name").fill("Bracket Parent");
    await page.getByLabel("Email Address").fill(`ui-bracket-${Date.now()}@family-portal.invalid`);
    await page.getByLabel("Password", { exact: true }).fill("ui-portal-password");
    await page.getByLabel("Confirm Password").fill("ui-portal-password");
    await page.getByLabel("Birthday").fill("1988-02-01");
    await page.getByRole("button", { name: "Create Account" }).click();
    await expect(page).toHaveURL(/\/dashboard$/);

    await page.goto("/activities");
    await page.getByRole("button", { name: "New program" }).click();
    await page.locator("#newActivityName").fill("Dance");
    await page.getByRole("button", { name: "Add program" }).click();
    await page.getByRole("button", { name: "New season" }).click();
    await page.locator("#newSeasonName").fill("2026");
    await page.getByRole("button", { name: "Add season" }).click();
    await page.getByRole("link", { name: "2026" }).click();
    await expect(page).toHaveURL(/\/season\/\d+$/);

    await page.getByRole("button", { name: "Add competition" }).click();
    await page.locator("#eventName").fill("Battle Fest");
    await page.locator("#eventStart").fill("2026-10-17");
    await page.getByRole("button", { name: "Add competition" }).click();
    await expect(page.getByRole("link", { name: "Battle Fest" })).toBeVisible();

    await page.getByRole("button", { name: "Add routine" }).click();
    await page.locator("#entryName").fill("Crew Battle");
    await page.getByRole("button", { name: "Add routine" }).click();
    await expect(page.getByRole("link", { name: "Crew Battle" })).toBeVisible();

    await page.getByRole("link", { name: "Battle Fest" }).click();
    await expect(page.getByRole("heading", { name: "Battle Fest", level: 1 })).toBeVisible();
  });

  const rounds = [
    { round: "Round 1", opponent: "Crew X", outcome: "Won" },
    { round: "Final", opponent: "Crew Z", outcome: "Lost" },
  ];
  for (const { round, opponent, outcome } of rounds) {
    await test.step(`add the ${round} against ${opponent}`, async () => {
      await page.getByRole("button", { name: "Add performance" }).click();
      await page.getByRole("button", { name: "Add head-to-head" }).click();
      await page.getByLabel("Round").fill(round);
      await page.getByLabel("Against").fill(opponent);
      await page.getByLabel("Outcome").selectOption({ label: outcome });
      await page.getByRole("button", { name: "Add performance" }).click();
      await expect(
        page.locator(".matchup-line", { hasText: `${round} · vs ${opponent} · ${outcome}` })
      ).toBeVisible();
    });
  }

  await test.step("the run is summed up above its rounds", async () => {
    await expect(page.locator(".matchup-summary")).toHaveText(
      "Crew Battle — Went 1–1, lost in the Final"
    );
  });

  await test.step("a score fills in the outcome it implies", async () => {
    const final = page.locator(".appearance-item", { hasText: "vs Crew Z" });
    await final.getByRole("button", { name: "Edit performance" }).click();
    await expect(page.getByLabel("Outcome")).toHaveValue("loss");
    await page.getByLabel("Outcome").selectOption("");
    await page.getByLabel("Our score").fill("3");
    await page.getByLabel("Their score").fill("2");
    await expect(page.getByLabel("Outcome")).toHaveValue("win");
    await page.getByRole("button", { name: "Save", exact: true }).click();
    await expect(
      page.locator(".matchup-line", { hasText: "Final · vs Crew Z · Won 3–2" })
    ).toBeVisible();
    await expect(page.locator(".matchup-summary")).toHaveText(
      "Crew Battle — Went 2–0, won the Final"
    );
  });
});
