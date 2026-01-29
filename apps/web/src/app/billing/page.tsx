 "use client";

import { useEffect, useState } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
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
  apiCancelSubscription,
  apiCreateCheckout,
  apiGetSubscription
} from "../../lib/api";

export default function BillingPage() {
  const [tier, setTier] = useState("starter");
  const [subscription, setSubscription] = useState<{
    business_id: string;
    tier: string;
    status: string;
    updated_at: string;
    entitlements?: {
      tier: string;
      max_staff: number;
      max_services: number;
      max_monthly_appointments: number;
    };
  } | null>(null);
  const [loading, setLoading] = useState(false);
  const [checkoutLoading, setCheckoutLoading] = useState(false);
  const [cancelLoading, setCancelLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let ignore = false;
    async function load() {
      setLoading(true);
      setError(null);
      try {
        const data = await apiGetSubscription();
        if (!ignore) {
          setSubscription(data);
          setTier(data.tier || "starter");
        }
      } catch {
        if (!ignore) {
          setError("Unable to load subscription.");
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

  async function onUpgrade() {
    setCheckoutLoading(true);
    setError(null);
    try {
      const origin =
        typeof window !== "undefined" ? window.location.origin : "http://localhost:3000";
      const result = await apiCreateCheckout({
        tier,
        success_url: `${origin}/billing/success?session_id={CHECKOUT_SESSION_ID}&state={STATE}`,
        cancel_url: `${origin}/billing/cancel?session_id={CHECKOUT_SESSION_ID}&state={STATE}`
      });
      if (result.url) {
        window.location.href = result.url;
      }
    } catch {
      setError("Unable to start checkout.");
    } finally {
      setCheckoutLoading(false);
    }
  }

  async function onCancelSubscription() {
    setCancelLoading(true);
    setError(null);
    try {
      await apiCancelSubscription();
      const data = await apiGetSubscription();
      setSubscription(data);
    } catch {
      setError("Unable to cancel subscription.");
    } finally {
      setCancelLoading(false);
    }
  }

  return (
    <RequireAuth>
      <AppShell>
        <SectionHeader
          title="Billing"
          description="Upgrade plans and manage entitlements."
          action={
            <Button onClick={onUpgrade} disabled={checkoutLoading}>
              {checkoutLoading ? "Redirecting..." : "Upgrade"}
            </Button>
          }
        />

        <div className="mt-6 grid gap-4 lg:grid-cols-[2fr_1fr]">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                Current plan{" "}
                <Badge variant="secondary">
                  {subscription?.tier ?? "Starter"}
                </Badge>
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-3 text-sm text-muted-foreground">
              {error ? <p className="text-destructive">{error}</p> : null}
              {loading ? <p>Loading subscription…</p> : null}
              {subscription?.entitlements ? (
                <div className="grid gap-2 text-xs">
                  <div>Tier: {subscription.entitlements.tier}</div>
                  <div>Max staff: {subscription.entitlements.max_staff}</div>
                  <div>Max services: {subscription.entitlements.max_services}</div>
                  <div>
                    Monthly appointments:{" "}
                    {subscription.entitlements.max_monthly_appointments}
                  </div>
                </div>
              ) : (
                <p>Entitlements will show once billing is activated.</p>
              )}
              <div className="flex flex-wrap gap-2">
                <AlertDialog>
                  <AlertDialogTrigger asChild>
                    <Button variant="destructive" disabled={cancelLoading}>
                      Cancel subscription
                    </Button>
                  </AlertDialogTrigger>
                  <AlertDialogContent>
                    <AlertDialogHeader>
                      <AlertDialogTitle>Cancel the subscription?</AlertDialogTitle>
                      <AlertDialogDescription>
                        This will downgrade the account and stop future charges.
                      </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                      <AlertDialogCancel disabled={cancelLoading}>
                        Keep
                      </AlertDialogCancel>
                      <AlertDialogAction
                        onClick={onCancelSubscription}
                        disabled={cancelLoading}
                      >
                        {cancelLoading ? "Canceling..." : "Confirm"}
                      </AlertDialogAction>
                    </AlertDialogFooter>
                  </AlertDialogContent>
                </AlertDialog>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Integrations</CardTitle>
            </CardHeader>
            <CardContent className="text-sm text-muted-foreground">
              <div className="space-y-3">
                <p>Stripe checkout + webhook flow is enabled in the backend.</p>
                <div className="space-y-2">
                  <Label htmlFor="tier">Tier to upgrade</Label>
                  <Input
                    id="tier"
                    value={tier}
                    onChange={(e) => setTier(e.target.value)}
                    placeholder="starter or pro"
                  />
                  <Button variant="outline" onClick={onUpgrade} disabled={checkoutLoading}>
                    {checkoutLoading ? "Redirecting..." : "Start checkout"}
                  </Button>
                </div>
              </div>
            </CardContent>
          </Card>
        </div>
      </AppShell>
    </RequireAuth>
  );
}
