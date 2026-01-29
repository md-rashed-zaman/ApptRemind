"use client";

import { useSearchParams } from "next/navigation";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

export default function BookingSuccessPage() {
  const searchParams = useSearchParams();
  const appointmentId = searchParams.get("appointment_id") ?? "";

  return (
    <div className="min-h-screen bg-background text-foreground">
      <div className="mx-auto flex w-full max-w-xl flex-col gap-4 px-6 py-12">
        <Badge variant="secondary">Booking confirmed</Badge>
        <h1 className="text-3xl font-semibold">You’re all set</h1>
        <p className="text-sm text-muted-foreground">
          The appointment is confirmed and a reminder will be sent automatically.
        </p>

        <Card>
          <CardHeader>
            <CardTitle>Confirmation</CardTitle>
          </CardHeader>
          <CardContent className="text-sm text-muted-foreground">
            Appointment ID:{" "}
            <code className="rounded bg-muted px-1">{appointmentId || "—"}</code>
          </CardContent>
        </Card>

        <Button onClick={() => window.history.back()}>Back to booking</Button>
      </div>
    </div>
  );
}
