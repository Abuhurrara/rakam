/**
 * The entire money surface of the frontend.
 *
 * Two rules, and this file is where they are kept:
 *
 *  1. A user-typed amount is never parsed in JS. It travels to the API as the
 *     raw string the user typed; `domain.ParseMoney` on the Go side turns it
 *     into int64 paisa. There is no rupees->paisa conversion here.
 *  2. A displayed amount is always built from the integer paisa the API
 *     returned, with integer arithmetic only.
 *
 * No parseFloat, no parseInt, no Number() on a money value anywhere below.
 */

/** Amounts arrive from the API as integer paisa (1/100 of a rupee). */
export type Paisa = number;

export type FormatOptions = {
  /** Force the .00 even when the value is a whole number of rupees. */
  alwaysPaisa?: boolean;
  /** Render a leading + for positive values (net balances, not row amounts). */
  signed?: boolean;
  /** Drop the "Rs " prefix — for inputs and tight columns. */
  bare?: boolean;
};

/**
 * Format integer paisa for display, e.g. 125050 -> "Rs 1,250.50".
 *
 * Paisa are shown whenever the value is not a whole number of rupees, and
 * hidden when it is. That choice is deliberate: rounding every row to the
 * nearest rupee would mean a column of rows no longer visibly sums to the
 * total printed above it (three rows of Rs 1,250.50 would each read
 * "Rs 1,251" under a total of "Rs 3,752"). In a money app a total that
 * disagrees with its own rows reads as a bug, so exactness wins over
 * tidiness.
 *
 * Totals are summed from raw integer paisa and then passed through this same
 * function, so rows and totals can never drift apart.
 *
 * Note this intentionally does NOT match Go's `domain.Money.String()`, which
 * rounds to the nearest rupee. Nothing forces them to agree: every money
 * field on the wire is `*_paisa` as a JSON integer, and the API never sends a
 * formatted money string, so this function is the only display authority.
 */
export function formatPaisa(paisa: Paisa, opts: FormatOptions = {}): string {
  const negative = paisa < 0;
  const abs = negative ? -paisa : paisa;

  // Exact integer split. `abs - frac` is a multiple of 100, so the division
  // is exact for any safe integer — no float rounding can creep in.
  const frac = abs % 100;
  const rupees = (abs - frac) / 100;

  let out = groupThousands(rupees);
  if (frac !== 0 || opts.alwaysPaisa) {
    out += "." + (frac < 10 ? "0" + frac : String(frac));
  }

  let sign = "";
  if (negative) sign = "-";
  else if (opts.signed && paisa > 0) sign = "+";

  return opts.bare ? sign + out : "Rs " + sign + out;
}

/** 1250 -> "1,250". Walks the digit string; never touches Intl or floats. */
function groupThousands(rupees: number): string {
  const digits = String(rupees);
  let out = "";
  for (let i = 0; i < digits.length; i++) {
    if (i > 0 && (digits.length - i) % 3 === 0) out += ",";
    out += digits[i];
  }
  return out;
}

/**
 * Would the API accept this as an `amount`?
 *
 * An exact mirror of the rules in api/internal/domain/money.go: the pattern
 * `^\d+(\.\d{1,2})?$`, with zero rejected. Commas and currency symbols are
 * rejected rather than stripped, same as the server.
 *
 * This only ever answers yes or no — it never converts. Its whole job is to
 * gate the Save button so the client refuses exactly what the server would
 * refuse, without duplicating the parse.
 */
export function isValidAmountInput(raw: string): boolean {
  const s = raw.trim();
  if (s === "") return false;
  if (!/^\d+(\.\d{1,2})?$/.test(s)) return false;
  // "0", "0.0", "0.00" — the server rejects a zero amount.
  if (/^0+(\.0{1,2})?$/.test(s)) return false;
  // The server caps the whole part at (2^63-1-99)/100, 17 digits. Cap well
  // below that so the value also stays inside JS's safe integer range once
  // it comes back as paisa.
  const whole = s.split(".")[0];
  if (whole.replace(/^0+/, "").length > 13) return false;
  return true;
}

/**
 * The only paisa -> editable-string path in the app, for prefilling the edit
 * sheet: 125050 -> "1250.50", 125000 -> "1250".
 *
 * Integer arithmetic, same as formatPaisa — no grouping, no "Rs", because the
 * result goes back into the amount field and must stay something the keypad
 * could have produced and `isValidAmountInput` accepts.
 */
export function paisaToInputString(paisa: Paisa): string {
  const abs = paisa < 0 ? -paisa : paisa;
  const frac = abs % 100;
  const rupees = (abs - frac) / 100;
  if (frac === 0) return String(rupees);
  return `${rupees}.${frac < 10 ? "0" + frac : frac}`;
}

/** Sum integer paisa. Exists so no caller is tempted to reduce with floats. */
export function sumPaisa(values: readonly Paisa[]): Paisa {
  let total = 0;
  for (const v of values) total += v;
  return total;
}
