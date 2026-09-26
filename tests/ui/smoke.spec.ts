import { test, expect, type Page } from "@playwright/test";

// `cmd/e2e` calls the procedures directly, so a bundle that throws on boot or a
// form wired to the wrong field passes it. Everything asserted here is what a
// person can see on the page instead.
//
// Navigate by clicking, not page.goto: vlens intercepts same-origin link clicks
// and routes in place, so clicking is what exercises the router.

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

// The deployment outlives an attempt and signup refuses an address it has seen,
// so a retry needs its own.
function freshEmail(): string {
  return `ui-${Date.now()}@family-portal.invalid`;
}

// An uncaught exception leaves the page half-rendered rather than failing it,
// so it has to be collected and asserted on.
let pageErrors: string[] = [];

test.beforeEach(({ page }) => {
  pageErrors = [];
  page.on("pageerror", error => pageErrors.push(error.message));
});

test.afterEach(() => {
  expect(pageErrors, "the page threw while the test ran").toEqual([]);
});

test("the landing page renders for a signed-out visitor", async ({ page }) => {
  // A visitor with no session has nothing to refresh, and asking answers 401.
  const refreshCalls: string[] = [];
  page.on("request", request => {
    if (request.url().includes("/api/refresh")) {
      refreshCalls.push(request.url());
    }
  });

  await page.goto("/");

  await expect(page.getByRole("heading", { name: "Family Record", level: 1 })).toBeVisible();
  await expect(page.getByRole("link", { name: "Log in" }).first()).toBeVisible();

  await page.getByRole("link", { name: "Create an account" }).first().click();

  await expect(page).toHaveURL(/\/create-account$/);
  await expect(page.getByRole("heading", { name: "Create Account" })).toBeVisible();

  expect(refreshCalls, "a signed-out visitor asked to refresh a session they do not have").toEqual(
    []
  );
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

  await test.step("the new session survives a reload", async () => {
    // Signup goes through /api/signup rather than the CreateAccount procedure
    // so that the browser is left holding cookies.
    await page.reload();

    await expect(page).toHaveURL(/\/dashboard$/);
    await expect(
      page.getByRole("heading", { name: `Welcome back, ${account.name}!` })
    ).toBeVisible();
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

    // The option's label carries an age alongside the name, so match on the
    // name and select by value.
    const option = page.locator("#person option").filter({ hasText: child.name });
    await expect(option).toHaveCount(1);
    await page.locator("#person").selectOption(await option.getAttribute("value"));

    // By id, not by label: the value field and the measurement-type radio are
    // both labelled "Height".
    await page.getByRole("radio", { name: "Height" }).check();
    await page.locator("#value").fill(measurement.value);
    await page.locator("#unit").selectOption(measurement.unit);
    await page.getByRole("radio", { name: "Today" }).check();

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

test("an infant's weight is entered and shown in pounds and ounces", async ({ page }) => {
  const baby = { name: "UI Baby", birthdate: monthsAgo(2) };

  await page.goto("/create-account");
  await page.getByLabel("Full Name").fill(account.name);
  await page.getByLabel("Email Address").fill(freshEmail());
  await page.getByLabel("Password", { exact: true }).fill(account.password);
  await page.getByLabel("Confirm Password").fill(account.password);
  await page.getByLabel("Birthday").fill(account.birthdate);
  await page.getByRole("button", { name: "Create Account" }).click();
  await expect(page).toHaveURL(/\/dashboard$/);

  await page.getByRole("link", { name: "Add family member" }).click();
  await page.locator("#name").fill(baby.name);
  await page.locator("#gender").selectOption("1");
  await page.locator("#birthdate").fill(baby.birthdate);
  await page.getByRole("button", { name: "Add Family Member" }).click();
  await expect(personCard(page, baby.name)).toBeVisible();

  await page.getByRole("link", { name: "Record growth" }).click();
  const option = page.locator("#person option").filter({ hasText: baby.name });
  await page.locator("#person").selectOption(await option.getAttribute("value"));
  await page.getByRole("radio", { name: "Weight" }).check();

  // Under two, weight entry defaults to pounds and ounces.
  await expect(page.getByRole("radio", { name: "Pounds & Ounces" })).toBeChecked();
  await page.locator("#pounds").fill("7");
  await page.locator("#ounces").fill("8");
  await page.getByRole("radio", { name: "Today" }).check();
  await expect(page.locator(".measurement-preview")).toContainText("7 lb 8 oz");
  await page.getByRole("button", { name: "Save Measurement" }).click();

  await expect(page).toHaveURL(/\/profile\/\d+$/);
  const entry = page.locator(".timeline-item.measurement-item").first();
  await expect(entry.locator(".measurement-value")).toContainText("7 lb 8 oz");
});

test("a person is deleted from their edit page after seeing what goes with them", async ({
  page,
}) => {
  const mistake = { name: "UI Mistake", birthdate: "2019-02-03" };

  await page.goto("/create-account");
  await page.getByLabel("Full Name").fill(account.name);
  await page.getByLabel("Email Address").fill(freshEmail());
  await page.getByLabel("Password", { exact: true }).fill(account.password);
  await page.getByLabel("Confirm Password").fill(account.password);
  await page.getByLabel("Birthday").fill(account.birthdate);
  await page.getByRole("button", { name: "Create Account" }).click();
  await expect(page).toHaveURL(/\/dashboard$/);

  await page.getByRole("link", { name: "Add family member" }).click();
  await page.locator("#name").fill(mistake.name);
  await page.locator("#gender").selectOption("0");
  await page.locator("#birthdate").fill(mistake.birthdate);
  await page.getByRole("button", { name: "Add Family Member" }).click();
  await expect(personCard(page, mistake.name)).toBeVisible();

  await page.getByRole("link", { name: "Record growth" }).click();
  const option = page.locator("#person option").filter({ hasText: mistake.name });
  await page.locator("#person").selectOption(await option.getAttribute("value"));
  await page.getByRole("radio", { name: "Height" }).check();
  await page.locator("#value").fill("40");
  await page.locator("#unit").selectOption("in");
  await page.getByRole("radio", { name: "Today" }).check();
  await page.getByRole("button", { name: "Save Measurement" }).click();
  await expect(page).toHaveURL(/\/profile\/\d+$/);

  await page.getByRole("link", { name: "✏️ Edit", exact: true }).click();
  await expect(page).toHaveURL(/\/edit-person\/\d+$/);

  await page.getByRole("button", { name: `Delete ${mistake.name}…` }).click();
  await expect(page.locator(".person-deletion-confirm")).toContainText("1 measurement");

  await page.getByRole("button", { name: `Delete ${mistake.name}`, exact: true }).click();

  await expect(page).toHaveURL(/\/dashboard$/);
  await expect(personCard(page, account.name)).toBeVisible();
  await expect(personCard(page, mistake.name)).toHaveCount(0);
});

function monthsAgo(months: number): string {
  const d = new Date();
  d.setMonth(d.getMonth() - months);
  return d.toISOString().split("T")[0];
}

function personCard(page: Page, name: string) {
  return page.locator(".person-card").filter({ hasText: name });
}
