// Singleton EventSource over GET /api/events. Pages subscribe a refetch
// callback; the server pushes a coarse "change" event on every run state
// change and we fan it out — debounced so an event storm (e.g. a 500-seed
// suite finishing) triggers at most one refetch per window per listener.
//
// EventSource can't set an Authorization header, so the stored base64
// "user:pass" blob rides along as the ?auth= query param (the server's
// basicAuth accepts it for this endpoint only).

const STORAGE_KEY = "ctf-evals.creds";
const DEBOUNCE_MS = 600;

type Listener = () => void;

const listeners = new Set<Listener>();
let source: EventSource | null = null;
let debounceTimer: ReturnType<typeof setTimeout> | null = null;

function flush() {
  debounceTimer = null;
  for (const fn of listeners) {
    try {
      fn();
    } catch {
      // a listener throwing shouldn't kill the others
    }
  }
}

function onChange() {
  if (debounceTimer !== null) return;
  debounceTimer = setTimeout(flush, DEBOUNCE_MS);
}

function ensureOpen() {
  if (source) return;
  const creds = localStorage.getItem(STORAGE_KEY);
  const url = creds
    ? `/api/events?auth=${encodeURIComponent(creds)}`
    : "/api/events";
  source = new EventSource(url);
  source.addEventListener("change", onChange);
  // EventSource auto-reconnects on error; nothing to do but let it.
}

function closeIfIdle() {
  if (listeners.size > 0 || !source) return;
  source.close();
  source = null;
  if (debounceTimer !== null) {
    clearTimeout(debounceTimer);
    debounceTimer = null;
  }
}

// subscribeEvents registers a callback fired (debounced) whenever the server
// reports a state change. Returns an unsubscribe function.
export function subscribeEvents(fn: Listener): () => void {
  listeners.add(fn);
  ensureOpen();
  return () => {
    listeners.delete(fn);
    closeIfIdle();
  };
}
