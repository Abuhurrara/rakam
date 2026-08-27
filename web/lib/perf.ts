/**
 * Instrumentation for the one number this phase is judged on: how long it
 * takes to add an expense.
 *
 * Marks are taken at the four moments that matter:
 *   fabTap    — pointerdown on the "+" button
 *   sheetReady — the sheet has painted with its keypad and chips usable
 *   saveTap   — pointerdown on Save
 *   saveDone  — the API answered
 *
 * Enabled in development, and in a production build by setting
 * localStorage "rakam.perf" to "1" — measuring a dev build would understate
 * the real thing, since dev mode is slower on purpose.
 */

const marks = new Map<string, number>();
let taps = 0;

function enabled(): boolean {
  if (typeof window === "undefined" || typeof performance === "undefined") {
    return false;
  }
  if (process.env.NODE_ENV !== "production") return true;
  try {
    return window.localStorage.getItem("rakam.perf") === "1";
  } catch {
    return false;
  }
}

export function mark(name: string): void {
  if (!enabled()) return;
  marks.set(name, performance.now());
}

export function countTap(): void {
  if (!enabled()) return;
  taps += 1;
}

export function resetAddFlow(): void {
  if (!enabled()) return;
  marks.clear();
  taps = 0;
}

/** Logs one line per completed add, for read_console_messages to pick up. */
export function reportAddFlow(outcome: "saved" | "failed"): void {
  if (!enabled()) return;

  const fab = marks.get("fabTap");
  const ready = marks.get("sheetReady");
  const saveTap = marks.get("saveTap");
  const saveDone = marks.get("saveDone");

  const ms = (a?: number, b?: number) =>
    a === undefined || b === undefined ? "?" : (b - a).toFixed(0);

  console.log(
    `[addflow] outcome=${outcome} open=${ms(fab, ready)}ms ` +
      `handsOff=${ms(fab, saveTap)}ms request=${ms(saveTap, saveDone)}ms ` +
      `machineTotal=${ms(fab, saveDone)}ms taps=${taps}`,
  );
}
