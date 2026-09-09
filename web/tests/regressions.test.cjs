/* eslint-disable @typescript-eslint/no-require-imports -- Node test runner uses CommonJS to load existing TS without extra dependencies. */
const { test } = require('node:test');
const assert = require('node:assert/strict');
const React = require('react');
const { renderToStaticMarkup } = require('react-dom/server');
const load = require('./load-ts.cjs');
const { useTap } = load('lib/useTap.ts');
const { applyKey } = load('components/Keypad.tsx');
const { expenseTimestamp } = load('lib/date.ts');
const { storeDraft, readDrafts, removeDraft } = load('lib/save-drafts.ts');

for (const scenario of ['quick taps', 'long holds', 'overlapping old timers', 'keyboard']) {
  test(`Android keypad: 1200 stays 1200 (${scenario})`, () => {
    let tap;
    function Harness() { tap = useTap(); return null; }
    renderToStaticMarkup(React.createElement(Harness));
    let amount = '';
    const timers = [];
    const realTimeout = global.setTimeout;
    global.setTimeout = fn => { timers.push(fn); return timers.length; };
    try {
      for (const digit of '1200') {
        const props = tap(() => { amount = applyKey(amount, digit); }, { fast: true });
        if (scenario !== 'keyboard') props.onPointerDown?.({ button: 0, preventDefault() {} });
        if (scenario === 'long holds' || scenario === 'overlapping old timers') timers.splice(0).forEach(fn => fn());
        props.onPointerUp?.({ button: 0 });
        props.onClick();
      }
      assert.equal(amount, '1200');
    } finally { global.setTimeout = realTimeout; }
  });
}

for (const tc of [
  { name: 'note-only edit preserves exact timestamp', date: '2026-09-01', original: '2026-08-31T21:00:00Z', want: '2026-08-31T21:00:00Z' },
  { name: 'changed date preserves original Karachi clock', date: '2026-08-20', original: '2026-08-31T21:00:00Z', want: '2026-08-20T02:00:00+05:00' },
  { name: 'new entry uses current Karachi clock', date: '2026-09-09', original: undefined, want: '2026-09-09T15:30:00+05:00' },
]) test(tc.name, () => assert.equal(expenseTimestamp(tc.date, tc.original, new Date('2026-09-09T10:30:00Z')), tc.want));

function storage() {
  const map = new Map();
  return { get length() { return map.size; }, key: i => [...map.keys()][i] ?? null, getItem: k => map.get(k) ?? null, setItem: (k,v) => map.set(k,v), removeItem: k => map.delete(k) };
}
const draft = {version:1,key:'stable-request-1234',input:{kind:'expense',amount:'1200',category_id:null,description:'Lunch',occurred_at:'2026-09-09T15:30:00+05:00'}};
test('unfinished save survives reopening with exactly the same retry key and amount', () => {
  const disk = storage(); storeDraft(disk,'alice',draft);
  assert.deepEqual(readDrafts(disk,'alice'),[draft]);
  assert.deepEqual(readDrafts(disk,'bob'),[]);
  storeDraft(disk,'alice',{...draft,key:'second'});
  removeDraft(disk,'alice',draft.key);
  assert.equal(readDrafts(disk,'alice')[0].key,'second');
});
test('storage failures are surfaced before the sheet can discard its input', () => {
  const disk = storage(); disk.setItem = () => { throw Error('quota'); };
  assert.throws(() => storeDraft(disk,'alice',draft), /quota/);
});
test('corrupt recovery records are preserved and reported', () => {
  const disk=storage(); disk.setItem('rakam.saves.v1.alice.broken','{');
  assert.throws(() => readDrafts(disk,'alice'));
  assert.equal(disk.length,1);
});
