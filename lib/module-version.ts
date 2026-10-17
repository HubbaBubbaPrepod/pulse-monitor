export function newerModuleVersion(candidate: string, installed: string): boolean {
  const pattern = /^(\d+)\.(\d+)\.(\d+)(?:-([a-zA-Z0-9-]+))?$/;
  const a = pattern.exec(candidate),
    b = pattern.exec(installed);
  if (!a || !b || candidate.length > 48 || installed.length > 48) return false;
  for (let i = 1; i <= 3; i++) {
    if (BigInt(a[i]) !== BigInt(b[i])) return BigInt(a[i]) > BigInt(b[i]);
  }
  const ap = a[4] ?? '',
    bp = b[4] ?? '';
  if (ap === bp) return false;
  if (!ap) return true;
  if (!bp) return false;
  const an = /^\d+$/.test(ap),
    bn = /^\d+$/.test(bp);
  if (an && bn) return BigInt(ap) > BigInt(bp);
  if (an !== bn) return !an;
  return ap > bp;
}
