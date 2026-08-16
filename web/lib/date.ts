/**
 * Everything time-related happens in Asia/Karachi, never in the device's
 * timezone. The phone may be anywhere; the ledger is always Karachi.
 *
 * Pakistan is UTC+05:00 all year with no DST, so the offset is a constant —
 * but the *conversion* still goes through Intl with an explicit timeZone,
 * because the device clock cannot be assumed to be on Karachi time.
 */

const KARACHI = "Asia/Karachi";
const KARACHI_OFFSET = "+05:00";

const partsFormatter = new Intl.DateTimeFormat("en-GB", {
  timeZone: KARACHI,
  year: "numeric",
  month: "2-digit",
  day: "2-digit",
  hour: "2-digit",
  minute: "2-digit",
  second: "2-digit",
  hourCycle: "h23",
});

type Parts = {
  year: string;
  month: string;
  day: string;
  hour: string;
  minute: string;
  second: string;
};

function karachiParts(d: Date): Parts {
  const out: Record<string, string> = {};
  for (const p of partsFormatter.formatToParts(d)) {
    if (p.type !== "literal") out[p.type] = p.value;
  }
  return out as unknown as Parts;
}

/**
 * The wire format for `occurred_at`, e.g. "2026-08-16T14:30:00+05:00".
 *
 * The API requires RFC3339 (api/internal/httpapi/transaction.go) and rejects
 * a bare "2026-08-16" with a 400. Never use toISOString() here — that yields
 * a Z offset, and the spec asks for the Karachi offset.
 */
export function toKarachiRFC3339(d: Date): string {
  const p = karachiParts(d);
  return `${p.year}-${p.month}-${p.day}T${p.hour}:${p.minute}:${p.second}${KARACHI_OFFSET}`;
}

/** "2026-08" — the ?month= query param, bucketed in Karachi. */
export function karachiMonthKey(d: Date): string {
  const p = karachiParts(d);
  return `${p.year}-${p.month}`;
}

/**
 * "2026-08-16" — the day bucket for grouping the expenses list.
 *
 * Takes the RFC3339 string the API returned. Do not slice the first 10
 * characters of that string instead: Go formats it from whatever offset the
 * database session is in, typically Z, so slicing would file a 2am Karachi
 * purchase under the previous day.
 */
export function karachiDayKey(rfc3339: string): string {
  const p = karachiParts(new Date(rfc3339));
  return `${p.year}-${p.month}-${p.day}`;
}

/** "2026-08-16" for today in Karachi — the value/max of a date input. */
export function karachiDateInputValue(d: Date): string {
  const p = karachiParts(d);
  return `${p.year}-${p.month}-${p.day}`;
}

/**
 * Turn a date input's "2026-08-14" back into RFC3339, keeping the current
 * Karachi time of day. Picking a past date should not also reset the clock to
 * midnight — the time a purchase happened is still roughly now.
 */
export function fromKarachiDateInput(dateValue: string, now: Date): string {
  const p = karachiParts(now);
  return `${dateValue}T${p.hour}:${p.minute}:${p.second}${KARACHI_OFFSET}`;
}

const dayHeaderFormatter = new Intl.DateTimeFormat("en-GB", {
  timeZone: KARACHI,
  weekday: "short",
  day: "numeric",
  month: "short",
});

const timeFormatter = new Intl.DateTimeFormat("en-GB", {
  timeZone: KARACHI,
  hour: "numeric",
  minute: "2-digit",
  hour12: true,
});

/** "Today", "Yesterday", or "Sat, 16 Aug" for a day-group header. */
export function formatDayHeader(dayKey: string, now: Date = new Date()): string {
  const today = karachiDateInputValue(now);
  if (dayKey === today) return "Today";

  const yesterday = karachiDateInputValue(new Date(now.getTime() - 86_400_000));
  if (dayKey === yesterday) return "Yesterday";

  // Noon avoids any chance of the parsed instant landing on the wrong side
  // of a day boundary when rendered back in Karachi.
  return dayHeaderFormatter.format(new Date(`${dayKey}T12:00:00${KARACHI_OFFSET}`));
}

/** "2:30 pm" — the time on a transaction row. */
export function formatTime(rfc3339: string): string {
  return timeFormatter.format(new Date(rfc3339)).toLowerCase();
}

const monthLabelFormatter = new Intl.DateTimeFormat("en-GB", {
  timeZone: KARACHI,
  month: "long",
  year: "numeric",
});

/** "2026-08" -> "August 2026". */
export function formatMonthLabel(monthKey: string): string {
  return monthLabelFormatter.format(
    new Date(`${monthKey}-01T12:00:00${KARACHI_OFFSET}`),
  );
}

/** Step a "2026-08" month key by whole months, e.g. -1 -> "2026-07". */
export function shiftMonthKey(monthKey: string, delta: number): string {
  const [y, m] = monthKey.split("-");
  // Month arithmetic on plain integers, then normalise. Using a Date here
  // would drag the device timezone back in.
  const total = Number(y) * 12 + (Number(m) - 1) + delta;
  const year = Math.floor(total / 12);
  const month = total - year * 12 + 1;
  return `${year}-${String(month).padStart(2, "0")}`;
}

/** Is this month key in the future relative to now in Karachi? */
export function isFutureMonth(monthKey: string, now: Date = new Date()): boolean {
  return monthKey > karachiMonthKey(now);
}
