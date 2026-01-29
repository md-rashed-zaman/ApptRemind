"use client";

import Link from "next/link";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

export default function DemoPage() {
  const steps = [
    {
      title: "1. Register or sign in",
      description:
        "Create a business account to access staff, services, and availability.",
      href: "/auth/register"
    },
    {
      title: "2. Run onboarding",
      description:
        "Add business profile, services, and staff to power bookings.",
      href: "/onboarding"
    },
    {
      title: "3. Configure availability",
      description:
        "Set working hours and time-off to control slot generation.",
      href: "/availability"
    },
    {
      title: "4. Share booking link",
      description:
        "Use the public booking page to simulate a customer booking.",
      href: "/public-demo"
    },
    {
      title: "5. Monitor appointments",
      description:
        "Review and cancel appointments from the owner dashboard.",
      href: "/dashboard"
    },
    {
      title: "6. Try billing",
      description: "Start a checkout session and observe webhook updates.",
      href: "/billing"
    }
  ];

  return (
    <div className="min-h-screen bg-background text-foreground">
      <div className="mx-auto w-full max-w-5xl px-6 py-10">
        <div className="mb-6 flex flex-col gap-2">
          <Badge variant="secondary">Guided demo</Badge>
          <h1 className="text-3xl font-semibold tracking-tight">
            ApptRemind walkthrough
          </h1>
          <p className="text-sm text-muted-foreground">
            Follow this end-to-end flow to experience the full platform.
          </p>
        </div>

        <div className="grid gap-4 md:grid-cols-2">
          {steps.map((step) => (
            <Card key={step.title}>
              <CardHeader>
                <CardTitle>{step.title}</CardTitle>
              </CardHeader>
              <CardContent className="space-y-2 text-sm text-muted-foreground">
                <p>{step.description}</p>
                <Link href={step.href}>
                  <Button variant="outline">Open</Button>
                </Link>
              </CardContent>
            </Card>
          ))}
        </div>

        <div className="mt-6 grid gap-4 md:grid-cols-2">
          <Card>
            <CardHeader>
              <CardTitle>Status shortcuts</CardTitle>
            </CardHeader>
            <CardContent className="flex flex-wrap gap-2">
              <a href="/status">
                <Button variant="outline">System status</Button>
              </a>
              <a href="http://localhost:8088/docs" target="_blank" rel="noreferrer">
                <Button variant="outline">Swagger UI</Button>
              </a>
              <a href="http://localhost:16686" target="_blank" rel="noreferrer">
                <Button variant="outline">Jaeger</Button>
              </a>
              <a href="http://localhost:8025" target="_blank" rel="noreferrer">
                <Button variant="outline">Mailpit</Button>
              </a>
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle>What to verify</CardTitle>
            </CardHeader>
            <CardContent className="space-y-2 text-sm text-muted-foreground">
              <p>✅ Booking creates an appointment and sends a reminder.</p>
              <p>✅ Cancelling updates status and triggers notifications.</p>
              <p>✅ Billing flow updates subscription tier.</p>
              <p>✅ Health/ready endpoints stay green.</p>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}
