import { test, expect, type Download, type Page } from "@playwright/test";

let pageErrors: string[] = [];
let clients = 0;

test.beforeEach(async ({ page }, testInfo) => {
  pageErrors = [];
  page.on("pageerror", error => pageErrors.push(error.message));
  clients++;
  await page.setExtraHTTPHeaders({
    "X-Forwarded-For": `10.${testInfo.workerIndex}.6.${clients}`,
  });
});

test.afterEach(() => {
  expect(pageErrors, "the page threw while the test ran").toEqual([]);
});

const child = { name: "Archive Child", birthdate: "2021-03-09" };

async function download(page: Page, start: () => Promise<void>, dir: string): Promise<string> {
  const pending = page.waitForEvent("download");
  await start();
  const file: Download = await pending;
  const path = `${dir}/${file.suggestedFilename()}`;
  await file.saveAs(path);
  return path;
}

async function previewAndImport(page: Page, file: string) {
  await page.locator("#json-file").setInputFiles(file);
  await page.getByRole("button", { name: "Preview Data" }).click();
  await expect(page.getByRole("heading", { name: "Select Data to Import" })).toBeVisible();
  await expect(page.locator(".person-name", { hasText: child.name })).toBeVisible();
  await page.getByRole("button", { name: "Import Data" }).click();
}

test("a family downloads Records and the Everything archive and imports them back", async ({
  page,
}, testInfo) => {
  await test.step("sign up and add a family member", async () => {
    await page.goto("/create-account");
    await page.getByLabel("Full Name").fill("Archive Parent");
    await page.getByLabel("Email Address").fill(`ui-archive-${Date.now()}@family-portal.invalid`);
    await page.getByLabel("Password", { exact: true }).fill("ui-portal-password");
    await page.getByLabel("Confirm Password").fill("ui-portal-password");
    await page.getByLabel("Birthday").fill("1987-05-20");
    await page.getByRole("button", { name: "Create Account" }).click();
    await expect(page).toHaveURL(/\/dashboard$/);

    await page.getByRole("link", { name: "Add family member" }).click();
    await expect(page.getByRole("heading", { name: "Add Family Member" })).toBeVisible();
    await page.locator("#name").fill(child.name);
    await page.locator("#gender").selectOption("1");
    await page.locator("#birthdate").fill(child.birthdate);
    await page.getByRole("button", { name: "Add Family Member" }).click();
    await expect(page).toHaveURL(/\/dashboard$/);
  });

  const archiveDownload = (name: string) =>
    page.locator(".archive-download", {
      has: page.locator(".archive-download-name", { hasText: new RegExp(`^${name}$`) }),
    });
  const records = archiveDownload("Records");
  const everything = archiveDownload("Everything archive");
  let recordsFile = "";
  let everythingFile = "";

  await test.step("settings describes both downloads with their sizes", async () => {
    await page.goto("/settings");
    await expect(records).toContainText("No photos or videos");
    await expect(records.locator(".archive-download-size")).toHaveText(/^About \d+(\.\d)? (B|KB)$/);
    await expect(everything).toContainText("every photo and video");
    await expect(everything.locator(".archive-download-size")).toHaveText(/^About \d/);
  });

  await test.step("download both", async () => {
    const dir = testInfo.outputPath("downloads");
    recordsFile = await download(
      page,
      () => records.getByRole("button", { name: "Download Records" }).click(),
      dir
    );
    expect(recordsFile).toMatch(/\.json$/);
    everythingFile = await download(
      page,
      () => everything.getByRole("link", { name: "Download Everything archive" }).click(),
      dir
    );
    expect(everythingFile).toMatch(/\.zip$/);
  });

  await test.step("import the Everything archive and follow its progress", async () => {
    await page.getByRole("link", { name: "Import Data" }).click();
    await expect(page.getByRole("heading", { name: "Import Family Data" })).toBeVisible();
    await previewAndImport(page, everythingFile);

    await expect(
      page.getByRole("status").filter({ hasText: "the import keeps running" })
    ).toContainText(/Waiting to start|Reading the archive|Importing/);
    await expect(page.getByRole("heading", { name: "Import Successful!" })).toBeVisible();
    await expect(page.locator(".stat", { hasText: "People Merged" })).toBeVisible();
  });

  await test.step("import the Records file the same way", async () => {
    await page.getByRole("button", { name: "Import More Data" }).click();
    await previewAndImport(page, recordsFile);
    await expect(page.getByRole("heading", { name: "Import Successful!" })).toBeVisible();
    await expect(page.locator(".stat", { hasText: "People Merged" })).toBeVisible();
  });
});
