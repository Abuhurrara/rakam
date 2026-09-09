/* eslint-disable @typescript-eslint/no-require-imports -- Existing Node test runner. */
const { test } = require('node:test');
const assert = require('node:assert/strict');
const React = require('react');
const { renderToStaticMarkup } = require('react-dom/server');
// Supply the same routing contexts as Next without starting a browser.
const { AppRouterContext } = require('next/dist/shared/lib/app-router-context.shared-runtime');
const { PathnameContext } = require('next/dist/shared/lib/hooks-client-context.shared-runtime');
const load = require('./load-ts.cjs');
const Layout = load('app/(app)/layout.tsx').default;

for (const route of ['/', '/expenses', '/ledger', '/budget', '/more']) {
  test(`initial ${route} shell renders without a backend or private content`, () => {
    let privateScreenMounted = false;
    function PrivateScreen() { privateScreenMounted = true; return 'PRIVATE EXPENSE'; }
    const originalFetch = global.fetch;
    let requests = 0;
    global.fetch = () => { requests++; throw Error('Backend unavailable'); };
    try {
      const html = renderToStaticMarkup(React.createElement(AppRouterContext.Provider, { value: {} },
        React.createElement(PathnameContext.Provider, { value: route },
          React.createElement(Layout, null, React.createElement(PrivateScreen)))));
      assert.equal(requests, 0);
      assert.equal(privateScreenMounted, false);
      assert.ok(!html.includes('PRIVATE EXPENSE'));
      assert.ok(html.includes('Checking your session'));
      assert.ok(html.includes('aria-label="Main"'));
      assert.match(html, /aria-label="Add expense" disabled=""/);
      for (const destination of ['/expenses', '/ledger', '/budget', '/more']) {
        assert.ok(html.includes(`href="${destination}"`));
      }
    } finally { global.fetch = originalFetch; }
  });
}
