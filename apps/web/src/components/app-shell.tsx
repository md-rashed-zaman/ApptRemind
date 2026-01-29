"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect } from "react";

import { Button } from "@/components/ui/button";
import { Sheet, SheetContent, SheetTrigger } from "@/components/ui/sheet";
import { useAuthStore } from "../lib/auth-store";

const navItems = [
  { href: "/", label: "Overview" },
  { href: "/dashboard", label: "Dashboard" },
  { href: "/onboarding", label: "Onboarding" },
  { href: "/availability", label: "Availability" },
  { href: "/public-demo", label: "Public Booking" },
  { href: "/status", label: "Status" },
  { href: "/billing", label: "Billing" }
];

export function AppShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const { user, hydrate, hydrated, logout, error } = useAuthStore();

  useEffect(() => {
    if (!hydrated) {
      hydrate();
    }
  }, [hydrate, hydrated]);
  return (
    <div className="min-h-screen">
      <div className="mx-auto flex min-h-screen w-full max-w-6xl flex-col gap-6 px-6 py-6 md:flex-row md:py-8">
        <div className="flex items-center justify-between rounded-xl border border-border bg-card px-4 py-3 shadow-sm md:hidden">
          <div>
            <p className="text-xs uppercase tracking-wider text-muted-foreground">
              ApptRemind
            </p>
            <h2 className="text-base font-semibold">Admin</h2>
          </div>
          <Sheet>
            <SheetTrigger asChild>
              <Button variant="outline" size="sm">
                Menu
              </Button>
            </SheetTrigger>
            <SheetContent side="right" className="w-80">
              <div className="mb-4">
                <p className="text-xs uppercase tracking-wider text-muted-foreground">
                  Navigation
                </p>
              </div>
              <nav className="flex flex-col gap-2">
                {navItems.map((item) => {
                  const active = pathname === item.href;
                  return (
                    <Link
                      key={item.href}
                      className={`rounded-md px-3 py-2 text-sm transition ${
                        active
                          ? "bg-accent text-foreground"
                          : "text-muted-foreground hover:bg-accent hover:text-foreground"
                      }`}
                      href={item.href}
                    >
                      {item.label}
                    </Link>
                  );
                })}
              </nav>
              <div className="mt-6 border-t pt-4">
                <p className="text-xs text-muted-foreground">Docs</p>
                <div className="mt-2 flex flex-col gap-2">
                  <a
                    className="text-sm text-foreground hover:underline"
                    href="http://localhost:8088/docs"
                    target="_blank"
                    rel="noreferrer"
                  >
                    Swagger UI
                  </a>
                  <a
                    className="text-sm text-muted-foreground hover:underline"
                    href="http://localhost:16686"
                    target="_blank"
                    rel="noreferrer"
                  >
                    Jaeger
                  </a>
                </div>
              </div>
            </SheetContent>
          </Sheet>
        </div>

        <aside className="hidden w-56 shrink-0 flex-col gap-4 md:flex">
          <div className="rounded-xl border border-border bg-card p-4 shadow-sm">
            <p className="text-xs uppercase tracking-wider text-muted-foreground">
              ApptRemind
            </p>
            <h2 className="mt-2 text-lg font-semibold">Control Panel</h2>
          </div>
          <nav className="flex flex-col gap-2 rounded-xl border border-border bg-card p-3 shadow-sm">
            {navItems.map((item) => {
              const active = pathname === item.href;
              return (
                <Link
                  key={item.href}
                  className={`rounded-md px-3 py-2 text-sm transition ${
                    active
                      ? "bg-accent text-foreground"
                      : "text-muted-foreground hover:bg-accent hover:text-foreground"
                  }`}
                  href={item.href}
                >
                  {item.label}
                </Link>
              );
            })}
          </nav>
          <div className="rounded-xl border border-border bg-card p-4 shadow-sm">
            <p className="text-xs text-muted-foreground">Docs</p>
            <div className="mt-2 flex flex-col gap-2">
              <a
                className="text-sm text-foreground hover:underline"
                href="http://localhost:8088/docs"
                target="_blank"
                rel="noreferrer"
              >
                Swagger UI
              </a>
              <a
                className="text-sm text-muted-foreground hover:underline"
                href="http://localhost:16686"
                target="_blank"
                rel="noreferrer"
              >
                Jaeger
              </a>
            </div>
          </div>
        </aside>

        <main className="flex-1">
          <header className="flex flex-col gap-4 rounded-2xl border border-border bg-card p-6 shadow-sm md:flex-row md:items-center md:justify-between">
            <div>
              <p className="text-xs uppercase tracking-wider text-muted-foreground">
                Production Demo
              </p>
              <h1 className="text-2xl font-semibold tracking-tight">
                ApptRemind Admin
              </h1>
              <p className="text-sm text-muted-foreground">
                Manage services, staff, bookings, and billing in one place.
              </p>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              {user ? (
                <>
                  <div className="text-right text-xs text-muted-foreground">
                    <div>{user.email}</div>
                    <div className="uppercase">{user.role}</div>
                  </div>
                  <Button variant="outline" onClick={logout}>
                    Sign out
                  </Button>
                </>
              ) : (
                <>
                  <Link href="/auth/login">
                    <Button variant="outline">Sign In</Button>
                  </Link>
                  <Link href="/auth/register">
                    <Button>Register</Button>
                  </Link>
                </>
              )}
            </div>
          </header>

          {error ? (
            <div className="mt-4 rounded-xl border border-destructive/40 bg-destructive/10 px-4 py-3 text-sm text-destructive">
              {error}. Please sign in again.
            </div>
          ) : null}

          <section className="mt-6">{children}</section>
        </main>
      </div>
    </div>
  );
}
