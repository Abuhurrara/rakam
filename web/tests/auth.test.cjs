/* eslint-disable @typescript-eslint/no-require-imports -- Existing Node test runner. */
const { test } = require('node:test');
const assert = require('node:assert/strict');
const load = require('./load-ts.cjs');
const { changePassword } = load('lib/api.ts');

test('password change uses the authenticated API and sends both passwords', async () => {
  const originalFetch = global.fetch;
  let request;
  global.fetch = async (path, init) => {
    request = { path, init };
    return new Response(null, { status: 204 });
  };
  try {
    await changePassword('current-password', 'a-new-strong-password');
    assert.equal(request.path, '/api/auth/password');
    assert.equal(request.init.method, 'PUT');
    assert.equal(request.init.credentials, 'include');
    assert.deepEqual(JSON.parse(request.init.body), {
      current_password: 'current-password',
      new_password: 'a-new-strong-password',
    });
  } finally {
    global.fetch = originalFetch;
  }
});
