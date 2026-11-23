import { test, expect } from "@playwright/test";

let pageErrors: string[] = [];
let clients = 0;

test.beforeEach(async ({ page }, testInfo) => {
  pageErrors = [];
  page.on("pageerror", error => pageErrors.push(error.message));
  clients++;
  await page.setExtraHTTPHeaders({
    "X-Forwarded-For": `10.${testInfo.workerIndex}.5.${clients}`,
  });
});

test.afterEach(() => {
  expect(pageErrors, "the page threw while the test ran").toEqual([]);
});

test("a family page shows only what was chosen, signed out, until it is unpublished", async ({
  page,
  browser,
}) => {
  const slug = `ui-family-${Date.now()}`;
  page.on("dialog", dialog => dialog.accept());

  await test.step("sign up and open the family page editor from settings", async () => {
    await page.goto("/create-account");
    await page.getByLabel("Full Name").fill("Page Parent");
    await page.getByLabel("Email Address").fill(`${slug}@family-portal.invalid`);
    await page.getByLabel("Password", { exact: true }).fill("ui-portal-password");
    await page.getByLabel("Confirm Password").fill("ui-portal-password");
    await page.getByLabel("Birthday").fill("1988-02-01");
    await page.getByRole("button", { name: "Create Account" }).click();
    await expect(page).toHaveURL(/\/dashboard$/);

    await page.getByRole("button", { name: "Add", exact: true }).click();
    await page
      .locator("#addSheetPhotoInput")
      .setInputFiles([
        "backend/seedphotos/bubbles-park.jpg",
        "backend/seedphotos/soccer-match.jpg",
      ]);
    await expect(page.locator(".upload-tile.upload-done")).toHaveCount(2);
    await page.getByRole("button", { name: "Done" }).click();
    await expect(page).toHaveURL(/\/dashboard$/);

    await page.goto("/settings");
    await page.getByRole("link", { name: "Edit family page" }).click();
    await expect(page.getByRole("heading", { name: "Family page", level: 1 })).toBeVisible();
    await expect(page.getByText("Not published.")).toBeVisible();
  });

  await test.step("choose one photo once processing finishes", async () => {
    await expect(async () => {
      await page.reload();
      await expect(page.locator(".photo-picker-item")).toHaveCount(2, { timeout: 2000 });
    }).toPass();
    await page.locator(".photo-picker-item").first().click();
    await expect(page.locator(".photo-picker-item.selected")).toHaveCount(1);
  });

  await test.step("choose a person, an age and a line, then publish", async () => {
    await page.getByLabel("Address").fill(slug);
    await page.getByLabel("Title").fill("The Page Family");
    await page.getByLabel("Introduction").fill("We like long walks.");
    await page.getByRole("checkbox", { name: "Page Parent" }).check();
    await page.getByLabel("Name shown for Page Parent").fill("Pat");
    await page.getByLabel("One line about them for Page Parent").fill("Makes pancakes");
    await page.getByRole("checkbox", { name: "Show age" }).check();

    const preview = page.getByRole("region", { name: "Preview" });
    await expect(preview.getByText("Makes pancakes")).toBeVisible();
    await expect(preview.getByText(/years old/)).toBeVisible();

    await page.getByRole("button", { name: "Publish", exact: true }).click();
    await expect(page.getByRole("link", { name: new RegExp(`/f/${slug}$`) })).toBeVisible();
    await expect(page.getByRole("button", { name: "Publish changes" })).toBeVisible();
  });

  const visitor = await browser.newContext({
    viewport: { width: 390, height: 844 },
    extraHTTPHeaders: { "X-Forwarded-For": `10.99.5.${clients}` },
  });
  const phone = await visitor.newPage();
  let photoUrl = "";

  await test.step("someone without an account opens it on a phone", async () => {
    const response = await phone.goto(`/f/${slug}`);
    expect(response?.status()).toBe(200);
    expect(response?.headers()["referrer-policy"]).toBe("no-referrer");
    expect(response?.headers()["cache-control"]).toBe("no-store");
    expect(response?.headers()["x-robots-tag"]).toContain("noindex");
    await expect(phone.getByRole("heading", { name: "The Page Family", level: 1 })).toBeVisible();
    await expect(phone.getByText("We like long walks.")).toBeVisible();
    await expect(phone.getByText("Pat", { exact: true })).toBeVisible();
    await expect(phone.getByText("Makes pancakes")).toBeVisible();
    await expect(phone.getByText(/^\d+ years old$/)).toBeVisible();
    await expect(phone.getByText("1988")).toHaveCount(0);
    await expect(phone.getByText("Page Parent")).toHaveCount(0);

    const photo = phone.locator(".family-page-photos img");
    await expect(photo).toHaveCount(1);
    photoUrl = (await photo.getAttribute("src")) ?? "";
    expect(photoUrl).toBe(`/api/family-page/${slug}/photo/0`);
    await expect
      .poll(() => photo.evaluate(img => (img as HTMLImageElement).naturalWidth))
      .toBeGreaterThan(0);
    const asset = await phone.request.get(photoUrl);
    expect(asset.status()).toBe(200);
    expect(asset.headers()["content-type"]).toMatch(/^image\//);
    expect(asset.headers()["cache-control"]).toBe("no-store");
    expect(asset.headers()["x-robots-tag"]).toContain("noindex");
  });

  await test.step("unpublishing takes it down at once", async () => {
    await page.getByRole("button", { name: "Unpublish" }).click();
    await expect(page.getByText("Not published.")).toBeVisible();
    await phone.reload();
    await expect(phone.getByRole("heading", { name: "This page isn't available" })).toBeVisible();
    await expect(phone.getByText("Makes pancakes")).toHaveCount(0);
    expect((await phone.request.get(photoUrl)).status()).toBe(404);
  });

  await visitor.close();
});
