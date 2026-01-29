# Frontend Phases (Next.js + Vite + shadcn/ui)

This document is the step-by-step execution plan to build a modern, scalable frontend for ApptRemind that showcases the backend features end-to-end.

## Progress Log (keep updated)
- 2026-01-28: Frontend workspace bootstrapped (Next.js app + Vite playground + shared UI/api packages).
- 2026-01-28: Added initial design system primitives (Button/Card/Badge), app shell, and error/loading UI.
- 2026-01-28: Added dashboard/public/billing routes with placeholder content and navigation state.
- 2026-01-28: Added auth hydration guard, protected dashboard/billing routes, and fixed login/register redirects.
- 2026-01-28: Added logout route and session-expired banner UX.
- 2026-01-28: Started onboarding wizard (profile/services/staff) and wired business endpoints.
- 2026-01-28: Added availability management UI (working hours + time-off) wired to staff endpoints.
- 2026-01-28: Added availability validation and conflict messaging for time-off + working-hours saves.
- 2026-01-28: Added quick presets for weekday hours and weekend off.
- 2026-01-28: Added copy-from-day shortcuts to propagate working hours.
- 2026-01-28: Started public booking UI with slots lookup + booking form wiring.
- 2026-01-28: Added duration/slot-step inputs and improved slot-loading UX.
- 2026-01-28: Auto-select first slot and added no-availability guidance.
- 2026-01-28: Wired dashboard to live appointments list and service/staff labels.
- 2026-01-28: Replaced UI primitives with official shadcn/ui component implementations.
- 2026-01-28: Standardized shadcn setup (components.json, Tailwind config, globals).
- 2026-01-28: Installed shadcn CLI components in `apps/web` and refactored UI imports to local shadcn components.
- 2026-01-28: Added appointment detail drawer with cancel action on dashboard.
- 2026-01-28: Added dashboard refresh button and cancelled-at display.
- 2026-01-28: Added dashboard filters (date/status) and paging controls.
- 2026-01-28: Added dashboard sorting and CSV export.
- 2026-01-28: Added public booking route `/b/[businessId]/book` and linked it from public demo.
- 2026-01-28: Wired billing UI (subscription view, checkout, cancel, success/cancel pages).
- 2026-01-28: Added public booking success page and validation for slots/booking inputs.
- 2026-01-28: Added status page and guided demo walkthrough.
- 2026-01-28: Added request ID on error screens and demo status shortcuts.
- 2026-01-28: Added Vitest + Playwright scaffolding with sample tests.
- 2026-01-29: Finished dashboard operations (copy booking link, filters, sorting, CSV export).
- 2026-01-29: Ran Playwright smoke tests against the local stack.
- 2026-01-29: Added Playwright full booking flow test (register -> book -> dashboard).
- 2026-01-29: Ran Playwright full flow + smoke suite and Vitest unit tests.
- 2026-01-29: Added GitHub Actions workflow for frontend lint/typecheck/tests/e2e.

Goals:
- Elegant, modern UI with a consistent design system (shadcn/ui + Tailwind).
- Production-grade frontend architecture (typed API client, auth flows, loading/error UX, testing).
- Clear separation between:
  - **Owner dashboard** (authenticated): business setup, staff, availability, appointments, billing.
  - **Public booking** (unauthenticated): slots + booking.

Non-goals (for MVP):
- Full marketing site/SEO content system.
- Multi-business search / discovery (backend currently doesn’t expose a “list businesses” API).

## Repo Layout (Frontend)

Use a small monorepo so parts are reusable in other projects:
- `apps/web` (Next.js App Router): the actual product UI
- `packages/api` (generated types + API client): reusable across other apps
- `packages/ui` (shadcn/ui wrapper + app design tokens): reusable component library
- `packages/config` (eslint/tsconfig/prettier shared configs)
- `apps/ui-playground` (Vite): fast component/dev sandbox for `packages/ui`

Why Vite:
- Next.js is the app runtime, but Vite is great for a “UI playground” and/or building the `packages/ui` library ergonomically.

## Environment Conventions

- Frontend talks to **gateway-service** only.
- Base URL configured with env:
  - `NEXT_PUBLIC_API_BASE_URL` (default: `http://localhost:8080`)
- For local dev, ensure gateway CORS allows the frontend origin (e.g. `http://localhost:3000`).

## Auth Strategy (MVP vs Production)

Backend currently returns tokens in JSON and expects `Authorization: Bearer <token>` for protected routes.

MVP approach (good for learning + local demo):
- Store `access_token` and `refresh_token` in memory + `localStorage`.
- Auto-refresh access token using `/api/v1/auth/refresh` (rotation supported) and update stored tokens.

Production approach (recommended future hardening):
- Use httpOnly cookies for refresh token to reduce XSS risk (requires backend changes).

## API Client Strategy (Typed, Maintainable)

Source of truth: `openapi/gateway.v1.yaml`.

Plan:
- Generate TS types with `openapi-typescript`.
- Use a small fetch wrapper that:
  - injects Authorization header
  - retries once on 401 by running refresh flow
  - attaches `X-Request-Id` (client generated) for traceability
- Keep the generated code in `packages/api` so future projects can reuse it.

## UI/UX Direction (Modern + Scalable)

Use shadcn/ui conventions:
- Tailwind design tokens
- Radix primitives
- `react-hook-form` + `zod` for forms
- `@tanstack/react-query` for server state
- `sonner` (or shadcn toast) for notifications
- `@tanstack/table` for data tables
- `recharts` (or similar) for charts (optional)

## Phases

### Phase F0 - Workspace Bootstrap (Monorepo + Tooling)
Status: DONE

Deliverables:
- Workspace using `pnpm` (or `bun`) with `apps/` + `packages/`.
- `apps/web` initialized with Next.js (App Router, TypeScript, Tailwind).
- `packages/ui` initialized with shadcn/ui conventions.
- `apps/ui-playground` initialized with Vite + React + Tailwind, consuming `packages/ui`.
- `packages/api` with OpenAPI type generation script.
- Shared lint/format config in `packages/config`.

Acceptance:
- `pnpm dev` runs Next app on `localhost:3000`.
- `pnpm ui:dev` runs Vite playground and renders a few shared UI components.
- `pnpm api:gen` generates types from `openapi/gateway.v1.yaml`.

Verify:
- `pnpm -v`
- `pnpm install`
- `pnpm dev`
- `pnpm ui:dev`
- `pnpm api:gen`

---

### Phase F1 - Design System + App Shell
Status: DONE

Deliverables:
- Global layout: top nav + side nav (dashboard), mobile drawer.
- Theme: typography, spacing scale, colors, consistent empty/loading states.
- Reusable components (in `packages/ui`):
  - buttons, inputs, selects, dialogs, sheets
  - data table wrapper
  - form field wrappers
  - skeletons
- Error boundary + “Something went wrong” page.
- Status notes:
  - ✅ App shell layout + mobile nav
  - ✅ Core UI primitives (Button/Card/Badge)
  - ✅ Error + loading boundaries wired
  - ✅ Placeholder routes for dashboard/public/billing

Acceptance:
- App looks “product-like” (not default template).
- Mobile navigation works.
- Consistent loading and error UI.

Verify:
- Visual check on desktop + mobile widths
- Lighthouse baseline (optional)

---

### Phase F2 - API Wiring + Auth
Status: DONE
Deliverables:
- `packages/api`:
  - typed client + request wrapper (`fetch`)
  - request-id injection (client side)
  - auth token injection
  - refresh-on-401 retry
- `apps/web`:
  - auth store (Zustand or Context)
  - pages: login, register, logout
  - route protection for owner pages (middleware or layout gate)
  - “session expired” UX

Acceptance:
- Can register/login and stay logged in across refresh.
- Protected routes redirect to login when unauthenticated.

Verify:
- Login/register flow against gateway
- Call a protected endpoint (appointments list) and render results

---

### Phase F3 - Owner Onboarding Wizard (Business Setup)
Status: DONE
Deliverables:
- Dashboard “Setup Wizard”:
  1) Business profile (timezone, reminder policy)
  2) Services catalog (create/list)
  3) Staff (create/list)
- UX: progressive steps with validation and success states.

Backend integration:
- `PUT /api/v1/business/profile`
- `POST/GET /api/v1/business/services`
- `POST/GET /api/v1/business/staff`

Acceptance:
- A new owner can complete setup without touching curl/postman.

Verify:
- End-to-end setup creates staff + service and shows them in UI.

---

### Phase F4 - Availability Management (Working Hours + Time Off)
Status: DONE
Deliverables:
- Working hours editor (per staff, per weekday)
- Time-off calendar/list + create + delete

Backend integration:
- `GET/PUT /api/v1/business/staff/working-hours`
- `POST/GET/DELETE /api/v1/business/staff/time-off`

Acceptance:
- Editing working hours changes the available slots on the public booking page.
- Time-off blocks slots.

Verify:
- Update working hours and confirm `GET /api/v1/public/slots` changes.

---

### Phase F5 - Public Booking Experience
Status: DONE
Deliverables:
- Public booking route: `/b/[businessId]/book` (shareable link)
- Slot picker UI:
  - date selector
  - slot list
  - “no availability” UX
- Booking form:
  - service selection, staff selection (for demo)
  - customer email/phone
  - confirmation screen
- Cancellation flow (authenticated owner UI can cancel; public cancel optional)

Backend integration:
- `GET /api/v1/public/slots`
- `POST /api/v1/public/book`
- `POST /api/v1/appointments/cancel` (owner path)

Acceptance:
- A user can book and receives a reminder email (Mailpit local).

Verify:
- Book -> wait -> Mailpit shows reminder (or run smoke scripts in parallel).

---

### Phase F6 - Owner Dashboard (Appointments + Operations)
Status: DONE
Deliverables:
- Appointments list (filter by date/status)
- Appointment detail drawer/modal
- Cancel appointment from UI (idempotent)
- “Copy public booking link” for the business

Backend integration:
- `GET /api/v1/appointments`
- `POST /api/v1/appointments/cancel`

Acceptance:
- Owner can manage bookings without CLI.

Verify:
- Create booking from public page, then see it in owner dashboard and cancel it.

---

### Phase F7 - Billing UI (Upgrade + Status + Cancel)
Status: DONE
Deliverables:
- Current plan view + entitlements (starter/pro/free as configured)
- Upgrade CTA (creates checkout session)
- Success/cancel pages (Next routes) that:
  - poll `/api/v1/billing/checkout/session`
  - call `/api/v1/billing/checkout/session/ack`
- Cancel subscription (owner/admin)

Backend integration:
- `GET /api/v1/billing/subscription`
- `POST /api/v1/billing/checkout`
- `GET /api/v1/billing/checkout/session`
- `POST /api/v1/billing/checkout/session/ack`
- `POST /api/v1/billing/subscription/cancel`

Acceptance:
- Owner can start checkout and see status update on return page.

Verify:
- Local webhook smoke (or Stripe test mode) + UI reflects tier changes.

---

### Phase F8 - Observability UX + “Demo Mode”
Status: DONE
Deliverables:
- Display request id on error screens (helps debugging).
- “System status” page:
  - checks `/healthz` and `/readyz`
  - links to Swagger UI and Jaeger
- Demo “guided tour” page that walks through the flows end-to-end.

Acceptance:
- A new person can run the system and understand the architecture + features from the UI alone.

Verify:
- Follow the guided tour without docs/runbook.

---

### Phase F9 - Quality Gates (CI + Tests)
Status: DONE
Deliverables:
- ESLint + Prettier + TypeScript strict
- Component tests (Vitest) for `packages/ui`
- E2E tests (Playwright) for critical flows:
  - register/login
  - business setup
  - public booking
  - billing upgrade (stubbed/local)
- CI job to run lint + typecheck + unit tests + e2e (optional in CI)

Acceptance:
- PR-quality checks catch obvious regressions.

Verify:
- `pnpm lint`
- `pnpm typecheck`
- `pnpm test`
- `pnpm e2e`

## Suggested Backend “Nice-to-Have” Additions (Optional)
- Analytics read endpoints via gateway (read-only):
  - daily appointment metrics
  - daily notification metrics
This would let the frontend show dashboards without direct DB access.
