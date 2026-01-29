import { test, expect } from "@playwright/test";

test("homepage loads", async ({ page }) => {
  await page.goto("/", { waitUntil: "domcontentloaded" });
  await expect(
    page.getByRole("heading", { name: "ApptRemind Admin" })
  ).toBeVisible();
});

test("status page loads", async ({ page }) => {
  await page.goto("/status", { waitUntil: "domcontentloaded" });
  await expect(page.getByText("System status", { exact: false })).toBeVisible();
});

test("gateway health endpoint responds", async ({ request }) => {
  const resp = await request.get("http://localhost:8080/healthz");
  expect(resp.ok()).toBeTruthy();
});
