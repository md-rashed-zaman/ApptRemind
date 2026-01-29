"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useAuthStore } from "../lib/auth-store";

export function RequireAuth({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const { user, loading, hydrated, hydrate } = useAuthStore();

  useEffect(() => {
    if (!hydrated) {
      hydrate();
    }
  }, [hydrate, hydrated]);

  useEffect(() => {
    if (hydrated && !loading && !user) {
      router.replace("/auth/login");
    }
  }, [hydrated, loading, router, user]);

  if (!hydrated || loading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Checking session</CardTitle>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground">
          Verifying your access. This only takes a moment.
        </CardContent>
      </Card>
    );
  }

  if (!user) return null;

  return <>{children}</>;
}
