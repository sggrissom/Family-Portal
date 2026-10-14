import { test, expect } from "@playwright/test";

let pageErrors: string[] = [];
let clients = 0;

test.beforeEach(async ({ page }, testInfo) => {
  pageErrors = [];
  page.on("pageerror", error => pageErrors.push(error.message));
  clients++;
  await page.setExtraHTTPHeaders({
    "X-Forwarded-For": `10.${testInfo.workerIndex}.2.${clients}`,
  });
});

test.afterEach(() => {
  expect(pageErrors, "the page threw while the test ran").toEqual([]);
});

test("search finds a moment by part of a word and keeps the results on Back", async ({ page }) => {
  const kid = "Sasha";

  await test.step("sign up and record a moment", async () => {
    await page.goto("/create-account");
    await page.getByLabel("Full Name").fill("Search Parent");
    await page.getByLabel("Email Address").fill(`ui-search-${Date.now()}@family-portal.invalid`);
    await page.getByLabel("Password", { exact: true }).fill("ui-portal-password");
    await page.getByLabel("Confirm Password").fill("ui-portal-password");
    await page.getByLabel("Birthday").fill("1988-02-01");
    await page.getByRole("button", { name: "Create Account" }).click();
    await expect(page).toHaveURL(/\/dashboard$/);

    await page.getByRole("link", { name: "Add family member" }).click();
    await expect(page.getByRole("heading", { name: "Add Family Member" })).toBeVisible();
    await page.locator("#name").fill(kid);
    await page.locator("#gender").selectOption("1");
    await page.locator("#birthdate").fill("2019-03-04");
    await page.getByRole("button", { name: "Add Family Member" }).click();
    const card = page.getByRole("navigation", { name: "Family" }).getByRole("link", { name: kid });
    await expect(card).toBeVisible();

    await page.getByRole("button", { name: "Add", exact: true }).click();
    const sheet = page.getByRole("dialog");
    await sheet.getByRole("button", { name: kid }).click();
    await sheet.getByRole("link", { name: "Moment" }).click();
    await expect(page).toHaveURL(/\/add-milestone/);
    await page.locator("#description").fill("Splashed through every puddle");
    await page.getByRole("button", { name: "Save", exact: true }).click();
    await expect(page).toHaveURL(/\/profile\/\d+$/);
  });

  await test.step("open search from the top bar and type part of a word", async () => {
    await page.getByRole("link", { name: "Search everything" }).click();
    await expect(page).toHaveURL(/\/search$/);
    const input = page.getByRole("searchbox", { name: "Search everything" });
    await expect(input).toBeFocused();
    await input.fill("pudd");
    await expect(page).toHaveURL(/\/search\?q=pudd$/);
  });

  const moment = page.locator(".search-results .day-milestone", { hasText: "every puddle" });

  await test.step("the moment is found, with the matched part marked", async () => {
    await expect(moment).toBeVisible();
    await expect(moment.locator("mark")).toHaveText("pudd");
    await moment.click();
    await expect(page).toHaveURL(/\/milestone\/\d+$/);
    // vlens ignores Back while a page is still loading, so wait for it.
    await expect(
      page.getByRole("heading", { name: "Splashed through every puddle", level: 1 })
    ).toBeVisible();
  });

  await test.step("Back returns to the same results", async () => {
    await page.goBack();
    await expect(page).toHaveURL(/\/search\?q=pudd$/);
    await expect(page.getByRole("searchbox", { name: "Search everything" })).toHaveValue("pudd");
    await expect(moment).toBeVisible();
  });

  await test.step("a name is understood as a person and can be removed", async () => {
    const input = page.getByRole("searchbox", { name: "Search everything" });
    await input.fill(`${kid} splash`);
    const chip = page.locator(".search-chip", { hasText: kid });
    await expect(chip).toBeVisible();
    await expect(moment).toBeVisible();
    await chip.getByRole("button", { name: `Remove ${kid}` }).click();
    await expect(chip).toHaveCount(0);
  });
});
