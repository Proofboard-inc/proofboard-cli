const assert = require('assert');
const { getBinaryName, getReleaseAssetName } = require('./index.js');

// The package bundles executables under the lowercase names its launcher
// looks for internally, while release assets carry the product name. Both are
// needed and they are not interchangeable: fetching the bundled name from a
// release 404s, and caching under the asset name makes the launcher miss its
// own bundled copy.
assert.strictEqual(getBinaryName('Darwin', 'arm64'), 'proofboard-darwin-arm64');
assert.strictEqual(getReleaseAssetName('Darwin', 'arm64'), 'Proofboard-Career-Agent-darwin-arm64');
assert.strictEqual(getReleaseAssetName('Linux', 'x64'), 'Proofboard-Career-Agent-linux-amd64');
assert.strictEqual(getReleaseAssetName('Windows_NT', 'x64'), 'Proofboard-Career-Agent-windows-amd64.exe');
console.log('release asset naming: ok');
