export function parseRetentionDays(value: string): number | null {
  if (!/^\d+$/.test(value.trim())) return null;
  const days = Number(value);
  return Number.isSafeInteger(days) && days >= 0 && days <= 3650 ? days : null;
}
export function retentionNeedsConfirmation(previous: number, next: number): boolean {
  return next > 0 && (previous === 0 || next < previous);
}
