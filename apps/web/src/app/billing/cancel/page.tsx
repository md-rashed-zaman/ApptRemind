"use client";

import { useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { apiAckCheckoutSession, apiGetCheckoutSession } from "../../../lib/api";

export default function BillingCancelPage() {
  const searchParams = useSearchParams();
  const sessionId = searchParams.get("session_id") ?? "";
  const state = searchParams.get("state") ?? "";

  const [status, setStatus] = useState<{
    session_id: string;
    tier: string;
    status: string;
    canceled_at?: string;
    updated_at?: string;
  } | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [ackDone, setAckDone] = useState(false);

  useEffect(() => {
    let ignore = false;
    async function load() {
      if (!sessionId) {
        setError("Missing session_id.");
        return;
      }
      setError(null);
      try {
        const data = await apiGetCheckoutSession(sessionId);
        if (!ignore) {
          setStatus(data);
        }
        if (state) {
          await apiAckCheckoutSession({
            session_id: sessionId,
            state,
            result: "cancel"
          });
          if (!ignore) {
            setAckDone(true);
          }
        }
      } catch {
        if (!ignore) {
          setError("Unable to verify checkout.");
        }
      }
    }

    load();
    return () => {
      ignore = true;
    };
  }, [sessionId, state]);

  return (
    <div className="min-h-screen bg-background text-foreground">
      <div className="mx-auto flex w-full max-w-xl flex-col gap-4 px-6 py-12">
        <Badge variant="secondary">Checkout canceled</Badge>
        <h1 className="text-3xl font-semibold">Checkout canceled</h1>
        <p className="text-sm text-muted-foreground">
          You can restart checkout anytime from the billing page.
        </p>

        <Card>
          <CardHeader>
            <CardTitle>Status</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2 text-sm text-muted-foreground">
            {error ? <p className="text-destructive">{error}</p> : null}
            <p>Session: {sessionId || "—"}</p>
            <p>Tier: {status?.tier ?? "—"}</p>
            <p>Status: {status?.status ?? "—"}</p>
            {status?.canceled_at ? (
              <p>Cancelled at: {new Date(status.canceled_at).toLocaleString()}</p>
            ) : null}
            {ackDone ? <p className="text-emerald-600">Return acknowledged.</p> : null}
          </CardContent>
        </Card>

        <Button onClick={() => (window.location.href = "/billing")}>
          Back to billing
        </Button>
      </div>
    </div>
  );
}
