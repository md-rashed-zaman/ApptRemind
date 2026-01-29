"use client";

import { useEffect, useMemo } from "react";

import { Button } from "@/components/ui/button";

export default function Error({
  error,
  reset
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  const requestId = useMemo(() => {
    if (typeof window === "undefined") return null;
    const maybeCrypto = window.crypto;
    return maybeCrypto?.randomUUID ? maybeCrypto.randomUUID() : null;
  }, []);

  useEffect(() => {
    // eslint-disable-next-line no-console
    console.error(error);
  }, [error]);

  return (
    <div className="mx-auto flex min-h-screen w-full max-w-xl flex-col items-center justify-center gap-4 px-6 text-center">
      <h1 className="text-2xl font-semibold">Something went wrong</h1>
      <p className="text-sm text-muted-foreground">
        Try again or refresh the page. If this keeps happening, check the
        backend health and logs.
      </p>
      {error?.message ? (
        <p className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-xs text-destructive">
          {error.message}
        </p>
      ) : null}
      <div className="flex gap-2">
        <Button onClick={reset}>Try again</Button>
        <Button variant="outline" onClick={() => window.location.reload()}>
          Refresh
        </Button>
      </div>
      {error.digest && (
        <p className="text-xs text-muted-foreground">Digest: {error.digest}</p>
      )}
      {requestId ? (
        <p className="text-xs text-muted-foreground">Request ID: {requestId}</p>
      ) : null}
    </div>
  );
}
