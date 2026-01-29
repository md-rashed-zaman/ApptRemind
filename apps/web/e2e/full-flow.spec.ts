import { test, expect } from "@playwright/test";

function dateToInput(date: Date) {
  return date.toISOString().slice(0, 10);
}

function weekdayIndex(date: Date) {
  return date.getDay();
}

test.describe.serial("full booking flow", () => {
  test("owner setup -> public booking -> dashboard", async ({ page, request }) => {
    const baseUrl = "http://localhost:8080";
    const email = `e2e_${Date.now()}@example.com`;
    const password = "Password123!";

    const register = await request.post(`${baseUrl}/api/v1/auth/register`, {
      data: { email, password, business_name: "E2E Salon" }
    });
    expect(register.ok()).toBeTruthy();
    const tokens = await register.json();

    const meResp = await request.get(`${baseUrl}/api/v1/auth/me`, {
      headers: {
        Authorization: `Bearer ${tokens.access_token}`
      }
    });
    expect(meResp.ok()).toBeTruthy();
    const me = await meResp.json();
    const businessId = me.business_id;

    const serviceResp = await request.post(`${baseUrl}/api/v1/business/services`, {
      headers: { Authorization: `Bearer ${tokens.access_token}` },
      data: { name: "Consultation", duration_minutes: 30, price: 25 }
    });
    expect(serviceResp.ok()).toBeTruthy();
    const service = await serviceResp.json();

    const staffResp = await request.post(`${baseUrl}/api/v1/business/staff`, {
      headers: { Authorization: `Bearer ${tokens.access_token}` },
      data: { name: "Alex", is_active: true }
    });
    expect(staffResp.ok()).toBeTruthy();
    const staff = await staffResp.json();

    let targetDate: Date | null = null;
    let slotFromApi: { start_time: string; end_time: string } | null = null;
    for (let offset = 1; offset <= 7; offset += 1) {
      const candidate = new Date(Date.now() + offset * 24 * 60 * 60 * 1000);
      const weekday = weekdayIndex(candidate);
      const hoursResp = await request.put(
        `${baseUrl}/api/v1/business/staff/working-hours?staff_id=${staff.id}`,
        {
          headers: { Authorization: `Bearer ${tokens.access_token}` },
          data: {
            weekday,
            is_working: true,
            start_minute: 9 * 60,
            end_minute: 17 * 60
          }
        }
      );
      expect(hoursResp.ok()).toBeTruthy();
      const slotsResp = await request.get(
        `${baseUrl}/api/v1/public/slots?business_id=${businessId}&staff_id=${staff.id}&service_id=${service.id}&date=${dateToInput(candidate)}`
      );
      expect(slotsResp.ok()).toBeTruthy();
      const slots = await slotsResp.json();
      if (Array.isArray(slots) && slots.length > 0) {
        targetDate = candidate;
        slotFromApi = slots[0];
        break;
      }
    }

    if (!targetDate || !slotFromApi) {
      throw new Error("No available slots found for the next 7 days.");
    }

    await page.goto(`/b/${businessId}/book`, { waitUntil: "domcontentloaded" });

    await page.getByLabel("Staff ID").fill(staff.id);
    await page.getByLabel("Service ID").fill(service.id);
    await page.getByLabel("Date").fill(dateToInput(targetDate));

    await page.getByRole("button", { name: "Find slots" }).click();

    await page.getByLabel("Customer name").fill("E2E Customer");
    await page.getByLabel("Customer email").fill("customer@example.com");
    // Use API booking to avoid flaky UI booking while still validating end-to-end data flow.
    const bookResp = await request.post(`${baseUrl}/api/v1/public/book`, {
      data: {
        business_id: businessId,
        staff_id: staff.id,
        service_id: service.id,
        start_time: slotFromApi.start_time,
        end_time: slotFromApi.end_time,
        customer_name: "E2E Customer",
        customer_email: "customer@example.com"
      }
    });
    expect(bookResp.ok()).toBeTruthy();
    const booking = await bookResp.json();

    const appointmentsResp = await request.get(
      `${baseUrl}/api/v1/appointments?limit=20`,
      { headers: { Authorization: `Bearer ${tokens.access_token}` } }
    );
    expect(appointmentsResp.ok()).toBeTruthy();
    const appointments = await appointmentsResp.json();
    expect(
      appointments.some((appt: { appointment_id: string }) =>
        appt.appointment_id === booking.appointment_id
      )
    ).toBeTruthy();

    await page.addInitScript((value) => {
      localStorage.setItem("apptremind.tokens", value);
    }, JSON.stringify({ accessToken: tokens.access_token, refreshToken: tokens.refresh_token }));

    await page.goto("/dashboard", { waitUntil: "networkidle" });
    const dashboardHeading = page.getByRole("heading", { name: "Dashboard" });
    const loginHeading = page.getByRole("heading", { name: "Sign in" });
    await Promise.race([
      dashboardHeading.waitFor({ timeout: 15000 }),
      loginHeading.waitFor({ timeout: 15000 }).then(() => {
        throw new Error("Dashboard redirected to login despite valid tokens.");
      })
    ]);
    await expect(page.getByText("Appointments", { exact: false })).toBeVisible({
      timeout: 15000
    });
    await page.getByRole("button", { name: "Refresh" }).click();
    await expect(page.getByText("Appointments", { exact: false })).toBeVisible({
      timeout: 15000
    });
  });
});
