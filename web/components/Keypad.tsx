"use client";

import { countTap } from "@/lib/perf";
import { useTap } from "@/lib/useTap";

/**
 * A custom on-screen keypad rather than <input inputMode="decimal">.
 *
 * Two reasons. First speed: the OS keyboard costs a few hundred milliseconds
 * of animation on iOS before the first digit can be typed, and its keys are
 * small. Second correctness: this pad simply cannot produce a string the
 * server would reject — it refuses a second decimal point, a third decimal
 * place, and a leading run of zeroes, so what the user types always matches
 * domain.ParseMoney's `^\d+(\.\d{1,2})?$`.
 *
 * The tradeoff, plainly: no OS paste into the amount field. For a one-handed
 * five-second flow that is the right trade.
 */

/** Far beyond any real expense, and keeps the resulting paisa a safe int. */
const MAX_WHOLE_DIGITS = 13;

export function applyKey(current: string, key: string): string {
  if (key === "back") return current.slice(0, -1);

  if (key === ".") {
    if (current.includes(".")) return current;
    return current === "" ? "0." : current + ".";
  }

  // A digit.
  const [whole, frac] = current.split(".");
  if (current.includes(".")) {
    if ((frac ?? "").length >= 2) return current;
    return current + key;
  }
  // No leading zeros: "0" then "5" should be "5", not "05".
  if (whole === "0") return key;
  if (whole.length >= MAX_WHOLE_DIGITS) return current;
  return current + key;
}

const KEYS = ["1", "2", "3", "4", "5", "6", "7", "8", "9", ".", "0", "back"];

export function Keypad({
  onKey,
}: {
  onKey: (next: (current: string) => string) => void;
}) {
  const tap = useTap();

  return (
    <div className="grid grid-cols-3 gap-1.5" role="group" aria-label="Amount keypad">
      {KEYS.map((key) => (
        <button
          key={key}
          type="button"
          aria-label={key === "back" ? "Delete last digit" : key}
          {...tap(() => {
            countTap();
            onKey((current) => applyKey(current, key));
          })}
          className={`flex h-14 select-none items-center justify-center rounded-xl text-2xl font-medium transition-colors active:bg-paper-sunken ${
            key === "back"
              ? "bg-transparent text-ink-soft"
              : "tabular bg-paper-raised text-ink"
          }`}
        >
          {key === "back" ? <BackspaceIcon /> : key}
        </button>
      ))}
    </div>
  );
}

function BackspaceIcon() {
  return (
    <svg width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path
        d="M9 5h11a1 1 0 0 1 1 1v12a1 1 0 0 1-1 1H9l-6-7 6-7Z"
        stroke="currentColor"
        strokeWidth="1.6"
        strokeLinejoin="round"
      />
      <path
        d="m12 10 5 4m0-4-5 4"
        stroke="currentColor"
        strokeWidth="1.6"
        strokeLinecap="round"
      />
    </svg>
  );
}
