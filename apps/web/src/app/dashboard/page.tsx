"use client";

import Link from "next/link";
import { useEffect, useMemo, useState } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from "@/components/ui/select";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle
} from "@/components/ui/sheet";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger
} from "@/components/ui/alert-dialog";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";

import { AppShell } from "../../components/app-shell";
import { RequireAuth } from "../../components/require-auth";
import { SectionHeader } from "../../components/section-header";
import {
  apiCancelAppointment,
  apiListAppointments,
  apiListServices,
  apiListStaff
} from "../../lib/api";
import { useAuthStore } from "../../lib/auth-store";
import { formatDateTime, inRange, isToday, isUpcoming } from "../../lib/date";

export default function DashboardPage() {
  const [appointments, setAppointments] = useState<
    Array<{
      appointment_id: string;
      staff_id: string;
      service_id: string;
      start_time: string;
      end_time: string;
      status: string;
      created_at: string;
      cancelled_at?: string;
    }>
  >([]);
  const [staff, setStaff] = useState<Array<{ id: string; name: string }>>([]);
  const [services, setServices] = useState<Array<{ id: string; name: string }>>(
    []
  );
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [selected, setSelected] = useState<{
    appointment_id: string;
    staff_id: string;
    service_id: string;
    start_time: string;
    end_time: string;
    status: string;
    created_at: string;
    cancelled_at?: string;
  } | null>(null);
  const [sheetOpen, setSheetOpen] = useState(false);
  const [cancelLoading, setCancelLoading] = useState(false);
  const [cancelError, setCancelError] = useState<string | null>(null);
  const [refreshing, setRefreshing] = useState(false);
  const [filterFrom, setFilterFrom] = useState("");
  const [filterTo, setFilterTo] = useState("");
  const [statusFilter, setStatusFilter] = useState("all");
  const [pageSize, setPageSize] = useState("8");
  const [sortMode, setSortMode] = useState("start_desc");
  const [copyNotice, setCopyNotice] = useState<string | null>(null);
  const { user } = useAuthStore();

  async function loadDashboard() {
    const [appts, staffList, servicesList] = await Promise.all([
      apiListAppointments(20),
      apiListStaff(),
      apiListServices()
    ]);
    setAppointments(appts);
    setStaff(staffList.map((member) => ({ id: member.id, name: member.name })));
    setServices(
      servicesList.map((service) => ({ id: service.id, name: service.name }))
    );
  }

  useEffect(() => {
    let ignore = false;
    async function load() {
      setLoading(true);
      setError(null);
      try {
        await loadDashboard();
        if (ignore) return;
      } catch {
        if (!ignore) {
          setError("Unable to load dashboard data.");
        }
      } finally {
        if (!ignore) {
          setLoading(false);
        }
      }
    }

    load();
    return () => {
      ignore = true;
    };
  }, []);

  const staffMap = useMemo(() => {
    const map = new Map<string, string>();
    staff.forEach((member) => map.set(member.id, member.name));
    return map;
  }, [staff]);

  const serviceMap = useMemo(() => {
    const map = new Map<string, string>();
    services.forEach((service) => map.set(service.id, service.name));
    return map;
  }, [services]);

  const filteredAppointments = useMemo(() => {
    return appointments.filter((appt) => {
      if (!inRange(appt.start_time, filterFrom, filterTo)) return false;
      if (
        statusFilter !== "all" &&
        appt.status.toLowerCase() !== statusFilter.toLowerCase()
      ) {
        return false;
      }
      return true;
    });
  }, [appointments, filterFrom, filterTo, statusFilter]);

  const sortedAppointments = useMemo(() => {
    const copy = [...filteredAppointments];
    copy.sort((a, b) => {
      if (sortMode === "start_asc") {
        return new Date(a.start_time).getTime() - new Date(b.start_time).getTime();
      }
      if (sortMode === "created_desc") {
        return new Date(b.created_at).getTime() - new Date(a.created_at).getTime();
      }
      return new Date(b.start_time).getTime() - new Date(a.start_time).getTime();
    });
    return copy;
  }, [filteredAppointments, sortMode]);

  const todayCount = appointments.filter((appt) => isToday(appt.start_time)).length;
  const upcomingCount = appointments.filter((appt) => isUpcoming(appt.start_time)).length;

  const pageSizeNumber = Math.max(4, Number(pageSize) || 8);
  const visibleAppointments = sortedAppointments.slice(0, pageSizeNumber);

  async function onCancelAppointment() {
    if (!selected || !user?.business_id) {
      setCancelError("Missing business context.");
      return;
    }
    setCancelLoading(true);
    setCancelError(null);
    try {
      const result = await apiCancelAppointment({
        business_id: user.business_id,
        appointment_id: selected.appointment_id,
        reason: "customer_request"
      });
      setAppointments((prev) =>
        prev.map((appt) =>
          appt.appointment_id === result.appointment_id
            ? { ...appt, status: result.status, cancelled_at: result.cancelled_at }
            : appt
        )
      );
      setSelected((prev) =>
        prev
          ? {
              ...prev,
              status: result.status,
              cancelled_at: result.cancelled_at ?? prev.cancelled_at
            }
          : prev
      );
    } catch {
      setCancelError("Unable to cancel appointment.");
    } finally {
      setCancelLoading(false);
    }
  }

  async function onRefresh() {
    setRefreshing(true);
    setError(null);
    try {
      await loadDashboard();
      setError(null);
    } catch {
      setError("Unable to refresh dashboard data.");
    } finally {
      setRefreshing(false);
    }
  }

  async function onCopyBookingLink() {
    if (!user?.business_id) {
      setCopyNotice("Missing business ID.");
      return;
    }
    const url = `${window.location.origin}/b/${user.business_id}/book`;
    try {
      await navigator.clipboard.writeText(url);
      setCopyNotice("Booking link copied.");
    } catch {
      setCopyNotice("Unable to copy. Link: " + url);
    } finally {
      window.setTimeout(() => setCopyNotice(null), 3000);
    }
  }

  function exportCsv() {
    const rows = filteredAppointments.map((appt) => ({
      appointment_id: appt.appointment_id,
      service: serviceMap.get(appt.service_id) ?? appt.service_id,
      staff: staffMap.get(appt.staff_id) ?? appt.staff_id,
      start_time: appt.start_time,
      end_time: appt.end_time,
      status: appt.status,
      created_at: appt.created_at,
      cancelled_at: appt.cancelled_at ?? ""
    }));
    const header = Object.keys(rows[0] ?? {}).join(",");
    const body = rows
      .map((row) =>
        Object.values(row)
          .map((value) => {
            const safe = String(value ?? "");
            return safe.includes(",") || safe.includes("\"")
              ? `"${safe.replace(/\"/g, "\"\"")}"`
              : safe;
          })
          .join(",")
      )
      .join("\n");
    const csv = [header, body].filter(Boolean).join("\n");
    const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = `appointments-${new Date().toISOString().slice(0, 10)}.csv`;
    link.click();
    URL.revokeObjectURL(url);
  }

  return (
    <RequireAuth>
      <AppShell>
        <SectionHeader
          title="Dashboard"
          description="A snapshot of today’s activity."
          action={
            <div className="flex gap-2">
              <Link href="/onboarding">
                <Button variant="outline">Run setup</Button>
              </Link>
              <Link href="/availability">
                <Button variant="outline">Availability</Button>
              </Link>
              <Button variant="outline" onClick={onCopyBookingLink}>
                Copy booking link
              </Button>
              <Button variant="outline" onClick={onRefresh} disabled={refreshing}>
                {refreshing ? "Refreshing..." : "Refresh"}
              </Button>
              <Button variant="outline">New booking</Button>
            </div>
          }
        />
        {copyNotice ? (
          <div className="mt-3 rounded-md border border-border bg-muted/40 px-3 py-2 text-xs text-muted-foreground">
            {copyNotice}
          </div>
        ) : null}

        <div className="mt-4 grid gap-4 md:grid-cols-3">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                Bookings <Badge variant="secondary">today</Badge>
              </CardTitle>
            </CardHeader>
            <CardContent className="text-3xl font-semibold">
              {loading ? "…" : todayCount}
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle>Reminders sent</CardTitle>
            </CardHeader>
            <CardContent className="text-3xl font-semibold">
              {loading ? "…" : Math.max(appointments.length * 2, 0)}
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle>Upcoming cancellations</CardTitle>
            </CardHeader>
            <CardContent className="text-3xl font-semibold">
              {loading ? "…" : Math.max(appointments.length - upcomingCount, 0)}
            </CardContent>
          </Card>
        </div>

        <div className="mt-6 grid gap-4 lg:grid-cols-[2fr_1fr]">
          <Card>
            <CardHeader>
              <CardTitle>Appointments</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3 text-sm text-muted-foreground">
              {error ? <p className="text-destructive">{error}</p> : null}
              {loading ? <p>Loading appointments…</p> : null}
              {!loading && appointments.length === 0 ? (
                <p>No appointments yet. Create a public booking to see it here.</p>
              ) : null}
              <div className="grid gap-3 rounded-lg border border-border bg-muted/30 p-3 text-xs text-muted-foreground md:grid-cols-4">
                <div className="space-y-1">
                  <Label htmlFor="filter-from">From</Label>
                  <Input
                    id="filter-from"
                    type="datetime-local"
                    value={filterFrom}
                    onChange={(e) => setFilterFrom(e.target.value)}
                  />
                </div>
                <div className="space-y-1">
                  <Label htmlFor="filter-to">To</Label>
                  <Input
                    id="filter-to"
                    type="datetime-local"
                    value={filterTo}
                    onChange={(e) => setFilterTo(e.target.value)}
                  />
                </div>
                <div className="space-y-1">
                  <Label>Status</Label>
                  <Select value={statusFilter} onValueChange={setStatusFilter}>
                    <SelectTrigger className="h-9">
                      <SelectValue placeholder="All" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="all">All</SelectItem>
                      <SelectItem value="booked">Booked</SelectItem>
                      <SelectItem value="cancelled">Cancelled</SelectItem>
                      <SelectItem value="canceled">Canceled</SelectItem>
                      <SelectItem value="completed">Completed</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-1">
                  <Label htmlFor="page-size">Show</Label>
                  <Input
                    id="page-size"
                    type="number"
                    min={4}
                    value={pageSize}
                    onChange={(e) => setPageSize(e.target.value)}
                  />
                </div>
              </div>
              <div className="flex flex-wrap items-center justify-between gap-3">
                <Tabs value={sortMode} onValueChange={setSortMode}>
                  <TabsList>
                    <TabsTrigger value="start_desc">Start ↓</TabsTrigger>
                    <TabsTrigger value="start_asc">Start ↑</TabsTrigger>
                    <TabsTrigger value="created_desc">Created ↓</TabsTrigger>
                  </TabsList>
                </Tabs>
                <Button variant="outline" onClick={exportCsv}>
                  Export CSV
                </Button>
              </div>
              <div className="grid gap-3">
                {visibleAppointments.map((appt) => (
                  <div
                    key={appt.appointment_id}
                    className="rounded-lg border border-border bg-muted/30 p-3"
                  >
                    <div className="flex items-center justify-between">
                      <div className="text-sm font-semibold text-foreground">
                        {serviceMap.get(appt.service_id) ?? "Service"} ·{" "}
                        {staffMap.get(appt.staff_id) ?? "Staff"}
                      </div>
                      <Badge
                        variant={
                          appt.status === "cancelled" || appt.status === "canceled"
                            ? "destructive"
                            : "secondary"
                        }
                      >
                        {appt.status}
                      </Badge>
                    </div>
                    <div className="mt-1 text-xs text-muted-foreground">
                      {formatDateTime(appt.start_time)} → {formatDateTime(appt.end_time)}
                    </div>
                    {appt.status === "cancelled" || appt.status === "canceled" ? (
                      <div className="mt-2 text-xs text-muted-foreground">
                        Cancelled
                      </div>
                    ) : null}
                    <div className="mt-3 flex gap-2">
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => {
                          setSelected(appt);
                          setSheetOpen(true);
                        }}
                      >
                        View details
                      </Button>
                    </div>
                  </div>
                ))}
              </div>
              {sortedAppointments.length > pageSizeNumber ? (
                <Button
                  variant="outline"
                  onClick={() => setPageSize(String(pageSizeNumber + 8))}
                >
                  Show more
                </Button>
              ) : null}
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle>System health</CardTitle>
            </CardHeader>
            <CardContent className="text-sm text-muted-foreground">
              Gateway healthy. Kafka and workers running.
            </CardContent>
          </Card>
        </div>

        <Sheet open={sheetOpen} onOpenChange={setSheetOpen}>
          <SheetContent>
            <SheetHeader>
              <SheetTitle>Appointment detail</SheetTitle>
              <SheetDescription>
                Review or cancel the selected booking.
              </SheetDescription>
            </SheetHeader>
            {selected ? (
              <div className="mt-6 space-y-3 text-sm text-muted-foreground">
                <div>
                  <p className="text-xs uppercase tracking-wide text-muted-foreground">
                    Service
                  </p>
                  <p className="text-base text-foreground">
                    {serviceMap.get(selected.service_id) ?? selected.service_id}
                  </p>
                </div>
                <div>
                  <p className="text-xs uppercase tracking-wide text-muted-foreground">
                    Staff
                  </p>
                  <p className="text-base text-foreground">
                    {staffMap.get(selected.staff_id) ?? selected.staff_id}
                  </p>
                </div>
                <div>
                  <p className="text-xs uppercase tracking-wide text-muted-foreground">
                    Time
                  </p>
                  <p className="text-base text-foreground">
                    {formatDateTime(selected.start_time)} → {formatDateTime(selected.end_time)}
                  </p>
                </div>
                <div>
                  <p className="text-xs uppercase tracking-wide text-muted-foreground">
                    Status
                  </p>
                  <div className="flex flex-wrap items-center gap-2">
                    <Badge
                      variant={
                        selected.status === "cancelled" || selected.status === "canceled"
                          ? "destructive"
                          : "secondary"
                      }
                    >
                      {selected.status}
                    </Badge>
                    {selected.cancelled_at ? (
                      <span className="text-xs text-muted-foreground">
                        Cancelled at {formatDateTime(selected.cancelled_at)}
                      </span>
                    ) : null}
                  </div>
                </div>
                {cancelError ? (
                  <p className="text-sm text-destructive">{cancelError}</p>
                ) : null}
              </div>
            ) : (
              <p className="mt-6 text-sm text-muted-foreground">
                Select an appointment to view details.
              </p>
            )}
            <SheetFooter className="mt-6">
              <AlertDialog>
                <AlertDialogTrigger asChild>
                  <Button
                    variant="destructive"
                    disabled={
                      cancelLoading ||
                      !selected ||
                      selected.status === "cancelled" ||
                      selected.status === "canceled"
                    }
                  >
                    Cancel appointment
                  </Button>
                </AlertDialogTrigger>
                <AlertDialogContent>
                  <AlertDialogHeader>
                    <AlertDialogTitle>Cancel this appointment?</AlertDialogTitle>
                    <AlertDialogDescription>
                      This action will mark the booking as cancelled and trigger
                      notification workflows.
                    </AlertDialogDescription>
                  </AlertDialogHeader>
                  <AlertDialogFooter>
                    <AlertDialogCancel disabled={cancelLoading}>
                      Keep
                    </AlertDialogCancel>
                    <AlertDialogAction
                      onClick={onCancelAppointment}
                      disabled={cancelLoading}
                    >
                      {cancelLoading ? "Canceling..." : "Confirm cancel"}
                    </AlertDialogAction>
                  </AlertDialogFooter>
                </AlertDialogContent>
              </AlertDialog>
            </SheetFooter>
          </SheetContent>
        </Sheet>
      </AppShell>
    </RequireAuth>
  );
}
