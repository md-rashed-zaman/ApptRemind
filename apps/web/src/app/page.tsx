import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { AppShell } from "../components/app-shell";

export default function Home() {
  return (
    <AppShell>
      <section className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              System Status <Badge variant="outline">local</Badge>
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-2 text-sm text-muted-foreground">
            <p>
              Gateway health:{" "}
              <a
                className="underline"
                href="http://localhost:8080/healthz"
                target="_blank"
                rel="noreferrer"
              >
                /healthz
              </a>{" "}
              and{" "}
              <a
                className="underline"
                href="http://localhost:8080/readyz"
                target="_blank"
                rel="noreferrer"
              >
                /readyz
              </a>
              .
            </p>
            <p>
              API base:{" "}
              <code className="rounded bg-muted px-1 py-0.5">
                http://localhost:8080
              </code>
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Developer Links</CardTitle>
          </CardHeader>
          <CardContent className="flex flex-wrap gap-3">
            <a href="http://localhost:8088/docs" target="_blank" rel="noreferrer">
              <Button>Swagger UI</Button>
            </a>
            <a href="http://localhost:8080/openapi" target="_blank" rel="noreferrer">
              <Button variant="outline">OpenAPI YAML</Button>
            </a>
            <a href="http://localhost:16686" target="_blank" rel="noreferrer">
              <Button variant="outline">Jaeger</Button>
            </a>
            <a href="http://localhost:8025" target="_blank" rel="noreferrer">
              <Button variant="outline">Mailpit</Button>
            </a>
            <a href="/status">
              <Button variant="outline">Status</Button>
            </a>
            <a href="/demo">
              <Button variant="outline">Guided demo</Button>
            </a>
          </CardContent>
        </Card>
      </section>

      <section className="mt-6 grid gap-4 lg:grid-cols-[2fr_1fr]">
        <Card>
          <CardHeader>
            <CardTitle>Experience the full flow</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3 text-sm text-muted-foreground">
            <p>
              This admin UI will guide you through business setup, staff schedules,
              booking availability, and notification delivery. The public booking
              view is designed for shareable links per business.
            </p>
            <ol className="list-decimal space-y-1 pl-5">
              <li>Register and complete the setup wizard.</li>
              <li>Configure staff working hours and time off.</li>
              <li>Book from the public page and verify reminders.</li>
            </ol>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Next Tasks</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2 text-sm text-muted-foreground">
            <p>Phase F1 focuses on app shell + design system polish.</p>
            <div className="flex flex-wrap gap-2">
              <Badge>Auth pages</Badge>
              <Badge variant="secondary">Onboarding wizard</Badge>
              <Badge variant="outline">Public booking</Badge>
            </div>
          </CardContent>
        </Card>
      </section>
    </AppShell>
  );
}
