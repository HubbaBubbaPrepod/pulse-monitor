type RequestStorage = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>;
type SavedRequest = { key: string; createdAt: number };
const storageKey = (scope: string) => `pulse:module-operation:${scope}`;

export function readOperationRequest(storage: RequestStorage, scope: string): SavedRequest | null {
  const raw = storage.getItem(storageKey(scope));
  if (raw === null) return null;
  let saved: SavedRequest;
  try {
    saved = JSON.parse(raw);
  } catch {
    throw new Error('operation.requestStorageInvalid');
  }
  if (
    !saved ||
    !/^[a-f0-9]{32}$/.test(saved.key) ||
    !Number.isFinite(saved.createdAt) ||
    saved.createdAt <= 0
  )
    throw new Error('operation.requestStorageInvalid');
  return saved;
}

export function clearOperationRequest(storage: RequestStorage, scope: string) {
  storage.removeItem(storageKey(scope));
}

export async function operationRequest<T>(
  storage: RequestStorage,
  scope: string,
  lookup: (key: string) => Promise<T | null>,
  send: (key: string) => Promise<T>,
  signal?: AbortSignal,
  now = Date.now(),
): Promise<T> {
  let saved = readOperationRequest(storage, scope);
  if (saved) {
    const previous = await lookup(saved.key);
    if (previous !== null) return previous;
    // The server retains keyed operations for at least 24h. Never silently reuse
    // an unresolved key after that window: its earlier command may have run.
    if (now - saved.createdAt >= 24 * 3600000 || saved.createdAt > now + 60000)
      throw new Error('operation.requestExpired');
  } else {
    const bytes = crypto.getRandomValues(new Uint8Array(16));
    saved = {
      key: Array.from(bytes, (value) => value.toString(16).padStart(2, '0')).join(''),
      createdAt: now,
    };
    storage.setItem(storageKey(scope), JSON.stringify(saved));
  }
  try {
    return await send(saved.key);
  } catch (error) {
    if (signal?.aborted || !(error instanceof TypeError || error instanceof SyntaxError))
      throw error;
    return send(saved.key);
  }
}
