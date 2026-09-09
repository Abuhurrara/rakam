/* eslint-disable @typescript-eslint/no-require-imports -- Node test runner uses CommonJS to load existing TS without extra dependencies. */
// Transpile with the project's existing TypeScript dependency; no test framework.
const fs = require('node:fs');
const path = require('node:path');
const Module = require('node:module');
const ts = require('typescript');
const loaded = new Map();
function load(relative) {
  let filename = path.resolve(__dirname, '..', relative);
  if (!path.extname(filename)) filename += fs.existsSync(filename + '.ts') ? '.ts' : '.tsx';
  if (loaded.has(filename)) return loaded.get(filename).exports;
  const mod = new Module(filename, module);
  mod.filename = filename;
  mod.paths = Module._nodeModulePaths(path.dirname(filename));
  const original = mod.require.bind(mod);
  mod.require = name => {
    if (name.startsWith('@/')) return load(name.slice(2));
    if (name.startsWith('.')) return load(path.relative(path.resolve(__dirname, '..'), path.resolve(path.dirname(filename), name)));
    return original(name);
  };
  loaded.set(filename, mod);
  const output = ts.transpileModule(fs.readFileSync(filename, 'utf8'), { compilerOptions: {
    module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022, jsx: ts.JsxEmit.ReactJSX,
  }, fileName: filename }).outputText;
  mod._compile(output, filename);
  return mod.exports;
}
module.exports = load;
