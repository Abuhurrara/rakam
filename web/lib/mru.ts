/**
 * "Most-used first" for the category chips, without an API for it.
 *
 * The API has no usage-ranking endpoint and phase 6 is not the place to add
 * one, so the client remembers the last few categories used and floats them
 * to the front. It costs nothing, needs no server change, and gets better the
 * more the app is used.
 */

const KEY = "rakam.category-mru";
const MAX = 8;

export function readMru(): string[] {
  if (typeof window === "undefined") return [];
  try {
    const raw = window.localStorage.getItem(KEY);
    if (!raw) return [];
    const parsed: unknown = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    return parsed.filter((v): v is string => typeof v === "string");
  } catch {
    // Private mode, quota, corrupt value — ordering is a nicety, never fatal.
    return [];
  }
}

export function pushMru(categoryId: string): void {
  if (typeof window === "undefined") return;
  try {
    const next = [categoryId, ...readMru().filter((id) => id !== categoryId)];
    window.localStorage.setItem(KEY, JSON.stringify(next.slice(0, MAX)));
  } catch {
    // Ignore — see above.
  }
}

/**
 * Recently used first (most recent leftmost), then everything else in the
 * server's sort_order. Stable, so the chip row does not reshuffle while the
 * user is looking at it.
 */
export function orderByMru<T extends { id: string }>(
  items: readonly T[],
  mru: readonly string[],
): T[] {
  const rank = new Map(mru.map((id, i) => [id, i]));
  return [...items].sort((a, b) => {
    const ra = rank.get(a.id);
    const rb = rank.get(b.id);
    if (ra !== undefined && rb !== undefined) return ra - rb;
    if (ra !== undefined) return -1;
    if (rb !== undefined) return 1;
    return 0;
  });
}
