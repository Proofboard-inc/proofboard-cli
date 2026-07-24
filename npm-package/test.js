const assert = require('assert');
const { getBinaryName } = require('./index');

assert.strictEqual(getBinaryName('Linux', 'x64'), 'proofboard-linux-amd64');
assert.strictEqual(getBinaryName('Darwin', 'x64'), 'proofboard-darwin-amd64');
assert.strictEqual(getBinaryName('Darwin', 'arm64'), 'proofboard-darwin-arm64');
assert.strictEqual(getBinaryName('Windows_NT', 'x64'), 'proofboard-windows-amd64.exe');
assert.throws(() => getBinaryName('Linux', 'arm64'), /Unsupported platform/);
