"use client";

import { useEffect, useState } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { apiHealthz, apiReadyz } from "../../lib/api";

export default function StatusPage() {
  const [health, setHealth] = useState<Record<string, unknown> | null>(null);
  const [ready, setReady] = useState<Record<string, unknown> | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function refresh() {
    setLoading(true);
    setError(null);
    try {
      const [healthz, readyz] = await Promise.all([apiHealthz(), apiReadyz()]);
      setHealth(healthz);
      setReady(readyz);
    } catch {
      setError("Unable to fetch gateway status.");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    refresh();
  }, []);

  return (
    <div className="min-h-screen bg-background text-foreground">
      <div className="mx-auto w-full max-w-5xl px-6 py-10">
        <div className="mb-6 flex flex-col gap-2">
          <Badge variant="secondary">System status</Badge>
          <h1 className="text-3xl font-semibold tracking-tight">Status</h1>
          <p className="text-sm text-muted-foreground">
            Live health checks and quick links for local services.
          </p>
        </div>

        {error ? <p className="text-sm text-destructive">{error}</p> : null}

        <div className="grid gap-4 lg:grid-cols-2">
          <Card>
            <CardHeader>
              <CardTitle>Gateway health</CardTitle>
            </CardHeader>
            <CardContent className="space-y-2 text-sm text-muted-foreground">
              <p>Healthz: {health ? "OK" : "—"}</p>
              <p>Readyz: {ready ? "OK" : "—"}</p>
              <Button variant="outline" onClick={refresh} disabled={loading}>
                {loading ? "Refreshing..." : "Refresh status"}
              </Button>
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle>Local tooling</CardTitle>
            </CardHeader>
            <CardContent className="space-y-2 text-sm text-muted-foreground">
              <div className="flex flex-wrap gap-2">
                <a href="http://localhost:8088/docs" target="_blank" rel="noreferrer">
                  <Button variant="outline">Swagger UI</Button>
                </a>
                <a href="http://localhost:8080/openapi" target="_blank" rel="noreferrer">
                  <Button variant="outline">OpenAPI</Button>
                </a>
                <a href="http://localhost:16686" target="_blank" rel="noreferrer">
                  <Button variant="outline">Jaeger</Button>
                </a>
                <a href="http://localhost:8025" target="_blank" rel="noreferrer">
                  <Button variant="outline">Mailpit</Button>
                </a>
              </div>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}
