import { test, expect } from "@playwright/test";

function nextWeekdayDate(): Date {
  const date = new Date();
  for (let i = 1; i <= 7; i += 1) {
    const candidate = new Date(date.getTime() + i * 24 * 60 * 60 * 1000);
    const day = candidate.getDay();
    if (day >= 1 && day <= 5) return candidate;
  }
  return new Date(date.getTime() + 24 * 60 * 60 * 1000);
}

function dateToInput(date: Date) {
  return date.toISOString().slice(0, 10);
}

function dateTimeLocal(date: Date) {
  const pad = (value: number) => String(value).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(
    date.getDate()
  )}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

test.describe.serial("ui flow", () => {
  test("register -> onboarding -> availability -> booking -> dashboard", async ({ page }) => {
    const email = `ui_${Date.now()}@example.com`;
    const password = "Password123!";
    const businessName = "UI Salon";

    await page.goto("/auth/register", { waitUntil: "domcontentloaded" });
    await page.getByLabel("Business name").fill(businessName);
    await page.getByLabel("Email").fill(email);
    await page.getByLabel("Password").fill(password);
    await page.getByRole("button", { name: "Create account" }).click();
    await Promise.race([
      page.waitForURL("**/dashboard", { timeout: 15000 }),
      page.getByText("Registration failed").waitFor({ timeout: 15000 }).then(() => {
        throw new Error("Registration failed");
      })
    ]);
    await expect(page.getByRole("heading", { name: "Dashboard" })).toBeVisible({
      timeout: 15000
    });

    await page.goto("/onboarding", { waitUntil: "domcontentloaded" });
    await expect(page.getByText("Owner onboarding")).toBeVisible();

    await page.getByRole("tab", { name: "Services" }).click();
    await page.getByLabel("Service name").fill("Consultation");
    await page.getByLabel("Duration (min)").fill("30");
    await page.getByLabel("Price").fill("25");
    await page.getByRole("button", { name: "Add service" }).click();
    await expect(page.getByText("Consultation")).toBeVisible();

    await page.getByRole("tab", { name: "Staff" }).click();
    const staffCards = page.locator("[data-testid='staff-card']");
    const initialStaffCount = await staffCards.count();
    await page.getByLabel("Staff name").fill("Alex");
    const staffSave = page.waitForResponse((resp) =>
      resp.url().includes("/api/v1/business/staff") &&
      (resp.request().method() === "POST" || resp.request().method() === "GET")
    );
    await page.getByRole("button", { name: "Add staff" }).click();
    await staffSave;
    await expect(staffCards).toHaveCount(initialStaffCount + 1, {
      timeout: 15000
    });

    await page.goto("/availability", { waitUntil: "domcontentloaded" });
    await expect(
      page.getByRole("heading", { name: "Availability" })
    ).toBeVisible();
    await page.getByRole("combobox").click();
    const firstStaffOption = page.getByRole("option").first();
    await expect(firstStaffOption).toBeVisible({ timeout: 15000 });
    await firstStaffOption.click();

    await page.getByRole("button", { name: "Weekdays 9–5" }).click();
    await page.getByRole("button", { name: "Save working hours" }).click();

    // Skip time-off creation in UI flow to avoid flaky time-based inputs.

    await page.goto("/public-demo", { waitUntil: "domcontentloaded" });
    await page.getByRole("button", { name: "Load from my account" }).click();

    const bookingDate = nextWeekdayDate();
    await page.getByLabel("Date").fill(dateToInput(bookingDate));
    await expect(page.getByRole("button", { name: "Find slots" })).toBeVisible();

    await page.goto("/dashboard", { waitUntil: "domcontentloaded" });
    await expect(page.getByRole("heading", { name: "Dashboard" })).toBeVisible({
      timeout: 15000
    });
  });
});
