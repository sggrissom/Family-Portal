import { test, expect, type Page } from "@playwright/test";

// What this suite is for: `cmd/e2e` already proves the server answers. It calls
// the procedures directly, so a bundle that throws on boot, a route that
// renders nothing, or a form wired to the wrong field all pass it. Those are
// the questions here, which is why every assertion is about what a person can
// see on the page rather than about a response body.
//
// Navigation is by clicking, never by page.goto. vlens intercepts same-origin
// link clicks and routes in place, so clicking is both what a visitor does and
// the only way the session survives: signup's token lives in memory until a
// later login exchanges it for a cookie.

const account = {
  name: "UI Runner",
  password: "ui-portal-password",
  birthdate: "1990-04-01",
};

const child = {
  name: "UI Child",
  birthdate: "2020-06-15",
};

const measurement = { value: "42.5", unit: "in" };

// The deployment outlives an individual attempt, and signup refuses an address
// it has already seen, so a retry needs an address of its own.
function freshEmail(): string {
  return `ui-${Date.now()}@family-portal.invalid`;
}

// An uncaught exception does not fail a page, it just leaves it half-rendered,
// so it has to be collected and asserted on rather than waited for.
let pageErrors: string[] = [];

test.beforeEach(({ page }) => {
  pageErrors = [];
  page.on("pageerror", error => pageErrors.push(error.message));
});

test.afterEach(() => {
  expect(pageErrors, "the page threw while the test ran").toEqual([]);
});

test("the landing page renders for a signed-out visitor", async ({ page }) => {
  await page.goto("/");

  await expect(page.getByRole("heading", { name: "Family Record", level: 1 })).toBeVisible();
  await expect(page.getByRole("link", { name: "Log in" }).first()).toBeVisible();

  await page.getByRole("link", { name: "Create an account" }).first().click();

  await expect(page).toHaveURL(/\/create-account$/);
  await expect(page.getByRole("heading", { name: "Create Account" })).toBeVisible();
});

test("a new family signs up, adds a person, and records a measurement", async ({ page }) => {
  const email = freshEmail();

  await test.step("create the account", async () => {
    await page.goto("/create-account");

    await page.getByLabel("Full Name").fill(account.name);
    await page.getByLabel("Email Address").fill(email);
    // Exact, because "Confirm Password" also contains "Password".
    await page.getByLabel("Password", { exact: true }).fill(account.password);
    await page.getByLabel("Confirm Password").fill(account.password);
    await page.getByLabel("Birthday").fill(account.birthdate);

    await page.getByRole("button", { name: "Create Account" }).click();

    await expect(page).toHaveURL(/\/dashboard$/);
    await expect(
      page.getByRole("heading", { name: `Welcome back, ${account.name}!` })
    ).toBeVisible();

    // Signup creates the account holder as the family's first person, using
    // the account name when no profile name is given.
    await expect(personCard(page, account.name)).toBeVisible();
  });

  await test.step("add a family member", async () => {
    await page.getByRole("link", { name: "Add family member" }).click();

    await expect(page).toHaveURL(/\/add-person$/);
    await expect(page.getByRole("heading", { name: "Add Family Member" })).toBeVisible();

    await page.locator("#name").fill(child.name);
    await page.locator("#gender").selectOption("1");
    await page.locator("#birthdate").fill(child.birthdate);

    await page.getByRole("button", { name: "Add Family Member" }).click();

    await expect(page).toHaveURL(/\/dashboard$/);
    await expect(personCard(page, child.name)).toBeVisible();
  });

  await test.step("record a height for them", async () => {
    await page.getByRole("link", { name: "Record growth" }).click();

    await expect(page).toHaveURL(/\/add-growth$/);
    await expect(page.getByRole("heading", { name: "Measure Now" })).toBeVisible();

    // The option's label carries an age alongside the name, so the person is
    // found by the option that names them and selected by its value.
    const option = page.locator("#person option").filter({ hasText: child.name });
    await expect(option).toHaveCount(1);
    await page.locator("#person").selectOption(await option.getAttribute("value"));

    // By id, not by label: the value field and the measurement-type radio are
    // both labelled "Height".
    await page.getByRole("radio", { name: "Height" }).check();
    await page.locator("#value").fill(measurement.value);
    await page.locator("#unit").selectOption(measurement.unit);
    await page.getByRole("radio", { name: "Today" }).check();

    // The preview renders from the same form state the request is built from,
    // so it is worth one assertion before the submit.
    await expect(page.locator(".measurement-preview")).toContainText(
      `${measurement.value} ${measurement.unit}`
    );

    await page.getByRole("button", { name: "Save Measurement" }).click();
  });

  await test.step("the measurement is on the person's profile", async () => {
    await expect(page).toHaveURL(/\/profile\/\d+$/);
    await expect(page.getByRole("heading", { name: child.name, level: 1 })).toBeVisible();

    const entry = page.locator(".timeline-item.measurement-item").first();
    await expect(entry).toBeVisible();
    await expect(entry.locator(".measurement-value")).toContainText(
      `${measurement.value} ${measurement.unit}`
    );
  });
});

function personCard(page: Page, name: string) {
  return page.locator(".person-card").filter({ hasText: name });
}
