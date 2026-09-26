/* eslint-disable @typescript-eslint/no-require-imports -- Existing Node test runner. */
const { test } = require('node:test');
const assert = require('node:assert/strict');
const load = require('./load-ts.cjs');
const {
  listCategories,
  createCategory,
  updateCategory,
  archiveCategory,
  restoreCategory,
} = load('lib/api.ts');

const category = {
  id: 'food', name: 'Food', kind: 'expense', icon: '🍔', color: '#8B4513',
  sort_order: 0, is_archived: false, created_at: '', updated_at: '',
};
const input = {
  name: 'Coffee', kind: 'expense', icon: '☕', color: '#8B4513', sort_order: 12,
};

test('category API keeps archived rows available for management and history labels', async () => {
  const originalFetch = global.fetch;
  const archived = { ...category, id: 'old', is_archived: true };
  global.fetch = async () => new Response(JSON.stringify([category, archived]), { status: 200 });
  try {
    assert.deepEqual(await listCategories(), [category, archived]);
  } finally {
    global.fetch = originalFetch;
  }
});

test('category mutations use typed payloads and the archive/restore routes', async () => {
  const originalFetch = global.fetch;
  const requests = [];
  global.fetch = async (path, init = {}) => {
    requests.push({ path, init });
    if (init.method === 'DELETE') return new Response(null, { status: 204 });
    return new Response(JSON.stringify(category), { status: 200 });
  };
  try {
    await createCategory(input);
    await updateCategory('food/id', input);
    await archiveCategory('food/id');
    await restoreCategory('food/id');
    assert.deepEqual(requests.map(({ path, init }) => [path, init.method ?? 'GET']), [
      ['/api/categories', 'POST'],
      ['/api/categories/food%2Fid', 'PATCH'],
      ['/api/categories/food%2Fid', 'DELETE'],
      ['/api/categories/food%2Fid/restore', 'POST'],
    ]);
    assert.deepEqual(JSON.parse(requests[0].init.body), input);
    assert.deepEqual(JSON.parse(requests[1].init.body), input);
    for (const request of requests) assert.equal(request.init.credentials, 'include');
  } finally {
    global.fetch = originalFetch;
  }
});
