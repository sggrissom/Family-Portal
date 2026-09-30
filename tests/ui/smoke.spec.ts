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

const measurement = { value: "42.5", shown: "3 ft 6.5 in" };

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
    await addFor(page, child.name, "Measurement");

    await expect(page).toHaveURL(/\/add-growth\/\d+$/);
    await expect(page.getByRole("heading", { name: "Measurement" })).toBeVisible();
    await expect(page.locator(".entry-subject")).toContainText(child.name);

    await page.locator("#height").fill(measurement.value);
    await page.getByRole("button", { name: "Save", exact: true }).click();
  });

  await test.step("saving shows the measurement in context", async () => {
    await expect(page).toHaveURL(/\/view-growth\/\d+$/);
    await expect(page.locator(".growth-detail-value")).toContainText(measurement.shown);
  });

  await test.step("the result links to everyone at the same age", async () => {
    await page.getByRole("link", { name: /See everything at this age/ }).click();
    await expect(page).toHaveURL(/\/same-age\?age=\d+m&from=\d+$/);
    await expect(page.locator(".same-age-row").filter({ hasText: child.name })).toContainText(
      measurement.shown
    );
    await page.goBack();
    await expect(page).toHaveURL(/\/view-growth\/\d+$/);
  });

  await test.step("the measurement is on the person's profile", async () => {
    await page.getByRole("link", { name: `Back to ${child.name}'s Profile` }).click();
    await expect(page).toHaveURL(/\/profile\/\d+$/);
    await expect(page.getByRole("heading", { name: child.name, level: 1 })).toBeVisible();

    await expect(page.locator(".day-checkup").first()).toContainText(measurement.shown);
  });

  await test.step("and in the family history", async () => {
    await page.getByRole("link", { name: "History", exact: true }).first().click();
    await expect(page).toHaveURL(/\/history$/);
    await expect(page.locator(".day-checkup").first()).toContainText(measurement.shown);
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

  await addFor(page, baby.name, "Measurement");
  await expect(page.locator(".entry-subject")).toContainText(baby.name);

  // Under two, weight entry defaults to pounds and ounces.
  await expect(page.getByRole("button", { name: "lb/oz" })).toHaveAttribute("aria-pressed", "true");
  await page.locator("#pounds").fill("7");
  await page.locator("#ounces").fill("8");
  await page.getByRole("button", { name: "Save", exact: true }).click();

  await expect(page).toHaveURL(/\/view-growth\/\d+$/);
  await expect(page.locator(".growth-detail-value")).toContainText("7 lb 8 oz");

  await page.getByRole("link", { name: "Growth" }).first().click();
  await page.getByRole("button", { name: "Add", exact: true }).click();
  await page.getByRole("dialog").getByRole("link", { name: "Measurement" }).click();
  await expect(page.locator(".entry-subject")).toContainText(baby.name);
  await page.locator("#height").fill("22");
  await page.locator("#pounds").fill("9");
  await page.locator("#ounces").fill("16");
  await page.getByRole("button", { name: "Save", exact: true }).click();
  await expect(page.getByRole("alert")).toContainText("under 16");
  await expect(page.locator("#height")).toHaveValue("22");

  await page.locator("#ounces").fill("2");
  await page.getByRole("button", { name: "Save", exact: true }).click();
  await expect(page).toHaveURL(/\/view-growth\/\d+,\d+$/);
  await expect(page.locator(".growth-detail-value")).toHaveCount(2);
  await expect(page.locator(".growth-detail-value").last()).toContainText("9 lb 2 oz");
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

  await addFor(page, mistake.name, "Measurement");
  await page.locator("#height").fill("40");
  await page.getByRole("button", { name: "Save", exact: true }).click();
  await expect(page).toHaveURL(/\/view-growth\/\d+$/);
  await page.getByRole("link", { name: `Back to ${mistake.name}'s Profile` }).click();
  await expect(page).toHaveURL(/\/profile\/\d+$/);

  await page
    .locator(".profile-actions")
    .getByRole("link", { name: "✏️ Edit", exact: true })
    .click();
  await expect(page).toHaveURL(/\/edit-person\/\d+$/);

  await page.getByRole("button", { name: `Delete ${mistake.name}…` }).click();
  await expect(page.locator(".person-deletion-confirm")).toContainText("1 measurement");

  await page.getByRole("button", { name: `Delete ${mistake.name}`, exact: true }).click();

  await expect(page).toHaveURL(/\/dashboard$/);
  await expect(personCard(page, account.name)).toBeVisible();
  await expect(personCard(page, mistake.name)).toHaveCount(0);
});

test("photos upload as soon as they are picked and take a caption after", async ({ page }) => {
  const caption = "UI Park Day";

  await page.goto("/create-account");
  await page.getByLabel("Full Name").fill(account.name);
  await page.getByLabel("Email Address").fill(freshEmail());
  await page.getByLabel("Password", { exact: true }).fill(account.password);
  await page.getByLabel("Confirm Password").fill(account.password);
  await page.getByLabel("Birthday").fill(account.birthdate);
  await page.getByRole("button", { name: "Create Account" }).click();
  await expect(page).toHaveURL(/\/dashboard$/);

  // The sheet's Photos button opens the file picker directly.
  await page.getByRole("button", { name: "Add", exact: true }).click();
  await page
    .locator("#addSheetPhotoInput")
    .setInputFiles(["backend/seedphotos/bubbles-park.jpg", "backend/seedphotos/soccer-match.jpg"]);

  await expect(page).toHaveURL(/\/add-photo$/);
  await expect(page.locator(".upload-tile.upload-done")).toHaveCount(2);

  await page.locator("#caption").fill(caption);
  await page.getByRole("button", { name: "Done" }).click();

  await expect(page).toHaveURL(/\/dashboard$/);
  await page.getByRole("link", { name: "Photos" }).first().click();
  await expect(page).toHaveURL(/\/photos$/);
  await expect(page.locator(".photo-card").filter({ hasText: caption })).toHaveCount(2);
});

test("a measurement and a milestone are edited without losing their units", async ({ page }) => {
  const kid = { name: "UI Editor Kid", birthdate: "2020-06-15" };

  await page.goto("/create-account");
  await page.getByLabel("Full Name").fill(account.name);
  await page.getByLabel("Email Address").fill(freshEmail());
  await page.getByLabel("Password", { exact: true }).fill(account.password);
  await page.getByLabel("Confirm Password").fill(account.password);
  await page.getByLabel("Birthday").fill(account.birthdate);
  await page.getByRole("button", { name: "Create Account" }).click();
  await expect(page).toHaveURL(/\/dashboard$/);

  await page.getByRole("link", { name: "Add family member" }).click();
  await page.locator("#name").fill(kid.name);
  await page.locator("#gender").selectOption("0");
  await page.locator("#birthdate").fill(kid.birthdate);
  await page.getByRole("button", { name: "Add Family Member" }).click();
  await expect(personCard(page, kid.name)).toBeVisible();

  await test.step("a centimetre height opens and saves in centimetres", async () => {
    await addFor(page, kid.name, "Measurement");
    await page.getByRole("button", { name: "cm", exact: true }).click();
    await page.locator("#height").fill("98");
    await page.getByRole("button", { name: "Save", exact: true }).click();
    await expect(page.locator(".growth-detail-value")).toContainText("98 cm");

    await page.locator(".growth-detail-actions").getByRole("link", { name: /Edit/ }).click();
    await expect(page).toHaveURL(/\/edit-growth\/\d+$/);
    await expect(page.getByRole("button", { name: "cm", exact: true })).toHaveAttribute(
      "aria-pressed",
      "true"
    );
    await expect(page.locator("#height")).toHaveValue("98");

    await page.locator("#height").fill("99abc");
    await page.getByRole("button", { name: "Save", exact: true }).click();
    await expect(page.getByRole("alert")).toContainText("above zero");
    await expect(page.locator("#height")).toHaveValue("99abc");

    await page.locator("#height").fill("99.5");
    await page.getByLabel("When").selectOption("age");
    await page.getByLabel("Age in years").fill("3");
    await page.getByRole("button", { name: "Save", exact: true }).click();
    await expect(page).toHaveURL(/\/profile\/\d+$/);
    await expect(page.locator(".day-checkup").first()).toContainText("99.5 cm");
  });

  await test.step("a milestone opens with its date and saves its edits", async () => {
    await page.getByRole("link", { name: "Home" }).first().click();
    await expect(page).toHaveURL(/\/dashboard$/);
    await page.getByRole("button", { name: "Add", exact: true }).click();
    await page.getByRole("dialog").getByRole("link", { name: "Milestone" }).click();
    await expect(page).toHaveURL(/\/add-milestone/);
    const chip = page.getByRole("button", { name: kid.name });
    if ((await chip.getAttribute("aria-pressed")) !== "true") await chip.click();
    await page.locator("#description").fill("Rode a bike");
    await page.getByLabel("When").selectOption("date");
    await page.getByLabel("Date", { exact: true }).fill("2024-05-04");
    await page.getByRole("button", { name: "Save", exact: true }).click();
    await expect(page).toHaveURL(/\/profile\/\d+$/);

    await page.locator(".day-milestone").filter({ hasText: "Rode a bike" }).click();
    await page.locator(".milestone-detail-actions").getByRole("link", { name: /Edit/ }).click();
    await expect(page).toHaveURL(/\/edit-milestone\/\d+$/);
    await expect(page.locator("#description")).toHaveValue("Rode a bike");
    await expect(page.getByLabel("Date", { exact: true })).toHaveValue("2024-05-04");

    await page.locator("#description").fill("Rode a bike alone");
    await page.getByRole("button", { name: "Save", exact: true }).click();
    await expect(page).toHaveURL(/\/profile\/\d+$/);
    await expect(page.locator(".day-milestone").filter({ hasText: "alone" })).toHaveCount(1);
  });
});

async function addFor(
  page: Page,
  personName: string,
  kind: "Photos" | "Measurement" | "Milestone"
) {
  await page.getByRole("button", { name: "Add", exact: true }).click();
  const sheet = page.getByRole("dialog");
  await sheet.getByRole("button", { name: personName }).click();
  await sheet.getByRole("link", { name: kind }).click();
}

function monthsAgo(months: number): string {
  const d = new Date();
  d.setMonth(d.getMonth() - months);
  return d.toISOString().split("T")[0];
}

function personCard(page: Page, name: string) {
  return page.getByRole("navigation", { name: "Family" }).getByRole("link", { name });
}
