"use client";

import { useEffect, useMemo, useState } from "react";

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
import { AppShell } from "../../components/app-shell";
import { RequireAuth } from "../../components/require-auth";
import { SectionHeader } from "../../components/section-header";
import {
  apiCreateTimeOff,
  apiDeleteTimeOff,
  apiListStaff,
  apiListTimeOff,
  apiListWorkingHours,
  apiUpsertWorkingHours
} from "../../lib/api";

const weekdays = [
  { value: 0, label: "Sunday" },
  { value: 1, label: "Monday" },
  { value: 2, label: "Tuesday" },
  { value: 3, label: "Wednesday" },
  { value: 4, label: "Thursday" },
  { value: 5, label: "Friday" },
  { value: 6, label: "Saturday" }
];

function minutesToTime(value: number) {
  const hours = Math.floor(value / 60);
  const minutes = value % 60;
  const pad = (num: number) => String(num).padStart(2, "0");
  return `${pad(hours)}:${pad(minutes)}`;
}

function timeToMinutes(value: string) {
  const [h, m] = value.split(":").map((part) => Number(part));
  if (Number.isNaN(h) || Number.isNaN(m)) return 0;
  return h * 60 + m;
}

function formatError(err: unknown, fallback: string) {
  if (!err || typeof err !== "object") return fallback;
  const maybe = err as { response?: { status?: number; data?: { message?: string } } };
  const status = maybe.response?.status;
  const message = maybe.response?.data?.message;
  if (status === 409) return "Conflict: time range overlaps an existing entry.";
  if (message) return message;
  return fallback;
}

function toLocalDateTimeValue(date: Date) {
  const pad = (num: number) => String(num).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(
    date.getDate()
  )}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

function addDays(date: Date, days: number) {
  const next = new Date(date);
  next.setDate(next.getDate() + days);
  return next;
}

type WorkingHoursRow = {
  weekday: number;
  is_working: boolean;
  start_minute: number;
  end_minute: number;
};

export default function AvailabilityPage() {
  const [staff, setStaff] = useState<
    Array<{ id: string; name: string; is_active: boolean }>
  >([]);
  const [staffId, setStaffId] = useState<string>("");

  const [hours, setHours] = useState<Record<number, WorkingHoursRow>>({});
  const [hoursLoading, setHoursLoading] = useState(false);
  const [hoursError, setHoursError] = useState<string | null>(null);
  const [hoursNotice, setHoursNotice] = useState<string | null>(null);
  const [quickPresetLoading, setQuickPresetLoading] = useState(false);

  const [timeOff, setTimeOff] = useState<
    Array<{
      id: string;
      start_time: string;
      end_time: string;
      reason?: string;
    }>
  >([]);
  const [timeOffLoading, setTimeOffLoading] = useState(false);
  const [timeOffError, setTimeOffError] = useState<string | null>(null);

  const [rangeFrom, setRangeFrom] = useState(
    toLocalDateTimeValue(addDays(new Date(), -7))
  );
  const [rangeTo, setRangeTo] = useState(
    toLocalDateTimeValue(addDays(new Date(), 30))
  );

  const [newStart, setNewStart] = useState(
    toLocalDateTimeValue(addDays(new Date(), 1))
  );
  const [newEnd, setNewEnd] = useState(
    toLocalDateTimeValue(addDays(new Date(), 1))
  );
  const [newReason, setNewReason] = useState("");

  const selectedStaff = useMemo(
    () => staff.find((member) => member.id === staffId),
    [staff, staffId]
  );

  useEffect(() => {
    let ignore = false;
    async function loadStaff() {
      try {
        const list = await apiListStaff();
        if (ignore) return;
        setStaff(list);
        if (list.length > 0) {
          setStaffId(list[0].id);
        }
      } catch {
        if (!ignore) {
          setStaff([]);
        }
      }
    }

    loadStaff();
    return () => {
      ignore = true;
    };
  }, []);

  useEffect(() => {
    if (!staffId) return;
    let ignore = false;
    async function loadHours() {
      setHoursLoading(true);
      setHoursError(null);
      setHoursNotice(null);
      try {
        const list = await apiListWorkingHours(staffId);
        if (ignore) return;
        const next: Record<number, WorkingHoursRow> = {};
        list.forEach((row) => {
          next[row.weekday] = row;
        });
        weekdays.forEach((day) => {
          if (!next[day.value]) {
            next[day.value] = {
              weekday: day.value,
              is_working: false,
              start_minute: 9 * 60,
              end_minute: 17 * 60
            };
          }
        });
        setHours(next);
      } catch {
        if (!ignore) {
          setHoursError("Unable to load working hours");
        }
      } finally {
        if (!ignore) {
          setHoursLoading(false);
        }
      }
    }

    loadHours();
    return () => {
      ignore = true;
    };
  }, [staffId]);

  useEffect(() => {
    if (!staffId) return;
    let ignore = false;
    async function loadTimeOff() {
      setTimeOffLoading(true);
      setTimeOffError(null);
      try {
        const list = await apiListTimeOff(
          staffId,
          new Date(rangeFrom).toISOString(),
          new Date(rangeTo).toISOString()
        );
        if (!ignore) {
          setTimeOff(list);
        }
      } catch {
        if (!ignore) {
          setTimeOffError("Unable to load time-off");
        }
      } finally {
        if (!ignore) {
          setTimeOffLoading(false);
        }
      }
    }

    loadTimeOff();
    return () => {
      ignore = true;
    };
  }, [rangeFrom, rangeTo, staffId]);

  async function onSaveHours() {
    if (!staffId) return;
    setHoursLoading(true);
    setHoursError(null);
    setHoursNotice(null);
    const rows = weekdays.map((day) => hours[day.value]);
    const invalid = rows.some((row) => {
      if (!row || !row.is_working) return false;
      return row.start_minute >= row.end_minute;
    });
    if (invalid) {
      setHoursError("Start time must be earlier than end time.");
      setHoursLoading(false);
      return;
    }
    try {
      for (const day of weekdays) {
        const row = hours[day.value];
        if (!row) continue;
        await apiUpsertWorkingHours(staffId, {
          weekday: row.weekday,
          is_working: row.is_working,
          start_minute: row.start_minute,
          end_minute: row.end_minute
        });
      }
      setHoursNotice("Working hours saved.");
    } catch (err) {
      setHoursError(formatError(err, "Unable to save working hours"));
    } finally {
      setHoursLoading(false);
    }
  }

  async function applyPreset(preset: "weekday" | "weekend") {
    if (!staffId) return;
    setQuickPresetLoading(true);
    setHoursError(null);
    setHoursNotice(null);
    const next = { ...hours };
    weekdays.forEach((day) => {
      const isWeekday = day.value >= 1 && day.value <= 5;
      if (preset === "weekday") {
        next[day.value] = {
          weekday: day.value,
          is_working: isWeekday,
          start_minute: 9 * 60,
          end_minute: 17 * 60
        };
      } else {
        next[day.value] = {
          weekday: day.value,
          is_working: isWeekday,
          start_minute: 9 * 60,
          end_minute: 17 * 60
        };
      }
      if (preset === "weekend" && (day.value === 0 || day.value === 6)) {
        next[day.value] = {
          weekday: day.value,
          is_working: false,
          start_minute: 9 * 60,
          end_minute: 17 * 60
        };
      }
    });
    setHours(next);
    try {
      for (const day of weekdays) {
        const row = next[day.value];
        if (!row) continue;
        await apiUpsertWorkingHours(staffId, {
          weekday: row.weekday,
          is_working: row.is_working,
          start_minute: row.start_minute,
          end_minute: row.end_minute
        });
      }
      setHoursNotice(
        preset === "weekday"
          ? "Weekday 9–5 preset applied."
          : "Weekend off preset applied."
      );
    } catch (err) {
      setHoursError(formatError(err, "Unable to apply preset"));
    } finally {
      setQuickPresetLoading(false);
    }
  }

  function copyFromDay(sourceDay: number, targetDays: number[]) {
    const source = hours[sourceDay];
    if (!source) return;
    const next = { ...hours };
    targetDays.forEach((day) => {
      next[day] = {
        weekday: day,
        is_working: source.is_working,
        start_minute: source.start_minute,
        end_minute: source.end_minute
      };
    });
    setHours(next);
    setHoursNotice("Copied hours. Remember to save.");
  }

  async function onCreateTimeOff(e: React.FormEvent) {
    e.preventDefault();
    if (!staffId) return;
    setTimeOffLoading(true);
    setTimeOffError(null);
    const start = new Date(newStart);
    const end = new Date(newEnd);
    if (Number.isNaN(start.getTime()) || Number.isNaN(end.getTime())) {
      setTimeOffError("Start and end times are required.");
      setTimeOffLoading(false);
      return;
    }
    if (start >= end) {
      setTimeOffError("Start time must be earlier than end time.");
      setTimeOffLoading(false);
      return;
    }
    try {
      await apiCreateTimeOff(staffId, {
        start_time: start.toISOString(),
        end_time: end.toISOString(),
        reason: newReason || undefined
      });
      const list = await apiListTimeOff(
        staffId,
        new Date(rangeFrom).toISOString(),
        new Date(rangeTo).toISOString()
      );
      setTimeOff(list);
      setNewReason("");
    } catch (err) {
      setTimeOffError(formatError(err, "Unable to create time-off entry"));
    } finally {
      setTimeOffLoading(false);
    }
  }

  async function onDeleteTimeOff(id: string) {
    if (!staffId) return;
    setTimeOffLoading(true);
    setTimeOffError(null);
    try {
      await apiDeleteTimeOff(id);
      const list = await apiListTimeOff(
        staffId,
        new Date(rangeFrom).toISOString(),
        new Date(rangeTo).toISOString()
      );
      setTimeOff(list);
    } catch (err) {
      setTimeOffError(formatError(err, "Unable to delete time-off"));
    } finally {
      setTimeOffLoading(false);
    }
  }

  return (
    <RequireAuth>
      <AppShell>
        <SectionHeader
          title="Availability"
          description="Define weekly working hours and manage staff time-off."
          action={
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <span>Staff</span>
              <Select value={staffId} onValueChange={setStaffId}>
                <SelectTrigger className="h-9 w-48">
                  <SelectValue placeholder="Select staff" />
                </SelectTrigger>
                <SelectContent>
                  {staff.map((member) => (
                    <SelectItem key={member.id} value={member.id}>
                      {member.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          }
        />

        <div className="mt-6 grid gap-4 lg:grid-cols-[2fr_1fr]">
          <Card>
            <CardHeader>
              <CardTitle>Weekly working hours</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              {selectedStaff ? (
                <p className="text-sm text-muted-foreground">
                  Editing availability for{" "}
                  <span className="font-semibold text-foreground">
                    {selectedStaff.name}
                  </span>
                  .
                </p>
              ) : null}

              <div className="grid gap-3">
                {weekdays.map((day) => {
                  const row = hours[day.value];
                  const startValue = minutesToTime(row?.start_minute ?? 540);
                  const endValue = minutesToTime(row?.end_minute ?? 1020);
                  return (
                    <div
                      key={day.value}
                      className="grid items-center gap-2 rounded-lg border border-border p-3 md:grid-cols-[120px_120px_1fr_1fr]"
                    >
                      <span className="text-sm font-medium">{day.label}</span>
                      <label className="flex items-center gap-2 text-xs text-muted-foreground">
                        <input
                          type="checkbox"
                          checked={row?.is_working ?? false}
                          onChange={(e) =>
                            setHours((prev) => ({
                              ...prev,
                              [day.value]: {
                                ...(row ?? {
                                  weekday: day.value,
                                  start_minute: 540,
                                  end_minute: 1020
                                }),
                                is_working: e.target.checked
                              }
                            }))
                          }
                        />
                        Working
                      </label>
                      <Input
                        type="time"
                        value={startValue}
                        disabled={!row?.is_working}
                        onChange={(e) =>
                          setHours((prev) => ({
                            ...prev,
                            [day.value]: {
                              ...(row ?? {
                                weekday: day.value,
                                is_working: true,
                                end_minute: 1020
                              }),
                              start_minute: timeToMinutes(e.target.value)
                            }
                          }))
                        }
                      />
                      <Input
                        type="time"
                        value={endValue}
                        disabled={!row?.is_working}
                        onChange={(e) =>
                          setHours((prev) => ({
                            ...prev,
                            [day.value]: {
                              ...(row ?? {
                                weekday: day.value,
                                is_working: true,
                                start_minute: 540
                              }),
                              end_minute: timeToMinutes(e.target.value)
                            }
                          }))
                        }
                      />
                    </div>
                  );
                })}
              </div>

              {hoursError ? (
                <p className="text-sm text-destructive">{hoursError}</p>
              ) : null}
              {hoursNotice ? (
                <p className="text-sm text-emerald-600">{hoursNotice}</p>
              ) : null}

              <div className="flex flex-wrap gap-2">
                <Button
                  variant="outline"
                  onClick={() => applyPreset("weekday")}
                  disabled={hoursLoading || quickPresetLoading}
                >
                  {quickPresetLoading ? "Applying..." : "Weekdays 9–5"}
                </Button>
                <Button
                  variant="outline"
                  onClick={() => applyPreset("weekend")}
                  disabled={hoursLoading || quickPresetLoading}
                >
                  {quickPresetLoading ? "Applying..." : "Weekend off"}
                </Button>
                <Button
                  variant="outline"
                  onClick={() => copyFromDay(1, [1, 2, 3, 4, 5])}
                  disabled={hoursLoading || quickPresetLoading}
                >
                  Copy Monday → Weekdays
                </Button>
                <Button
                  variant="outline"
                  onClick={() => copyFromDay(1, [0, 1, 2, 3, 4, 5, 6])}
                  disabled={hoursLoading || quickPresetLoading}
                >
                  Copy Monday → All days
                </Button>
                <Button onClick={onSaveHours} disabled={hoursLoading || quickPresetLoading}>
                  {hoursLoading ? "Saving..." : "Save working hours"}
                </Button>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Notes</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3 text-sm text-muted-foreground">
              <p>
                Working hours drive the public slot search. Keep them accurate to
                avoid false availability.
              </p>
              <p>
                Use time-off to block specific days (vacation, meetings, sick
                leave).
              </p>
            </CardContent>
          </Card>
        </div>

        <div className="mt-6 grid gap-4 lg:grid-cols-[2fr_1fr]">
          <Card>
            <CardHeader>
              <CardTitle>Time-off (blackouts)</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid gap-3 md:grid-cols-2">
                <div className="space-y-2">
                  <Label htmlFor="range-from">From</Label>
                  <Input
                    id="range-from"
                    type="datetime-local"
                    value={rangeFrom}
                    onChange={(e) => setRangeFrom(e.target.value)}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="range-to">To</Label>
                  <Input
                    id="range-to"
                    type="datetime-local"
                    value={rangeTo}
                    onChange={(e) => setRangeTo(e.target.value)}
                  />
                </div>
              </div>

              <form className="grid gap-3" onSubmit={onCreateTimeOff}>
                <div className="grid gap-3 md:grid-cols-2">
                  <div className="space-y-2">
                    <Label htmlFor="new-start">Start time</Label>
                    <Input
                      id="new-start"
                      type="datetime-local"
                      value={newStart}
                      onChange={(e) => setNewStart(e.target.value)}
                      required
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="new-end">End time</Label>
                    <Input
                      id="new-end"
                      type="datetime-local"
                      value={newEnd}
                      onChange={(e) => setNewEnd(e.target.value)}
                      required
                    />
                  </div>
                </div>
                <div className="space-y-2">
                  <Label htmlFor="new-reason">Reason (optional)</Label>
                  <Input
                    id="new-reason"
                    value={newReason}
                    onChange={(e) => setNewReason(e.target.value)}
                    placeholder="Vacation, meeting, sick leave"
                  />
                </div>
                {timeOffError ? (
                  <p className="text-sm text-destructive">{timeOffError}</p>
                ) : null}
                <Button type="submit" disabled={timeOffLoading}>
                  {timeOffLoading ? "Adding..." : "Add time-off"}
                </Button>
              </form>

              {timeOffLoading && timeOff.length === 0 ? (
                <p className="text-sm text-muted-foreground">Loading...</p>
              ) : null}

              <div className="grid gap-3 md:grid-cols-2">
                {timeOff.map((entry) => (
                  <Card key={entry.id}>
                    <CardHeader>
                      <CardTitle className="text-base">
                        {new Date(entry.start_time).toLocaleString()} →{" "}
                        {new Date(entry.end_time).toLocaleString()}
                      </CardTitle>
                    </CardHeader>
                    <CardContent className="space-y-2 text-sm text-muted-foreground">
                      <p>{entry.reason ?? "No reason provided"}</p>
                      <AlertDialog>
                        <AlertDialogTrigger asChild>
                          <Button variant="outline" disabled={timeOffLoading}>
                            Remove
                          </Button>
                        </AlertDialogTrigger>
                        <AlertDialogContent>
                          <AlertDialogHeader>
                            <AlertDialogTitle>Remove time-off?</AlertDialogTitle>
                            <AlertDialogDescription>
                              This will unblock slots in the selected range for
                              this staff member.
                            </AlertDialogDescription>
                          </AlertDialogHeader>
                          <AlertDialogFooter>
                            <AlertDialogCancel disabled={timeOffLoading}>
                              Cancel
                            </AlertDialogCancel>
                            <AlertDialogAction
                              onClick={() => onDeleteTimeOff(entry.id)}
                              disabled={timeOffLoading}
                            >
                              Confirm
                            </AlertDialogAction>
                          </AlertDialogFooter>
                        </AlertDialogContent>
                      </AlertDialog>
                    </CardContent>
                  </Card>
                ))}
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Safety checks</CardTitle>
            </CardHeader>
            <CardContent className="text-sm text-muted-foreground">
              Overlapping time-off entries are rejected by the backend. If you
              see a 409 error, adjust the time range.
            </CardContent>
          </Card>
        </div>
      </AppShell>
    </RequireAuth>
  );
}
