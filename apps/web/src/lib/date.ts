export function formatDateTime(value: string) {
  return new Date(value).toLocaleString();
}

export function isToday(value: string) {
  const date = new Date(value);
  const now = new Date();
  return (
    date.getFullYear() === now.getFullYear() &&
    date.getMonth() === now.getMonth() &&
    date.getDate() === now.getDate()
  );
}

export function isUpcoming(value: string) {
  return new Date(value).getTime() > Date.now();
}

export function inRange(value: string, from?: string, to?: string) {
  const date = new Date(value).getTime();
  if (from) {
    const fromTs = new Date(from).getTime();
    if (date < fromTs) return false;
  }
  if (to) {
    const toTs = new Date(to).getTime();
    if (date > toTs) return false;
  }
  return true;
}
