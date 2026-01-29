"use client";

import { useEffect, useMemo, useState } from "react";
import { useParams, useRouter } from "next/navigation";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  apiListServices,
  apiListStaff,
  apiPublicBook,
  apiPublicSlots
} from "../../../../lib/api";

export default function PublicBookingPage() {
  const params = useParams<{ businessId: string }>();
  const businessId = params?.businessId ?? "";
  const router = useRouter();

  const [staffId, setStaffId] = useState("");
  const [serviceId, setServiceId] = useState("");
  const [date, setDate] = useState(() => new Date().toISOString().slice(0, 10));
  const [durationMinutes, setDurationMinutes] = useState("30");
  const [slotStepMinutes, setSlotStepMinutes] = useState("15");
  const [slots, setSlots] = useState<Array<{ start_time: string; end_time: string }>>([]);
  const [selectedSlot, setSelectedSlot] = useState<number | null>(null);
  const [slotsLoading, setSlotsLoading] = useState(false);
  const [slotsError, setSlotsError] = useState<string | null>(null);

  const [services, setServices] = useState<
    Array<{ id: string; name: string; duration_minutes: number }>
  >([]);
  const [staff, setStaff] = useState<Array<{ id: string; name: string }>>([]);

  const [customerName, setCustomerName] = useState("");
  const [customerEmail, setCustomerEmail] = useState("");
  const [customerPhone, setCustomerPhone] = useState("");
  const [bookingLoading, setBookingLoading] = useState(false);
  const [bookingResult, setBookingResult] = useState<string | null>(null);
  const [bookingError, setBookingError] = useState<string | null>(null);

  const selectedSlotValue = useMemo(() => {
    if (selectedSlot === null) return null;
    return slots[selectedSlot] ?? null;
  }, [selectedSlot, slots]);

  function isValidEmail(value: string) {
    return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value);
  }

  useEffect(() => {
    let ignore = false;
    async function loadReference() {
      setSlotsError(null);
      try {
        const [servicesList, staffList] = await Promise.all([
          apiListServices(),
          apiListStaff()
        ]);
        if (ignore) return;
        setServices(
          servicesList.map((service) => ({
            id: service.id,
            name: service.name,
            duration_minutes: service.duration_minutes
          }))
        );
        setStaff(staffList.map((member) => ({ id: member.id, name: member.name })));
        if (servicesList[0]) {
          setServiceId(servicesList[0].id);
        }
        if (staffList[0]) {
          setStaffId(staffList[0].id);
        }
      } catch {
        if (!ignore) {
          setSlotsError("Unable to load services/staff. Sign in required.");
        }
      }
    }

    loadReference();
    return () => {
      ignore = true;
    };
  }, []);

  async function fetchSlots() {
    if (!businessId || !staffId || !serviceId) {
      setSlotsError("Business, staff, and service are required.");
      return;
    }
    setSlotsLoading(true);
    setSlotsError(null);
    setBookingResult(null);
    setSelectedSlot(null);
    try {
      const list = await apiPublicSlots({
        business_id: businessId,
        staff_id: staffId,
        service_id: serviceId,
        date,
        duration_minutes: Number(durationMinutes) || undefined,
        slot_step_minutes: Number(slotStepMinutes) || undefined
      });
      setSlots(list);
      if (list.length > 0) {
        setSelectedSlot(0);
      }
      if (list.length === 0) {
        setSlotsError("No available slots for that date.");
      }
    } catch {
      setSlotsError("Unable to load slots. Check the IDs and try again.");
    } finally {
      setSlotsLoading(false);
    }
  }

  async function onBook(e: React.FormEvent) {
    e.preventDefault();
    if (!businessId || !staffId || !serviceId) {
      setBookingError("Business, staff, and service are required.");
      return;
    }
    if (!selectedSlotValue) {
      setBookingError("Select a slot first.");
      return;
    }
    if (!customerName.trim()) {
      setBookingError("Customer name is required.");
      return;
    }
    if (customerEmail && !isValidEmail(customerEmail)) {
      setBookingError("Enter a valid email address.");
      return;
    }
    setBookingLoading(true);
    setBookingError(null);
    try {
      const result = await apiPublicBook({
        business_id: businessId,
        staff_id: staffId,
        service_id: serviceId,
        start_time: selectedSlotValue.start_time,
        end_time: selectedSlotValue.end_time,
        customer_name: customerName,
        customer_email: customerEmail || undefined,
        customer_phone: customerPhone || undefined
      });
      setBookingResult(result.appointment_id);
      router.push(
        `/b/${businessId}/book/success?appointment_id=${result.appointment_id}`
      );
    } catch {
      setBookingError("Unable to create booking. Please try again.");
    } finally {
      setBookingLoading(false);
    }
  }

  return (
    <div className="min-h-screen bg-background text-foreground">
      <div className="mx-auto w-full max-w-5xl px-6 py-10">
        <div className="mb-6 flex flex-col gap-2">
          <Badge variant="secondary">Public booking</Badge>
          <h1 className="text-3xl font-semibold tracking-tight">
            Book an appointment
          </h1>
          <p className="text-sm text-muted-foreground">
            Business ID: <code className="rounded bg-muted px-1">{businessId}</code>
          </p>
        </div>

        <div className="grid gap-4 lg:grid-cols-[2fr_1fr]">
          <Card>
            <CardHeader>
              <CardTitle>Booking context</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="staff-id">Staff ID</Label>
                <Input
                  id="staff-id"
                  value={staffId}
                  onChange={(e) => setStaffId(e.target.value)}
                  placeholder="staff uuid"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="service-id">Service ID</Label>
                <Input
                  id="service-id"
                  value={serviceId}
                  onChange={(e) => setServiceId(e.target.value)}
                  placeholder="service uuid"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="date">Date</Label>
                <Input
                  id="date"
                  type="date"
                  value={date}
                  onChange={(e) => setDate(e.target.value)}
                />
              </div>
              <div className="grid gap-3 md:grid-cols-2">
                <div className="space-y-2">
                  <Label htmlFor="duration">Duration (min)</Label>
                  <Input
                    id="duration"
                    type="number"
                    min={5}
                    value={durationMinutes}
                    onChange={(e) => setDurationMinutes(e.target.value)}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="slot-step">Slot step (min)</Label>
                  <Input
                    id="slot-step"
                    type="number"
                    min={5}
                    value={slotStepMinutes}
                    onChange={(e) => setSlotStepMinutes(e.target.value)}
                  />
                </div>
              </div>
              <div className="flex flex-wrap gap-2">
                {services.map((service) => (
                  <Badge
                    key={service.id}
                    variant={service.id === serviceId ? "default" : "secondary"}
                  >
                    {service.name}
                  </Badge>
                ))}
                {staff.map((member) => (
                  <Badge
                    key={member.id}
                    variant={member.id === staffId ? "default" : "secondary"}
                  >
                    {member.name}
                  </Badge>
                ))}
              </div>
              {slotsError ? (
                <p className="text-sm text-destructive">{slotsError}</p>
              ) : null}
              <Button onClick={fetchSlots} disabled={slotsLoading}>
                {slotsLoading ? "Loading..." : "Find slots"}
              </Button>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Available slots</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3 text-sm text-muted-foreground">
              {slots.length === 0 && !slotsLoading ? (
                <div className="rounded-lg border border-dashed border-border bg-muted/40 p-4 text-sm text-muted-foreground">
                  <p className="font-medium text-foreground">No availability</p>
                  <p>Try another date or contact the business for updates.</p>
                </div>
              ) : null}
              {slotsLoading ? <p>Loading slots...</p> : null}
              <div className="grid gap-2">
                {slots.map((slot, idx) => {
                  const label = `${new Date(slot.start_time).toLocaleTimeString()} - ${new Date(
                    slot.end_time
                  ).toLocaleTimeString()}`;
                  const active = selectedSlot === idx;
                  return (
                    <button
                      key={`${slot.start_time}-${idx}`}
                      className={`rounded-md border px-3 py-2 text-left text-sm ${
                        active
                          ? "border-foreground bg-foreground text-background"
                          : "border-border text-foreground hover:border-foreground"
                      }`}
                      onClick={() => setSelectedSlot(idx)}
                      type="button"
                    >
                      {label}
                    </button>
                  );
                })}
              </div>
            </CardContent>
          </Card>
        </div>

        <div className="mt-6 grid gap-4 lg:grid-cols-[2fr_1fr]">
          <Card>
            <CardHeader>
              <CardTitle>Booking form</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <form className="grid gap-3" onSubmit={onBook}>
                <div className="grid gap-3 md:grid-cols-2">
                  <div className="space-y-2">
                    <Label htmlFor="customer-name">Customer name</Label>
                    <Input
                      id="customer-name"
                      value={customerName}
                      onChange={(e) => setCustomerName(e.target.value)}
                      required
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="customer-email">Customer email</Label>
                    <Input
                      id="customer-email"
                      type="email"
                      value={customerEmail}
                      onChange={(e) => setCustomerEmail(e.target.value)}
                    />
                  </div>
                </div>
                <div className="space-y-2">
                  <Label htmlFor="customer-phone">Customer phone</Label>
                  <Input
                    id="customer-phone"
                    value={customerPhone}
                    onChange={(e) => setCustomerPhone(e.target.value)}
                  />
                </div>
                {bookingError ? (
                  <p className="text-sm text-destructive">{bookingError}</p>
                ) : null}
                {bookingResult ? (
                  <div className="rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
                    Booking confirmed. Appointment ID: {bookingResult}
                  </div>
                ) : null}
                <Button type="submit" disabled={bookingLoading}>
                  {bookingLoading ? "Booking..." : "Confirm booking"}
                </Button>
              </form>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Request summary</CardTitle>
            </CardHeader>
            <CardContent className="space-y-2 text-sm text-muted-foreground">
              <p>Business: {businessId || "—"}</p>
              <p>Staff: {staffId || "—"}</p>
              <p>Service: {serviceId || "—"}</p>
              <p>Date: {date}</p>
              <p>Duration: {durationMinutes} min</p>
              <p>Slot step: {slotStepMinutes} min</p>
              <p>
                Slot:{" "}
                {selectedSlotValue
                  ? `${new Date(selectedSlotValue.start_time).toLocaleTimeString()} → ${new Date(
                      selectedSlotValue.end_time
                    ).toLocaleTimeString()}`
                  : "—"}
              </p>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}
