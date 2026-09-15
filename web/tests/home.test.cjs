/* eslint-disable @typescript-eslint/no-require-imports -- Existing Node test runner. */
const { test } = require("node:test");
const assert = require("node:assert/strict");
const load = require("./load-ts.cjs");
const { formatDueDate } = load("lib/date.ts");
const { budgetState, sortUpcomingBills } = load("lib/home.ts");

test("budget progress uses the warning thresholds and caps its visual width", () => {
  assert.deepEqual(budgetState(0, 0), {
    percent: 0,
    visualPercent: 0,
    tone: "primary",
  });
  assert.deepEqual(budgetState(79_000, 100_000), {
    percent: 79,
    visualPercent: 79,
    tone: "primary",
  });
  assert.deepEqual(budgetState(80_000, 100_000), {
    percent: 80,
    visualPercent: 80,
    tone: "gold",
  });
  assert.deepEqual(budgetState(125_000, 100_000), {
    percent: 125,
    visualPercent: 100,
    tone: "brick",
  });
});

test("upcoming bills are ordered by actual due date without mutating input", () => {
  const make = (id, name, due_at) => ({
    due_at,
    bill: { id, name },
  });
  const input = [
    make("rent", "Rent", "2026-10-01"),
    make("phone", "Phone", "2026-09-29"),
  ];
  const sorted = sortUpcomingBills(input);
  assert.deepEqual(
    sorted.map((item) => item.bill.id),
    ["phone", "rent"],
  );
  assert.equal(input[0].bill.id, "rent");
});

test("bill dates use Karachi day boundaries", () => {
  const now = new Date("2026-09-15T20:30:00Z"); // Sep 16, 01:30 in Karachi
  assert.equal(formatDueDate("2026-09-16", now), "Today");
  assert.equal(formatDueDate("2026-09-17", now), "Tomorrow");
});
