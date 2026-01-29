import { ApiClient } from "@apptremind/api";

const baseUrl =
  process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080";

type StoredTokens = {
  accessToken: string;
  refreshToken: string;
} | null;

const storageKey = "apptremind.tokens";

export const tokenStore = {
  get(): StoredTokens {
    if (typeof window === "undefined") return null;
    const raw = window.localStorage.getItem(storageKey);
    if (!raw) return null;
    try {
      return JSON.parse(raw) as StoredTokens;
    } catch {
      return null;
    }
  },
  set(tokens: StoredTokens) {
    if (typeof window === "undefined") return;
    if (!tokens) {
      window.localStorage.removeItem(storageKey);
      return;
    }
    window.localStorage.setItem(storageKey, JSON.stringify(tokens));
  }
};

function safeUuid() {
  if (typeof globalThis !== "undefined") {
    const maybeCrypto = globalThis.crypto as Crypto | undefined;
    if (maybeCrypto?.randomUUID) {
      return maybeCrypto.randomUUID();
    }
  }
  return `${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

const client = new ApiClient({
  baseUrl,
  tokenStore,
  requestId: safeUuid
});

export async function apiLogin(email: string, password: string) {
  const { data } = await client.request<{
    access_token: string;
    refresh_token: string;
  }>("/api/v1/auth/login", {
    method: "POST",
    data: { email, password }
  });
  tokenStore.set({
    accessToken: data.access_token,
    refreshToken: data.refresh_token
  });
  return data;
}

export async function apiRegister(payload: {
  email: string;
  password: string;
  business_name: string;
}) {
  const { data } = await client.request<{
    access_token: string;
    refresh_token: string;
  }>("/api/v1/auth/register", {
    method: "POST",
    data: payload
  });
  tokenStore.set({
    accessToken: data.access_token,
    refreshToken: data.refresh_token
  });
  return data;
}

export async function apiMe() {
  try {
    const { data } = await client.request<{
      id: string;
      business_id: string;
      email: string;
      role: string;
    }>("/api/v1/auth/me");
    return data;
  } catch {
    return null;
  }
}

export async function apiGetBusinessProfile() {
  const { data } = await client.request<{
    business_id: string;
    name: string;
    timezone: string;
    reminder_offsets_minutes: number[];
  }>("/api/v1/business/profile");
  return data;
}

export async function apiUpdateBusinessProfile(payload: {
  name?: string;
  timezone?: string;
  reminder_offsets_minutes?: number[];
}) {
  await client.request("/api/v1/business/profile", {
    method: "PUT",
    data: payload
  });
}

export async function apiListServices() {
  const { data } = await client.request<
    Array<{
      id: string;
      business_id: string;
      name: string;
      duration_minutes: number;
      price: string;
      description?: string;
      created_at: string;
    }>
  >("/api/v1/business/services");
  return data;
}

export async function apiCreateService(payload: {
  name: string;
  duration_minutes: number;
  price: number;
  description?: string;
}) {
  const { data } = await client.request<{ id: string }>(
    "/api/v1/business/services",
    {
      method: "POST",
      data: payload
    }
  );
  return data;
}

export async function apiListStaff() {
  const { data } = await client.request<
    Array<{
      id: string;
      business_id: string;
      name: string;
      is_active: boolean;
    }>
  >("/api/v1/business/staff");
  return data;
}

export async function apiCreateStaff(payload: { name: string; is_active?: boolean }) {
  const { data } = await client.request<{ id: string }>(
    "/api/v1/business/staff",
    {
      method: "POST",
      data: payload
    }
  );
  return data;
}

export async function apiListWorkingHours(staffId: string) {
  const { data } = await client.request<
    Array<{
      staff_id: string;
      weekday: number;
      is_working: boolean;
      start_minute: number;
      end_minute: number;
    }>
  >("/api/v1/business/staff/working-hours", {
    params: { staff_id: staffId }
  });
  return data;
}

export async function apiUpsertWorkingHours(
  staffId: string,
  payload: {
    weekday: number;
    is_working: boolean;
    start_minute?: number;
    end_minute?: number;
  }
) {
  await client.request("/api/v1/business/staff/working-hours", {
    method: "PUT",
    params: { staff_id: staffId },
    data: payload
  });
}

export async function apiListTimeOff(staffId: string, from: string, to: string) {
  const { data } = await client.request<
    Array<{
      id: string;
      staff_id: string;
      start_time: string;
      end_time: string;
      reason?: string;
      created_at: string;
    }>
  >("/api/v1/business/staff/time-off", {
    params: { staff_id: staffId, from, to }
  });
  return data;
}

export async function apiCreateTimeOff(
  staffId: string,
  payload: { start_time: string; end_time: string; reason?: string }
) {
  const { data } = await client.request<{ id: string }>(
    "/api/v1/business/staff/time-off",
    {
      method: "POST",
      params: { staff_id: staffId },
      data: payload
    }
  );
  return data;
}

export async function apiDeleteTimeOff(id: string) {
  await client.request("/api/v1/business/staff/time-off", {
    method: "DELETE",
    params: { id }
  });
}

export async function apiPublicSlots(params: {
  business_id: string;
  staff_id: string;
  service_id: string;
  date: string;
  duration_minutes?: number;
  slot_step_minutes?: number;
}) {
  const { data } = await client.request<
    Array<{
      start_time: string;
      end_time: string;
    }>
  >("/api/v1/public/slots", {
    params
  });
  return data;
}

export async function apiPublicBook(payload: {
  business_id: string;
  staff_id: string;
  service_id: string;
  start_time: string;
  end_time: string;
  customer_name: string;
  customer_email?: string;
  customer_phone?: string;
}) {
  const { data } = await client.request<{ appointment_id: string }>(
    "/api/v1/public/book",
    {
      method: "POST",
      headers: {
        "Idempotency-Key": safeUuid()
      },
      data: payload
    }
  );
  return data;
}

export async function apiListAppointments(limit = 10) {
  const { data } = await client.request<
    Array<{
      appointment_id: string;
      staff_id: string;
      service_id: string;
      start_time: string;
      end_time: string;
      status: string;
      created_at: string;
    }>
  >("/api/v1/appointments", {
    params: { limit }
  });
  return data;
}

export async function apiCancelAppointment(payload: {
  business_id: string;
  appointment_id: string;
  reason?: string;
}) {
  const { data } = await client.request<{
    appointment_id: string;
    status: string;
    cancelled_at?: string;
  }>("/api/v1/appointments/cancel", {
    method: "POST",
    data: payload
  });
  return data;
}

export async function apiGetSubscription() {
  const { data } = await client.request<{
    business_id: string;
    tier: string;
    status: string;
    updated_at: string;
    entitlements?: {
      tier: string;
      max_staff: number;
      max_services: number;
      max_monthly_appointments: number;
    };
  }>("/api/v1/billing/subscription");
  return data;
}

export async function apiCreateCheckout(payload: {
  tier: string;
  success_url?: string;
  cancel_url?: string;
}) {
  const { data } = await client.request<{
    session_id: string;
    url: string;
  }>("/api/v1/billing/checkout", {
    method: "POST",
    data: payload
  });
  return data;
}

export async function apiGetCheckoutSession(sessionId: string) {
  const { data } = await client.request<{
    session_id: string;
    tier: string;
    status: string;
    updated_at?: string;
    completed_at?: string;
    canceled_at?: string;
    expired_at?: string;
  }>("/api/v1/billing/checkout/session", {
    params: { session_id: sessionId }
  });
  return data;
}

export async function apiAckCheckoutSession(payload: {
  session_id: string;
  state: string;
  result: "success" | "cancel";
}) {
  await client.request("/api/v1/billing/checkout/session/ack", {
    method: "POST",
    data: payload
  });
}

export async function apiCancelSubscription(payload?: { business_id?: string }) {
  await client.request("/api/v1/billing/subscription/cancel", {
    method: "POST",
    data: payload ?? {}
  });
}

export async function apiHealthz() {
  const { data } = await client.request<Record<string, unknown>>("/healthz");
  return data;
}

export async function apiReadyz() {
  const { data } = await client.request<Record<string, unknown>>("/readyz");
  return data;
}
