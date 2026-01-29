export default function Loading() {
  return (
    <div className="mx-auto flex min-h-screen w-full max-w-4xl flex-col gap-4 px-6 py-20">
      <div className="h-10 w-64 animate-pulse rounded-md bg-muted" />
      <div className="h-24 w-full animate-pulse rounded-xl bg-muted" />
      <div className="grid gap-4 md:grid-cols-2">
        <div className="h-32 animate-pulse rounded-xl bg-muted" />
        <div className="h-32 animate-pulse rounded-xl bg-muted" />
      </div>
    </div>
  );
}

